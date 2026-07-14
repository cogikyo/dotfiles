/** @jsxImportSource @opentui/solid */
import type {
  TuiPlugin,
  TuiPluginApi,
  TuiPluginModule,
} from "@opencode-ai/plugin/tui";
import { createSignal, onCleanup } from "solid-js";
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
import type { ProviderAdapter, ProviderUsage } from "./types.ts";
import { UsageDashboard } from "./ui.tsx";

const id = "cullyn.usage-sidebar";
const INTERNAL_CONTEXT_PLUGIN_ID = "internal:sidebar-context";
const UI_REFRESH_MS = 60_000;
const EVENT_REFRESH_DELAY_MS = 5_000;
const adapters = usageAdapters;

function isInformationalNote(usage: ProviderUsage) {
  return usage.noteKind === "info" || usage.noteKind === "warn";
}

function pendingUsage(adapter: ProviderAdapter): ProviderUsage {
  return {
    id: adapter.id,
    label: adapter.label,
    placeholders: adapter.placeholders,
    windows: [],
  };
}

function cachedUsage(
  adapter: ProviderAdapter,
  cache: CachedProviderUsage,
): ProviderUsage {
  // Always stamp identity from the adapter so renamed labels and per-provider placeholders
  // take effect immediately, even while a stale cache still holds the old id/label/note.
  const { id, label, placeholders } = adapter;

  if (cache.usage?.windows.length) {
    const age = cacheAgeMS(cache.fetchedAt) ?? 0;
    if (cache.error) {
      return {
        ...cache.usage,
        id,
        label,
        placeholders,
        note: cache.error,
        noteKind: "error",
      };
    }
    const note = isCacheStale(cache.fetchedAt, adapter.poll.staleAfterMS)
      ? `stale ${formatAge(age)}`
      : undefined;
    return { ...cache.usage, id, label, placeholders, note };
  }

  // Windowless informational/warn usage (e.g. opencode-go no-route) stays visible instead of
  // collapsing to pending, unless a later fetch recorded a hard error.
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

function formatAge(ms: number) {
  const minutes = Math.max(1, Math.floor(ms / 60_000));
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const remainder = minutes % 60;
  return remainder === 0 ? `${hours}h` : `${hours}h ${remainder}m`;
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

async function recordError(
  adapter: ProviderAdapter,
  previous: CachedProviderUsage,
  error: string,
) {
  const cache = {
    fetchedAt: previous.fetchedAt,
    usage: previous.usage,
    error,
    backoffUntil:
      Date.now() +
      (error === "429" ? adapter.poll.rateLimitBackoffMS : adapter.poll.errorBackoffMS),
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

async function fetchAndCache(adapter: ProviderAdapter, force = false) {
  const latest = await readProviderCache(adapter.id);
  // Force (manual click) bypasses minFetchIntervalMS and backoffUntil; user intent is explicit.
  if (!force && !shouldFetch(adapter, latest)) return cachedUsage(adapter, latest);

  let usage: ProviderUsage;
  try {
    usage = await adapter.load();
  } catch {
    return recordError(adapter, latest, "unavailable");
  }

  if (usage.note === "429") {
    return recordError(adapter, latest, "429");
  }

  // Windowless usage is an error only when it is not a benign info/warn state.
  if (usage.windows.length === 0 && !isInformationalNote(usage)) {
    return recordError(adapter, latest, usage.note || "unavailable");
  }

  const cache = {
    fetchedAt: Date.now(),
    usage: cleanUsage(usage),
    backoffUntil:
      usage.windows.length === 0 && isInformationalNote(usage)
        ? Date.now() + adapter.poll.warnBackoffMS
        : undefined,
  } satisfies CachedProviderUsage;
  await writeProviderCache(adapter.id, cache).catch(() => undefined);
  return cachedUsage(adapter, cache);
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

async function manualRefresh(adapter: ProviderAdapter) {
  const result = await withProviderLock(adapter.id, () =>
    fetchAndCache(adapter, true),
  );
  if (result) return result;

  return cachedUsage(adapter, await readProviderCache(adapter.id));
}

function UsagePanel(props: { api: TuiPluginApi; sessionID: string }) {
  const [providers, setProviders] = createSignal<ProviderUsage[]>(
    adapters.map(pendingUsage),
  );
  const [activeProviderID, setActiveProviderID] = createSignal(
    sessionProviderID(props.api, props.sessionID),
  );
  const [refreshingProviderIDs, setRefreshingProviderIDs] = createSignal(
    new Set<string>(),
  );

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
    ).then((next) => setProviders(next));
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
      .then((next) => {
        setProviders((current) =>
          current.map((provider) =>
            provider.id === adapter.id ? next : provider,
          ),
        );
      })
      .finally(() => markRefreshing(adapter.id, false));
  };

  refresh(true);
  const timer = setInterval(() => refresh(true), UI_REFRESH_MS);
  const disposeMessageUpdated = props.api.event.on(
    "message.updated",
    (event) => {
      if (event.properties.sessionID !== props.sessionID) return;
      scheduleRefresh();
    },
  );
  const disposeMessageRemoved = props.api.event.on(
    "message.removed",
    (event) => {
      if (event.properties.sessionID !== props.sessionID) return;
      scheduleRefresh();
    },
  );
  const disposeSessionUpdated = props.api.event.on(
    "session.updated",
    (event) => {
      if (event.properties.sessionID !== props.sessionID) return;
      scheduleRefresh();
    },
  );
  onCleanup(() => clearInterval(timer));
  onCleanup(() => {
    if (eventRefreshTimer) clearTimeout(eventRefreshTimer);
  });
  onCleanup(disposeMessageUpdated);
  onCleanup(disposeMessageRemoved);
  onCleanup(disposeSessionUpdated);

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

const tui: TuiPlugin = async (api) => {
  let didDeactivateContext = false;
  const contextPlugin = api.plugins
    .list()
    .find((item) => item.id === INTERNAL_CONTEXT_PLUGIN_ID);
  if (contextPlugin?.active) {
    didDeactivateContext = await api.plugins
      .deactivate(INTERNAL_CONTEXT_PLUGIN_ID)
      .catch(() => false);
  }

  api.lifecycle.onDispose(() => {
    if (!didDeactivateContext) return;
    return api.plugins
      .activate(INTERNAL_CONTEXT_PLUGIN_ID)
      .then(() => undefined);
  });

  api.slots.register({
    order: 100,
    slots: {
      sidebar_title() {
        return null;
      },
      sidebar_content(_ctx, props: { session_id: string }) {
        return <UsagePanel api={api} sessionID={props.session_id} />;
      },
    },
  });
};

const plugin: TuiPluginModule & { id: string } = {
  id,
  tui,
};

export default plugin;
