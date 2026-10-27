import { ancestors, conditional, type Step } from "./parse.ts";
import { ink, pill, type Row, type Segment } from "./style.ts";

type Tree =
  | { kind: "step"; step: Step }
  | { kind: "rail" }
  | { kind: "series"; parts: Tree[] }
  | { kind: "parallel"; branches: Tree[] };

type Edge = { from: string; to: string; tree: Tree };
type Point = { x: number; y: number };
type Size = { width: number; height: number };
type Cell = Omit<Segment, "text"> & { char: string };
type Ends = { open: boolean; close: boolean };
type Side = { kind: "node" | "fork" | "rail"; dashed: boolean };

const rail: Tree = { kind: "rail" };
const junctions: Partial<Record<string, string>> = { "─│": "┼", "╭│": "├", "╮│": "┤", "┬│": "┼" };
const reduced = new WeakMap<readonly Step[], Tree | null>();
const sizes = new WeakMap<Tree, Size>();

export function draw(steps: readonly Step[], width: number): Row[] | undefined {
  if (steps.length < 2 || 5 * chain(steps) - 4 > width) return;
  if (!reduced.has(steps)) reduced.set(steps, reduce(steps) ?? null);
  const tree = reduced.get(steps);
  if (!tree) return;
  const loops = steps.filter(({ repair }) => repair).length;
  const top = loops ? loops + 1 : 0;
  const sketch = new Sketch();
  sketch.place(tree, 0, top, { open: true, close: true });
  if (!sketch.loop(steps, top) || sketch.canvas.broken || sketch.canvas.width > width) return;
  return sketch.canvas.rows();
}

function chain(steps: readonly Step[]) {
  const deps = new Map(steps.map(({ n, after }) => [n, after]));
  const lengths = new Map<number, number>();
  const length = (n: number): number => {
    const known = lengths.get(n);
    if (known !== undefined) return known;
    lengths.set(n, 0);
    const value = 1 + Math.max(0, ...(deps.get(n) ?? []).map(length));
    lengths.set(n, value);
    return value;
  };
  return Math.max(...steps.map(({ n }) => length(n)));
}

function reduce(steps: readonly Step[]): Tree | undefined {
  const order = new Map(steps.map(({ n }, index) => [n, index]));
  const rank = (tree: Tree) => Math.min(Infinity, ...members(tree).map(({ n }) => order.get(n) ?? Infinity));
  const edges = wire(steps);
  let joins = 0;
  while (merge(edges, rank) || (joins < steps.length && factor(edges, `j${joins++}`)));
  const [only] = edges;
  return edges.length === 1 && only.from === "S" && only.to === "E" ? only.tree : undefined;
}

function wire(steps: readonly Step[]): Edge[] {
  const deps = trim(steps);
  const fed = new Set([...deps.values()].flat());
  return steps.flatMap((step): Edge[] => {
    const into = deps.get(step.n) ?? [];
    return [
      { from: `i${step.n}`, to: `o${step.n}`, tree: { kind: "step", step } },
      ...into.map((n) => ({ from: `o${n}`, to: `i${step.n}`, tree: rail })),
      ...(into.length ? [] : [{ from: "S", to: `i${step.n}`, tree: rail }]),
      ...(fed.has(step.n) ? [] : [{ from: `o${step.n}`, to: "E", tree: rail }]),
    ];
  });
}

function trim(steps: readonly Step[]) {
  const known = new Map(steps.map((step) => [step.n, step]));
  const upstream = ancestors(steps);
  // Conditional predecessors may be skipped, so keep their upstream dependencies explicit.
  const covers = (by: number, n: number) => {
    const step = known.get(by);
    return by !== n && step !== undefined && !conditional(step) && upstream.get(by)?.has(n);
  };
  return new Map(steps.map(({ n, after }) => [n, after.filter((dep) => !after.some((by) => covers(by, dep)))]));
}

