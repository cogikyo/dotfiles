import { Yoga, type RGBA, type Renderable, type TextNodeRenderable } from "@opentui/core";
import type { Ledger } from "./ledger.ts";

type Text = Renderable & { readonly textNode: TextNodeRenderable };
export type Tint = { tone: RGBA };
export type Lane = Tint & {
  text: Text;
  role: string;
  actor: string;
  state: string;
  title: string;
  summary: string;
};
export type Letter = Tint & { icon: Text & { fg: RGBA | undefined }; text: Text; full: string };
export type Mark = { node: Renderable; x: number; glyph: string };
type Block = { node: Renderable; hr: boolean; tail: boolean };
export type Item =
  | { kind: "notice" | "failed"; row: Renderable; lane: Lane }
  | { kind: "mail" | "sent"; row: Renderable; lane: Letter }
  | { kind: "plain" | "stop" | "prompt" | "content"; row: Renderable }
  | { kind: "markdown"; row: Renderable; blocks: readonly Block[]; streaming: boolean };
type Unit = { node: Renderable; row: boolean; rule: boolean; flush?: boolean; cut?: boolean; lane?: Tint };
export type Run = { lane: Tint; top: Renderable; bottom: Renderable };

type Head = { lane: Tint; inside: boolean; pending: Unit[]; note?: boolean };
type Value = { unit: number; value: number };

export const layout = { flow: 3, left: 4, right: 1, pad: 2 };
const start = layout.left + 1;
const end = layout.right + layout.pad;
export const dashes = /\n[ \t]*-{3,}[ \t]*\n*$/;
const point: number = Yoga.Unit.Point;
const percent: number = Yoga.Unit.Percent;
// Preserve incoming lane state when earlier rows leave the mounted window.
const carry = new WeakMap<Renderable, readonly Pick<Head, "lane" | "inside" | "note">[]>();
const widths = new WeakMap<Renderable, number | "auto" | `${number}%`>();
const notes = new WeakMap<Renderable, Tint>();

export function walk(items: readonly Item[], tint?: RGBA) {
  const units: Unit[] = [];
  const closed = new Set<Tint>();
  let mailed = false;
  const first = items[0];
  let open: Head[] = (first ? (carry.get(first.row) ?? []) : []).map(({ lane, inside, note }) => ({
    lane,
    inside,
    note,
    pending: [],
  }));
  for (const item of items) {
    carry.set(
      item.row,
      open.map(({ lane, inside, note }) => ({ lane, inside, note })),
    );
    if (item.kind === "notice" || item.kind === "mail") {
      open = arrive({ kind: item.kind, row: item.row, lane: item.lane }, open, units);
      mailed ||= item.kind === "mail";
      continue;
    }
    if (item.kind === "sent") units.push({ node: item.row, row: true, rule: false, lane: item.lane });
    else if (item.kind === "markdown") open = read(item, open, units, closed);
    else if (item.kind === "content") place({ node: item.row, row: true, rule: false }, open, units);
    else {
      if (item.kind === "stop" || item.kind === "prompt")
        open = commentary(item.row, item.kind === "prompt" && mailed ? tint : undefined);
      units.push({ node: item.row, row: true, rule: false });
    }
    mailed = false;
  }
  return { units, closed, open };
}

type Arrival = { kind: "notice" | "mail"; row: Renderable; lane: Tint };

function arrive(item: Arrival, open: readonly Head[], units: Unit[]): Head[] {
  units.push({ node: item.row, row: true, rule: false, lane: item.lane });
  const mail = item.kind === "mail";
  const kept = open.filter((head) => head.inside || !(head.note || (mail && "full" in head.lane)));
  return [...kept, { lane: item.lane, inside: false, pending: [] }];
}

function commentary(row: Renderable, tint?: RGBA): Head[] {
  if (!tint) return [];
  const lane = notes.get(row) ?? { tone: tint };
  notes.set(row, lane);
  return [{ lane, inside: false, pending: [], note: true }];
}

function settle(item: Extract<Item, { kind: "markdown" }>, open: Head[]) {
  const [head] = open;
  const [block] = item.blocks;
  if (head?.note && !head.inside && !block?.hr) open.shift();
}

