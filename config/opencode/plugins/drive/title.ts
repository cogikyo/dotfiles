import type { Renderable, RGBA, TextRenderable } from "@opentui/core";
import { colors } from "../statusline/palette.ts";

export const glyph = "\u{f04d4}";
const prefix = `${glyph} `;
const depth = 4;
const arrival = 450;

type Rename = { title: string; pending: boolean };
type Title = TextRenderable & { _rename?: Rename; rename?: Rename };
type Decor = { fg: RGBA; core?: Rename; set: (value?: Rename) => void; busy: boolean; hold: number };

function isTitle(node: Renderable): node is Title {
  return "_rename" in node && "pendingTitle" in node && "textNode" in node;
}

function find(node: Renderable, level: number): Title | undefined {
  if (isTitle(node)) return node;
  if (level === 0) return undefined;
  for (const child of node.getChildren()) {
    const found = find(child, level - 1);
    if (found) return found;
  }
  return undefined;
}

function shown(node: Renderable, root: Renderable) {
  for (let at: Renderable | null = node; at; at = at.parent) {
    if (at.isDestroyed || !at.visible) return false;
    if (at === root) return true;
  }
  return false;
}

function setter(node: Title) {
  for (let proto = Object.getPrototypeOf(node); proto; proto = Object.getPrototypeOf(proto)) {
    if (Object.hasOwn(proto, "rename")) return (value?: Rename) => void Reflect.set(proto, "rename", value, node);
  }
  return undefined;
}

function forward(node: Title, decor: Decor, before?: Rename) {
  const core = decor.core;
  if (!core || !decor.busy || Date.now() < decor.hold) return decor.set(core);
  const current = node._rename;
  if (current && current.title !== core.title) {
    if (before?.pending) {
      decor.hold = Date.now() + arrival;
      return decor.set(core);
    }
    decor.set({ title: current.title, pending: false });
    decor.set({ title: core.title, pending: false });
  }
  decor.set({ title: core.title, pending: true });
}

export function title(root: Renderable) {
  const decors = new WeakMap<Title, Decor>();
  let row: Renderable | undefined;
  let node: Title | undefined;

  const locate = () => {
    if (!row || !shown(row, root)) row = root.findDescendantById("session-pane")?.parent ?? undefined;
    for (const child of row?.getChildren() ?? []) {
      if (child.id === "session-pane") continue;
      const found = find(child, depth);
      if (found) return found;
    }
    return undefined;
  };

  const apply = (target: Title, busy: boolean) => {
    let decor = decors.get(target);
    if (!decor) {
      const set = setter(target);
      if (!set) return false;
      const created: Decor = { fg: target.fg, core: target._rename, set, busy: false, hold: 0 };
      Object.defineProperty(target, "rename", {
        configurable: true,
        set: (value?: Rename) => {
          const before = created.core;
          created.core = value;
          forward(target, created, before);
        },
      });
      decors.set(target, created);
      decor = created;
    }
    if (target.textNode.children[0] !== prefix) target.textNode.add(prefix, 0);
    if (!target.fg.equals(colors.brightBlue)) {
      decor.fg = target.fg;
      target.fg = colors.brightBlue;
    }
    if (decor.busy !== busy || (decor.hold !== 0 && Date.now() >= decor.hold)) {
      decor.busy = busy;
      decor.hold = 0;
      forward(target, decor, decor.core);
    }
    return true;
  };

  const restore = (target: Title) => {
    const decor = decors.get(target);
    if (!decor) return;
    decors.delete(target);
    if (target.isDestroyed) return;
    Reflect.deleteProperty(target, "rename");
    target.rename = decor.core;
    const children = target.textNode.children;
    if (children[0] === prefix) target.textNode.children = children.slice(1);
    if (target.fg.equals(colors.brightBlue)) target.fg = decor.fg;
  };

  return {
    show(busy: boolean) {
      const found = node && shown(node, root) ? node : locate();
      if (node && node !== found) restore(node);
      node = found;
      if (!found || !apply(found, busy)) return false;
      return shown(found, root) && found.width > 0 && found.height > 0;
    },
    hide() {
      if (node) restore(node);
      node = undefined;
    },
  };
}
