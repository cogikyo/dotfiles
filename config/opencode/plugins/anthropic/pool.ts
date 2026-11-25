import { inspectProviderCache } from "../usage/cache.ts";
import { claudeAccounts } from "./accounts.ts";

export const staleAfter = 10 * 60_000;
const quotaDelay = 30_000;
const hold = 15 * 60_000;

export type Availability = {
  id: string;
  age?: number;
  known: boolean;
  capped: number[];
  weekly: number;
  headroom?: { label: string; left: number; reset: number };
};

const ids = Object.values(claudeAccounts).map(({ id }) => id);

async function availability(id: string, modelID: string, now = Date.now(), hourly = 100): Promise<Availability> {
  const { ageMS: age, windows: all, issue } = await inspectProviderCache(id, staleAfter, now);
  const scoped = modelID
    .split("-")
    .find((part) => part !== "claude")?.[0]
    ?.toUpperCase();
  const windows = all
    .filter(({ label }) => label === "H" || label === "W" || label === scoped)
    .map((window) => ({
      label: window.label,
      left:
        window.postReset || window.usedPercent === undefined
          ? undefined
          : (window.label === "H" ? hourly : 100) - window.usedPercent,
      reset: window.resetAt ? Date.parse(window.resetAt) : Infinity,
    }));
  const known = windows.filter((window) => window.left !== undefined);
  const weekly = known.find((window) => window.label === "W");
  const [tightest] = known.toSorted((a, b) => a.left! - b.left!);
  return {
    id,
    age,
    known: !issue && windows.length > 0 && known.length === windows.length,
    capped: known.filter((window) => window.left! <= 0).map((window) => window.reset),
    weekly: weekly?.reset ?? Infinity,
    headroom: tightest && { label: tightest.label, left: Math.max(0, tightest.left!), reset: tightest.reset },
  };
}

export function survey(modelID: string, now = Date.now(), hourly: Record<string, number> = {}) {
  return Promise.all(ids.map((id) => availability(id, modelID, now, hourly[id])));
}

export const reopens = (account: Availability) => Math.max(...account.capped);

export function ranked(list: Availability[], current?: string) {
  const trend = (account: Availability) => Number(account.id === claudeAccounts.trend.id);
  return list
    .filter((account) => !account.capped.length)
    .toSorted(
      (a, b) =>
        Number(b.id === current) - Number(a.id === current) ||
        Number(b.known) - Number(a.known) ||
        trend(b) - trend(a) ||
        a.weekly - b.weekly,
    );
}

export async function capped(response: Response, id: string, modelID: string, now = Date.now()) {
  if (response.status !== 429) return undefined;
  const future = (time: number | undefined) =>
    time !== undefined && Number.isFinite(time) && time > now ? time : undefined;
  const rejected = response.headers.get("anthropic-ratelimit-unified-status") === "rejected";
  const delay = Number.parseInt(response.headers.get("retry-after") ?? "", 10) * 1000;
  const usage = await availability(id, modelID, now);
  const cached = usage.capped.length ? reopens(usage) : undefined;
  if (!rejected && !(delay > quotaDelay) && cached === undefined) return undefined;
  return (
    future(Number(response.headers.get("anthropic-ratelimit-unified-reset")) * 1000) ??
    future(cached) ??
    now + (delay > quotaDelay ? delay : hold)
  );
}
