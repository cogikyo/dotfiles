import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import type { DelegateConfig } from "./config.ts";

export type CapacityDecision =
  | { action: "proceed"; notes: string[] }
  | { action: "report"; report: CapacityReport };

export type CapacityReport = {
  capped: true;
  providerID: string;
  window: string;
  usedPercent: number;
  threshold: number;
  resetAt?: string;
  maxWaitMinutes: number;
  windows: Array<{
    window: string;
    usedPercent: number;
    threshold: number;
    resetAt?: string;
  }>;
};

type UsageWindow = {
  label: string;
  usedPercent: number;
  resetAt?: string;
};

type CappedWindow = CapacityReport["windows"][number];

type ResetWindow = {
  window: CappedWindow;
  resetAtMs?: number;
};

type CacheRead = {
  path: string;
  fetchedAt?: number;
  windows?: UsageWindow[];
  note?: string;
};

export async function decideCapacity(
  providerID: string,
  config: DelegateConfig,
  signal: AbortSignal,
): Promise<CapacityDecision> {
  if (!Object.hasOwn(config.providers, providerID)) {
    throw new Error(`delegate capacity has no provider policy for ${providerID}; add it to delegate.json.providers`);
  }

  const cache = await readUsageCache(providerID);
  if (cache.note) return proceed(cache.note);
  if (!cache.windows?.length) return proceed(`delegate capacity: ${providerID} usage cache has no windows; proceeding un-gated`);

  const age = cache.fetchedAt ? Date.now() - cache.fetchedAt : Number.POSITIVE_INFINITY;
  const staleMs = config.staleCacheMinutes * 60_000;
  if (age > staleMs) {
    return proceed(`delegate capacity: ${providerID} usage cache is stale by ${formatMinutes(age)}; proceeding un-gated`);
  }

  const capped = cappedWindows(cache.windows, config);
  if (!capped.length) return proceed();

  const now = Date.now();
  const maxWaitMs = config.maxWaitMinutes * 60_000;
  const resets = capped.map((window): ResetWindow => ({ window, resetAtMs: resetAtMs(window.resetAt) }));
  const blocked = resets.find((item) => item.resetAtMs === undefined || item.resetAtMs - now > maxWaitMs);
  if (blocked) return reportCapacity(providerID, config, capped, blocked.window);

  const latest = resets.reduce<ResetWindow | undefined>((current, item) => {
    if (item.resetAtMs === undefined || item.resetAtMs <= now) return current;
    if (!current?.resetAtMs || item.resetAtMs > current.resetAtMs) return item;
    return current;
  }, undefined);

  if (!latest?.resetAtMs) {
    return proceed(`delegate capacity: ${providerID} capped window reset times have passed; proceeding with stale usage data`);
  }

  const waitMs = latest.resetAtMs - now;
  await sleepAbortably(waitMs, signal);
  return proceed(`delegate capacity: waited ${formatMinutes(waitMs)} for ${providerID} capped windows to reset`);
}

function reportCapacity(
  providerID: string,
  config: DelegateConfig,
  capped: CappedWindow[],
  primary: CappedWindow,
): CapacityDecision {
  return {
    action: "report",
    report: {
      capped: true,
      providerID,
      window: primary.window,
      usedPercent: primary.usedPercent,
      threshold: primary.threshold,
      resetAt: primary.resetAt,
      maxWaitMinutes: config.maxWaitMinutes,
      windows: capped,
    },
  };
}

function cappedWindows(windows: UsageWindow[], config: DelegateConfig) {
  return windows.flatMap((window) => {
    const threshold = config.thresholds[window.label];
    if (threshold === undefined) return [];
    if (window.usedPercent <= threshold) return [];
    return [
      {
        window: window.label,
        usedPercent: window.usedPercent,
        threshold,
        resetAt: window.resetAt,
      },
    ];
  });
}

async function readUsageCache(providerID: string): Promise<CacheRead> {
  const filePath = usageCachePath(providerID);
  let raw: string;
  let mtimeMs: number | undefined;

  try {
    const stat = await fs.stat(filePath);
    mtimeMs = stat.mtimeMs;
    raw = await fs.readFile(filePath, "utf8");
  } catch (error) {
    if (isMissing(error)) {
      return { path: filePath, note: `delegate capacity: ${providerID} usage cache missing; proceeding un-gated` };
    }
    return {
      path: filePath,
      note: `delegate capacity: ${providerID} usage cache unreadable: ${errorMessage(error)}; proceeding un-gated`,
    };
  }

  try {
    const parsed = JSON.parse(raw) as Record<string, unknown>;
    const nested = object(parsed.usage) ?? parsed;
    const windows = Array.isArray(nested.windows) ? nested.windows.flatMap(parseWindow) : undefined;
    const fetchedAt = timestampMs(parsed.fetchedAt) ?? timestampMs(nested.fetchedAt) ?? mtimeMs;
    return { path: filePath, fetchedAt, windows };
  } catch (error) {
    return {
      path: filePath,
      note: `delegate capacity: ${providerID} usage cache invalid: ${errorMessage(error)}; proceeding un-gated`,
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

function timestampMs(value: unknown) {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) return undefined;
  return value < 10_000_000_000 ? value * 1000 : value;
}

function resetAtMs(value: string | undefined) {
  if (!value) return undefined;
  const ms = Date.parse(value);
  return Number.isFinite(ms) ? ms : undefined;
}

function sleepAbortably(ms: number, signal: AbortSignal) {
  if (signal.aborted) throw new Error("delegate capacity wait aborted");
  return new Promise<void>((resolve, reject) => {
    const done = () => {
      signal.removeEventListener("abort", abort);
      resolve();
    };
    const timeout = setTimeout(done, Math.max(0, ms));
    const abort = () => {
      clearTimeout(timeout);
      signal.removeEventListener("abort", abort);
      reject(new Error("delegate capacity wait aborted"));
    };
    signal.addEventListener("abort", abort, { once: true });
    void Promise.resolve().then(() => {
      if (!signal.aborted) return;
      abort();
    });
  });
}

function proceed(note?: string): CapacityDecision {
  return { action: "proceed", notes: note ? [note] : [] };
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