function merge(edges: Edge[], rank: (tree: Tree) => number) {
  const first = new Map<string, number>();
  for (const [index, edge] of edges.entries()) {
    const key = `${edge.from}>${edge.to}`;
    const twin = first.get(key);
    if (twin === undefined) {
      first.set(key, index);
      continue;
    }
    edges[twin] = { ...edges[twin], tree: parallel(edges[twin].tree, edge.tree, rank) };
    edges.splice(index, 1);
    return true;
  }
  const outs = Map.groupBy(edges, ({ from }) => from);
  const ins = Map.groupBy(edges, ({ to }) => to);
  for (const [index, edge] of edges.entries()) {
    const [out, ...rest] = outs.get(edge.to) ?? [];
    if (!out || rest.length || out === edge || ins.get(edge.to)?.length !== 1) continue;
    edges[index] = { from: edge.from, to: out.to, tree: series(edge.tree, out.tree) };
    edges.splice(edges.indexOf(out), 1);
    return true;
  }
  return false;
}

function factor(edges: Edge[], junction: string) {
  const sources = new Map<string, Set<string>>();
  for (const { from, to, tree } of edges) {
    if (tree.kind === "rail") sources.set(to, (sources.get(to) ?? new Set()).add(from));
  }
  const join = biclique(sources);
  if (!join) return false;
  const doomed = edges.filter(
    ({ from, to, tree }) => tree.kind === "rail" && join.from.includes(from) && join.to.includes(to),
  );
  for (const edge of doomed) edges.splice(edges.indexOf(edge), 1);
  edges.push(
    ...join.from.map((from) => ({ from, to: junction, tree: rail })),
    ...join.to.map((to) => ({ from: junction, to, tree: rail })),
  );
  return true;
}

function biclique(sources: ReadonlyMap<string, ReadonlySet<string>>) {
  const targets = [...sources.keys()];
  const feeds = (target: string, from: string) => sources.get(target)?.has(from) ?? false;
  let best: { from: string[]; to: string[] } | undefined;
  for (const [index, first] of targets.entries()) {
    for (const second of targets.slice(index + 1)) {
      const from = [...(sources.get(first) ?? [])].filter((source) => feeds(second, source));
      const to = targets.filter((target) => from.every((source) => feeds(target, source)));
      const better = !best || from.length * to.length > best.from.length * best.to.length;
      if (from.length >= 2 && better) best = { from, to };
    }
  }
  return best;
}

function series(...trees: Tree[]): Tree {
  const parts = trees.flatMap((tree) => (tree.kind === "series" ? tree.parts : tree.kind === "rail" ? [] : [tree]));
  return parts.length > 1 ? { kind: "series", parts } : (parts[0] ?? rail);
}

function parallel(a: Tree, b: Tree, rank: (tree: Tree) => number): Tree {
  const all = [a, b].flatMap((tree) => (tree.kind === "parallel" ? tree.branches : [tree]));
  const branches = all.filter((tree, index) => tree.kind !== "rail" || all.indexOf(rail) === index);
  if (branches.length === 1) return branches[0];
  return { kind: "parallel", branches: branches.toSorted((x, y) => rank(x) - rank(y)) };
}

function members(tree: Tree): Step[] {
  switch (tree.kind) {
    case "step":
      return [tree.step];
    case "series":
      return tree.parts.flatMap(members);
    case "parallel":
      return tree.branches.flatMap(members);
    default:
      return [];
  }
}

function lead(tree: Tree): Step | undefined {
  if (tree.kind === "series") return lead(tree.parts[0]);
  return tree.kind === "step" ? tree.step : undefined;
}

function side(tree: Tree, end: "head" | "tail"): Side {
  switch (tree.kind) {
    case "series":
      return side(tree.parts[end === "head" ? 0 : tree.parts.length - 1], end);
    case "step":
      return { kind: "node", dashed: conditional(tree.step) };
    case "parallel":
      return { kind: "fork", dashed: false };
    case "rail":
      return { kind: "rail", dashed: false };
  }
}

function link(a: Tree, b: Tree) {
  const tail = side(a, "tail");
  const head = side(b, "head");
  const dash = tail.dashed || head.dashed ? "╌" : "─";
  return `${tail.kind === "node" ? " " : ""}${dash}${head.kind === "node" ? "→ " : ""}`;
}

function enter(tree: Tree) {
  const head = side(tree, "head");
  if (head.kind === "rail") return "";
  return `${head.dashed ? "╌" : "─"}${head.kind === "node" ? "→ " : ""}`;
}

