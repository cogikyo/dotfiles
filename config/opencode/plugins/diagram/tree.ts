import type { Layout } from "./layout.ts";

type Entry = { col: number; path: string };

export function tree(layout: Layout) {
  const stack: Entry[] = [];
  const paths: string[] = [];
  const notes = new Map<number, number[]>();

  for (const [r, row] of layout.grid.entries()) {
    const head = row.findIndex((found) => found.kind !== "space");
    if (head < 0 || layout.tip(r, head)) continue;
    const col = layout.geometry(r, head) ? limb(layout, r, stack) : root(layout, r, head, stack);
    if (col === undefined) continue;
    paths.push(stack[stack.length - 1].path);
    const span = layout.text(r, col);
    const note = row.findIndex((_, c) => c > col && layout.text(r, c) !== undefined && layout.text(r, c) !== span);
    if (note >= 0) notes.set(note, [...(notes.get(note) ?? []), r]);
  }

  if (notes.size > 1) {
    const groups = [...notes].toSorted((a, b) => a[1].length - b[1].length);
    const cols = groups.map(([c, rows]) => `cell ${c + 1} on ${rows.length} row(s)`).join(", ");
    layout.flag(groups[0][1][0], groups[0][0], `annotations are not aligned: ${cols}`);
  }
  return paths.length ? `tree paths:\n${paths.map((p) => `- ${p}`).join("\n")}` : "";
}

function root(layout: Layout, r: number, head: number, stack: Entry[]) {
  const span = layout.text(r, head);
  if (span === undefined) return undefined;
  stack.length = 0;
  stack.push({ col: head, path: layout.spans[span] });
  return head;
}

function limb(layout: Layout, r: number, stack: Entry[]) {
  const at = layout.grid[r].findIndex((_, c) => layout.glyph(r, c) === "├" || layout.glyph(r, c) === "└");
  if (at < 0) return undefined;
  let entry = at + 1;
  while (layout.glyph(r, entry) === "─") entry++;
  const rails = entry - at - 1;
  if (layout.glyph(r, entry) === " ") entry++;
  const span = layout.text(r, entry);
  if (rails < 2 || span === undefined) {
    layout.flag(r, at, "limb must continue as `──` into an entry name");
    return undefined;
  }
  while (stack.length && stack[stack.length - 1].col > at) stack.pop();
  const parent = stack.at(-1);
  if (parent?.col !== at) {
    layout.flag(r, at, "limb does not sit under the first cell of its parent entry");
    return undefined;
  }
  const name = layout.spans[span];
  stack.push({ col: entry, path: parent.path.endsWith("/") ? `${parent.path}${name}` : `${parent.path}/${name}` });
  return entry;
}
