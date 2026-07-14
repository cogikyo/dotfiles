import { inspectProviderCache } from "../usage/cache.ts";
import { usageProvider } from "../usage/providers.ts";
import type { DelegateConfig } from "./config.ts";

export async function enforceProviderPolicy(providerID: string, config: DelegateConfig, signal: AbortSignal) {
  if (!Object.hasOwn(config.providers, providerID)) {
    throw new Error(`delegate provider policy missing for ${providerID}; add it to delegate.json.providers`);
  }

  const provider = usageProvider(providerID);
  if (!provider) {
    return [`delegate provider policy: ${providerID} has no usage provider spec; proceeding un-gated`];
  }

  const cache = await inspectProviderCache(providerID, provider.staleAfterMS);
  if (cache.issue) {
    return [`delegate provider policy: ${providerID} usage cache is ${cache.issue}; proceeding un-gated`];
  }
  if (!cache.windows.length) return [`delegate provider policy: ${providerID} usage cache has no windows; proceeding un-gated`];

  const notes: string[] = [];
  if (cache.windows.some((window) => window.postReset)) {
    notes.push(`delegate provider policy: ${providerID} has post-reset usage treated as unknown`);
  }
  if (cache.windows.some((window) => window.usedPercent === undefined)) {
    notes.push(`delegate provider policy: ${providerID} has unknown usage percentages`);
  }
  const capped = cache.windows.filter(
    (window) => !window.postReset && window.usedPercent !== undefined && window.usedPercent >= 100,
  );
  if (!capped.length) return notes;

  const waits = capped.flatMap((window) => {
    const ms = resetAtMs(window.resetAt);
    return ms === undefined ? [] : [{ window, ms }];
  });
  if (waits.length !== capped.length) {
    const missing = capped.find((window) => resetAtMs(window.resetAt) === undefined);
    throw new Error(`delegate provider ${providerID} is capped on ${missing?.label ?? "unknown window"} with no reset time`);
  }

  const latest = waits.reduce((current, item) => (item.ms > current.ms ? item : current));
  const waitMs = latest.ms - Date.now();
  if (waitMs <= 0) {
    return [...notes, `delegate provider policy: ${providerID} capped window reset time has passed; proceeding with stale usage data`];
  }

  await sleepAbortably(waitMs, signal);
  return [...notes, `delegate provider policy: waited ${formatMinutes(waitMs)} for ${providerID} usage reset`];
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
    void Promise.resolve().then(() => {
      if (!signal.aborted) return;
      abort();
    });
  });
}

function formatMinutes(ms: number) {
  if (!Number.isFinite(ms)) return "unknown age";
  return `${Math.max(0, Math.round(ms / 60_000))}m`;
}
