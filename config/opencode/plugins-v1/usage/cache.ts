import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { z } from "zod";
import { readJson, writeJson } from "../shared/file.ts";
import { lenient, type ProviderUsage, type UsageWindow } from "./types.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Provider cache                                                                                │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

// ├─ Cache record ────────────────────────────────────────────────────────────────────────────────┤

// Cache reads tolerate invalid notes; inspection checks window values more strictly.
const Window = z.object({
  label: z.string(),
  usedPercent: z.number().optional(),
  resetAt: z.string().optional(),
}) satisfies z.ZodType<UsageWindow>;

const Usage = z.object({
  id: z.string(),
  label: z.string(),
  windows: z.array(Window),
  note: lenient(z.string()),
  noteKind: lenient(z.enum(["info", "warn", "error"])),
  placeholders: z.array(z.string()).optional(),
}) satisfies z.ZodType<ProviderUsage>;

const Cache = z.object({
  fetchedAt: z.number().optional(),
  backoffUntil: z.number().optional(),
  usage: Usage.optional(),
  error: z.string().optional(),
});

export type CachedProviderUsage = z.infer<typeof Cache>;

// ├─ Inspection ──────────────────────────────────────────────────────────────────────────────────┤

/** Reason cached usage cannot be treated as current. */
export type ProviderCacheIssue = "missing" | "unreadable" | "malformed" | "error" | "stale" | "unknown";

/** A cached window whose `postReset` flag means its usage predates a passed reset. */
export type CachedUsageWindow = UsageWindow & {
  postReset: boolean;
};

/** Validated cache data; even a fresh view can have post-reset or unknown windows. */
export type ProviderCacheView = {
  fetchedAt?: number;
  ageMS?: number;
  windows: CachedUsageWindow[];
  issue?: ProviderCacheIssue;
};

const MAX_CACHE_WINDOWS = 12;
const MAX_FUTURE_SKEW_MS = 5_000;
const WINDOW_LABEL = /^[A-Za-z0-9_-]{1,8}$/;

/** Validates cached usage and reports its freshness; unreadable or malformed files return an issue without windows. */
export async function inspectProviderCache(
  providerID: string,
  staleAfterMS: number,
  now = Date.now(),
): Promise<ProviderCacheView> {
  let cache: CachedProviderUsage | undefined;
  try {
    cache = await readJson(cachePath(providerID), Cache);
  } catch (error) {
    return unknownCache(error instanceof SyntaxError || error instanceof z.ZodError ? "malformed" : "unreadable");
  }
  if (!cache) return unknownCache("missing");

  const { fetchedAt, error } = cache;
  if (fetchedAt !== undefined && (fetchedAt < 0 || fetchedAt > now + MAX_FUTURE_SKEW_MS)) {
    return unknownCache("malformed");
  }
  if (error === "") return unknownCache("malformed");

  const windows = inspectWindows(cache.usage?.windows ?? [], fetchedAt, now);
  if (!windows) return unknownCache("malformed");

  const view = { fetchedAt, ageMS: cacheAgeMS(fetchedAt, now), windows } satisfies ProviderCacheView;
  const issue = cacheIssue(view, error, staleAfterMS, now);
  return issue ? { ...view, issue } : view;
}

export function cacheAgeMS(fetchedAt: number | undefined, now = Date.now()) {
  if (fetchedAt === undefined) return undefined;
  return Math.max(0, now - fetchedAt);
}

export function isCacheStale(fetchedAt: number | undefined, staleAfterMS: number, now = Date.now()) {
  const age = cacheAgeMS(fetchedAt, now);
  return age !== undefined && age > staleAfterMS;
}

function cacheIssue(
  view: ProviderCacheView,
  error: string | undefined,
  staleAfterMS: number,
  now: number,
): ProviderCacheIssue | undefined {
  if (error) return "error";
  if (!view.fetchedAt || !view.windows.length) return "unknown";
  if (isCacheStale(view.fetchedAt, staleAfterMS, now)) return "stale";
  return undefined;
}

