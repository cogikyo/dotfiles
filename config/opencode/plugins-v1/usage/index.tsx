/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from "@opencode-ai/plugin/tui";
import { Show, createComputed, createRenderEffect, createSignal, on, onCleanup, untrack } from "solid-js";
import { sessionProviderID } from "../shared/session.ts";
import { usageAdapters } from "./adapters.ts";
import {
  cacheAgeMS,
  type CachedProviderUsage,
  isCacheStale,
  readProviderCache,
  withProviderLock,
  writeProviderCache,
} from "./cache.ts";
import { declaredWindows, type ProviderAdapter, type ProviderUsage } from "./types.ts";
import { UsageDashboard } from "./ui.tsx";

const INTERNAL_CONTEXT_PLUGIN_ID = "internal:sidebar-context";
const UI_REFRESH_MS = 60_000;
const EVENT_REFRESH_DELAY_MS = 5_000;
const adapters = usageAdapters;

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Usage sidebar                                                                                 │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

// ├─ Plugin entry ────────────────────────────────────────────────────────────────────────────────┤

const tui: TuiPlugin = async (api) => {
  let didDeactivateContext = false;
  const contextPlugin = api.plugins.list().find((item) => item.id === INTERNAL_CONTEXT_PLUGIN_ID);
  if (contextPlugin?.active) {
    didDeactivateContext = await api.plugins.deactivate(INTERNAL_CONTEXT_PLUGIN_ID).catch(() => false);
  }

  api.lifecycle.onDispose(async () => {
    if (didDeactivateContext) await api.plugins.activate(INTERNAL_CONTEXT_PLUGIN_ID);
  });

  api.slots.register({
    order: 100,
    slots: {
      sidebar_title() {
        return null;
      },
      sidebar_content(_ctx, props: { session_id: string }) {
        return (
          <Show when={!api.state.session.get(props.session_id)?.parentID}>
            <UsagePanel api={api} sessionID={props.session_id} />
          </Show>
        );
      },
    },
  });
};

/** Shows cached usage for root sessions and temporarily replaces the built-in sidebar context. */
const plugin: TuiPluginModule & { id: string } = {
  id: "cullyn.usage-sidebar",
  tui,
};

export default plugin;

// ├─ Session panel ───────────────────────────────────────────────────────────────────────────────┤

function UsagePanel(props: { api: TuiPluginApi; sessionID: string }) {
  const [providers, setProviders] = createSignal<ProviderUsage[]>(adapters.map(pendingUsage));
  const [activeProviderID, setActiveProviderID] = createSignal("");
  const [refreshingProviderIDs, setRefreshingProviderIDs] = createSignal(new Set<string>());

  const refresh = (allowNetwork: boolean) => {
    setActiveProviderID(sessionProviderID(props.api, props.sessionID));
    void Promise.all(
      adapters.map((adapter) =>
        loadCached(adapter, allowNetwork).catch(() => ({
          ...pendingUsage(adapter),
          note: "unavailable",
          noteKind: "error" as const,
        })),
      ),
    ).then((loaded) => setProviders(loaded));
  };

  let eventRefreshTimer: ReturnType<typeof setTimeout> | undefined;
  const scheduleRefresh = () => {
    setActiveProviderID(sessionProviderID(props.api, props.sessionID));
    if (eventRefreshTimer) clearTimeout(eventRefreshTimer);
    eventRefreshTimer = setTimeout(() => refresh(true), EVENT_REFRESH_DELAY_MS);
  };

  const markRefreshing = (providerID: string, active: boolean) => {
    setRefreshingProviderIDs((current) => {
      const next = new Set(current);
      if (active) next.add(providerID);
      else next.delete(providerID);
      return next;
    });
  };

  const refreshProvider = (providerID: string) => {
    const adapter = adapters.find((item) => item.id === providerID);
    if (!adapter) return;
    if (refreshingProviderIDs().has(adapter.id)) return;
    markRefreshing(adapter.id, true);
    void manualRefresh(adapter)
      .catch(() => ({
        ...pendingUsage(adapter),
        note: "unavailable",
        noteKind: "error" as const,
      }))
      .then((usage) =>
        setProviders((current) => current.map((provider) => (provider.id === adapter.id ? usage : provider))),
      )
      .finally(() => markRefreshing(adapter.id, false));
  };

  createComputed(
    on(
      () => props.sessionID,
      () => refresh(true),
    ),
  );
  const timer = setInterval(() => refresh(true), UI_REFRESH_MS);
  createRenderEffect(() => {
    for (const type of ["message.updated", "message.removed", "session.updated"] as const) {
      const dispose = props.api.event.on(type, (event) => {
        if (event.properties.sessionID !== untrack(() => props.sessionID)) return;
        untrack(scheduleRefresh);
      });
      onCleanup(dispose);
    }
  });
  onCleanup(() => clearInterval(timer));
  onCleanup(() => {
    if (eventRefreshTimer) clearTimeout(eventRefreshTimer);
  });

  return (
    <UsageDashboard
      api={props.api}
      providers={providers()}
      activeProviderID={activeProviderID()}
      refreshingProviderIDs={refreshingProviderIDs()}
      onRefresh={refreshProvider}
    />
  );
}

