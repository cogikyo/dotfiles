import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import type { DelegateConfig } from "./config.ts";

type UsageWindow = {
  label: string;
  usedPercent: number;
  resetAt?: string;
};

type CacheRead = {
  path: string;
  windows?: UsageWindow[];
  note?: string;
};

export async function enforceProviderPolicy(providerID: string, config: DelegateConfig, signal: AbortSignal) {
  if (!Object.hasOwn(config.providers, providerID)) {
    throw new Error(`delegate provider policy missing for ${providerID}; add it to delegate.json.providers`);
  }

  const cache = await readUsageCache(providerID);
  if (cache.note) return [cache.note];
  if (!cache.windows?.length) return [`delegate provider policy: ${providerID} usage cache has no windows; proceeding un-gated`];

  const capped = cache.windows.filter((window) => window.usedPercent >= 100);
  if (!capped.length) return [];

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
  if (waitMs <= 0) return [`delegate provider policy: ${providerID} capped window reset time has passed; proceeding with stale usage data`];

  await sleepAbortably(waitMs, signal);
  return [`delegate provider policy: waited ${formatMinutes(waitMs)} for ${providerID} usage reset`];
}

async function readUsageCache(providerID: string): Promise<CacheRead> {
  const filePath = usageCachePath(providerID);
  let raw: string;

  try {
    raw = await fs.readFile(filePath, "utf8");
  } catch (error) {
    if (isMissing(error)) {
      return { path: filePath, note: `delegate provider policy: ${providerID} usage cache missing; proceeding un-gated` };
    }
    return {
      path: filePath,
      note: `delegate provider policy: ${providerID} usage cache unreadable: ${errorMessage(error)}; proceeding un-gated`,
    };
  }

  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    const nested = object(parsed.usage) ?? parsed;
    const windows = Array.isArray(nested.windows) ? nested.windows.flatMap(parseWindow) : undefined;
    return { path: filePath, windows };
  } catch (error) {
    return {
      path: filePath,
      note: `delegate provider policy: ${providerID} usage cache invalid: ${errorMessage(error)}; proceeding un-gated`,
    };
  }
}

function parseWindow(value: unknown): UsageWindow[] {
  const root = object(value);
  if (!root) return [];
  if (typeof root.label !== "string") return [];
  if (typeof root.usedPercent !== "number" || !Number.isFinite(root.usedPercent)) return [];
  return [
    {
      label: root.label,
      usedPercent: root.usedPercent,
      resetAt: typeof root.resetAt === "string" ? root.resetAt : undefined,
    },
  ];
}

function usageCachePath(providerID: string) {
  const xdg = process.env.XDG_CACHE_HOME?.trim();
  const cacheRoot = xdg ? path.resolve(xdg) : path.join(os.homedir(), ".cache");
  return path.join(cacheRoot, "opencode", "usage-sidebar", `${providerID}.json`);
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

function object(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;
}

function isMissing(error: unknown) {
  return typeof error === "object" && error !== null && "code" in error && (error as { code?: unknown }).code === "ENOENT";
}

function formatMinutes(ms: number) {
  if (!Number.isFinite(ms)) return "unknown age";
  return `${Math.max(0, Math.round(ms / 60_000))}m`;
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}
