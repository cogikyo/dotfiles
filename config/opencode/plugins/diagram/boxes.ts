import { E, key, type Layout, N, W } from "./layout.ts";

export type Box = { top: number; left: number; bottom: number; right: number; name: string };
export type Side = "top" | "bottom" | "left" | "right" | "corner";
export type Wall = { box: Box; side: Side };
export type Boxes = { list: Box[]; walls: Map<string, Wall>; titles: Map<string, Box> };

export const ports: Record<Side, string> = { top: "┴", bottom: "┬", left: "┤", right: "├", corner: "" };

export function findBoxes(layout: Layout): Boxes {
  const boxes: Boxes = { list: [], walls: new Map(), titles: new Map() };
  for (const { r, c } of layout.cells()) {
    const box = trace(layout, r, c);
    if (!box) continue;
    boxes.list.push(box);
    title(layout, box, boxes.titles);
  }
  for (const box of boxes.list) {
    name(layout, box, boxes.titles);
    enclose(layout, box, boxes);
  }
  overlaps(layout, boxes.list);
  return boxes;
}

function trace(layout: Layout, top: number, left: number): Box | undefined {
  if (layout.glyph(top, left) !== "┌") return undefined;
  let right = left + 1;
  while (layout.cell(top, right) && layout.glyph(top, right) !== "┐") {
    if (layout.tip(top, right) || (layout.geometry(top, right) && !layout.has(top, right, E | W))) return undefined;
    right++;
  }
  let bottom = top + 1;
  while (layout.glyph(bottom, left) !== "└" && layout.has(bottom, left, N)) bottom++;
  const box = { top, left, bottom, right, name: `box at ${layout.at(top, left)}` };
  return closed(layout, box) ? box : undefined;
}

function closed(layout: Layout, { top, left, bottom, right }: Box) {
  if (layout.glyph(top, right) !== "┐" || layout.glyph(bottom, left) !== "└" || layout.glyph(bottom, right) !== "┘") {
    return false;
  }
  for (let r = top + 1; r < bottom; r++) if (!layout.has(r, right, N)) return false;
  for (let c = left + 1; c < right; c++) if (!layout.has(bottom, c, E | W)) return false;
  return true;
}

function title(layout: Layout, box: Box, titles: Map<string, Box>) {
  const { top, left, right } = box;
  const zone: number[] = [];
  for (let c = left + 1; c < right; c++) if (!layout.has(top, c, E | W)) zone.push(c);
  if (zone.length === 0) return;

  const [start, end] = [zone[0], zone[zone.length - 1]];
  const span = layout.text(top, start + 1);
  if (span !== undefined) box.name = layout.spans[span];
  for (let c = start; c <= end; c++) titles.set(key(top, c), box);
  const [lead, tail] = [start - left - 1, right - end - 1];
  const padded = layout.glyph(top, start) === " " && layout.glyph(top, end) === " ";
  const tight = layout.glyph(top, start + 1) !== " " && layout.glyph(top, end - 1) !== " ";
  if (end - start + 1 !== zone.length) layout.flag(top, start, `${box.name} top rail is broken in more than one place`);
  if (!padded || !tight) layout.flag(top, start, `${box.name} title needs exactly one space on each side`);
  if (lead < 2 || tail < 2) layout.flag(top, start, `${box.name} title needs at least two rail cells on each side`);
  if (tail - lead !== 0 && tail - lead !== 1) {
    layout.flag(top, start, `${box.name} title is off center: ${lead} rail cells left, ${tail} right`);
  }
}

function name(layout: Layout, box: Box, titles: Map<string, Box>) {
  if (titles.get(key(box.top, box.left + 1)) === box || !box.name.startsWith("box at")) return;
  for (let r = box.top + 1; r < box.bottom; r++) {
    for (let c = box.left + 1; c < box.right; c++) {
      const span = layout.text(r, c);
      if (span === undefined) continue;
      box.name = layout.spans[span];
      return;
    }
  }
}

function enclose(layout: Layout, box: Box, { walls, titles }: Boxes) {
  for (const [r, c] of [
    [box.top, box.left],
    [box.top, box.right],
    [box.bottom, box.left],
    [box.bottom, box.right],
  ]) {
    walls.set(key(r, c), { box, side: "corner" });
  }
  const wall = (r: number, c: number, side: Side, rail: string) => {
    if (titles.get(key(r, c)) === box) return;
    walls.set(key(r, c), { box, side });
    const found = layout.glyph(r, c);
    if (found === rail || found === ports[side]) return;
    layout.flag(
      r,
      c,
      `\`${found}\` on the ${side} wall of ${box.name}; an outgoing edge there uses \`${ports[side]}\``,
    );
  };
  for (let c = box.left + 1; c < box.right; c++) {
    wall(box.top, c, "top", "─");
    wall(box.bottom, c, "bottom", "─");
  }
  for (let r = box.top + 1; r < box.bottom; r++) {
    wall(r, box.left, "left", "│");
    wall(r, box.right, "right", "│");
    if (box.right - box.left < 3) continue;
    if (layout.glyph(r, box.left + 1) !== " ")
      layout.flag(r, box.left + 1, `content touches the left wall of ${box.name}`);
    if (layout.glyph(r, box.right - 1) !== " ") {
      layout.flag(r, box.right - 1, `content touches the right wall of ${box.name}`);
    }
  }
}

function overlaps(layout: Layout, list: Box[]) {
  const nests = (outer: Box, inner: Box) =>
    outer.top < inner.top && outer.bottom > inner.bottom && outer.left < inner.left && outer.right > inner.right;
  const touches = (a: Box, b: Box) => a.top <= b.bottom && b.top <= a.bottom && a.left <= b.right && b.left <= a.right;
  for (const [i, a] of list.entries()) {
    for (const b of list.slice(i + 1)) {
      if (touches(a, b) && !nests(a, b) && !nests(b, a)) {
        layout.flag(b.top, b.left, `${b.name} overlaps ${a.name} without nesting`);
      }
    }
  }
}
