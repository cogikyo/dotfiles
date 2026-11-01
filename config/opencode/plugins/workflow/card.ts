import { clip } from "../byline/notice.ts";
import { family } from "../lanes/label.ts";
import { routeFor, type Ref } from "../lanes/routing.ts";
import { draw, type Graph } from "./graph.ts";
import { topic, type Detail, type Owner, type Step, type Workflow } from "./parse.ts";
import { complete, symbols, type Progress } from "./progress.ts";
import { entries, type Diff } from "./state.ts";
import { icon, ink, pill, shades, type Row, type Segment } from "./style.ts";

export type Routes = Record<string, Ref>;

type Scene = {
  steps: readonly Step[];
  thinking: ReadonlyMap<number, string>;
  digits: number;
  names: number;
  graph?: Graph;
  progress?: Progress;
};

type Section = { lead: Row; badge: Row; title: Row; right: Row; notes: Row[] };

const brain = "\u{f09d1}";
const box: Segment = { text: "☐ ", tone: "muted" };
const watch: Segment = { text: "◎ ", tone: "orange" };
const checked: Segment = { text: "☑ ", tone: "success" };
const seen: Segment = { text: "◉ ", tone: "success" };
const gap = 2;

export function card(flow: Workflow, width: number, routes?: Routes, progress?: Progress): Row[] {
  const head: Row[] = flow.errors.map(({ line, message }) => [{ text: `✗ line ${line}: ${message}`, tone: "error" }]);
  const title = headline(flow.title, { tone: "accent", bold: true });
  if (flow.title) head.push(progress ? heading(title, flow.steps, progress, width) : title);
  for (const text of wrap(flow.summary ?? "", width)) head.push([{ text, tone: "text" }]);
  if (progress?.diff) head.push(...(head.length ? [[]] : []), ...revision(progress.diff));
  const body = plan(flow.steps, width, routes, progress);
  const rows = head.length && body.length ? [...head, [], ...body] : [...head, ...body];
  return rows.map((row) => fit(row, width));
}

function heading(title: Row, steps: readonly Step[], progress: Progress, width: number): Row {
  const { rev, status, phases: states } = progress;
  const right: Row = [
    { text: `rev ${rev} · ${complete(states)}/${steps.length}`, tone: "muted" },
    ...(status === "paused" ? [{ text: " · paused", tone: "yellow" } as const] : []),
  ];
  if (span(right) + gap >= width) return title;
  return opening({ lead: [], badge: [], title, right }, width);
}

function revision(diff: Diff): Row[] {
  const rows: Row[] = entries(diff).map(({ label, text }) => [
    { text: `  ${label} `, tone: "muted" },
    { text, tone: "text" },
  ]);
  return [
    [
      { text: "↻ ", tone: "orange" },
      { text: `revised from rev ${diff.from}${rows.length ? "" : " with no step changes"}`, tone: "muted" },
    ],
    ...rows,
  ];
}

function plan(list: readonly Step[], width: number, routes: Routes | undefined, progress?: Progress): Row[] {
  if (!list.length) return [];
  const graph = draw(list, width, progress);
  const thinking = new Map<number, string>();
  for (const { n, owner } of list) {
    const word = effort(owner, routes);
    if (word) thinking.set(n, word);
  }
  const digits = Math.max(...list.map(({ n }) => String(n).length));
  const names = Math.max(...list.map(({ owner }) => Bun.stringWidth(scope(owner))));
  const scene: Scene = {
    steps: list,
    thinking,
    digits,
    names: names && digits + names + 5 < width ? names : 0,
    graph,
    progress,
  };
  const sections = list.map((step, index) => section(step, index, scene));
  const wide = sections.every(
    ({ lead, badge, right }) => !right.length || span(lead) + span(badge) < width - gap - span(right),
  );
  const rows = graph ? [...center(graph.rows, width), []] : [];
  for (const [index, { notes, ...parts }] of sections.entries()) {
    if (index) rows.push([]);
    rows.push(opening(wide ? parts : { ...parts, right: [] }, width), ...notes);
  }
  return rows;
}

function scope(owner: Owner) {
  return owner.kind === "lane" ? owner.scope : "";
}

function model(owner: Owner) {
  return owner.kind === "lane" && owner.model ? family(owner.model) : "";
}

function effort(owner: Owner, routes: Routes | undefined) {
  if (owner.kind !== "lane" || !owner.role || !owner.model || !routes) return;
  const route = routeFor(routes, `${owner.role}/${owner.scope}`);
  if (route && route.variant !== owner.variant) return owner.variant ?? "default";
}

