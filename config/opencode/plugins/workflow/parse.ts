const roles = ["build", "review", "scout", "verify"] as const;
const plain = ["self", "gate", "you"] as const;
const texts = ["done", "if", "why"] as const;

export type Role = (typeof roles)[number];

export type Owner =
  | { kind: (typeof plain)[number] }
  | { kind: "lane"; role?: Role; scope: string; lane?: string; model?: string; variant?: string };

export type Detail = { kind: "item" | (typeof texts)[number]; text: string };

export type Step = {
  n: number;
  line: number;
  owner: Owner;
  headline: string;
  details: Detail[];
  after: number[];
  repair?: { to: number[]; max: number };
};

export type Issue = { line: number; message: string };

export type Workflow = { title: string; summary?: string; steps: Step[]; errors: Issue[] };

type Lane = Extract<Owner, { kind: "lane" }>;

type Draft = Omit<Step, "after" | "repair"> & {
  declared?: { steps: number[]; line: number };
  repair?: { to: number[]; max: number; line: number };
};

type Reader = { flow: Workflow; drafts: Draft[]; current?: Draft; dropped: boolean };

type Fail = (message: string) => void;

type Log = { errors: Issue[]; partial: boolean };

export function parse(source: string, partial = false): Workflow {
  const lines = source.split("\n");
  const flow = read(lines, partial);
  if (!partial) return flow;
  const last = lines.findLastIndex((text) => text.trim() !== "");
  const settled = read(lines.slice(0, last), partial);
  const tentative = flow.summary !== settled.summary || flow.errors.some(({ line }) => line === last + 1);
  return tentative ? settled : flow;
}

export function conditional(step: Step) {
  return step.details.some(({ kind }) => kind === "if");
}

export function ancestors(steps: readonly Pick<Step, "n" | "after">[]) {
  const deps = new Map(steps.map(({ n, after }) => [n, after]));
  return new Map(
    steps.map(({ n }) => {
      const found = new Set<number>();
      const queue = [...(deps.get(n) ?? [])];
      for (let next = queue.pop(); next !== undefined; next = queue.pop()) {
        if (found.has(next)) continue;
        found.add(next);
        queue.push(...(deps.get(next) ?? []));
      }
      return [n, found];
    }),
  );
}

function read(lines: readonly string[], partial: boolean): Workflow {
  const reader: Reader = { flow: { title: "", steps: [], errors: [] }, drafts: [], dropped: false };
  const { flow } = reader;
  for (const [index, raw] of lines.entries()) {
    const text = raw.trimEnd();
    const line = index + 1;
    const fail = (message: string) => flow.errors.push({ line, message });
    if (!text.trim()) continue;
    if (/^\s/.test(text)) indented(reader, text.trim(), line, fail);
    else if (!flow.title) flow.title = text;
    else unindented(reader, text, line, fail);
  }
  flow.steps = link(reader.drafts, { errors: flow.errors, partial });
  flow.errors.sort((a, b) => a.line - b.line);
  return flow;
}

function indented(reader: Reader, body: string, line: number, fail: Fail) {
  if (reader.current) detail(reader.current, body, line, fail);
  else if (!reader.dropped) fail("detail line before any step");
}

function unindented(reader: Reader, text: string, line: number, fail: Fail) {
  const { flow, drafts } = reader;
  const match = /^(\d+) (.+?):(?: (.*)|$)/.exec(text);
  if (!match && flow.summary === undefined && !drafts.length) {
    flow.summary = text;
    return;
  }
  const step = match ? head(match, line) : "expected `N owner: headline`";
  if (typeof step === "string") {
    fail(step);
    reader.current = undefined;
    reader.dropped = true;
    return;
  }
  if (step.issue) fail(step.issue);
  reader.dropped = drafts.some(({ n }) => n === step.draft.n);
  if (reader.dropped) fail(`duplicate step ${step.draft.n}`);
  reader.current = reader.dropped ? undefined : step.draft;
  if (reader.current) drafts.push(reader.current);
}

function head([, number, agent, headline = ""]: RegExpExecArray, line: number) {
  const n = Number(number);
  if (!Number.isSafeInteger(n)) return `step number ${number} is too large`;
  const { owner, issue } = parseOwner(agent.trim());
  const draft: Draft = { n, line, owner, headline: headline.trim(), details: [] };
  return { draft, issue: n < 1 ? "step numbers start at 1" : issue };
}

