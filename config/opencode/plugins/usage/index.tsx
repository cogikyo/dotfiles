/** @jsxImportSource @opentui/solid */
import type {
  TuiPlugin,
  TuiPluginApi,
  TuiPluginModule,
} from "@opencode-ai/plugin/tui";
import { createSignal, onCleanup } from "solid-js";
import { sessionProviderID } from "../shared/session.ts";
import { anthropicUsage } from "./anthropic.ts";
import {
  type CachedProviderUsage,
  readProviderCache,
  withProviderLock,
  writeProviderCache,
} from "./cache.ts";
import { opencodeGoUsage } from "./opencode-go.ts";
import { openaiUsage } from "./openai.ts";
import type { ProviderAdapter, ProviderUsage } from "./types.ts";
import { UsageDashboard } from "./ui.tsx";
import { xaiUsage } from "./xai.ts";

const id = "cullyn.usage-sidebar";
const INTERNAL_CONTEXT_PLUGIN_ID = "internal:sidebar-context";
const UI_REFRESH_MS = 60_000;
const DEFAULT_MIN_FETCH_INTERVAL_MS = 60_000;
const DEFAULT_ERROR_BACKOFF_MS = 60_000;
const DEFAULT_RATE_LIMIT_BACKOFF_MS = 10 * 60_000;
const DEFAULT_STALE_AFTER_MS = 2 * 60_000;
const EVENT_REFRESH_DELAY_MS = 5_000;
const adapters = [
  openaiUsage,
  anthropicUsage,
  xaiUsage,
  opencodeGoUsage,
] satisfies ProviderAdapter[];

function isInformationalNote(usage: ProviderUsage) {
  return usage.noteKind === "info" || usage.noteKind === "warn";
}

function pendingUsage(adapter: ProviderAdapter): ProviderUsage {
  return { id: adapter.id, label: adapter.label, windows: [] };
}

function unavailableUsage(adapter: ProviderAdapter): ProviderUsage {
  return {
    id: adapter.id,
    label: adapter.label,
    windows: [],
    note: "usage unavailable",
  };
}

function minFetchIntervalMS(adapter: ProviderAdapter) {
  return adapter.poll?.minFetchIntervalMS ?? DEFAULT_MIN_FETCH_INTERVAL_MS;
}

function errorBackoffMS(adapter: ProviderAdapter) {
  return adapter.poll?.errorBackoffMS ?? DEFAULT_ERROR_BACKOFF_MS;
}

function rateLimitBackoffMS(adapter: ProviderAdapter) {
  return adapter.poll?.rateLimitBackoffMS ?? DEFAULT_RATE_LIMIT_BACKOFF_MS;
}

function staleAfterMS(adapter: ProviderAdapter) {
  return adapter.poll?.staleAfterMS ?? DEFAULT_STALE_AFTER_MS;
}

function cachedUsage(
  adapter: ProviderAdapter,
  cache: CachedProviderUsage,
): ProviderUsage {
  if (cache.usage?.windows.length) {
    const age = cache.fetchedAt ? Date.now() - cache.fetchedAt : 0;
    const note = cache.error
      ? `stale; ${cache.error}`
      : age >= staleAfterMS(adapter)
        ? `stale ${formatAge(age)}`
        : undefined;
    return { ...cache.usage, note };
  }

  // Windowless informational/warn usage (e.g. xai reset/tier, opencode-go no-route) stays
  // visible instead of collapsing to pending, unless a later fetch recorded a hard error.
  if (cache.usage && !cache.error && isInformationalNote(cache.usage)) {
    return cache.usage;
  }

  if (cache.error) {
    const isCoolingDown = Boolean(
      cache.backoffUntil && Date.now() < cache.backoffUntil,
    );
    return {
      id: adapter.id,
      label: adapter.label,
      windows: [],
      note: isCoolingDown ? `${cache.error}; cooling down` : cache.error,
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
      (error === "HTTP 429" ? rateLimitBackoffMS(adapter) : errorBackoffMS(adapter)),
  } satisfies CachedProviderUsage;

  await writeProviderCache(adapter.id, cache).catch(() => undefined);
  return cachedUsage(adapter, cache);
}

function shouldFetch(adapter: ProviderAdapter, cache: CachedProviderUsage) {
  const now = Date.now();
  if (cache.backoffUntil && now < cache.backoffUntil) return false;
  if (!cache.fetchedAt) return true;
  return now - cache.fetchedAt >= minFetchIntervalMS(adapter);
}

async function fetchAndCache(adapter: ProviderAdapter) {
  const latest = await readProviderCache(adapter.id);
  if (!shouldFetch(adapter, latest)) return cachedUsage(adapter, latest);

  let usage: ProviderUsage;
  try {
    usage = await adapter.load();
  } catch {
    return recordError(adapter, latest, "usage unavailable");
  }

  if (usage.note === "HTTP 429") {
    return recordError(adapter, latest, "HTTP 429");
  }

  // Windowless usage is an error only when it is not a benign info/warn state.
  if (usage.windows.length === 0 && !isInformationalNote(usage)) {
    return recordError(adapter, latest, usage.note || "usage unavailable");
  }

  const cache = {
    fetchedAt: Date.now(),
    usage: cleanUsage(usage),
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

function UsagePanel(props: { api: TuiPluginApi; sessionID: string }) {
  const [providers, setProviders] = createSignal<ProviderUsage[]>(
    adapters.map(pendingUsage),
  );
  const [activeProviderID, setActiveProviderID] = createSignal(
    sessionProviderID(props.api, props.sessionID),
  );

  const refresh = (allowNetwork: boolean) => {
    setActiveProviderID(sessionProviderID(props.api, props.sessionID));
    void Promise.all(
      adapters.map((adapter) =>
        loadCached(adapter, allowNetwork).catch(() => unavailableUsage(adapter)),
      ),
    ).then((next) => setProviders(next));
  };

  let eventRefreshTimer: ReturnType<typeof setTimeout> | undefined;
  const scheduleRefresh = () => {
    setActiveProviderID(sessionProviderID(props.api, props.sessionID));
    if (eventRefreshTimer) clearTimeout(eventRefreshTimer);
    eventRefreshTimer = setTimeout(() => refresh(true), EVENT_REFRESH_DELAY_MS);
  };

  refresh(true);
  const timer = setInterval(() => refresh(false), UI_REFRESH_MS);
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
