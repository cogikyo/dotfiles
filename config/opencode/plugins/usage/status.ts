import {
  inspectProviderCache,
  type ProviderCacheView,
} from "./cache.ts";
import { usageProviderList } from "./providers.ts";

export async function renderUsageStatus(now = Date.now()) {
  const providers = await Promise.all(
    usageProviderList.map(async (provider) => ({
      provider,
      view: await inspectProviderCache(provider.id, provider.staleAfterMS, now),
    })),
  );

  return [
    "Cached provider headroom (read-only; stale/error/unknown values are not current):",
    ...providers.map(({ provider, view }) =>
      renderProviderStatus(provider.label, view, now),
    ),
  ].join("\n");
}

export function renderProviderStatus(
  label: string,
  view: ProviderCacheView,
  now = Date.now(),
) {
  const fetched = view.fetchedAt
    ? new Date(view.fetchedAt).toISOString()
    : "?";
  const windows = view.windows.length
    ? view.windows.map((window) => renderWindow(view, window, now)).join("; ")
    : "windows=?";
  return `${label} [${providerState(view)}] fetched=${fetched} age=${formatAge(view.ageMS)} | ${windows}`;
}

function renderWindow(
  view: ProviderCacheView,
  window: ProviderCacheView["windows"][number],
  now: number,
) {
  const current = !view.issue && !window.postReset;
  const remaining =
    current && window.usedPercent !== undefined
      ? `${Math.round(100 - window.usedPercent)}%`
      : "?";
  const reset = window.resetAt ?? "?";
  const proximity = window.resetAt ? formatProximity(Date.parse(window.resetAt) - now) : "?";
  return `${window.label} remaining=${remaining} reset=${reset} (${proximity})`;
}

function providerState(view: ProviderCacheView) {
  if (view.issue) return view.issue;
  const details: string[] = [];
  if (view.windows.some((window) => window.postReset)) details.push("post-reset");
  if (view.windows.some((window) => window.usedPercent === undefined)) {
    details.push("unknown");
  }
  return details.length ? `fresh+${details.join("+")}` : "fresh";
}

function formatAge(ms: number | undefined) {
  return ms === undefined ? "?" : formatDuration(ms);
}

function formatProximity(ms: number) {
  if (ms === 0) return "now";
  return ms > 0 ? `in ${formatDuration(ms)}` : `${formatDuration(-ms)} ago`;
}

function formatDuration(ms: number) {
  if (ms < 60_000) return `${Math.floor(ms / 1000)}s`;
  const minutes = Math.floor(ms / 60_000);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 48) return `${hours}h`;
  return `${Math.floor(hours / 24)}d`;
}