function read(item: Extract<Item, { kind: "markdown" }>, open: Head[], units: Unit[], closed: Set<Tint>) {
  settle(item, open);
  const [lead] = open;
  const waiting = lead !== undefined && !lead.inside;
  let rules = 0;
  let opened = false;
  for (const block of item.blocks) {
    const [head] = open;
    if (!head || !(block.hr || block.tail)) {
      place({ node: block.node, row: false, rule: false, flush: opened }, open, units);
      opened = false;
      continue;
    }
    rules++;
    if (block.tail) place({ node: block.node, row: false, rule: false, flush: opened, cut: true }, open, units);
    else units.push({ node: block.node, row: false, rule: true, lane: head.lane });
    opened = false;
    if (head.inside) {
      closed.add(head.lane);
      open.shift();
      continue;
    }
    head.inside = true;
    opened = block.hr;
    if (!head.note) for (const unit of head.pending) unit.lane = head.lane;
    head.pending = [];
  }
  return rules === 0 && waiting && !item.streaming ? [] : open;
}

function place(unit: Unit, open: readonly Head[], units: Unit[]) {
  units.push(unit);
  const [head] = open;
  if (head?.inside) unit.lane = head.lane;
  else head?.pending.push(unit);
}

export function grow(open: readonly Head[], live: (lane: Tint) => boolean) {
  for (const head of open) {
    if (head.inside || head.note || !live(head.lane)) continue;
    for (const unit of head.pending) unit.lane = head.lane;
  }
}

export function runs(units: readonly Unit[]) {
  const found: Run[] = [];
  let last: Run | undefined;
  for (const { node, lane, rule } of units) {
    if (rule) continue;
    if (!lane) last = undefined;
    else if (last?.lane === lane) last.bottom = node;
    else {
      last = { lane, top: node, bottom: node };
      found.push(last);
    }
  }
  return found;
}

export function apply(units: readonly Unit[], ledger: Ledger) {
  for (const unit of units) {
    if (!unit.lane) {
      ledger.restore(unit.node);
      continue;
    }
    if (unit.rule) ledger.hide(unit.node);
    else if (unit.row) pad(unit.node, ledger);
    else shift(unit.node, ledger);
    if (unit.flush) flush(unit.node, ledger);
    if (unit.cut) cut(unit.node, ledger);
  }
}

function cut(block: Renderable, ledger: Ledger) {
  if (!("content" in block) || typeof block.content !== "string") return;
  const content = block.content;
  ledger.adjust(block, "cut", content.replace(dashes, ""), {
    get: () => Reflect.get(block, "content"),
    set: (value) => Reflect.set(block, "content", value),
  });
}

function pad(row: Renderable, ledger: Ledger) {
  ledger.adjust(row, "indent", start, {
    get: () => padding(row, Yoga.Edge.Left),
    set: (value) => (row.paddingLeft = value),
  });
  ledger.adjust(row, "inset", end, {
    get: () => padding(row, Yoga.Edge.Right),
    set: (value) => (row.paddingRight = value),
  });
}

function shift(block: Renderable, ledger: Ledger) {
  ledger.adjust(block, "indent", true, {
    get: () => widths.has(block),
    set: (on) => {
      if (on) {
        widths.set(block, width(measure(block, "getWidth", Yoga.Edge.Left)));
        block.width = "auto";
        block.marginLeft = start;
        block.marginRight = end;
        return;
      }
      block.width = widths.get(block) ?? "100%";
      block.marginLeft = 0;
      block.marginRight = 0;
      widths.delete(block);
    },
  });
}

function flush(block: Renderable, ledger: Ledger) {
  ledger.adjust(block, "flush", 0, {
    get: () => number(measure(block, "getMargin", Yoga.Edge.Top)),
    set: (value) => (block.marginTop = value),
  });
}

export function padding(node: Renderable, edge: Yoga.Edge) {
  return number(measure(node, "getPadding", edge));
}

function measure(node: Renderable, key: "getWidth" | "getPadding" | "getMargin", edge: Yoga.Edge): Value | undefined {
  const getter: unknown = Reflect.get(node, key);
  if (typeof getter !== "function") return;
  const value: unknown = Reflect.apply(getter, node, [edge]);
  if (typeof value !== "object" || value === null || !("unit" in value) || !("value" in value)) return;
  const { unit, value: amount } = value;
  return typeof unit === "number" && typeof amount === "number" ? { unit, value: amount } : undefined;
}

function number(value: Value | undefined) {
  return value && Number.isFinite(value.value) ? value.value : 0;
}

function width(value: Value | undefined): number | "auto" | `${number}%` {
  if (!value) return "100%";
  if (value.unit === point) return value.value;
  if (value.unit === percent) return `${value.value}%`;
  return "auto";
}