function inspectWindows(windows: UsageWindow[], fetchedAt: number | undefined, now: number) {
  if (windows.length > MAX_CACHE_WINDOWS) return undefined;
  const inspected = windows.map((window) => inspectWindow(window, fetchedAt, now));
  if (inspected.includes(undefined)) return undefined;
  return inspected.filter((window) => window !== undefined);
}

function inspectWindow(window: UsageWindow, fetchedAt: number | undefined, now: number): CachedUsageWindow | undefined {
  const { label, usedPercent } = window;
  if (!WINDOW_LABEL.test(label)) return undefined;
  if (usedPercent !== undefined && (usedPercent < 0 || usedPercent > 100)) return undefined;

  if (window.resetAt === undefined) return { label, usedPercent, postReset: false };
  const resetMS = Date.parse(window.resetAt);
  if (!Number.isFinite(resetMS)) return undefined;
  const postReset = resetMS <= now && (fetchedAt === undefined || fetchedAt <= resetMS);
  return { label, usedPercent, resetAt: new Date(resetMS).toISOString(), postReset };
}

function unknownCache(issue: ProviderCacheIssue): ProviderCacheView {
  return { windows: [], issue };
}

// ├─ Read and write ──────────────────────────────────────────────────────────────────────────────┤

/** Reads cached usage, returning an empty record on any read or parse failure. */
export async function readProviderCache(providerID: string): Promise<CachedProviderUsage> {
  return (await readJson(cachePath(providerID), Cache).catch(() => undefined)) ?? {};
}

/** Atomically writes provider usage; write errors propagate. */
export async function writeProviderCache(providerID: string, cache: CachedProviderUsage) {
  await writeJson(cachePath(providerID), cache);
}

function cacheDir() {
  const xdg = process.env.XDG_CACHE_HOME?.trim();
  const root = xdg ? path.resolve(xdg) : path.join(os.homedir(), ".cache");
  return path.join(root, "opencode", "usage-sidebar");
}

function cachePath(providerID: string) {
  return path.join(cacheDir(), `${providerID}.json`);
}

// ├─ Provider lock ───────────────────────────────────────────────────────────────────────────────┤

/** Runs one provider operation under a lock; busy locks return undefined, and locks older than 30 seconds are replaced. */
export async function withProviderLock<T>(providerID: string, run: () => Promise<T>) {
  const release = await acquireLock(providerID);
  if (!release) return undefined;

  try {
    return await run();
  } finally {
    await release();
  }
}

async function acquireLock(providerID: string) {
  const file = lockPath(providerID);
  await fs.mkdir(path.dirname(file), { recursive: true, mode: 0o700 });

  const release = await createLock(file);
  if (release) return release;

  if (await isStaleLock(file)) {
    await fs.rm(file, { force: true }).catch(() => undefined);
    return createLock(file);
  }

  return undefined;
}

async function createLock(file: string) {
  let handle: fs.FileHandle | undefined;
  try {
    handle = await fs.open(file, "wx");
    await handle.writeFile(JSON.stringify({ pid: process.pid, createdAt: Date.now() }), "utf8");
    await handle.close();

    let released = false;
    return async () => {
      if (released) return;
      released = true;
      await fs.rm(file, { force: true }).catch(() => undefined);
    };
  } catch {
    await handle?.close().catch(() => undefined);
    return undefined;
  }
}

const Lock = z.object({ createdAt: z.number() });
const LOCK_STALE_MS = 30_000;

async function isStaleLock(file: string) {
  const lock = await readJson(file, Lock).catch(() => undefined);
  return lock !== undefined && Date.now() - lock.createdAt > LOCK_STALE_MS;
}

function lockPath(providerID: string) {
  const xdg = process.env.XDG_RUNTIME_DIR?.trim();
  const dir = xdg
    ? path.join(path.resolve(xdg), "opencode")
    : path.join("/tmp", `opencode-${process.getuid?.() ?? os.userInfo().uid}`);
  return path.join(dir, `usage-sidebar-${providerID}.lock`);
}
