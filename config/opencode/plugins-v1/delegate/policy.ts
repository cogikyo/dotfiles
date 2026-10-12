import { claudeAccounts } from "../anthropic/accounts.ts";
import { limitsModel } from "../usage/anthropic.ts";
import { type CachedUsageWindow, inspectProviderCache } from "../usage/cache.ts";
import { GROK_BOT } from "../usage/cursor.ts";
import { usageProvider } from "../usage/providers.ts";
import type { ModelRef } from "./args.ts";
import type { DelegateConfig } from "./config.ts";

export type Usage = {
  windows: CachedUsageWindow[];
  capped: CachedUsageWindow[];
  notes: string[];
};

/** Waits for capped provider usage to reset and returns notes when usage is unknown or stale. */
export async function enforceProviderPolicy(model: ModelRef, config: DelegateConfig, signal: AbortSignal) {
  const { providerID } = model;
  if (!Object.hasOwn(config.providers, providerID)) {
    throw new Error(`delegate provider policy missing for ${providerID}; add it to delegate.json.providers`);
  }

  const { capped, notes } = await readUsage(model);
  if (!capped.length) return notes;

  const waits = capped.flatMap((window) => {
    const ms = resetAtMs(window.resetAt);
    return ms === undefined ? [] : [{ window, ms }];
  });
  if (waits.length !== capped.length) {
    const missing = capped.find((window) => resetAtMs(window.resetAt) === undefined);
    throw new Error(
      `delegate provider ${providerID} is capped on ${missing?.label ?? "unknown window"} with no reset time`,
    );
  }

  const latest = waits.reduce((current, item) => (item.ms > current.ms ? item : current));
  const waitMs = latest.ms - Date.now();
  if (waitMs <= 0) {
    return [
      ...notes,
      `delegate provider policy: ${providerID} capped window reset time has passed; proceeding with stale usage data`,
    ];
  }

  await sleepAbortably(waitMs, signal);
  return [...notes, `delegate provider policy: waited ${formatMinutes(waitMs)} for ${providerID} usage reset`];
}

export async function readUsage({ providerID, modelID }: ModelRef): Promise<Usage> {
  const provider = usageProvider(providerID);
  if (!provider) return ungated(`${providerID} has no usage provider spec`);

  const cache = await inspectProviderCache(providerID, provider.staleAfterMS);
  if (cache.issue) return ungated(`${providerID} usage cache is ${cache.issue}`);
  const claude = Object.values(claudeAccounts).some(({ id }) => id === providerID);
  const windows = claude
    ? cache.windows.filter((window) => limitsModel(window.label, modelID))
    : cache.windows.filter((window) => window.label !== GROK_BOT);
  if (!windows.length) return ungated(`${providerID} usage cache has no windows`);

  const notes: string[] = [];
  if (windows.some((window) => window.postReset)) {
    notes.push(`delegate provider policy: ${providerID} has post-reset usage treated as unknown`);
  }
  if (windows.some((window) => window.usedPercent === undefined)) {
    notes.push(`delegate provider policy: ${providerID} has unknown usage percentages`);
  }
  const capped = windows.filter(
    (window) => !window.postReset && window.usedPercent !== undefined && window.usedPercent >= 100,
  );
  return { windows, capped, notes };
}

function ungated(reason: string): Usage {
  return { windows: [], capped: [], notes: [`delegate provider policy: ${reason}; proceeding un-gated`] };
}

function resetAtMs(value: string | undefined) {
  if (!value) return undefined;
  const ms = Date.parse(value);
  return Number.isFinite(ms) ? ms : undefined;
}

function sleepAbortably(ms: number, signal: AbortSignal) {
  if (signal.aborted) throw new Error("delegate provider policy wait aborted");
  return new Promise<void>((resolve, reject) => {
    const done = () => {
      signal.removeEventListener("abort", abort);
      resolve();
    };
    const timeout = setTimeout(done, Math.max(0, ms));
    const abort = () => {
      clearTimeout(timeout);
      signal.removeEventListener("abort", abort);
      reject(new Error("delegate provider policy wait aborted"));
    };
    signal.addEventListener("abort", abort, { once: true });
    queueMicrotask(() => {
      if (signal.aborted) abort();
    });
  });
}

function formatMinutes(ms: number) {
  if (!Number.isFinite(ms)) return "unknown age";
  return `${Math.max(0, Math.round(ms / 60_000))}m`;
}
