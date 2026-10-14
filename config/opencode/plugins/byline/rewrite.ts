import {
  BoxRenderable,
  isTextNodeRenderable,
  RGBA,
  TextAttributes,
  TextNodeRenderable,
  TextRenderable,
  Yoga,
  type CliRenderer,
  type Renderable,
} from "@opentui/core";
import type { Plugin } from "@opencode/plugin/tui";
import { roles } from "../statusline/palette.ts";

type Text = Renderable & { readonly textNode: TextNodeRenderable; fg: RGBA };
type Frame = Renderable & { readonly border: unknown };
type Scroll = Renderable & { readonly content: Renderable };
type Undo = () => void;
type Names = () => ReadonlySet<string>;
type Host = { context: Plugin.Context; sessionID?: string };
type Mark = { tone: RGBA; top: number };
type Scan = { names: Names; ledger: Ledger; renderer: CliRenderer; width: number; info: RGBA };

const separator = " · ";

const tones = {
  rule: RGBA.fromHex("#3f4578"),
  model: RGBA.fromHex("#5a61aa"),
  stats: RGBA.fromHex("#585f8d"),
};

const italic = { attributes: TextAttributes.ITALIC };

// Exclude injected nodes from host-tree matching and repeated italic wrapping.
const owned = new WeakSet<object>();
const rules = new WeakMap<Renderable, TextRenderable[]>();
const rails = new WeakMap<Renderable, BoxRenderable>();
const spans = new WeakMap<Renderable, RGBA | undefined>();

export class Ledger {
  readonly #entries = new Map<Renderable, Map<unknown, Undo>>();

  has(owner: Renderable) {
    return this.#entries.has(owner);
  }

  record(owner: Renderable, key: unknown, undo: Undo) {
    const entries = this.#entries.get(owner) ?? new Map<unknown, Undo>();
    entries.set(key, undo);
    this.#entries.set(owner, entries);
  }

  restore(owner: Renderable) {
    this.#entries.get(owner)?.forEach((undo) => undo());
    this.#entries.delete(owner);
  }

