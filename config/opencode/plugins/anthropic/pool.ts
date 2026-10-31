import { inspectProviderCache } from "../usage/cache.ts";
import { claudeAccounts } from "./accounts.ts";

export const staleAfter = 10 * 60_000;
const quotaDelay = 30_000;
const hold = 15 * 60_000;

export type Availability = { id: string; age?: number; known: boolean; capped: number[]; weekly: number };

const ids = Object.values(claudeAccounts).map(({ id }) => id);

async function availability(id: string, modelID: string, now = Date.now()): Promise<Availability> {
  const { ageMS: age, windows: all, issue } = await inspectProviderCache(id, staleAfter, now);
  const scoped = modelID
    .split("-")
    .find((part) => part !== "claude")?.[0]
    ?.toUpperCase();
  const windows = all
    .filter(({ label }) => label === "H" || label === "W" || label === scoped)
    .map((window) => ({
      label: window.label,
      used: window.postReset ? undefined : window.usedPercent,
      reset: window.resetAt ? Date.parse(window.resetAt) : Infinity,
    }));
  const known = windows.filter((window) => window.used !== undefined);
  return {
    id,
    age,
    known: !issue && windows.length > 0 && known.length === windows.length,
    capped: known.filter((window) => window.used! >= 100).map((window) => window.reset),
    weekly: known.find((window) => window.label === "W")?.reset ?? Infinity,
  };
}

export function survey(modelID: string, now = Date.now()) {
  return Promise.all(ids.map((id) => availability(id, modelID, now)));
}

export const reopens = (account: Availability) => Math.max(...account.capped);

export function ranked(list: Availability[], current?: string) {
  return list
    .filter((account) => !account.capped.length)
    .toSorted(
      (a, b) =>
        Number(b.id === current) - Number(a.id === current) || Number(b.known) - Number(a.known) || a.weekly - b.weekly,
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
