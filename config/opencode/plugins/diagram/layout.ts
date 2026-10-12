export const N = 1;
export const E = 2;
export const S = 4;
export const W = 8;

export const dirs = [
  { bit: N, dr: -1, dc: 0, name: "north" },
  { bit: E, dr: 0, dc: 1, name: "east" },
  { bit: S, dr: 1, dc: 0, name: "south" },
  { bit: W, dr: 0, dc: -1, name: "west" },
] as const;
export type Dir = (typeof dirs)[number];

export const strokes = new Map([
  ["─", E | W],
  ["│", N | S],
  ["┌", E | S],
  ["┐", S | W],
  ["└", N | E],
  ["┘", N | W],
  ["├", N | E | S],
  ["┤", N | S | W],
  ["┬", E | S | W],
  ["┴", N | E | W],
  ["┼", N | E | S | W],
]);
const tips = new Map([
  ["▶", E],
  ["◀", W],
  ["▲", N],
  ["▼", S],
]);
const foreign = /^[\u2190-\u21ff\u2500-\u257f\u25b2-\u25c5\u27f0-\u27ff\u2900-\u297f]/u;
const ascii = /-{2,}>|<-{2,}|\+-{2,}|-{2,}\+/u;
const segmenter = new Intl.Segmenter();

export type Cell =
  | { kind: "space" }
  | { kind: "text"; glyph: string; span: number }
  | { kind: "stroke" | "arrow"; glyph: string };
export type Point = { r: number; c: number };

export class Layout {
  readonly grid: Cell[][] = [];
  readonly spans: string[] = [];
  readonly violations: string[] = [];
  readonly widest = { cells: 0, row: 0 };
  readonly cols: number;
  private span = -1;
  private gap = 0;

  constructor(
    lines: string[],
    private readonly first: number,
    prefix: string,
    width?: number,
  ) {
    const blank = prefix.trimEnd();
    const lead = hostedWidth(prefix);
    for (const [r, line] of lines.entries()) {
      const row: Cell[] = [];
      this.grid.push(row);
      if (line.trim() === "" || line.trimEnd() === blank) continue;
      if (!line.startsWith(prefix)) {
        this.flag(r, 0, `does not start with the host prefix ${JSON.stringify(prefix)}`);
        continue;
      }
      this.parse(r, row, line.slice(prefix.length));
      const hosted = lead + row.length;
      if (hosted > this.widest.cells) Object.assign(this.widest, { cells: hosted, row: r });
      if (width !== undefined && hosted > width) this.flag(r, 0, `is ${hosted} cells wide; the limit is ${width}`);
    }
    this.cols = Math.max(0, ...this.grid.map((row) => row.length));
  }

