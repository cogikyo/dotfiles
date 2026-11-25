import path from "node:path";
import { claudeAccounts } from "../anthropic/accounts.ts";
import { ranked, reopens, staleAfter, survey } from "../anthropic/pool.ts";
import { Model } from "@opencode/schema/model";
import { missing } from "../sessions/model.ts";
import { inspectProviderCache, type ProviderCacheView } from "../usage/cache.ts";
import { usageProviders, type UsageProviderSpec } from "../usage/providers.ts";
import { formatProximity } from "../usage/status.ts";
import type { Card } from "./step.ts";

export type Ref = { providerID: string; id: string; variant?: string };

export type Resolved = { model?: string; note?: string; anchor?: Ref };

export type Launch = { model?: string; card?: Card; resume?: { current?: Ref; stored?: Ref } };

type Listed = Parameters<typeof missing>[0][number] & {
  readonly family?: string;
  readonly status: string;
  readonly time: { readonly released: number };
};

type Room = { label: string; left: number; reset?: number };

type Reading = { ref: Ref; spent?: string; room?: Room; unsure?: string; notes: string[] };

type Chosen = { model: string; note?: string };

export const routing = path.join(import.meta.dirname, "..", "..", "ROUTING.md");

export const format = (ref: Ref) => `${ref.providerID}/${ref.id}${ref.variant ? `#${ref.variant}` : ""}`;

const pool = "claude";

const accounts = new Set<string>(Object.values(claudeAccounts).map(({ id }) => id));

const pooled = (providerID: string) => (accounts.has(providerID) ? pool : providerID);

const same = (a: Ref, b: Ref) =>
  a.providerID === b.providerID && a.id === b.id && (a.variant ?? "default") === (b.variant ?? "default");

const trendHourly = 90;

const own = /^(?:composer|grok)-|^(?:auto|default)$/u;

const pointer = `${routing} says which model, effort, and seat count fit the task`;

const age = (ms: number) => `${Math.round(ms / 60_000)}m old`;

const when = (reset: number | undefined, now = Date.now()) =>
  reset === undefined || !Number.isFinite(reset) ? undefined : formatProximity(reset - now);

const resetting = (reset: number | undefined, now?: number) => {
  const at = when(reset, now);
  return at ? `, resetting ${at}` : "";
};

const describe = (room: Room, now?: number) =>
  `${room.label} ${Math.round(room.left)}% left${resetting(room.reset, now)}`;

function doubt({ issue, ageMS, windows }: ProviderCacheView, staleAfterMS: number) {
  if (ageMS !== undefined && ageMS > staleAfterMS) return `is stale (${age(ageMS)})`;
  if (issue && !(issue === "error" && ageMS !== undefined && windows.length)) return `is unusable (${issue})`;
}

async function claude(ref: Ref, current?: Ref): Promise<Reading> {
  const now = Date.now();
  const list = await survey(ref.id, now, { [claudeAccounts.trend.id]: trendHourly });
  const open = ranked(list, current?.providerID);
  const stale = list
    .filter((account) => account.age !== undefined && account.age > staleAfter)
    .map((account) => `${account.id} ${age(account.age!)}`);
  const notes = stale.length
    ? [`lanes: Claude usage is stale (${stale.join(", ")}); picked by last known numbers.`]
    : [];
  const [account] = open;
  if (!account) {
    const first = list.toSorted((a, b) => reopens(a) - reopens(b))[0];
    const at = when(reopens(first), now);
    return {
      ref: { ...ref, providerID: first.id },
      spent: `every Claude account is capped for ${ref.id}${at ? `; the first reopens ${at}` : ""}`,
      notes,
    };
  }
  const room = account.headroom && { ...account.headroom, label: `${account.id} ${account.headroom.label}` };
  const why = doubt(await inspectProviderCache(account.id, staleAfter), staleAfter);
  return { ref: { ...ref, providerID: account.id }, room, unsure: why && `${account.id} usage ${why}`, notes };
}

async function windowed(
  spec: UsageProviderSpec,
  ref: Ref,
  required: readonly string[],
  only?: string,
): Promise<Reading> {
  const open = (why: string): Reading => ({
    ref,
    notes: [`lanes: ${spec.label} usage ${why}; treating ${only ? `the ${only} pool` : "it"} as open.`],
  });
  const view = await inspectProviderCache(spec.id, spec.staleAfterMS);
  const why = doubt(view, spec.staleAfterMS);
  if (why) return { ...open(why), unsure: `${spec.label} usage ${why}` };
  const { windows } = view;
  const [tightest] = windows
    .filter((window) => (!only || window.label === only) && !window.postReset && window.usedPercent !== undefined)
    .map((window) => ({
      label: `${spec.label} ${window.label}`,
      left: 100 - window.usedPercent!,
      reset: window.resetAt ? Date.parse(window.resetAt) : undefined,
    }))
    .toSorted((a, b) => a.left - b.left);
  if (tightest && tightest.left <= 0)
    return {
      ref,
      spent: `the ${tightest.label} ${only ? "pool" : "window"} is spent for ${ref.id}${resetting(tightest.reset)}`,
      notes: [],
    };
  const gaps = required.filter((tag) => windows.find((window) => window.label === tag)?.usedPercent === undefined);
  if (gaps.length) return { ...open(`has no ${gaps.join("/")} window`), room: tightest };
  return { ref, room: tightest, notes: [] };
}

function cursor(ref: Ref) {
  const window = own.test(ref.id) ? "C" : "O";
  return windowed(usageProviders.cursor, ref, [window], window);
}