function leave(tree: Tree) {
  const tail = side(tree, "tail");
  if (tail.kind === "rail") return "";
  return `${tail.kind === "node" ? " " : ""}${tail.dashed ? "╌" : "─"}`;
}

function label({ n, owner }: Step): { pieces: Segment[]; offset: number } {
  const style = ink(owner);
  if (owner.kind === "gate") return { pieces: [{ text: `◆${n}`, ...style }], offset: 1 };
  if (owner.kind === "you") return { pieces: [{ text: `\u{f007} ${n}`, ...style }], offset: 2 };
  if (owner.kind === "lane" && owner.lane) return { pieces: pill(String(n), style.tone), offset: 1 };
  return { pieces: [{ text: `${n}`, ...style }], offset: 0 };
}

function measure(tree: Tree): Size {
  const known = sizes.get(tree);
  if (known) return known;
  const value = extent(tree);
  sizes.set(tree, value);
  return value;
}

function extent(tree: Tree): Size {
  switch (tree.kind) {
    case "step":
      return { width: label(tree.step).pieces.reduce((sum, { text }) => sum + Bun.stringWidth(text), 0), height: 1 };
    case "rail":
      return { width: 0, height: 1 };
    case "series": {
      const parts = tree.parts.map(measure);
      const links = tree.parts.slice(1).map((part, index) => link(tree.parts[index], part).length);
      return {
        width: [...parts.map(({ width }) => width), ...links].reduce((sum, width) => sum + width, 0),
        height: Math.max(...parts.map(({ height }) => height)),
      };
    }
    case "parallel": {
      const branches = tree.branches.map((branch) => {
        const { width, height } = measure(branch);
        return { width: enter(branch).length + width + leave(branch).length, height };
      });
      return {
        width: 2 + Math.max(...branches.map(({ width }) => width)),
        height: branches.reduce((sum, { height }) => sum + height, 0),
      };
    }
  }
}

class Canvas {
  private readonly grid: ((Cell | undefined)[] | undefined)[] = [];
  broken = false;

  get width() {
    return Math.max(0, ...Array.from(this.grid, (row) => row?.length ?? 0));
  }

  put(x: number, y: number, text: string, style: Omit<Cell, "char"> = { tone: "muted" }) {
    for (const char of text) this.set(x++, y, { char, ...style });
  }

  clear(x: number, y: number, length: number) {
    const row = this.grid[y] ?? [];
    return Array.from({ length }, (_, at) => row[x + at]).every((cell) => cell === undefined);
  }

  rows(): Row[] {
    const blank: Cell = { char: " ", tone: "muted" };
    const first = Math.max(
      0,
      this.grid.findIndex((row) => row?.length),
    );
    return Array.from(this.grid.slice(first), (row = []) =>
      Array.from(row, (cell = blank) => cell).reduce<Segment[]>((segments, cell) => {
        const last = segments.at(-1);
        const { char, ...style } = cell;
        const same = last?.tone === style.tone && last.bold === style.bold && last.italic === style.italic;
        if (last && same && last.bg === style.bg) last.text += char;
        else segments.push({ text: char, ...style });
        return segments;
      }, []),
    );
  }

  private set(x: number, y: number, cell: Cell) {
    const row = (this.grid[y] ??= []);
    const prev = row[x];
    const char = prev && prev.char !== cell.char ? junctions[prev.char + cell.char] : cell.char;
    if (char) row[x] = { ...(prev ?? cell), char };
    else this.broken = true;
  }
}

class Sketch {
  readonly canvas = new Canvas();
  private readonly nodes = new Map<number, Point>();
  private readonly forks: { point: Point; heads: (number | undefined)[] }[] = [];

  place(tree: Tree, x: number, y: number, ends: Ends) {
    switch (tree.kind) {
      case "step": {
        const { pieces, offset } = label(tree.step);
        let at = x;
        for (const { text, ...style } of pieces) {
          this.canvas.put(at, y, text, style);
          at += Bun.stringWidth(text);
        }
        this.nodes.set(tree.step.n, { x: x + offset, y });
        return;
      }
      case "rail":
        return;
      case "series":
        return this.series(tree.parts, x, y, ends);
      case "parallel":
        return this.parallel(tree, x, y, ends);
    }
  }

