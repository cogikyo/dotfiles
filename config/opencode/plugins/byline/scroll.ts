import type { Renderable } from "@opentui/core";

export type Scroll = Renderable & { readonly content: Renderable; readonly viewport: Renderable };

export function locate(anchor: Renderable) {
  return search(anchor.parent?.parent, 2);
}

function search(node: Renderable | null | undefined, depth: number): Scroll | undefined {
  for (const child of node?.getChildren() ?? []) {
    const found = scrollable(child) ? child : depth > 1 ? search(child, depth - 1) : undefined;
    if (found) return found;
  }
}

function scrollable(node: Renderable | undefined): node is Scroll {
  return node !== undefined && "content" in node && "viewport" in node;
}
