import type { ProviderAdapter, ProviderUsage } from "./types.ts";

// opencode-go has no API-key usage route upstream. The console's `queryLiteSubscription`
// is a browser-session `/_server` call, and cookie replay was deferred pending live approval.
// This adapter is honest and offline: it reports no usable route rather than faking windows.
const id = "opencode-go";
const label = "OpenCode Go";
const POLL_INTERVAL_MS = 60 * 60_000;

async function load(): Promise<ProviderUsage> {
  return {
    id,
    label,
    windows: [],
    note: "usage unavailable: no API-key route upstream",
    noteKind: "warn",
  };
}

export const opencodeGoUsage: ProviderAdapter = {
  id,
  label,
  poll: {
    minFetchIntervalMS: POLL_INTERVAL_MS,
    errorBackoffMS: POLL_INTERVAL_MS,
    rateLimitBackoffMS: POLL_INTERVAL_MS,
    staleAfterMS: POLL_INTERVAL_MS,
  },
  load,
};