const readers: Partial<Record<string, (ref: Ref, current?: Ref) => Promise<Reading>>> = {
  [pool]: claude,
  [usageProviders.cursor.id]: cursor,
  [usageProviders.gemini.id]: (ref) => windowed(usageProviders.gemini, ref, ["H", "W"]),
  [usageProviders.xai.id]: (ref) => windowed(usageProviders.xai, ref, ["W"]),
  [usageProviders.openai.id]: (ref) => windowed(usageProviders.openai, ref, []),
  [usageProviders.opencodeGo.id]: (ref) => windowed(usageProviders.opencodeGo, ref, []),
};

function chosen({ ref, room, notes }: Reading, lead: readonly string[] = []): Chosen {
  const lines = room ? [...lead, ...notes, `lanes: ${describe(room)}.`] : [...lead, ...notes];
  return { model: format(ref), note: lines.join("\n") || undefined };
}

function backup(ref: Ref, absent: (ref: Ref) => string | undefined) {
  if (ref.providerID !== pool) return undefined;
  const spare = { ...ref, providerID: usageProviders.cursor.id };
  if (!absent(spare)) return spare;
  const bare = { providerID: spare.providerID, id: ref.id };
  return absent(bare) ? undefined : bare;
}

async function options(models: readonly Listed[], skip: readonly Ref[]) {
  const now = Date.now();
  const refs = new Map<string, Ref>();
  const families = new Set<string>();
  for (const { providerID, id, family, status } of models.toSorted((a, b) => b.time.released - a.time.released)) {
    const ref = { providerID: pooled(providerID), id };
    const kin = family && `${ref.providerID}/${family}`;
    if (status === "deprecated" || (kin && families.has(kin) && !refs.has(format(ref)))) continue;
    if (kin) families.add(kin);
    if (!skip.some((taken) => taken.providerID === ref.providerID && taken.id === ref.id)) refs.set(format(ref), ref);
  }
  const read = await Promise.all(
    [...refs.values()].map(async (ref) => ({ ref, reading: await readers[ref.providerID]?.(ref) })),
  );
  const open = read
    .filter(({ reading }) => !reading?.spent)
    .toSorted((a, b) => (b.reading?.room?.left ?? -1) - (a.reading?.room?.left ?? -1));
  const groups = Map.groupBy(open, ({ reading }) =>
    !reading
      ? "usage untracked"
      : reading.unsure
        ? reading.unsure
        : reading.room
          ? describe(reading.room, now)
          : "usage unknown",
  );
  return [...groups].map(([text, items]) => `- ${items.map(({ ref }) => format(ref)).join(", ")}: ${text}`);
}

async function choose(
  ref: Ref,
  current: Ref | undefined,
  absent: (ref: Ref) => string | undefined,
  models: readonly Listed[],
): Promise<Chosen | { error: string }> {
  const read = readers[ref.providerID];
  if (!read) return { model: format(ref) };
  const first = await read(ref, current);
  if (!first.spent) return chosen(first);
  const spare = backup(ref, absent);
  const second = spare && (await cursor(spare));
  if (second && !second.spent)
    return chosen(second, [...first.notes, `lanes: ${first.spent}; using ${format(spare)}.`]);
  const open = await options(models, spare ? [ref, spare] : [ref]);
  return {
    error: [
      `lanes: ${format(ref)} is capped: ${first.spent}${second?.spent ? `, and ${second.spent}` : ""}.`,
      `Pass another \`model\`; ${pointer}.`,
      ...(open.length ? ["Pools with headroom:", ...open] : ["No other pool has headroom."]),
    ].join("\n"),
  };
}

async function resumed(
  { current, stored }: NonNullable<Launch["resume"]>,
  absent: (ref: Ref) => string | undefined,
  models: readonly Listed[],
): Promise<Resolved | { error: string }> {
  if (!current) return { error: `lanes: pass \`model\` as \`provider/model#variant\`; ${pointer}.` };
  const target = stored ?? current;
  const lost = absent(target);
  if (lost)
    return {
      error: `lanes: this lane's launch model ${format(target)} is gone (${lost}); pass \`model\`. ${pointer}.`,
    };
  const gone = absent(current);
  const spare = gone ? undefined : backup(target, absent);
  if (spare && same(spare, current)) {
    const kept = await cursor(current);
    if (!kept.spent) return { ...chosen(kept), model: undefined };
  }
  const picked = await choose(target, current, absent, models);
  if ("error" in picked) return picked;
  const moved = gone || !same(Model.Ref.parse(picked.model), current);
  return {
    model: moved ? picked.model : undefined,
    note:
      [picked.note, gone && `lanes: ${format(current)} is gone (${gone}); using ${picked.model}.`]
        .filter(Boolean)
        .join("\n") || undefined,
  };
}

export async function pick(
  { model, card, resume }: Launch,
  models: readonly Listed[],
): Promise<Resolved | { error: string }> {
  const view = models.map(({ providerID, id, variants }) => ({ providerID: pooled(providerID), id, variants }));
  const absent = (ref: Ref) => missing(ref.providerID === pool ? view : models, ref);
  const asked = model ?? card?.token;
  if (asked) {
    const source = model ? "lanes: `model`" : `Step ${card!.step}'s card model`;
    let ref: Ref;
    try {
      ref = Model.Ref.parse(asked);
    } catch {
      return { error: `${source} \`${asked}\` is not \`provider/model#variant\`; ${pointer}.` };
    }
    const why = absent(ref);
    if (why)
      return {
        error: `${source} \`${asked}\` is unusable: ${why}. ${model ? `${pointer}.` : "Pass model, or fix the card."}`,
      };
    const picked = await choose(ref, resume?.current, absent, models);
    return "error" in picked ? picked : { ...picked, anchor: ref };
  }
  return resumed(resume ?? {}, absent, models);
}
