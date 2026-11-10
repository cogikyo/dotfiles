import { claudeAccounts } from "../anthropic/accounts.ts";
import { ranked, reopens, staleAfter, survey } from "../anthropic/pool.ts";
import { inspectProviderCache } from "../usage/cache.ts";
import { usageProviders } from "../usage/providers.ts";
import { format, type Ref } from "./routing.ts";

export const pool = "claude";

type Step = { ref: Ref; spent?: string; notes: string[] };

const own = /^(?:composer|grok)-|^(?:auto|default)$/u;

const accounts = new Set<string>(Object.values(claudeAccounts).map(({ id }) => id));

const key = (ref: Ref) => `${accounts.has(ref.providerID) ? pool : ref.providerID}/${ref.id}`;

const minutes = (ms: number) => `${Math.round(ms / 60_000)}m old`;

async function claude(route: Ref, current: Ref | undefined): Promise<Step> {
  const list = await survey(route.id);
  const open = ranked(list, current?.providerID);
  const stale = list
    .filter((account) => account.age !== undefined && account.age > staleAfter)
    .map((account) => `${account.id} ${minutes(account.age!)}`);
  const account = open[0] ?? list.toSorted((a, b) => reopens(a) - reopens(b))[0];
  return {
    ref: { ...route, providerID: account.id },
    spent: open.length ? undefined : `all Claude accounts are capped for ${route.id}`,
    notes: stale.length ? [`lanes: Claude usage is stale (${stale.join(", ")}); picked by last known numbers.`] : [],
  };
}

async function cursor(route: Ref): Promise<Step> {
  const window = own.test(route.id) ? "C" : "O";
  const { id, staleAfterMS } = usageProviders.cursor;
  const { issue, ageMS, windows } = await inspectProviderCache(id, staleAfterMS);
  const open = (why: string) => ({
    ref: route,
    notes: [`lanes: Cursor usage ${why}; treating the ${window} pool as open.`],
  });
  if (ageMS !== undefined && ageMS > staleAfterMS) return open(`is stale (${minutes(ageMS)})`);
  if (issue && !(issue === "error" && ageMS !== undefined && windows.length)) return open(`is unusable (${issue})`);
  const found = windows.find((item) => item.label === window);
  if (!found || found.usedPercent === undefined) return open(`has no ${window} window`);
  if (found.postReset || found.usedPercent < 100) return { ref: route, notes: [] };
  return { ref: route, spent: `the Cursor ${window} pool is spent for ${route.id}`, notes: [] };
}

const check = (ref: Ref, current: Ref | undefined): Promise<Step> | Step =>
  ref.providerID === pool ? claude(ref, current) : ref.providerID === "cursor" ? cursor(ref) : { ref, notes: [] };

function chain(route: Ref, fallbacks: Record<string, Ref>) {
  const refs = [route];
  const seen = new Set([key(route)]);
  for (let next = fallbacks[key(route)]; next && !seen.has(key(next)); next = fallbacks[key(next)]) {
    seen.add(key(next));
    refs.push({ providerID: next.providerID, id: next.id });
  }
  return refs;
}

export async function pick(route: Ref, current: Ref | undefined, fallbacks: Record<string, Ref>) {
  const refs = chain(route, fallbacks);
  const notes: string[] = [];
  const walk = async (at: number): Promise<Ref> => {
    const step = await check(refs[at], current);
    notes.push(...step.notes);
    if (!step.spent) return step.ref;
    const next = refs[at + 1];
    if (!next) {
      const why = fallbacks[key(refs[at])] ? "the fallbacks loop" : "no fallback remains";
      notes.push(`lanes: ${step.spent}; ${why}, so using ${format(step.ref)}.`);
      return step.ref;
    }
    notes.push(`lanes: ${step.spent}; using ${format(next)} with no effort.`);
    return walk(at + 1);
  };
  const ref = await walk(0);
  return { ref, note: [...new Set(notes)].join("\n") || undefined };
}

export async function resume(route: Ref, current: Ref | undefined, fallbacks: Record<string, Ref>) {
  const refs = chain(route, fallbacks);
  const at = current ? refs.findIndex((ref) => key(ref) === key(current)) : -1;
  if (at > 0 && refs[at].id === route.id) {
    const step = await check(refs[at], current);
    if (!step.spent) return { ref: step.ref, note: step.notes.join("\n") || undefined };
  }
  return pick(route, current, fallbacks);
}
