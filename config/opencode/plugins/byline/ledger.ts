import { RGBA, type Renderable, type TextNodeRenderable } from "@opentui/core";

type Undo = () => void;
type Entry = { undo: Undo; value?: unknown };
type Field<T> = { get: () => T; set: (value: T) => void };

// Exclude injected nodes from host-tree matching and repeated italic wrapping.
export const owned = new WeakSet<object>();

export class Ledger {
  readonly #entries = new Map<Renderable, Map<unknown, Entry>>();

  has(owner: Renderable) {
    return this.#entries.has(owner);
  }

  record(owner: Renderable, key: unknown, undo: Undo) {
    this.#set(owner, key, { undo });
  }

  restore(owner: Renderable) {
    this.#entries.get(owner)?.forEach(({ undo }) => undo());
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
    if (target.fg?.equals(color)) return false;
    this.#change(owner, target, color, { get: () => target.fg, set: (value) => (target.fg = value) });
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

  adjust<T>(owner: Renderable, key: string, value: T, field: Field<T>) {
    if (field.get() === value) return false;
    this.#change(owner, key, value, field);
    owner.requestRender();
    return true;
  }

  mount(owner: Renderable, child: Renderable, index?: number) {
    if (child.parent === owner) return false;
    owner.add(child, index);
    this.record(owner, child, () => child.destroy());
    return true;
  }

  #change<T>(owner: Renderable, key: unknown, value: T, field: Field<T>) {
    const known = this.#entries.get(owner)?.get(key);
    const from = field.get();
    field.set(value);
    if (known && "value" in known && same(known.value, from)) {
      known.value = value;
      return;
    }
    const entry: Entry = {
      value,
      undo: () => {
        if (!same(field.get(), entry.value)) return;
        field.set(from);
        owner.requestRender();
      },
    };
    this.#set(owner, key, entry);
  }

  #set(owner: Renderable, key: unknown, entry: Entry) {
    const entries = this.#entries.get(owner) ?? new Map<unknown, Entry>();
    entries.set(key, entry);
    this.#entries.set(owner, entries);
  }
}

function same(a: unknown, b: unknown) {
  return a === b || (a instanceof RGBA && b instanceof RGBA && a.equals(b));
}