  prune() {
    for (const owner of this.#entries.keys()) if (owner.isDestroyed) this.#entries.delete(owner);
  }

  clear() {
    for (const owner of this.#entries.keys()) if (!owner.isDestroyed) this.restore(owner);
    this.#entries.clear();
  }

  hide(node: Renderable) {
    if (!node.visible) return false;
    node.visible = false;
    this.record(node, node, () => (node.visible = true));
    return true;
  }

  paint(owner: Renderable, target: { fg: RGBA | undefined }, color: RGBA) {
    const from = target.fg;
    if (from?.equals(color)) return false;
    target.fg = color;
    this.record(owner, target, () => {
      if (target.fg === color) target.fg = from;
    });
    return true;
  }

  write(owner: Renderable, node: TextNodeRenderable, index: number, value: string | TextNodeRenderable) {
    const from = node.children[index];
    if (typeof from !== "string" || from === value) return false;
    node.replace(value, index);
    this.record(owner, typeof value === "string" ? node : value, () => {
      if (node.children[index] === value) node.replace(from, index);
    });
    return true;
  }

  adjust<T>(owner: Renderable, key: string, value: T, field: { get: () => T; set: (value: T) => void }) {
    const from = field.get();
    if (from === value) return false;
    field.set(value);
    owner.requestRender();
    this.record(owner, key, () => {
      if (field.get() !== value) return;
      field.set(from);
      owner.requestRender();
    });
    return true;
  }

  mount(owner: Renderable, child: Renderable, index?: number) {
    if (child.parent === owner) return false;
    owner.add(child, index);
    this.record(owner, child, () => child.destroy());
    return true;
  }
}

export function prompt(anchor: Renderable, names: Names, ledger: Ledger, fallback = "") {
  ledger.prune();
  const row = metadata(anchor) ?? [];
  const group = row.at(-1);
  if (!group || textual(group)) return;
  const nodes = group.getChildren();
  const [dot] = nodes;
  const [agent] = row;
  const labelled = textual(dot) && read(dot) === "·" && textual(agent);
  const model = labelled ? nodes[1] : dot;
  if (!names().has(labelled ? read(agent) : fallback)) {
    for (const node of [...row, ...nodes]) ledger.restore(node);
    return;
  }
  if (!textual(model)) return;
  const edits = [...emphasize(model, model.textNode, ledger), labelled && relabel(row, dot, model, ledger)];
  if (edits.includes(true)) model.onLifecyclePass?.();
}

function relabel(row: Renderable[], dot: Renderable, model: Text, ledger: Ledger) {
  const [agent] = row;
  if (!textual(agent)) return false;
  ledger.hide(agent);
  if (row.some((node) => node !== agent && node.visible && textual(node) && read(node) === "auto")) ledger.restore(dot);
  else ledger.hide(dot);
  return ledger.paint(model, model, agent.fg);
}

function metadata(anchor: Renderable) {
  const frame = anchor.parent?.parent?.getChildren()[0];
  if (!frame || !framed(frame)) return;
  const inner = frame.getChildren()[0]?.getChildren() ?? [];
  const editor = inner.findIndex(editable);
  if (editor === -1) return;
  return inner[editor + 1]?.getChildren()[0]?.getChildren();
}

export function transcript(anchor: Renderable, names: Names, ledger: Ledger, { context, sessionID }: Host) {
  const scroll = scroller(anchor.parent?.parent, 2);
  if (!scroll) return;
  ledger.prune();
  const renderer = context.renderer;
  if (renderer.width < 28) return ledger.clear();
  const scan = { names, ledger, renderer, width: scroll.content.width, info: context.theme.text.feedback.info.base };
  const rows = scroll.content.getChildren();
  // Retained rows preserve tone when the completion notice leaves the mounted window.
  let tone = spans.get(rows[0]);
  for (const row of rows) {
    const box = native(row).at(-1);
    const edge = box && boundary(box, scan);
    if (edge) tone = edge.tone;
    if (sessionID && context.data.session.message.get(sessionID, row.id)?.type === "user") tone = undefined;
    rail(row, box && tone ? { tone, top: edge?.top ?? -1 } : undefined, ledger, renderer);
    spans.set(row, tone);
  }
}

function boundary(box: Renderable, scan: Scan): { tone?: RGBA; top: number } | undefined {
  const [text, ...extra] = native(box);
  if (extra.length > 0 || !textual(text)) return;
  if (byline(text, scan.names, scan.ledger)) {
    rule(box, scan.width - Bun.stringWidth(read(text)), scan.ledger, scan.renderer);
    return { top: -1 };
  }
  const done = completion(text);
  if (done) {
    notice(done, text, scan.ledger, scan.info);
    return { tone: done.tone, top: 0 };
  }
  if (turned(text)) return { top: -1 };
}

function byline(text: Text, names: Names, ledger: Ledger) {
  const [agent, model, ...stats] = text.textNode.children;
  if (!isTextNodeRenderable(agent) || !isTextNodeRenderable(model)) return false;
  const label = flatten(agent);
  if (label ? !claimed(label, flatten(model), names) : !ledger.has(text)) return false;
  const edits = [
    ...leaves(agent).map(([node, index]) => ledger.write(text, node, index, "")),
    strip(text, model, ledger),
    ledger.paint(text, model, tones.model),
    ...emphasize(text, model, ledger),
    ...stats.filter(isTextNodeRenderable).map((node) => ledger.paint(text, node, tones.stats)),
  ];
  if (edits.includes(true)) text.onLifecyclePass?.();
  return true;
}

function rule(box: Renderable, room: number, ledger: Ledger, renderer: CliRenderer) {
  if (room < 4) return ledger.restore(box);
  const node = box.getLayoutNode();
  ledger.adjust(box, "direction", Yoga.FlexDirection.Row, {
    get: () => node.getFlexDirection(),
    set: (value) => node.setFlexDirection(value),
  });
  ledger.adjust(box, "padding", 0, {
    get: () => node.getPadding(Yoga.Edge.Left).value,
    set: (value) => node.setPadding(Yoga.Edge.Left, value),
  });
  const [left, right] = sides(box, renderer);
  ledger.mount(box, left, 0);
  ledger.mount(box, right);
}

function sides(box: Renderable, renderer: CliRenderer) {
  const known = rules.get(box);
  const pair =
    known && !known.some((side) => side.isDestroyed)
      ? known
      : [{ marginRight: 1 }, { marginLeft: 1 }].map(
          (margin) =>
            new TextRenderable(renderer, {
              ...margin,
              fg: tones.rule,
              wrapMode: "none",
              flexGrow: 1,
              flexBasis: 0,
              selectable: false,
            }),
        );
  rules.set(box, pair);
  for (const side of pair) owned.add(side);
  for (const side of pair) if (side.plainText.length !== renderer.width) side.content = "─".repeat(renderer.width);
  return pair;
}

function rail(row: Renderable, mark: Mark | undefined, ledger: Ledger, renderer: CliRenderer) {
  if (!mark) return ledger.restore(row);
  const known = rails.get(row);
  const bar =
    known && !known.isDestroyed
      ? known
      : new BoxRenderable(renderer, {
          position: "absolute",
          left: 0,
          bottom: 0,
          width: 1,
          border: ["left"],
          customBorderChars: {
            topLeft: "",
            topRight: "",
            bottomLeft: "",
            bottomRight: "",
            horizontal: " ",
            vertical: "▎",
            topT: "",
            bottomT: "",
            leftT: "",
            rightT: "",
            cross: "",
          },
        });
  rails.set(row, bar);
  owned.add(bar);
  if (!bar.borderColor.equals(mark.tone)) bar.borderColor = mark.tone;
  if (bar.top !== mark.top) bar.top = mark.top;
  ledger.mount(row, bar);
}

function completion(text: Text) {
  const [heading, suffix, ...rest] = text.textNode.children;
  if (rest.length > 0 || !isTextNodeRenderable(heading) || !isTextNodeRenderable(suffix)) return;
  if (!/^( ·|…| …|$)/.test(flatten(suffix))) return;
  const [, glyph, role] = /^([↳!]) (?:([^/\s]+)\/)?\S/.exec(flatten(heading)) ?? [];
  if (glyph === "!") return { heading, tone: undefined };
  if (glyph && role) return { heading, tone: roles[role.toLowerCase()] };
}

function notice(done: { heading: TextNodeRenderable; tone?: RGBA }, text: Text, ledger: Ledger, info: RGBA) {
  if (!done.tone || !done.heading.fg?.equals(info)) return;
  if (ledger.paint(text, done.heading, done.tone)) text.onLifecyclePass?.();
}

function turned(text: Text) {
  const [agent, model] = text.textNode.children;
  if (!isTextNodeRenderable(agent) || !isTextNodeRenderable(model)) return false;
  return !/^[↳!] /.test(flatten(agent)) && flatten(model).startsWith(separator);
}

function emphasize(owner: Renderable, node: TextNodeRenderable, ledger: Ledger) {
  return leaves(node).map(([parent, index, value]) => {
    const parts = value.split(/(\bfast\b)/i);
    if (parts.length === 1) return false;
    const wrapper = TextNodeRenderable.fromNodes(
      parts.flatMap((part, at) => (part ? [TextNodeRenderable.fromString(part, at % 2 ? italic : undefined)] : [])),
    );
    owned.add(wrapper);
    return ledger.write(owner, parent, index, wrapper);
  });
}

function claimed(label: string, rest: string, names: Names) {
  return names().has(label) && rest.startsWith(separator);
}

function strip(text: Text, model: TextNodeRenderable, ledger: Ledger) {
  const lead = leaves(model).find(([, , value]) => value);
  if (!lead?.[2].startsWith(separator)) return false;
  return ledger.write(text, lead[0], lead[1], lead[2].slice(separator.length));
}

function leaves(node: TextNodeRenderable): [TextNodeRenderable, number, string][] {
  return node.children.flatMap((child, index): [TextNodeRenderable, number, string][] =>
    typeof child === "string" ? [[node, index, child]] : owned.has(child) ? [] : leaves(child),
  );
}

function flatten(node: TextNodeRenderable): string {
  return node.children.map((child) => (typeof child === "string" ? child : flatten(child))).join("");
}

function native(node: Renderable) {
  return node.getChildren().filter((child) => !owned.has(child));
}

function read(node: Text) {
  return flatten(node.textNode);
}

function textual(node: Renderable | undefined): node is Text {
  return node !== undefined && "textNode" in node;
}

function scroller(node: Renderable | null | undefined, depth: number): Scroll | undefined {
  for (const child of node?.getChildren() ?? []) {
    const found = scrollable(child) ? child : depth > 1 ? scroller(child, depth - 1) : undefined;
    if (found) return found;
  }
}

function scrollable(node: Renderable | undefined): node is Scroll {
  return node !== undefined && "content" in node && "viewport" in node;
}

function framed(node: Renderable): node is Frame {
  return "borderColor" in node && "border" in node && Array.isArray(node.border) && node.border.join() === "left";
}

function editable(node: Renderable) {
  return "plainText" in node && "cursorOffset" in node && "extmarks" in node;
}