// ├─ Fetch and cache ─────────────────────────────────────────────────────────────────────────────┤

// A busy provider lock leaves the current cache row in place.
async function manualRefresh(adapter: ProviderAdapter) {
  const result = await withProviderLock(adapter.id, () => fetchAndCache(adapter, true));
  if (result) return result;

  return cachedUsage(adapter, await readProviderCache(adapter.id));
}

async function loadCached(adapter: ProviderAdapter, allowNetwork: boolean) {
  const cache = await readProviderCache(adapter.id);
  if (!allowNetwork || !shouldFetch(adapter, cache)) {
    return cachedUsage(adapter, cache);
  }

  const result = await withProviderLock(adapter.id, () => fetchAndCache(adapter));
  if (result) return result;

  return cachedUsage(adapter, await readProviderCache(adapter.id));
}

async function fetchAndCache(adapter: ProviderAdapter, force = false) {
  const latest = await readProviderCache(adapter.id);
  if (!force && !shouldFetch(adapter, latest)) return cachedUsage(adapter, latest);

  let usage: ProviderUsage;
  try {
    usage = await adapter.load();
  } catch {
    return recordError(adapter, latest, "unavailable");
  }

  // The literal `429` selects rate-limit backoff.
  if (usage.note === "429") {
    return recordError(adapter, latest, "429");
  }

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

// A failed fetch preserves cached windows and fetch time while recording backoff.
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

// ├─ Cached view ─────────────────────────────────────────────────────────────────────────────────┤

function cachedUsage(adapter: ProviderAdapter, cache: CachedProviderUsage): ProviderUsage {
  // Current adapter labels override cached labels after configuration changes.
  const { id, label, placeholders } = adapter;

  if (cache.usage?.windows.length) {
    const cached = { ...cache.usage, windows: declaredWindows(cache.usage, placeholders) };
    const age = cacheAgeMS(cache.fetchedAt) ?? 0;
    if (cache.error) {
      return {
        ...cached,
        id,
        label,
        placeholders,
        note: cache.error,
        noteKind: "error",
      };
    }
    const note = isCacheStale(cache.fetchedAt, adapter.poll.staleAfterMS) ? `stale ${formatAge(age)}` : undefined;
    return { ...cached, id, label, placeholders, note };
  }

  if (cache.usage && !cache.error && isInformationalNote(cache.usage)) {
    return { ...cache.usage, id, label, placeholders };
  }

  if (cache.error) {
    return {
      id,
      label,
      placeholders,
      windows: [],
      note: cache.error,
      noteKind: "error",
    };
  }

  return pendingUsage(adapter);
}

function pendingUsage(adapter: ProviderAdapter): ProviderUsage {
  return {
    id: adapter.id,
    label: adapter.label,
    placeholders: adapter.placeholders,
    windows: [],
  };
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
