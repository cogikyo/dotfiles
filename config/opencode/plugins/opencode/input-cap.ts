import type { Config, Plugin, PluginModule } from "@opencode-ai/plugin";
import path from "node:path";
import { z } from "zod";
import { readJson } from "../shared/file.ts";
import {
  COMPACTION_LIMIT,
  COMPACTION_RESERVED,
  compactionInputCap,
  contextCompactionLimit,
} from "../shared/session.ts";

const id = "opencode-input-cap";

type Model = NonNullable<NonNullable<Config["provider"]>[string]["models"]>[string];
type Limit = z.infer<typeof Limit>;
type Catalog = Record<string, Record<string, Limit>>;

const number = z.number().optional().catch(undefined);
const Limit = z.object({ context: number, input: number, output: number });
const Configured = Limit.catch({});
const Reserved = z.object({ compaction: z.object({ reserved: number }).optional().catch(undefined) }).catch({});

const Catalog = z
  .record(
    z.string(),
    z
      .object({ models: z.record(z.string(), z.object({ limit: Limit }).optional().catch(undefined)) })
      .optional()
      .catch(undefined),
  )
  .transform((providers) => {
    const catalog: Catalog = {};
    for (const [providerID, provider] of Object.entries(providers)) {
      if (!provider) continue;
      const limits: Record<string, Limit> = {};
      for (const [modelID, model] of Object.entries(provider.models)) {
        if (model) limits[modelID] = model.limit;
      }
      catalog[providerID] = limits;
    }
    return catalog;
  });

const server: Plugin = async () => ({
  config: async (cfg) => {
    const catalog = await readCatalog();
    const reserved = Reserved.parse(cfg).compaction?.reserved ?? COMPACTION_RESERVED;
    const inputCap = compactionInputCap(reserved);
    capProviderModels(cfg, catalog, inputCap);
    capCatalogModels(cfg, catalog, inputCap);
  },
});

/** Server plugin that caps enabled models' input limits to leave room for compaction, using cached model limits when needed. */
export default { id, server } satisfies PluginModule;

function capProviderModels(cfg: Config, catalog: Catalog, inputCap: number) {
  for (const [providerID, provider] of Object.entries(cfg.provider ?? {})) {
    if (cfg.enabled_providers && !cfg.enabled_providers.includes(providerID)) continue;
    if (cfg.disabled_providers?.includes(providerID)) continue;
    const catalogModels = catalog[providerID] ?? {};
    for (const [modelID, model] of Object.entries(provider.models ?? {})) {
      applyCap(model, catalogModels[model.id ?? modelID], inputCap);
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

function applyCap(model: Model, catalogLimit: Limit | undefined, inputCap: number) {
  const configured = Configured.parse(model.limit);
  const { context, output, input } = effectiveLimit(configured, catalogLimit);
  if (context === undefined || output === undefined) return;
  if ((input || context) <= inputCap) return;
  const limit = { context, output, input };
  const threshold = contextCompactionLimit({ limit }, inputCap - COMPACTION_LIMIT);
  if (threshold === undefined || threshold <= COMPACTION_LIMIT) return;
  const capped = { ...model.limit, context, output, input: inputCap };
  model.limit = capped;
}

function effectiveLimit(configured: Limit, catalogLimit: Limit | undefined): Limit {
  return {
    context: configured.context ?? catalogLimit?.context,
    output: configured.output ?? catalogLimit?.output,
    input: configured.input ?? catalogLimit?.input,
  };
}

async function readCatalog(): Promise<Catalog> {
  const cacheHome = process.env.XDG_CACHE_HOME || path.join(process.env.HOME ?? "", ".cache");
  try {
    return (await readJson(path.join(cacheHome, "opencode", "models.json"), Catalog)) ?? {};
  } catch {
    return {};
  }
}
