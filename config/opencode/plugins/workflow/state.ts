import { Schema } from "effect";
import { ceiling } from "../compact/limits.ts";
import { ancestors, items, parse, topic, type Owner, type Step } from "./parse.ts";
import { occupant, phases, symbols, type Child } from "./progress.ts";

export const Mark = Schema.Literals(["done", "skipped", "failed"]);
export type Mark = typeof Mark.Type;

const Numbers = Schema.Array(Schema.Int);

export const Diff = Schema.Struct({
  from: Schema.Int,
  added: Numbers,
  moved: Schema.Array(Schema.Struct({ from: Schema.Int, to: Schema.Int })),
  changed: Schema.Array(Schema.Struct({ n: Schema.Int, mark: Schema.optionalKey(Mark) })),
  dropped: Schema.Array(Schema.Struct({ n: Schema.Int, headline: Schema.String, mark: Schema.optionalKey(Mark) })),
});
export type Diff = typeof Diff.Type;

export const Flow = Schema.Struct({
  rev: Schema.Int,
  source: Schema.String,
  status: Schema.Literals(["active", "paused"]),
  marks: Schema.Record(Schema.String, Mark),
  ticks: Schema.Record(Schema.String, Numbers),
  rounds: Schema.Record(Schema.String, Schema.Int),
  links: Schema.Record(Schema.String, Schema.Int),
  diff: Schema.optionalKey(Diff),
});
export type Flow = typeof Flow.Type;

export type Change = {
  readonly done?: readonly number[];
  readonly skip?: readonly number[];
  readonly fail?: readonly number[];
  readonly repair?: readonly number[];
  readonly check?: { readonly step: number; readonly items: readonly number[] };
};

function steps(flow: Flow) {
  return parse(flow.source).steps;
}

export function start(prior: Flow | undefined, source: string): Flow {
  const { steps: next, errors } = parse(source);
  if (errors.length)
    throw new Error(
      ["The workflow fence has errors:", ...errors.map(({ line, message }) => `line ${line}: ${message}`)].join("\n"),
    );
  if (!next.length) throw new Error("The workflow fence has no steps.");
  const fresh: Flow = {
    rev: (prior?.rev ?? 0) + 1,
    source,
    status: "active",
    marks: {},
    ticks: {},
    rounds: {},
    links: {},
  };
  if (!prior) return fresh;
  const old = steps(prior);
  const pairs = match(old, next);
  const renumber = new Map(pairs.map(({ from, to }) => [from.n, to.n]));
  const relink = new Map(pairs.filter(({ linked }) => linked).map(({ from, to }) => [from.n, to.n]));
  const marks: Record<string, Mark> = {};
  const ticks: Record<string, readonly number[]> = {};
  const rounds: Record<string, number> = {};
  for (const { from, to, keep } of pairs) {
    const mark = prior.marks[from.n];
    const ticked = prior.ticks[from.n];
    const used = prior.rounds[from.n];
    if (keep && mark) marks[to.n] = mark;
    if (keep && ticked) ticks[to.n] = ticked;
    if (keep && used) rounds[to.n] = used;
  }
  const links = Object.fromEntries(
    Object.entries(prior.links).flatMap(([child, n]) => (relink.has(n) ? [[child, relink.get(n)!]] : [])),
  );
  const diff: Diff = {
    from: prior.rev,
    added: next.filter((step) => !pairs.some((pair) => pair.to === step)).map(({ n }) => n),
    moved: pairs.filter(({ from, to }) => from.n !== to.n).map(({ from, to }) => ({ from: from.n, to: to.n })),
    changed: pairs
      .filter(({ changed }) => changed)
      .map(({ from, to, keep }) => marked({ n: to.n }, keep ? undefined : prior.marks[from.n])),
    dropped: old
      .filter((step) => !renumber.has(step.n))
      .map(({ n, headline }) => marked({ n, headline }, prior.marks[n])),
  };
  return { ...fresh, marks, ticks, rounds, links, diff };
}

export function update(flow: Flow, change: Change): Flow {
  const list = steps(flow);
  const known = new Map(list.map((step) => [step.n, step]));
  const { done = [], skip = [], fail = [], repair = [], check } = change;
  const named = [...done, ...skip, ...fail, ...repair, ...(check ? [check.step] : [])];
  const unknown = [...new Set(named.filter((n) => !known.has(n)))];
  if (unknown.length) throw new Error(`Rev ${flow.rev} has no step ${unknown.join(", ")}.`);
  const twice = [...new Set([...done, ...skip, ...fail])].filter(
    (n) => [done, skip, fail].filter((group) => group.includes(n)).length > 1,
  );
  if (twice.length) throw new Error(`Step ${twice.join(", ")} has more than one mark in this call.`);
  const marks: Record<string, Mark> = { ...flow.marks };
  const ticks: Record<string, readonly number[]> = { ...flow.ticks };
  const rounds: Record<string, number> = { ...flow.rounds };
  for (const [mark, group] of [
    ["done", done],
    ["skipped", skip],
    ["failed", fail],
  ] as const)
    for (const n of group) marks[n] = mark;
  const { gates, reopened } = reopen(list, { ...flow, marks }, repair);
  for (const gate of gates) rounds[gate] = (rounds[gate] ?? 0) + 1;
  for (const n of reopened) {
    delete marks[n];
    delete ticks[n];
  }
  if (check) ticks[check.step] = tick(known.get(check.step)!, ticks[check.step] ?? [], check.items);
  return { ...flow, marks, ticks, rounds };
}

