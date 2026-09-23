import type { Config, Plugin, PluginModule } from "@opencode-ai/plugin";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { record } from "../shared/record.ts";
import {
  COMPACTION_LIMIT,
  COMPACTION_RESERVED,
  compactionInputCap,
  contextCompactionLimit,
} from "../shared/session.ts";

const id = "opencode-input-cap";

type Limit = {
  context?: number;
  input?: number;
  output?: number;
};

type Catalog = Record<string, Record<string, Limit>>;

const server: Plugin = async () => ({
  config: async (cfg) => {
    const catalog = await readCatalog();
    const reserved = number(object(object(cfg)?.compaction)?.reserved) ?? COMPACTION_RESERVED;
    const inputCap = compactionInputCap(reserved);
    capProviderModels(cfg, catalog, inputCap);
    capCatalogModels(cfg, catalog, inputCap);
  },
});

function capProviderModels(cfg: Config, catalog: Catalog, inputCap: number) {
  const providers = object(cfg.provider);
  if (!providers) return;

  for (const [providerID, providerValue] of Object.entries(providers)) {
    if (cfg.enabled_providers && !cfg.enabled_providers.includes(providerID)) continue;
    if (cfg.disabled_providers?.includes(providerID)) continue;
    const models = object(object(providerValue)?.models);
    if (!models) continue;
    const catalogModels = catalog[providerID] ?? {};
    for (const [modelID, modelValue] of Object.entries(models)) {
      const model = object(modelValue);
      if (!model) continue;
      applyCap(model, catalogModels[typeof model.id === "string" ? model.id : modelID], inputCap);
    }
  }
}

function capCatalogModels(cfg: Config, catalog: Catalog, inputCap: number) {
  const enabled = cfg.enabled_providers;
  const providerIDs = enabled ?? Object.keys(catalog);
  const providers = (cfg.provider ??= {});

  for (const providerID of providerIDs) {
    if (cfg.disabled_providers?.includes(providerID)) continue;
    const catalogModels = catalog[providerID];
    if (!catalogModels) continue;
    const provider = (providers[providerID] ??= {});
    const models = (provider.models ??= {});
    for (const [modelID, catalogLimit] of Object.entries(catalogModels)) {
      if (models[modelID]) continue;
      const model = (models[modelID] ??= {});
      applyCap(model, catalogLimit, inputCap);
    }
  }
}

function applyCap(
  model: { limit?: { context: number; input?: number; output: number } },
  catalogLimit: Limit | undefined,
  inputCap: number,
) {
  const configured = object(model.limit);
  const { context, output, input } = effectiveLimit(configured, catalogLimit);
  if (context === undefined || output === undefined) return;
  // The cap is the compaction threshold plus the reserved input budget.
  if ((input || context) <= inputCap) return;
  const limit = { context, output, input };
  const threshold = contextCompactionLimit({ limit }, inputCap - COMPACTION_LIMIT);
  if (threshold === undefined || threshold <= COMPACTION_LIMIT) return;
  model.limit = { ...configured, context, output, input: inputCap };
}

function effectiveLimit(configured: Record<string, unknown> | undefined, catalogLimit: Limit | undefined): Limit {
  return {
    context: number(configured?.context) ?? number(catalogLimit?.context),
    output: number(configured?.output) ?? number(catalogLimit?.output),
    input: number(configured?.input) ?? number(catalogLimit?.input),
  };
}

async function readCatalog(): Promise<Catalog> {
  const cacheHome = process.env.XDG_CACHE_HOME || path.join(process.env.HOME ?? "", ".cache");
  const file = path.join(cacheHome, "opencode", "models.json");
  try {
    const parsed = object(JSON.parse(await readFile(file, "utf8")));
    if (!parsed) return {};
    const catalog: Catalog = {};
    for (const [providerID, providerValue] of Object.entries(parsed)) {
      const models = object(object(providerValue)?.models);
      if (!models) continue;
      const limits: Record<string, Limit> = {};
      for (const [modelID, modelValue] of Object.entries(models)) {
        const limit = object(object(modelValue)?.limit);
        if (!limit) continue;
        limits[modelID] = {
          context: number(limit.context),
          input: number(limit.input),
          output: number(limit.output),
        };
      }
      catalog[providerID] = limits;
    }
    return catalog;
  } catch {
    return {};
  }
}

function object(value: unknown): Record<string, unknown> | undefined {
  return record(value);
}

function number(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

/** Runs the server config hook to cap model input limits and preserve the compaction reserve. */
export default { id, server } satisfies PluginModule;
