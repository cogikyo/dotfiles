import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { Option, Schema } from "effect";
import { Usage } from "./rpc.ts";
import { num, record, str, type NoteKind, type ProviderUsage, type UsageWindow } from "./types.ts";

export type CachedProviderUsage = {
  fetchedAt?: number;
  backoffUntil?: number;
  usage?: ProviderUsage;
  error?: string;
};

export type ProviderCacheIssue = "missing" | "unreadable" | "malformed" | "error" | "stale" | "unknown";

export type CachedUsageWindow = UsageWindow & {
  postReset: boolean;
};

export type ProviderCacheView = {
  fetchedAt?: number;
  ageMS?: number;
  windows: CachedUsageWindow[];
  issue?: ProviderCacheIssue;
};

class Malformed extends Error {}

const MAX_CACHE_WINDOWS = 12;
const MAX_FUTURE_SKEW_MS = 5_000;
const WINDOW_LABEL = /^[A-Za-z0-9_-]{1,8}$/;
const NOTE_KINDS = new Set<unknown>(["info", "warn", "error"]);

function isNoteKind(value: unknown): value is NoteKind {
  return NOTE_KINDS.has(value);
}

const Cache = Schema.Struct({
  fetchedAt: Schema.optional(Schema.Finite),
  backoffUntil: Schema.optional(Schema.Finite),
  usage: Schema.optional(
    Usage.mapFields((fields) => ({
      ...fields,
      note: Schema.optional(Schema.Unknown),
      noteKind: Schema.optional(Schema.Unknown),
    })),
  ),
  error: Schema.optional(Schema.String),
});
const decodeCache = Schema.decodeUnknownOption(Cache);

async function readCache(providerID: string): Promise<CachedProviderUsage | undefined> {
  let text: string;
  try {
    text = await fs.readFile(cachePath(providerID), "utf8");
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "ENOENT") return undefined;
    throw error;
  }
  const decoded = decodeCache(JSON.parse(text));
  if (Option.isNone(decoded)) throw new Malformed();
  const { usage, ...cache } = decoded.value;
  if (!usage) return cache;
  const { note, noteKind } = usage;
  return { ...cache, usage: { ...usage, note: str(note), noteKind: isNoteKind(noteKind) ? noteKind : undefined } };
}

export async function inspectProviderCache(
  providerID: string,
  staleAfterMS: number,
  now = Date.now(),
): Promise<ProviderCacheView> {
  let cache: CachedProviderUsage | undefined;
  try {
    cache = await readCache(providerID);
  } catch (error) {
    return unknownCache(error instanceof SyntaxError || error instanceof Malformed ? "malformed" : "unreadable");
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

export async function readProviderCache(providerID: string): Promise<CachedProviderUsage> {
  return (await readCache(providerID).catch(() => undefined)) ?? {};
}

export async function writeProviderCache(providerID: string, cache: CachedProviderUsage) {
  const file = cachePath(providerID);
  await fs.mkdir(path.dirname(file), { recursive: true, mode: 0o700 });
  const temp = `${file}.${process.pid}.${crypto.randomUUID()}.tmp`;
  try {
    await fs.writeFile(temp, `${JSON.stringify(cache, null, 2)}\n`);
    await fs.rename(temp, file);
  } catch (error) {
    await fs.rm(temp, { force: true }).catch(() => undefined);
    throw error;
  }
}

function cacheDir() {
  const xdg = process.env.XDG_CACHE_HOME?.trim();
  const root = xdg ? path.resolve(xdg) : path.join(os.homedir(), ".cache");
  return path.join(root, "opencode", "usage-sidebar");
}

function cachePath(providerID: string) {
  return path.join(cacheDir(), `${providerID}.json`);
}

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

const LOCK_STALE_MS = 30_000;

async function isStaleLock(file: string) {
  const text = await fs.readFile(file, "utf8").catch(() => undefined);
  if (text === undefined) return false;
  let createdAt: number | undefined;
  try {
    createdAt = num(record(JSON.parse(text))?.createdAt);
  } catch {
    return false;
  }
  return createdAt !== undefined && Date.now() - createdAt > LOCK_STALE_MS;
}

function lockPath(providerID: string) {
  const xdg = process.env.XDG_RUNTIME_DIR?.trim();
  const dir = xdg
    ? path.join(path.resolve(xdg), "opencode")
    : path.join("/tmp", `opencode-${process.getuid?.() ?? os.userInfo().uid}`);
  return path.join(dir, `usage-sidebar-${providerID}.lock`);
}