function parseOwner(text: string): { owner: Owner; issue?: string } {
  const kind = plain.find((name) => name === text);
  if (kind) return { owner: { kind } };
  const [agent = "", ...words] = text.split(/\s+/);
  const slash = agent.indexOf("/");
  const name = agent.slice(0, Math.max(0, slash));
  const owner: Lane = { kind: "lane", role: roles.find((role) => role === name), scope: agent.slice(slash + 1) };
  if (slash === -1 || !owner.scope) return { owner, issue: `unknown owner \`${text}\`` };
  if (!owner.role) return { owner, issue: `unknown role \`${name}\`` };
  if (words.length > 2) return { owner, issue: "owner takes `role/scope [lane] provider/model[#variant]`" };
  return { owner, issue: seat(owner, words) };
}

function seat(owner: Lane, words: readonly string[]) {
  const [lane, token] = words.length === 2 || !words[0]?.includes("/") ? words : [undefined, words[0]];
  if (lane?.includes("/")) return `lane \`${lane}\` must be a bare word`;
  owner.lane = lane;
  if (!token) return "delegated step needs a model";
  if (!token.includes("/")) return `model \`${token}\` must be provider/model`;
  const [model, variant] = token.split("#", 2);
  owner.model = model;
  owner.variant = variant || undefined;
}

function detail(draft: Draft, body: string, line: number, fail: Fail) {
  if (body.startsWith("- ") || body === "-") {
    const text = body.slice(1).trim();
    if (text) draft.details.push({ kind: "item", text });
    else fail("empty checklist item");
    return;
  }
  const match = /^([a-z]+):(?:\s+(.*))?$/i.exec(body);
  if (!match) return fail("expected `- item` or `key: value`");
  const [, key, value = ""] = match;
  const kind = texts.find((name) => name === key);
  if (!value.trim()) fail(`\`${key}\` needs a value`);
  else if (kind) draft.details.push({ kind, text: value.trim() });
  else if (key === "after") depend(draft, value.trim(), line, fail);
  else if (key === "repair") repair(draft, value.trim(), line, fail);
  else fail(`unknown key \`${key}\``);
}

function depend(draft: Draft, value: string, line: number, fail: Fail) {
  if (draft.declared) return fail("repeated `after`");
  const words = value === "none" ? [] : value.split(/[\s,]+/);
  if (!words.every((word) => /^\d+$/.test(word))) return fail("`after` takes step numbers or `none`");
  const large = words.find((word) => !Number.isSafeInteger(Number(word)));
  if (large) return fail(`step number ${large} is too large`);
  draft.declared = { steps: words.map(Number), line };
}

function repair(draft: Draft, value: string, line: number, fail: Fail) {
  if (draft.repair) return fail("repeated `repair`");
  if (draft.owner.kind !== "gate") return fail("`repair` belongs on a gate step");
  const match = /^(\d+(?:\s+\d+)*)\s+max\s+(\d+)$/.exec(value);
  if (!match) return fail("`repair` takes `N [N ...] max K`");
  const max = Number(match[2]);
  const to = match[1].split(/\s+/).map(Number);
  if (![max, ...to].every(Number.isSafeInteger)) return fail("`repair` numbers are too large");
  if (max < 1) return fail("`repair` needs `max` of at least 1");
  draft.repair = { to: [...new Set(to)], max, line };
}

function link(drafts: readonly Draft[], log: Log): Step[] {
  const known = new Set(drafts.map(({ n }) => n));
  const steps = drafts.map((draft, index): Step => {
    const { n, line, owner, headline, details } = draft;
    return { n, line, owner, headline, details, after: resolve(draft, drafts[index - 1], known, log) };
  });
  const upstream = ancestors(steps);
  const looped = steps.filter(({ n }) => upstream.get(n)?.has(n));
  if (looped[0]) {
    const names = looped.map(({ n }) => n).join(", ");
    defer(log, { line: looped[0].line, message: `steps ${names} wait on each other` });
  }
  for (const [index, { n, repair: loop }] of drafts.entries()) {
    if (!loop) continue;
    const to = loop.to.filter((target) => {
      const message = !known.has(target)
        ? `unknown step ${target}`
        : !upstream.get(n)?.has(target)
          ? `repair target ${target} is not upstream of step ${n}`
          : undefined;
      if (message) defer(log, { line: loop.line, message });
      return !message;
    });
    if (to.length) steps[index].repair = { to, max: loop.max };
  }
  return steps;
}

function resolve(draft: Draft, previous: Draft | undefined, known: ReadonlySet<number>, log: Log) {
  if (!draft.declared) return previous ? [previous.n] : [];
  const { steps, line } = draft.declared;
  const valid = steps.filter((n) => {
    if (n === draft.n) log.errors.push({ line, message: `step ${n} cannot wait on itself` });
    else if (!known.has(n)) defer(log, { line, message: `unknown step ${n}` });
    return n !== draft.n && known.has(n);
  });
  return [...new Set(valid)];
}

function defer({ errors, partial }: Log, issue: Issue) {
  if (!partial) errors.push(issue);
}