export function link(flow: Flow, child: string, step: number): Flow {
  if (!steps(flow).some(({ n }) => n === step)) throw new Error(`Rev ${flow.rev} has no step ${step}.`);
  return { ...flow, links: { ...flow.links, [child]: step } };
}

export function unlink(flow: Flow, child: string): Flow {
  const { [child]: _, ...links } = flow.links;
  return { ...flow, links };
}

export function ready(flow: Flow, list: readonly Step[] = steps(flow)) {
  return list.filter((step) => !flow.marks[step.n] && waits(flow, step).length === 0).map(({ n }) => n);
}

export function status(flow: Flow, lanes: Readonly<Record<string, Child>> = {}) {
  const list = steps(flow);
  const open = ready(flow, list);
  const held = (id: string) => lanes[id]?.lane;
  const phased = phases(flow, list, open, held);
  const { title } = parse(flow.source);
  const head = `Workflow rev ${flow.rev}: ${title}${flow.status === "active" ? "" : ` [${flow.status}]`}`;
  const rows = list.map((step) => {
    const count = items(step).length;
    const phase = phased[step.n] ?? "waiting";
    const name = lanes[occupant(flow.links, step.n, phase, held) ?? ""]?.name;
    const state = phase === "waiting" ? `waits on ${waits(flow, step).join(", ")}` : name ? `${phase}: ${name}` : phase;
    const extra = [
      count ? `${flow.ticks[step.n]?.length ?? 0}/${count} items` : "",
      step.repair ? `repair ${flow.rounds[step.n] ?? 0}/${step.repair.max}` : "",
    ].filter(Boolean);
    return `${symbols[phase]} ${step.n} ${who(step.owner)}: ${step.headline} [${state}]${extra.map((text) => ` ${text}`).join("")}`;
  });
  const checklist = list
    .filter(({ n }) => open.includes(n))
    .flatMap((step) =>
      items(step).map(({ n, text }) => `  ${step.n}.${n} [${flow.ticks[step.n]?.includes(n) ? "x" : " "}] ${text}`),
    );
  const launch = open.filter((n) => phased[n] === "ready");
  return [head, ...rows, `Ready: ${launch.join(", ") || "none"}`, ...checklist].join("\n");
}

const perStep = 18_000;

function thousands(tokens: number) {
  return `${Math.round(tokens / 1000)}K`;
}

export function headroom(flow: Flow, tokens: number | undefined) {
  const open = steps(flow).filter(({ n }) => !settled(flow, n)).length;
  const cost = open * perStep;
  const estimate = `${open} unfinished step${open === 1 ? "" : "s"} × ~${thousands(perStep)} ≈ ${thousands(cost)} estimated growth`;
  if (tokens === undefined)
    return `Headroom: root context not measured since the last compaction; forced compaction at ${thousands(ceiling)}; ${estimate}.`;
  const room = Math.max(ceiling - tokens, 0);
  return [
    `Headroom: root context ${thousands(tokens)}; ${thousands(room)} until forced compaction at ${thousands(ceiling)}; ${estimate}.`,
    cost > room &&
      "Warning: estimated growth exceeds headroom; assess the actual work and end the workflow at an earlier boundary if needed.",
  ]
    .filter(Boolean)
    .join(" ");
}

export function entries({ added, moved, changed, dropped }: Diff) {
  return [
    { label: "added", text: added.join(", ") },
    { label: "renumbered", text: moved.map(({ from, to }) => `${from}→${to}`).join(", ") },
    { label: "changed", text: changed.map(({ n, mark }) => `${n}${mark ? ` (was ${mark}, reset)` : ""}`).join(", ") },
    ...dropped.map(({ n, mark, headline }) => ({
      label: "dropped",
      text: `${n}${mark ? ` (was ${mark})` : ""} ${headline}`,
    })),
  ].filter(({ text }) => text);
}

export function describe(diff: Diff) {
  const lines = entries(diff);
  const remark = diff.changed.filter(({ mark }) => mark).map(({ n }) => n);
  return [
    `Revised from rev ${diff.from}${lines.length ? ":" : " with no step changes."}`,
    ...lines.map(({ label, text }) => `  ${label}: ${text}`),
    ...(remark.length ? [`Re-mark ${remark.join(", ")} if the earlier result still holds.`] : []),
  ].join("\n");
}

function who(owner: Owner) {
  if (owner.kind !== "lane") return owner.kind;
  return [`${owner.role}/${owner.scope}`, owner.lane].filter(Boolean).join(" ");
}

function identity({ owner }: Step) {
  return owner.kind === "lane" ? JSON.stringify([owner.kind, owner.role, owner.scope]) : owner.kind;
}

function lane({ owner }: Step) {
  return owner.kind === "lane" ? owner.lane : undefined;
}

