import {
  cacheAgeMS,
  isCacheStale,
  readProviderCache,
  withProviderLock,
  writeProviderCache,
  type CachedProviderUsage,
} from "./cache.ts";
import { declaredWindows, type ProviderAdapter, type ProviderUsage } from "./types.ts";

export async function refresh(adapter: ProviderAdapter, force = false) {
  const cache = await readProviderCache(adapter.id);
  if (!force && !shouldFetch(adapter, cache)) return cachedUsage(adapter, cache);

  const result = await withProviderLock(adapter.id, () => fetchAndCache(adapter, force));
  if (result) return result;

  return cachedUsage(adapter, await readProviderCache(adapter.id));
}

async function fetchAndCache(adapter: ProviderAdapter, force: boolean) {
  const latest = await readProviderCache(adapter.id);
  if (!force && !shouldFetch(adapter, latest)) return cachedUsage(adapter, latest);

  let usage: ProviderUsage;
  try {
    usage = await adapter.load();
  } catch {
    return recordError(adapter, latest, "unavailable");
  }

  if (usage.note === "429") return recordError(adapter, latest, "429");

  if (usage.windows.length === 0 && !isInformationalNote(usage)) {
    return recordError(adapter, latest, usage.note || "unavailable");
  }

  const cache = {
    fetchedAt: Date.now(),
    usage: cleanUsage(usage),
    backoffUntil:
      usage.windows.length === 0 && isInformationalNote(usage) ? Date.now() + adapter.poll.warnBackoffMS : undefined,
  } satisfies CachedProviderUsage;
  await writeProviderCache(adapter.id, cache).catch(() => undefined);
  return cachedUsage(adapter, cache);
}

function shouldFetch(adapter: ProviderAdapter, cache: CachedProviderUsage) {
  const now = Date.now();
  if (cache.backoffUntil && now < cache.backoffUntil) return false;
  if (!cache.fetchedAt) return true;
  return now - cache.fetchedAt >= adapter.poll.minFetchIntervalMS;
}

async function recordError(adapter: ProviderAdapter, previous: CachedProviderUsage, error: string) {
  const cache = {
    fetchedAt: previous.fetchedAt,
    usage: previous.usage,
    error,
    backoffUntil: Date.now() + (error === "429" ? adapter.poll.rateLimitBackoffMS : adapter.poll.errorBackoffMS),
  } satisfies CachedProviderUsage;

  await writeProviderCache(adapter.id, cache).catch(() => undefined);
  return cachedUsage(adapter, cache);
}

export function cachedUsage(adapter: ProviderAdapter, cache: CachedProviderUsage): ProviderUsage {
  const { id, label, placeholders } = adapter;

  if (cache.usage?.windows.length) {
    const cached = { ...cache.usage, windows: declaredWindows(cache.usage, placeholders) };
    const age = cacheAgeMS(cache.fetchedAt) ?? 0;
    if (cache.error) {
      return { ...cached, id, label, placeholders, note: cache.error, noteKind: "error" };
    }
    const note = isCacheStale(cache.fetchedAt, adapter.poll.staleAfterMS) ? `stale ${formatAge(age)}` : undefined;
    return { ...cached, id, label, placeholders, note };
  }

  if (cache.usage && !cache.error && isInformationalNote(cache.usage)) {
    return { ...cache.usage, id, label, placeholders };
  }

  if (cache.error) {
    return { id, label, placeholders, windows: [], note: cache.error, noteKind: "error" };
  }

  return pendingUsage(adapter);
}

export function pendingUsage(adapter: ProviderAdapter): ProviderUsage {
  return { id: adapter.id, label: adapter.label, placeholders: adapter.placeholders, windows: [] };
}

function cleanUsage(usage: ProviderUsage): ProviderUsage {
  return {
    id: usage.id,
    label: usage.label,
    windows: usage.windows,
    note: usage.note,
    noteKind: usage.noteKind,
  };
}

function formatAge(ms: number) {
  const minutes = Math.max(1, Math.floor(ms / 60_000));
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const remainder = minutes % 60;
  return remainder === 0 ? `${hours}h` : `${hours}h ${remainder}m`;
}

function isInformationalNote(usage: ProviderUsage) {
  return usage.noteKind === "info" || usage.noteKind === "warn";
}