  private parse(r: number, row: Cell[], bare: string) {
    if (/^\s*(```|~~~)/u.test(bare)) this.flag(r, 0, "is a fence line; keep fences outside the layout region");
    const drawn = ascii.exec(bare);
    if (drawn) this.flag(r, drawn.index, `uses ASCII geometry ${JSON.stringify(drawn[0])}`);
    this.span = -1;
    this.gap = 0;
    for (const { segment } of segmenter.segment(bare)) this.place(r, row, segment);
  }

  private place(r: number, row: Cell[], segment: string) {
    const c = row.length;
    if (segment === " " || segment === "\t") {
      if (segment === "\t") this.flag(r, c, "has a tab; tabs have no fixed display width");
      row.push({ kind: "space" });
      this.gap++;
      return;
    }
    if (strokes.has(segment) || tips.has(segment)) {
      row.push({ kind: strokes.has(segment) ? "stroke" : "arrow", glyph: segment });
      this.span = -1;
      this.gap = 0;
      return;
    }
    const problem = inspect(segment);
    if (problem) this.flag(r, c, problem);
    const cells = Bun.stringWidth(segment);
    if (cells === 0) return;
    if (this.span >= 0 && this.gap <= 1) this.spans[this.span] += `${this.gap ? " " : ""}${segment}`;
    else this.span = this.spans.push(segment) - 1;
    this.gap = 0;
    for (let i = 0; i < cells; i++) row.push({ kind: "text", glyph: i ? "" : segment, span: this.span });
  }

  flag(r: number, c: number, message: string) {
    this.violations.push(`${this.at(r, c)} ${message}`);
  }

  at(r: number, c: number) {
    return `${r + this.first}:${c + 1}`;
  }

  *cells() {
    for (const [r, row] of this.grid.entries()) for (const c of row.keys()) yield { r, c };
  }

  cell(r: number, c: number): Cell | undefined {
    return this.grid[r]?.[c];
  }

  glyph(r: number, c: number) {
    const found = this.cell(r, c);
    return found && found.kind !== "space" ? found.glyph : " ";
  }

  inside(r: number, c: number) {
    return r >= 0 && r < this.grid.length && c >= 0 && c < this.cols;
  }

  geometry(r: number, c: number) {
    const kind = this.cell(r, c)?.kind;
    return kind === "stroke" || kind === "arrow";
  }

  has(r: number, c: number, bits: number) {
    return ((strokes.get(this.glyph(r, c)) ?? 0) & bits) === bits;
  }

  text(r: number, c: number) {
    const found = this.cell(r, c);
    return found?.kind === "text" ? found.span : undefined;
  }

  label(point: Point) {
    const span = this.text(point.r, point.c);
    return span === undefined ? "?" : JSON.stringify(this.spans[span]);
  }

  tip(r: number, c: number) {
    return tips.get(this.glyph(r, c)) ?? 0;
  }

  inline(r: number, c: number) {
    const bit = this.tip(r, c);
    const dir = step(bit);
    const [nr, nc] = [r + dir.dr, c + dir.dc];
    return this.geometry(nr, nc) && (this.raw(nr, nc) & opposite(bit)) !== 0;
  }

  mask(r: number, c: number) {
    const bit = this.tip(r, c);
    if (!bit) return this.raw(r, c);
    return this.inline(r, c) ? bit | opposite(bit) : opposite(bit);
  }

  joined(r: number, c: number, dir: Dir) {
    return (this.mask(r, c) & dir.bit) !== 0 && (this.mask(r + dir.dr, c + dir.dc) & opposite(dir.bit)) !== 0;
  }

  opaque(r: number, c: number, dir: Dir): Point | undefined {
    const near = { r: r + dir.dr, c: c + dir.dc };
    const far = { r: r + 2 * dir.dr, c: c + 2 * dir.dc };
    if (this.text(near.r, near.c) !== undefined) return near;
    if (this.glyph(near.r, near.c) === " " && this.text(far.r, far.c) !== undefined) return far;
    return undefined;
  }

  private raw(r: number, c: number) {
    const found = this.cell(r, c);
    if (found?.kind === "stroke") return strokes.get(found.glyph) ?? 0;
    if (found?.kind === "arrow") return opposite(tips.get(found.glyph) ?? 0);
    return 0;
  }
}

export function step(bit: number) {
  return dirs.find((dir) => dir.bit === bit) ?? dirs[0];
}

export function opposite(bit: number) {
  if (bit === N) return S;
  if (bit === S) return N;
  return bit === E ? W : E;
}

export function key(r: number, c: number) {
  return `${r},${c}`;
}

function inspect(segment: string) {
  const code = segment.codePointAt(0) ?? 0;
  const base = String.fromCodePoint(code);
  if (base === segment && /[\p{Cc}\p{Cf}]/u.test(segment))
    return `has invisible or control character ${codes(segment)}`;
  if (strokes.has(base) || tips.has(base)) {
    return `\`${base}\` carries ${codes(segment.slice(base.length))}; renderers may draw it wide or as emoji`;
  }
  if (foreign.test(segment)) return `\`${segment}\` is outside the geometry alphabet (light box glyphs and ▶ ◀ ▲ ▼)`;
  if (Bun.stringWidth(segment) === 0) return `\`${segment}\` has zero display width`;
  return undefined;
}

function hostedWidth(prefix: string) {
  let cells = 0;
  for (const { segment } of segmenter.segment(prefix)) {
    cells = segment === "\t" ? cells + 8 - (cells % 8) : cells + Bun.stringWidth(segment);
  }
  return cells;
}

function codes(value: string) {
  const out: string[] = [];
  for (const char of value) out.push(`U+${(char.codePointAt(0) ?? 0).toString(16).toUpperCase().padStart(4, "0")}`);
  return out.join(" ");
}
