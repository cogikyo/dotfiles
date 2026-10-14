import {
  isTextNodeRenderable,
  RGBA,
  TextRenderable,
  Yoga,
  type CliRenderer,
  type Renderable,
  type TextNodeRenderable,
} from "@opentui/core";
import type { Plugin } from "@opencode/plugin/tui";
import { roles } from "../statusline/palette.ts";

type Text = Renderable & { readonly textNode: TextNodeRenderable; fg: RGBA };
type Frame = Renderable & { readonly border: unknown };
type Scroll = Renderable & { readonly content: Renderable };
type Undo = () => void;
type Names = () => ReadonlySet<string>;

const separator = " · ";

const tones = {
  rule: RGBA.fromHex("#3f4578"),
  model: RGBA.fromHex("#5a61aa"),
  stats: RGBA.fromHex("#585f8d"),
};

const rules = new WeakMap<Renderable, TextRenderable[]>();

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

  write(owner: Renderable, node: TextNodeRenderable, index: number, value: string) {
    const from = node.children[index];
    if (typeof from !== "string" || from === value) return false;
    node.replace(value, index);
    this.record(owner, node, () => {
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

export function prompt(anchor: Renderable, names: Names, ledger: Ledger) {
  ledger.prune();
  const row = metadata(anchor) ?? [];
  const [agent] = row;
  const group = row.at(-1);
  if (!textual(agent) || !group || textual(group)) return;
  const [dot, model] = group.getChildren();
  if (!names().has(read(agent))) {
    for (const node of [agent, ...group.getChildren()]) ledger.restore(node);
    return;
  }
  if (!textual(dot) || !textual(model) || read(dot) !== "·") return;
  ledger.hide(agent);
  if (row.some((node) => node !== agent && node.visible && textual(node) && read(node) === "auto")) ledger.restore(dot);
  else ledger.hide(dot);
  if (ledger.paint(model, model, agent.fg)) model.onLifecyclePass?.();
}

function metadata(anchor: Renderable) {
  const frame = anchor.parent?.parent?.getChildren()[0];
  if (!frame || !framed(frame)) return;
  const inner = frame.getChildren()[0]?.getChildren() ?? [];
  const editor = inner.findIndex(editable);
  if (editor === -1) return;
  return inner[editor + 1]?.getChildren()[0]?.getChildren();
}

export function transcript(anchor: Renderable, names: Names, ledger: Ledger, context: Plugin.Context) {
  const scroll = scroller(anchor.parent?.parent, 2);
  if (!scroll) return;
  ledger.prune();
  const renderer = context.renderer;
  if (renderer.width < 28) return ledger.clear();
  for (const row of scroll.content.getChildren()) {
    const box = row.getChildren().at(-1);
    if (!box) continue;
    const [text, ...extra] = box.getChildren().filter((node) => !rules.get(box)?.some((side) => side === node));
    if (extra.length > 0 || !textual(text)) continue;
    if (byline(text, names, ledger)) rule(box, scroll.content.width - Bun.stringWidth(read(text)), ledger, renderer);
    else notice(text, ledger, context.theme.text.feedback.info.base);
  }
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
  for (const side of pair) if (side.plainText.length !== renderer.width) side.content = "─".repeat(renderer.width);
  return pair;
}

function notice(text: Text, ledger: Ledger, info: RGBA) {
  const [heading] = text.textNode.children;
  if (!isTextNodeRenderable(heading) || !heading.fg?.equals(info)) return;
  const tone = roles[/^↳ ([^/\s]+)\//.exec(flatten(heading))?.[1].toLowerCase() ?? ""];
  if (tone && ledger.paint(text, heading, tone)) text.onLifecyclePass?.();
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
    typeof child === "string" ? [[node, index, child]] : leaves(child),
  );
}

function flatten(node: TextNodeRenderable): string {
  return node.children.map((child) => (typeof child === "string" ? child : flatten(child))).join("");
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
