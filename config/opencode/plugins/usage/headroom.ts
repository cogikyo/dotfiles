import { usageAdapters } from "./adapters.ts";
import {
  inspectProviderCache,
  type ProviderCacheView,
} from "./cache.ts";

const SNAPSHOT_MARKER = "\n\n[Cached headroom: ";

export async function appendHeadroomSnapshot(description: string, now = Date.now()) {
  const base = description.split(SNAPSHOT_MARKER, 1)[0];
  const views = await Promise.all(
    usageAdapters.map(async (adapter) => ({
      adapter,
      view: await inspectProviderCache(adapter.id, adapter.poll.staleAfterMS, now),
    })),
  );

  return `${base}${SNAPSHOT_MARKER}${views
    .map(({ adapter, view }) => renderProvider(adapter.label, view, now))
    .join("; ")}. Fresh values may guide fanout/provider choice; stale or unknown values cannot justify fanout.]`;
}

export function renderProvider(
  label: string,
  view: ProviderCacheView,
  now = Date.now(),
) {
  const windows = view.windows.length
    ? view.windows.map((window) => {
        const current = !view.issue && !window.postReset;
        const headroom =
          current && window.usedPercent !== undefined
            ? `${Math.round(100 - window.usedPercent)}%`
            : "?";
        return `${window.label}:${headroom}/${formatReset(window.resetAt, now)}`;
      })
    : ["?"];
  const reasons = new Set<string>();
  if (view.issue) reasons.add(view.issue);
  if (view.windows.some((window) => window.postReset)) reasons.add("post-reset");
  if (view.windows.some((window) => window.usedPercent === undefined)) {
    reasons.add("unknown");
  }
  const reason = reasons.size ? ` ${[...reasons].join("+")}` : "";
  return `${label} ${windows.join(",")} @${formatAge(view.ageMS)}${reason}`;
}

function formatAge(ms: number | undefined) {
  if (ms === undefined) return "?";
  if (ms < 60_000) return `${Math.floor(ms / 1000)}s`;
  return formatDuration(ms, false);
}

function formatReset(value: string | undefined, now: number) {
  if (!value) return "-";
  const ms = Date.parse(value) - now;
  if (ms <= 0) return "past";
  return formatDuration(ms, true);
}

function formatDuration(ms: number, roundUp: boolean) {
  const totalMinutes = Math.max(
    roundUp ? 1 : 0,
    (roundUp ? Math.ceil : Math.floor)(ms / 60_000),
  );
  if (totalMinutes < 60) return `${totalMinutes}m`;
  const totalHours = (roundUp ? Math.ceil : Math.floor)(totalMinutes / 60);
  if (totalHours < 48) return `${totalHours}h`;
  return `${(roundUp ? Math.ceil : Math.floor)(totalHours / 24)}d`;
}
