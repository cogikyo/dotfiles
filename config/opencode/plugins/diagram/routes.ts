import { type Boxes, ports, type Side, type Wall } from "./boxes.ts";
import { type Dir, dirs, E, key, type Layout, N, type Point, S, step, W } from "./layout.ts";

const entry: Record<number, Side> = { [E]: "left", [W]: "right", [S]: "top", [N]: "bottom" };

export function connect(layout: Layout, boxes: Boxes) {
  for (const { r, c } of layout.cells()) {
    if (!layout.geometry(r, c)) continue;
    reciprocate(layout, boxes, { r, c });
    if (layout.tip(r, c)) arrive(layout, boxes, { r, c });
  }
}

function reciprocate(layout: Layout, { walls, titles }: Boxes, { r, c }: Point) {
  const here = layout.glyph(r, c);
  const arrow = layout.tip(r, c) !== 0;
  let endings = 0;
  for (const dir of dirs) {
    if (!(layout.mask(r, c) & dir.bit) || layout.joined(r, c, dir)) continue;
    const label = arrow ? undefined : layout.opaque(r, c, dir);
    if (label) {
      const title = titles.get(key(label.r, label.c));
      if (title && walls.get(key(r, c))?.box !== title)
        layout.flag(r, c, `\`${here}\` connects to the title of ${title.name}`);
      if (++endings > 1) layout.flag(r, c, `\`${here}\` ends at text in more than one direction`);
      continue;
    }
    const next = layout.glyph(r + dir.dr, c + dir.dc);
    const wall = walls.get(key(r + dir.dr, c + dir.dc));
    let reason = `faces \`${next}\`, which does not connect back`;
    if (next === " ") reason = "is dangling";
    else if (wall?.side === "corner") reason = `meets a corner of ${wall.box.name}`;
    else if (wall)
      reason = `meets the ${wall.side} wall of ${wall.box.name}; an outgoing edge there uses \`${ports[wall.side]}\``;
    layout.flag(r, c, `\`${here}\` ${dir.name} side ${reason}`);
  }
}

function arrive(layout: Layout, boxes: Boxes, point: Point) {
  const problem = landing(layout, boxes, point);
  if (problem) layout.flag(point.r, point.c, `\`${layout.glyph(point.r, point.c)}\` ${problem}`);
}

function landing(layout: Layout, { walls, titles }: Boxes, { r, c }: Point) {
  const dir = step(layout.tip(r, c));
  const [tr, tc] = [r + dir.dr, c + dir.dc];
  const target = walls.get(key(tr, tc));
  if (layout.inline(r, c)) {
    return target && `runs into the ${target.side} wall of ${target.box.name}; stop outside an unchanged wall`;
  }
  const facing = layout.cell(tr, tc);
  if (facing?.kind === "arrow") return "points at another arrow";
  if (facing?.kind === "stroke") return port(dir, facing.glyph, target);
  const label = layout.opaque(r, c, dir);
  if (label) {
    const title = titles.get(key(label.r, label.c));
    return title && `points at the title of ${title.name}`;
  }
  let [sr, sc] = [tr, tc];
  while (layout.inside(sr, sc) && layout.glyph(sr, sc) === " ") [sr, sc] = [sr + dir.dr, sc + dir.dc];
  return layout.inside(sr, sc) ? `leaves a gap before \`${layout.glyph(sr, sc)}\` at ${layout.at(sr, sc)}` : undefined;
}

function port(dir: Dir, glyph: string, target: Wall | undefined) {
  if (!target) return `points at \`${glyph}\`, which is not a box wall`;
  if (target.side === "corner") return `points at a corner of ${target.box.name}`;
  if (target.side !== entry[dir.bit]) return `reaches the ${target.side} wall of ${target.box.name} from inside`;
  const rail = dir.dc ? "│" : "─";
  return glyph === rail ? undefined : `must stop at an unchanged \`${rail}\`, not \`${glyph}\``;
}

export function edges(layout: Layout, boxes: Boxes) {
  const seen = new Set<string>();
  const lines: string[] = [];
  for (const start of layout.cells()) {
    if (!layout.geometry(start.r, start.c) || boxes.walls.has(key(start.r, start.c))) continue;
    if (seen.has(key(start.r, start.c))) continue;
    const route = walk(layout, boxes, start, seen);
    const from = [...route.sources].join(", ") || "?";
    if (!route.arrows)
      layout.flag(start.r, start.c, `route from ${from} has no arrowhead, so its direction is ambiguous`);
    lines.push(`- ${from} -> ${[...route.targets].join(", ") || "?"}`);
  }
  return lines.length ? `edges:\n${lines.join("\n")}` : "";
}

function walk(layout: Layout, { walls }: Boxes, start: Point, seen: Set<string>) {
  const route = { sources: new Set<string>(), targets: new Set<string>(), arrows: 0 };
  const queue = [start];
  seen.add(key(start.r, start.c));
  for (let point = queue.pop(); point; point = queue.pop()) {
    const { r, c } = point;
    const bit = layout.tip(r, c);
    if (bit) route.arrows++;
    if (bit && !layout.inline(r, c)) {
      const dir = step(bit);
      const wall = walls.get(key(r + dir.dr, c + dir.dc));
      const label = layout.opaque(r, c, dir);
      route.targets.add(wall ? wall.box.name : label ? layout.label(label) : "(edge of diagram)");
    }
    for (const dir of dirs) {
      if (!(layout.mask(r, c) & dir.bit)) continue;
      const next = { r: r + dir.dr, c: c + dir.dc };
      const label = bit ? undefined : layout.opaque(r, c, dir);
      const wall = walls.get(key(next.r, next.c));
      if (!layout.joined(r, c, dir)) {
        if (label) route.sources.add(layout.label(label));
      } else if (wall) {
        route.sources.add(wall.box.name);
      } else if (!seen.has(key(next.r, next.c))) {
        seen.add(key(next.r, next.c));
        queue.push(next);
      }
    }
  }
  return route;
}