function section(step: Step, index: number, scene: Scene): Section {
  const { tone, ...weight } = ink(step.owner);
  const { progress } = scene;
  const phase = progress && progress.phases[step.n];
  const name = scope(step.owner);
  const label = scene.names && name ? `${name} ` : "";
  const lead: Row = [
    ...(phase ? [{ text: `${phase === "running" ? progress.frame : symbols[phase]} `, tone: shades[phase] }] : []),
    { text: " ".repeat(scene.names && scene.names + 1 - Bun.stringWidth(label)), tone },
    { text: `${icon(step.owner)} `, tone },
    { text: label, tone },
    { text: String(step.n), tone: phase ? shades[phase] : tone, ...weight },
    { text: `${" ".repeat(scene.digits - String(step.n).length)}  `, tone },
  ];
  const indent: Row = [{ text: " ".repeat(span(lead)), tone: "muted" }];
  const gate = step.owner.kind === "gate";
  const ticks = progress?.ticks[step.n] ?? [];
  const notes = [
    ...(scene.graph
      ? undrawn(step, scene.graph, progress)
      : [...waits(step, scene.steps[index - 1]), ...repairs(step, progress)]),
    ...step.details.map((detail) =>
      detail.kind === "item" && ticks.includes(detail.n)
        ? mark(detail, gate ? seen : checked, "muted")
        : mark(detail, gate ? watch : box),
    ),
  ];
  const badge =
    step.owner.kind === "lane" && step.owner.lane ? [...pill(step.owner.lane, tone), { text: " ", tone }] : [];
  const right = side(step.owner, scene.thinking.get(step.n));
  const style = {
    tone: step.owner.kind === "self" ? "text" : tone,
    bold: !weight.italic,
    italic: weight.italic,
  } as const;
  return { lead, badge, title: headline(step.headline, style), right, notes: notes.map((row) => indent.concat(row)) };
}

function side(owner: Owner, word: string | undefined): Row {
  if (owner.kind !== "lane") return [];
  const thinking: Row = word ? [{ text: `${brain} ${word}  `, tone: "magenta" }] : [];
  const name = model(owner);
  return name ? [...thinking, { text: name, ...ink(owner) }] : [];
}

function headline(text: string, style: Omit<Segment, "text">): Row {
  const lead = topic(text);
  const main: Segment = { text: lead, ...style };
  return lead === text ? [main] : [main, { text: text.slice(lead.length), tone: "text" }];
}

function opening({ lead, badge, title, right }: Omit<Section, "notes">, width: number): Row {
  const room = right.length ? width - gap - span(right) : width;
  const left = fit([...lead, ...(span(lead) + span(badge) <= room ? badge : []), ...title], room);
  if (!right.length) return left;
  return [...left, { text: " ".repeat(width - span(left) - span(right)), tone: "muted" }, ...right];
}

function undrawn(step: Step, { hidden, looped }: Graph, progress: Progress | undefined): Row[] {
  const missing = hidden.get(step.n);
  return [
    ...(missing ? [[{ text: `⇠ also waits on ${missing.join(", ")}`, tone: "muted" }] satisfies Row] : []),
    ...(looped ? [] : repairs(step, progress)),
  ];
}

function waits(step: Step, previous: Step | undefined): Row[] {
  const usual = previous ? step.after.length === 1 && step.after[0] === previous.n : !step.after.length;
  return usual ? [] : [[{ text: `⇠ waits on ${step.after.join(", ") || "nothing"}`, tone: "muted" }]];
}

function repairs({ n, repair }: Step, progress: Progress | undefined): Row[] {
  if (!repair) return [];
  const budget = progress ? `${progress.rounds[n] ?? 0}/${repair.max} rounds` : `max ${repair.max}`;
  return [
    [
      { text: "↺ ", tone: "orange" },
      { text: `repairs ${repair.to.join(", ")} · ${budget}`, tone: "muted" },
    ],
  ];
}

function mark({ kind, text }: Detail, bullet: Segment, tone: Segment["tone"] = "text"): Row {
  switch (kind) {
    case "item":
      return [bullet, { text, tone }];
    case "done":
      return [
        { text: "⚑ ", tone: "success" },
        { text, tone: "text" },
      ];
    case "if":
      return [
        { text: "⑂ only if ", tone: "muted" },
        { text, tone: "text", italic: true },
      ];
    case "why":
      return [
        { text: "↳ why ", tone: "muted" },
        { text, tone: "muted", italic: true },
      ];
  }
}

function center(graph: readonly Row[], width: number): Row[] {
  const pad: Row = [{ text: " ".repeat(Math.floor((width - Math.max(...graph.map(span))) / 2)), tone: "muted" }];
  return graph.map((row) => pad.concat(row));
}

function wrap(text: string, width: number) {
  const lines: string[] = [];
  let current = "";
  for (const word of text.split(/\s+/).filter(Boolean)) {
    const joined = current ? `${current} ${word}` : word;
    if (Bun.stringWidth(joined) <= width) {
      current = joined;
      continue;
    }
    if (current) lines.push(current);
    current = word;
    while (Bun.stringWidth(current) > width) {
      const head = cut(current, width);
      lines.push(head);
      current = current.slice(head.length);
    }
  }
  if (current) lines.push(current);
  return lines;
}

function cut(word: string, width: number) {
  let head = "";
  for (const char of word) {
    if (head && Bun.stringWidth(head + char) > width) break;
    head += char;
  }
  return head;
}

function fit(row: Row, width: number): Row {
  if (span(row) <= width) return row;
  const kept: Row = [];
  let room = width;
  for (const [index, segment] of row.entries()) {
    const size = Bun.stringWidth(segment.text);
    if (size < room) {
      kept.push(segment);
      room -= size;
      continue;
    }
    const rest = row
      .slice(index)
      .map(({ text }) => text)
      .join("");
    kept.push({ ...segment, text: clip(rest, room) });
    break;
  }
  return kept;
}

function span(row: Row) {
  return row.reduce((sum, { text }) => sum + Bun.stringWidth(text), 0);
}