  loop(steps: readonly Step[], top: number) {
    const arcs = steps.flatMap(({ n, repair }) => {
      if (!repair) return [];
      return [{ gate: this.nodes.get(n), drops: this.drops(repair.to, top), tag: `repair ≤${repair.max}` }];
    });
    const span = ({ gate, drops = [] }: (typeof arcs)[number]) => (gate?.x ?? 0) - Math.min(...drops);
    const placed: { left: number; right: number; level: number }[] = [];
    const tags: { x: number; row: number; tag: string }[] = [];
    for (const { gate, drops, tag } of arcs.toSorted((a, b) => span(a) - span(b))) {
      if (!gate || gate.y !== top || !drops || drops.some((x) => x >= gate.x)) return false;
      const left = Math.min(...drops);
      const right = gate.x + 2 + tag.length;
      const below = placed.filter((arc) => arc.left <= right && left <= arc.right).map((arc) => arc.level);
      const level = Math.max(-1, ...below) + 1;
      placed.push({ left, right, level });
      const row = top - 2 - level;
      const rule = Array.from({ length: gate.x - left + 1 }, (_, at) =>
        at === 0 ? "╭" : at === gate.x - left ? "╮" : drops.includes(left + at) ? "┬" : "─",
      );
      this.canvas.put(left, row, rule.join(""));
      for (const x of [...drops, gate.x]) {
        for (let y = row + 1; y < top - 1; y++) this.canvas.put(x, y, "│");
        this.canvas.put(x, top - 1, x === gate.x ? "│" : "↓");
      }
      tags.push({ x: gate.x + 2, row, tag });
    }
    for (const { x, row, tag } of tags) {
      let at = x;
      while (!this.canvas.clear(at - 1, row, tag.length + 2)) at++;
      this.canvas.put(at, row, tag, { tone: "orange" });
    }
    return true;
  }

  private drops(targets: readonly number[], top: number) {
    const fork = this.forks.find(
      ({ heads }) => targets.length > 1 && heads.length === targets.length && targets.every((n) => heads.includes(n)),
    );
    if (fork) return fork.point.y === top ? [fork.point.x] : undefined;
    const points = targets.map((n) => this.nodes.get(n));
    if (points.some((point) => point?.y !== top)) return;
    return points.flatMap((point) => (point ? [point.x] : []));
  }

  private series(parts: readonly Tree[], x: number, y: number, { open, close }: Ends) {
    let at = x;
    for (const [index, part] of parts.entries()) {
      if (index > 0) {
        const text = link(parts[index - 1], part);
        this.canvas.put(at, y, text);
        at += text.length;
      }
      this.place(part, at, y, { open: open && index === 0, close: close && index === parts.length - 1 });
      at += measure(part).width;
    }
  }

  private parallel(tree: Extract<Tree, { kind: "parallel" }>, x: number, y: number, { open, close }: Ends) {
    const { branches } = tree;
    const join = x + measure(tree).width - 1;
    this.forks.push({ point: { x, y }, heads: branches.map((branch) => lead(branch)?.n) });
    let top = y;
    for (const [index, branch] of branches.entries()) {
      const last = index === branches.length - 1;
      this.canvas.put(x, top, index === 0 ? (open ? "╭" : "┬") : last ? "╰" : "├");
      this.canvas.put(join, top, index === 0 ? (close ? "╮" : "┬") : last ? "╯" : "┤");
      const into = enter(branch);
      const out = leave(branch);
      const { width, height } = measure(branch);
      this.canvas.put(x + 1, top, into);
      this.place(branch, x + 1 + into.length, top, { open: false, close: false });
      const after = x + 1 + into.length + width;
      this.canvas.put(after, top, out.padEnd(join - after, out.at(-1) ?? "─"));
      if (!last) {
        for (let row = top + 1; row < top + height; row++) {
          this.canvas.put(x, row, "│");
          this.canvas.put(join, row, "│");
        }
      }
      top += height;
    }
  }
}
