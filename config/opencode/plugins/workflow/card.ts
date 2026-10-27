import { clip } from "../byline/notice.ts";
import { family, glyphs } from "../lanes/label.ts";
import { routeFor, type Ref } from "../lanes/routing.ts";
import { draw } from "./graph.ts";
import type { Detail, Owner, Step, Workflow } from "./parse.ts";
import { ink, pill, type Row, type Segment } from "./style.ts";

export type Routes = Record<string, Ref>;

type Scene = {
  steps: readonly Step[];
  thinking: ReadonlyMap<number, string>;
  digits: number;
  names: number;
  listed: boolean;
};

type Section = { lead: Row; badge: Row; title: Row; right: Row; notes: Row[] };

const icons = { self: "◇", gate: "◆", you: "\u{f007}" } as const;
const brain = "\u{f09d1}";
const box: Segment = { text: "☐ ", tone: "muted" };
const watch: Segment = { text: "◎ ", tone: "orange" };
const gap = 2;

export function card(flow: Workflow, width: number, routes?: Routes): Row[] {
  const head: Row[] = flow.errors.map(({ line, message }) => [{ text: `✗ line ${line}: ${message}`, tone: "error" }]);
  if (flow.title) head.push([{ text: flow.title, tone: "accent", bold: true }]);
  for (const text of wrap(flow.summary ?? "", width)) head.push([{ text, tone: "text" }]);
  const body = plan(flow.steps, width, routes);
  const rows = head.length && body.length ? [...head, [], ...body] : [...head, ...body];
  return rows.map((row) => fit(row, width));
}

function plan(list: readonly Step[], width: number, routes: Routes | undefined): Row[] {
  if (!list.length) return [];
  const graph = draw(list, width);
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
    listed: !graph,
  };
  const sections = list.map((step, index) => section(step, index, scene));
  const wide = sections.every(
    ({ lead, badge, right }) => !right.length || span(lead) + span(badge) < width - gap - span(right),
  );
  const rows = graph ? [...center(graph, width), []] : [];
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
  const name = scope(step.owner);
  const label = scene.names && name ? `${name} ` : "";
  const lead: Row = [
    { text: " ".repeat(scene.names && scene.names + 1 - Bun.stringWidth(label)), tone },
    { text: `${icon(step.owner)} `, tone },
    { text: label, tone },
    { text: String(step.n), tone, ...weight },
    { text: `${" ".repeat(scene.digits - String(step.n).length)}  `, tone },
  ];
  const indent: Row = [{ text: " ".repeat(span(lead)), tone: "muted" }];
  const notes = [
    ...(scene.listed ? [...waits(step, scene.steps[index - 1]), ...repairs(step)] : []),
    ...step.details.map((detail) => mark(detail, step.owner.kind === "gate" ? watch : box)),
  ];
  const badge =
    step.owner.kind === "lane" && step.owner.lane ? [...pill(step.owner.lane, tone), { text: " ", tone }] : [];
  const right = side(step.owner, scene.thinking.get(step.n));
  return { lead, badge, title: headline(step), right, notes: notes.map((row) => indent.concat(row)) };
}

function side(owner: Owner, word: string | undefined): Row {
  if (owner.kind !== "lane") return [];
  const thinking: Row = word ? [{ text: `${brain} ${word}  `, tone: "magenta" }] : [];
  const name = model(owner);
  return name ? [...thinking, { text: name, ...ink(owner) }] : [];
}

function headline({ owner, headline: text }: Step): Row {
  const { tone, italic } = ink(owner);
  const at = text.indexOf(": ");
  const main: Segment = {
    text: at === -1 ? text : text.slice(0, at),
    tone: owner.kind === "self" ? "text" : tone,
    bold: !italic,
    italic,
  };
  return at === -1 ? [main] : [main, { text: text.slice(at), tone: "text" }];
}

function opening({ lead, badge, title, right }: Omit<Section, "notes">, width: number): Row {
  const room = right.length ? width - gap - span(right) : width;
  const left = fit([...lead, ...(span(lead) + span(badge) <= room ? badge : []), ...title], room);
  if (!right.length) return left;
  return [...left, { text: " ".repeat(width - span(left) - span(right)), tone: "muted" }, ...right];
}

function waits(step: Step, previous: Step | undefined): Row[] {
  const usual = previous ? step.after.length === 1 && step.after[0] === previous.n : !step.after.length;
  return usual ? [] : [[{ text: `⇠ waits on ${step.after.join(", ") || "nothing"}`, tone: "muted" }]];
}

function repairs({ repair }: Step): Row[] {
  if (!repair) return [];
  return [
    [
      { text: "↺ ", tone: "orange" },
      { text: `repairs ${repair.to.join(", ")} · max ${repair.max}`, tone: "muted" },
    ],
  ];
}

function mark({ kind, text }: Detail, bullet: Segment): Row {
  switch (kind) {
    case "item":
      return [bullet, { text, tone: "text" }];
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

function icon(owner: Owner) {
  if (owner.kind !== "lane") return icons[owner.kind];
  if (!owner.role) return "?";
  const glyph = glyphs(`${owner.role}/${owner.scope}`);
  return glyph.scope ?? glyph.role ?? "?";
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
