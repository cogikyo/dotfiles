import fs from "node:fs/promises";
import path from "node:path";
import { usageCachePath, usageLockPath } from "./auth.ts";
import type { ProviderUsage, UsageWindow } from "./types.ts";

export type CachedProviderUsage = {
  fetchedAt?: number;
  backoffUntil?: number;
  usage?: ProviderUsage;
  error?: string;
};

const LOCK_STALE_MS = 30_000;
const MAX_CACHE_WINDOWS = 12;
const MAX_FUTURE_SKEW_MS = 5_000;

export type ProviderCacheIssue =
  | "missing"
  | "unreadable"
  | "malformed"
  | "error"
  | "stale"
  | "unknown";

export type CachedUsageWindow = UsageWindow & {
  postReset: boolean;
};

export type ProviderCacheView = {
  fetchedAt?: number;
  ageMS?: number;
  windows: CachedUsageWindow[];
  issue?: ProviderCacheIssue;
};

export async function readProviderCache(providerID: string) {
  try {
    return JSON.parse(
      await fs.readFile(usageCachePath(providerID), "utf8"),
    ) as CachedProviderUsage;
  } catch {
    return {};
  }
}

export async function inspectProviderCache(
  providerID: string,
  staleAfterMS: number,
  now = Date.now(),
): Promise<ProviderCacheView> {
  try {
    return decodeProviderCache(
      await fs.readFile(usageCachePath(providerID), "utf8"),
      staleAfterMS,
      now,
    );
  } catch (error) {
    return {
      windows: [],
      issue: isMissing(error) ? "missing" : "unreadable",
    };
  }
}

export function decodeProviderCache(
  raw: string,
  staleAfterMS: number,
  now = Date.now(),
): ProviderCacheView {
  try {
    const root = object(JSON.parse(raw));
    if (!root) return unknownCache("malformed");

    const fetchedAt = optionalNumber(root.fetchedAt);
    if (root.fetchedAt !== undefined && fetchedAt === undefined) {
      return unknownCache("malformed");
    }
    if (
      fetchedAt !== undefined &&
      (fetchedAt < 0 || fetchedAt > now + MAX_FUTURE_SKEW_MS)
    ) {
      return unknownCache("malformed");
    }

    const error = optionalString(root.error);
    if (root.error !== undefined && error === undefined) {
      return unknownCache("malformed");
    }

    const usage = root.usage === undefined ? undefined : object(root.usage);
    if (root.usage !== undefined && !usage) return unknownCache("malformed");
    const rawWindows = usage?.windows;
    if (rawWindows !== undefined && !Array.isArray(rawWindows)) {
      return unknownCache("malformed");
    }
    if (Array.isArray(rawWindows) && rawWindows.length > MAX_CACHE_WINDOWS) {
      return unknownCache("malformed");
    }

    const windows = (rawWindows ?? []).map((value) => parseCachedWindow(value, fetchedAt, now));
    if (windows.some((window) => !window)) return unknownCache("malformed");

    const ageMS = cacheAgeMS(fetchedAt, now);
    const view = {
      fetchedAt,
      ageMS,
      windows: windows as CachedUsageWindow[],
    } satisfies ProviderCacheView;
    if (error) return { ...view, issue: "error" };
    if (!fetchedAt || !view.windows.length) return { ...view, issue: "unknown" };
    if (isCacheStale(fetchedAt, staleAfterMS, now)) {
      return { ...view, issue: "stale" };
    }
    return view;
  } catch {
    return unknownCache("malformed");
  }
}

export function cacheAgeMS(fetchedAt: number | undefined, now = Date.now()) {
  if (fetchedAt === undefined) return undefined;
  return Math.max(0, now - fetchedAt);
}

export function isCacheStale(
  fetchedAt: number | undefined,
  staleAfterMS: number,
  now = Date.now(),
) {
  const age = cacheAgeMS(fetchedAt, now);
  return age !== undefined && age > staleAfterMS;
}

export async function writeProviderCache(
  providerID: string,
  cache: CachedProviderUsage,
) {
  const cachePath = usageCachePath(providerID);
  const tempPath = `${cachePath}.${process.pid}.tmp`;

  await fs.mkdir(path.dirname(cachePath), { recursive: true, mode: 0o700 });
  await fs.writeFile(tempPath, JSON.stringify(cache), "utf8");
  await fs.rename(tempPath, cachePath);
}

export async function withProviderLock<T>(
  providerID: string,
  run: () => Promise<T>,
) {
  const release = await acquireLock(providerID);
  if (!release) return undefined;

  try {
    return await run();
  } finally {
    await release();
  }
}

async function acquireLock(providerID: string) {
  const lockPath = usageLockPath(providerID);
  await fs.mkdir(path.dirname(lockPath), { recursive: true, mode: 0o700 });

  const release = await createLock(lockPath);
  if (release) return release;

  if (await isStaleLock(lockPath)) {
    await fs.rm(lockPath, { force: true }).catch(() => undefined);
    return createLock(lockPath);
  }

  return undefined;
}

async function createLock(lockPath: string) {
  let handle: fs.FileHandle | undefined;
  try {
    handle = await fs.open(lockPath, "wx");
    await handle.writeFile(
      JSON.stringify({ pid: process.pid, createdAt: Date.now() }),
      "utf8",
    );
    await handle.close();

    let released = false;
    return async () => {
      if (released) return;
      released = true;
      await fs.rm(lockPath, { force: true }).catch(() => undefined);
    };
  } catch {
    await handle?.close().catch(() => undefined);
    return undefined;
  }
}

async function isStaleLock(lockPath: string) {
  try {
    const raw = await fs.readFile(lockPath, "utf8");
    const parsed = JSON.parse(raw) as { createdAt?: unknown };
    return (
      typeof parsed.createdAt === "number" &&
      Date.now() - parsed.createdAt > LOCK_STALE_MS
    );
  } catch {
    return false;
  }
}

function parseCachedWindow(
  value: unknown,
  fetchedAt: number | undefined,
  now: number,
): CachedUsageWindow | undefined {
  const root = object(value);
  if (!root) return undefined;
  if (typeof root.label !== "string" || !root.label.trim() || root.label.length > 8) {
    return undefined;
  }

  const usedPercent = optionalNumber(root.usedPercent);
  if (
    root.usedPercent !== undefined &&
    (usedPercent === undefined || usedPercent < 0 || usedPercent > 100)
  ) {
    return undefined;
  }

  const resetAt = optionalString(root.resetAt);
  if (root.resetAt !== undefined && resetAt === undefined) return undefined;
  const resetMS = resetAt === undefined ? undefined : Date.parse(resetAt);
  if (resetMS !== undefined && !Number.isFinite(resetMS)) return undefined;

  return {
    label: root.label.trim(),
    usedPercent,
    resetAt,
    postReset:
      resetMS !== undefined &&
      resetMS <= now &&
      (fetchedAt === undefined || fetchedAt <= resetMS),
  };
}

function unknownCache(issue: ProviderCacheIssue): ProviderCacheView {
  return { windows: [], issue };
}

function object(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined;
}

function optionalNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function optionalString(value: unknown) {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function isMissing(error: unknown) {
  return (
    typeof error === "object" &&
    error !== null &&
    "code" in error &&
    (error as { code?: unknown }).code === "ENOENT"
  );
}