function phrases({ headline }: Step) {
  const title = topic(headline);
  const parts = title === headline ? [headline] : [headline, title, headline.slice(title.length + 2)];
  return parts.map((part) => part.trim().toLowerCase()).filter(Boolean);
}

function similar(a: Step, b: Step) {
  const mine = phrases(a);
  return phrases(b).some((phrase) => mine.includes(phrase));
}

function kin(from: Step, to: Step) {
  const [a, b] = [lane(from), lane(to)];
  return identity(from) === identity(to) && (a === undefined || b === undefined || a === b);
}

function match(old: readonly Step[], next: readonly Step[]) {
  const pool = new Set(old);
  const found = new Map<Step, Step>();
  const pass = (fits: (from: Step, to: Step) => boolean, sole = true) => {
    for (const to of next) {
      if (found.has(to)) continue;
      const fit = [...pool].filter((step) => fits(step, to));
      const from = fit[0];
      if (!from) continue;
      if (sole && (fit.length > 1 || next.filter((step) => !found.has(step) && fits(from, step)).length > 1)) continue;
      pool.delete(from);
      found.set(to, from);
    }
  };
  const lanes = (from: Step, to: Step) => lane(from) !== undefined && lane(from) === lane(to) && kin(from, to);
  pass((from, to) => kin(from, to) && from.headline === to.headline, false);
  pass((from, to) => lanes(from, to) && similar(from, to));
  pass(lanes);
  pass((from, to) => kin(from, to) && similar(from, to));
  pass(kin);
  const upstream = { old: ancestors(old), next: ancestors(next) };
  const numbered = new Map(next.map((step) => [step.n, step]));
  const decided = new Map<Step, boolean>();
  const kept = (to: Step): boolean => {
    const known = decided.get(to);
    if (known !== undefined) return known;
    const from = found.get(to);
    const keep =
      !!from &&
      similar(from, to) &&
      JSON.stringify(from.details) === JSON.stringify(to.details) &&
      (to.owner.kind !== "gate" || same(from, to));
    decided.set(to, keep);
    return keep;
  };
  const same = (from: Step, to: Step) => {
    const mine = judged(from, upstream.old);
    const theirs = judged(to, upstream.next).map((n) => numbered.get(n)!);
    return mine.length === theirs.length && theirs.every((step) => kept(step) && mine.includes(found.get(step)!.n));
  };
  return next.flatMap((to) => {
    const from = found.get(to);
    if (!from) return [];
    const keep = kept(to);
    const linked = similar(from, to) && JSON.stringify(items(from)) === JSON.stringify(items(to));
    return [{ from, to, keep, linked, changed: !keep || from.headline !== to.headline }];
  });
}

function judged({ n, after, repair }: Step, upstream: ReadonlyMap<number, ReadonlySet<number>>) {
  const loop = [...(upstream.get(n) ?? [])].filter((step) => looped(upstream, repair?.to ?? [], step));
  return [...new Set([...after, ...loop])];
}

function looped(upstream: ReadonlyMap<number, ReadonlySet<number>>, targets: readonly number[], n: number) {
  return targets.includes(n) || targets.some((target) => upstream.get(n)?.has(target));
}

function marked<T extends object>(value: T, mark: Mark | undefined) {
  return mark ? { ...value, mark } : value;
}

export function settled(flow: Flow, n: number) {
  return flow.marks[n] === "done" || flow.marks[n] === "skipped";
}

export function waits(flow: Flow, step: Step) {
  return step.after.filter((n) => !settled(flow, n));
}

function reopen(list: readonly Step[], { marks, rounds }: Flow, targets: readonly number[]) {
  if (!targets.length) return { gates: [], reopened: [] };
  const gates = new Map<Step, number[]>();
  for (const n of targets) {
    const listing = list.filter((step) => step.repair?.to.includes(n));
    const gate = listing.find((step) => marks[step.n] === "failed") ?? listing[0];
    if (!gate) throw new Error(`Step ${n} is not a repair target of any gate.`);
    gates.set(gate, [...(gates.get(gate) ?? []), n]);
  }
  for (const gate of gates.keys()) {
    const used = rounds[gate.n] ?? 0;
    const max = gate.repair!.max;
    if (used >= max)
      throw new Error(
        `Gate ${gate.n} has used ${used}/${max} repair rounds. Stop repairing and bring the premise back to the user.`,
      );
  }
  const upstream = ancestors(list);
  const reopened = list.filter(({ n }) => looped(upstream, targets, n)).map(({ n }) => n);
  return { gates: [...gates.keys()].map(({ n }) => n), reopened };
}

function tick(step: Step, ticked: readonly number[], picked: readonly number[]) {
  const count = items(step).length;
  const bad = picked.filter((index) => !Number.isInteger(index) || index < 1 || index > count);
  if (bad.length)
    throw new Error(
      `Step ${step.n} has ${count ? `items 1-${count}` : "no checklist items"}; ${bad.join(", ")} ${bad.length > 1 ? "are" : "is"} out of range.`,
    );
  return [...new Set([...ticked, ...picked])].toSorted((a, b) => a - b);
}
