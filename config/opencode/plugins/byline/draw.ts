import { RGBA, type CliRenderer, type OptimizedBuffer, type Renderable } from "@opentui/core";
import { heat, mix, motion, period } from "../statusline/working.tsx";
import { layout, type Lane, type Mark, type Run } from "./lane.ts";
import type { Reply } from "./notice.ts";

export type Scene = {
  content: Renderable;
  viewport: Renderable;
  base: RGBA;
  runs: readonly Run[];
  lanes: readonly { lane: Lane; reply?: Reply; mark?: Mark }[];
};
type Clip = { left: number; top: number; right: number; bottom: number };
type Row = { y: number; left: number; right: number; start: number; x: number };
type Frame = Row & { icon: string; tone: RGBA; p: number };
type Look = { a?: number; hot?: number; band?: number; glyph?: string; noise?: string };
type Scope = { row: (x: number, frame: Frame) => Look; icon: (pen: Pen, frame: Frame) => void };
type Seen = { first: number; revealing: boolean; live: boolean; settled?: number };

const fill = 0.1;
const reveal = 1_200;
const settle = 300;
const fresh = 3_000;
const rings = ["·", "◦", "○", "◎"];
const pooled = 0x8000_0000;
const grain = ["░", "▒", "▓"];
const slam = 0.72;
const decode = 0.7;
const ember = RGBA.fromHex("#ffb86c");

export class Stage {
  #scene: Scene | undefined;
  #broken = false;
  readonly #mounted = Date.now();
  readonly #seen = new Map<string, Seen>();
  readonly #renderer: CliRenderer;
  readonly #post = (buffer: OptimizedBuffer) => {
    try {
      this.#draw(buffer);
    } catch {
      this.#scene = undefined;
      this.#broken = true;
      this.#renderer.requestRender();
    }
  };

  constructor(renderer: CliRenderer) {
    this.#renderer = renderer;
    renderer.addPostProcessFn(this.#post);
  }

  dispose() {
    this.#renderer.removePostProcessFn(this.#post);
    this.#scene = undefined;
  }

  get broken() {
    return this.#broken;
  }

  clear() {
    this.#scene = undefined;
  }

  update(scene: Scene) {
    this.#scene = scene;
    const now = performance.now();
    let moving = false;
    for (const { reply } of scene.lanes) {
      if (!reply) continue;
      const seen = this.#seen.get(reply.id) ?? {
        first: now,
        revealing: reply.created >= this.#mounted && Date.now() - reply.created < fresh,
        live: false,
      };
      if (seen.live && !reply.live) seen.settled = now;
      seen.live = reply.live;
      this.#seen.set(reply.id, seen);
      moving ||=
        reply.live || reveals(seen, now) !== undefined || (seen.settled !== undefined && now - seen.settled < settle);
    }
    return moving;
  }

  #draw(buffer: OptimizedBuffer) {
    const scene = this.#scene;
    if (!scene || scene.content.isDestroyed || scene.viewport.isDestroyed) return;
    const pen = Pen.open(buffer, bounds(scene.viewport), scene.base);
    if (!pen) return;
    const origin = Math.trunc(scene.content.screenX);
    const left = origin + layout.flow + layout.left;
    const right = origin + Math.trunc(scene.content.width) - 1 - layout.right;
    for (const run of scene.runs) {
      if (run.top.isDestroyed || run.bottom.isDestroyed) continue;
      const top = Math.trunc(run.top.screenY);
      const bottom = Math.trunc(run.bottom.screenY + run.bottom.height) - 1;
      pen.box({ left, top, right, bottom }, run.lane);
    }
    if (!motion()) return;
    for (const entry of scene.lanes) this.#lane(pen, entry, left, right);
  }

  #lane(pen: Pen, { lane: { tone, role }, reply, mark }: Scene["lanes"][number], left: number, right: number) {
    const seen = reply && this.#seen.get(reply.id);
    if (!reply || !seen || !mark || mark.node.isDestroyed) return;
    const { node, x: offset, glyph: icon } = mark;
    const x = Math.trunc(node.screenX) + offset;
    const y = Math.trunc(node.screenY);
    const claimed = pen.claim(icon, x, y);
    const now = performance.now();
    const p = reveals(seen, now);
    const scope = scopes[role];
    if (p !== undefined && scope) {
      const frame = { y, left, right, start: Math.trunc(node.screenX), x, icon, tone, p };
      pen.sweep(frame, scope.row);
      if (claimed) scope.icon(pen, frame);
      return;
    }
    if (!claimed) return;
    const since = now - seen.first - (seen.revealing ? reveal : 0);
    if (reply.live) return pen.glyph(icon, x, y, heat(tone, wave(since)));
    const ended = seen.settled;
    if (ended !== undefined && now - ended < settle) pen.glyph(icon, x, y, heat(tone, 1 - (now - ended) / settle));
  }
}

class Pen {
  readonly #buffer: OptimizedBuffer;
  readonly #bg: Uint16Array;
  readonly #chars: Uint32Array;
  readonly #width: number;
  readonly #fg: Uint16Array;
  readonly #tints = new Set<number>();
  readonly #owned = new Set<number>();
  #claimed = -1;
  readonly clip: Clip;
  readonly base: RGBA;

  private constructor(
    buffer: OptimizedBuffer,
    planes: { bg: Uint16Array; fg: Uint16Array; char: Uint32Array },
    clip: Clip,
    base: RGBA,
  ) {
    this.#buffer = buffer;
    this.#bg = planes.bg;
    this.#fg = planes.fg;
    this.#chars = planes.char;
    this.#width = buffer.width;
    this.clip = clip;
    this.base = base;
  }

  static open(buffer: OptimizedBuffer, clip: Clip, base: RGBA) {
    const { bg, fg, char }: { bg: unknown; fg: unknown; char: unknown } = buffer.buffers;
    if (!(bg instanceof Uint16Array) || !(fg instanceof Uint16Array) || !(char instanceof Uint32Array)) return;
    const inside = {
      left: Math.max(0, clip.left),
      top: Math.max(0, clip.top),
      right: Math.min(buffer.width - 1, clip.right),
      bottom: Math.min(buffer.height - 1, clip.bottom),
    };
    return new Pen(buffer, { bg, fg, char }, inside, base);
  }

  box({ left, top, right, bottom }: Clip, { tone }: Lane) {
    const tint = mix(this.base, tone, fill);
    this.#tints.add(pack(tint));
    for (let y = Math.max(top, this.clip.top); y <= Math.min(bottom, this.clip.bottom); y++) {
      const ruled = this.#inside(right, y) && this.#plain(right, y);
      for (let x = Math.max(left, this.clip.left); x <= Math.min(right - 1, this.clip.right); x++) {
        if (this.#plain(x, y)) this.#paint(x, y, tint);
      }
      if (ruled) this.#buffer.drawText("▎", right, y, tone);
    }
  }

  sweep(frame: Frame, look: Scope["row"]) {
    const { y, left, right, tone } = frame;
    for (let x = left; x <= right; x++) {
      if (!this.#inside(x, y) || !this.#ours(x, y)) continue;
      const { a = 1, hot = 0, glyph, noise, ...rest } = look(x, frame);
      const edge = x === right;
      const band = edge ? 0 : (rest.band ?? 0);
      const blank = this.#holds(x, y, " ");
      const bg = mix(mix(this.base, this.#read(x, y), a), tone, band);
      const fg = mix(mix(bg, this.#ink(x, y), a), ember, hot);
      this.#paint(x, y, bg);
      this.#color(x, y, fg);
      this.#owned.add(this.#cell(x, y));
      if (noise && !edge && !blank) this.#buffer.drawText(noise, x, y, mix(this.base, tone, 0.7));
      else if (glyph && a > 0) this.#buffer.drawText(glyph, x, y, mix(fg, heat(tone, 1), 0.5));
    }
  }

  claim(icon: string, x: number, y: number) {
    this.#claimed = this.#inside(x, y) && this.#ours(x, y) && this.#holds(x, y, icon) ? this.#cell(x, y) : -1;
    return this.#claimed !== -1;
  }

  glyph(char: string, x: number, y: number, fg: RGBA) {
    if (!this.#inside(x, y)) return;
    if (this.#cell(x, y) !== this.#claimed && !(this.#ours(x, y) && this.#holds(x, y, " "))) return;
    this.#buffer.drawText(char, x, y, fg);
  }

  shade(x: number, y: number, color: RGBA, amount: number) {
    if (!this.#inside(x, y)) return;
    if (this.#cell(x, y) !== this.#claimed && !this.#ours(x, y)) return;
    this.#paint(x, y, mix(this.#read(x, y), color, amount));
  }

  #cell(x: number, y: number) {
    return y * this.#width + x;
  }

  #ours(x: number, y: number) {
    return this.#owned.has(this.#cell(x, y)) || this.#plain(x, y) || this.#tints.has(pack(this.#read(x, y)));
  }

  #ink(x: number, y: number) {
    const at = (y * this.#width + x) * 4;
    const fg = this.#fg;
    return RGBA.fromInts(fg[at] & 0xff, fg[at + 1] & 0xff, fg[at + 2] & 0xff, 255);
  }

  #color(x: number, y: number, color: RGBA) {
    const at = (y * this.#width + x) * 4;
    this.#fg[at] = byte(color.r);
    this.#fg[at + 1] = byte(color.g);
    this.#fg[at + 2] = byte(color.b);
    this.#fg[at + 3] = 255;
  }

  #holds(x: number, y: number, char: string) {
    const held = this.#chars[this.#cell(x, y)];
    const code = char.codePointAt(0) ?? 0;
    if (char === " ") return held === code || held === 0;
    return held === code || (held >= pooled && code > 0xffff);
  }

  #inside(x: number, y: number) {
    return x >= this.clip.left && x <= this.clip.right && y >= this.clip.top && y <= this.clip.bottom;
  }

  #plain(x: number, y: number) {
    const at = (y * this.#width + x) * 4;
    const bg = this.#bg;
    if (((bg[at + 1] >> 8) & 0xff) === 2 || (bg[at + 3] & 0xff) === 0) return true;
    const base = this.base;
    return near(bg[at], base.r) && near(bg[at + 1], base.g) && near(bg[at + 2], base.b);
  }

  #read(x: number, y: number) {
    const at = (y * this.#width + x) * 4;
    const bg = this.#bg;
    return RGBA.fromInts(bg[at] & 0xff, bg[at + 1] & 0xff, bg[at + 2] & 0xff, 255);
  }

  #paint(x: number, y: number, color: RGBA) {
    const at = (y * this.#width + x) * 4;
    this.#bg[at] = byte(color.r);
    this.#bg[at + 1] = byte(color.g);
    this.#bg[at + 2] = byte(color.b);
    this.#bg[at + 3] = 255;
  }
}

const scopes: Partial<Record<string, Scope>> = {
  build: {
    row: (x, { left, right, start, p }) => {
      const head = 0.6;
      const span = Math.max(1, start - left);
      const at =
        x < start ? ((x - left) / span) * head : head + ((right - x) / Math.max(1, right - start)) * (slam - head);
      if (p >= at) return { hot: cool(p - at) };
      if (x < start && p < head && at - p <= head / span) return { glyph: "✦", hot: 1 };
      return { a: 0 };
    },
    icon: (pen, { x, y, icon, tone, p }) => {
      if (p < slam) return;
      const k = 1 - (p - slam) / (1 - slam);
      pen.glyph(icon, x, y, heat(tone, k));
      if (p >= 0.95) return;
      const spark = p < 0.84 ? "✦" : "✧";
      pen.glyph(spark, x - 1, y, heat(tone, k));
      pen.glyph(spark, x + 1, y, heat(tone, k));
    },
  },
  review: {
    row: (x, frame) => {
      const passed = scan(frame) - x;
      if (passed < 0) return { a: 0 };
      return { band: [0.45, 0.25, 0.12][Math.floor(passed)] ?? 0, hot: Math.max(0, 1 - passed / 6) * 0.5 };
    },
    icon: (pen, frame) => {
      const { x, y, icon, tone } = frame;
      const landed = scan(frame) - x;
      if (landed < 0) return;
      const k = Math.max(0, 1 - landed / 12);
      pen.shade(x, y, tone, 0.45 * k);
      pen.glyph(icon, x, y, heat(tone, k));
    },
  },
  scout: {
    row: (x, { left, right, x: icon, p }) => {
      const reach = Math.max(icon - left, right - icon) + 4;
      const front = reach * (1 - (1 - Math.min(1, p / 0.8)) ** 2) - Math.abs(x - icon);
      return { a: clamp(front / 4), band: front >= 0 && front < 2 ? 0.3 * (1 - front / 2) : 0 };
    },
    icon: (pen, { x, y, icon, tone, p }) => {
      if (p < 0.6) return pen.glyph(rings[Math.min(rings.length - 1, Math.floor(p / 0.15))], x, y, heat(tone, 1));
      pen.glyph(icon, x, y, heat(tone, 1 - (p - 0.6) / 0.4));
    },
  },
  verify: {
    row: (x, { left, right, p }) => {
      const front = left + (right + 1 - left) * (p / decode);
      if (x >= front) return { a: 0, noise: grain[Math.abs(Math.trunc(x * 7 + p * 40)) % grain.length] };
      const flash = p >= decode && p < 0.9 ? 1 - (p - decode) / 0.2 : 0;
      return { band: 0.35 * flash, hot: 0.4 * flash };
    },
    icon: (pen, { x, y, icon, tone, p }) => {
      if (p < decode) return;
      const k = 1 - (p - decode) / (1 - decode);
      pen.glyph(icon, x, y, heat(tone, k));
      pen.glyph("✓", x - 1, y, heat(tone, Math.max(k, 0.6)));
    },
  },
};

function bounds(node: Renderable): Clip {
  const left = Math.trunc(node.screenX);
  const top = Math.trunc(node.screenY);
  return { left, top, right: left + Math.trunc(node.width) - 1, bottom: top + Math.trunc(node.height) - 1 };
}

function reveals({ first, revealing }: Seen, now: number) {
  if (!revealing || now - first >= reveal) return;
  return (now - first) / reveal;
}

function pack(color: RGBA) {
  return (byte(color.r) << 16) | (byte(color.g) << 8) | byte(color.b);
}

function scan({ left, right, p }: Frame) {
  return left + (right + 3 - left) * (p / 0.75);
}

function cool(age: number) {
  return Math.max(0, 1 - (age * reveal) / 300);
}

function clamp(value: number) {
  return Math.min(1, Math.max(0, value));
}

function wave(since: number) {
  return (1 - Math.cos((Math.max(0, since) / period) * 2 * Math.PI)) / 2;
}

function near(channel: number, value: number) {
  return Math.abs((channel & 0xff) - byte(value)) <= 1;
}

function byte(value: number) {
  return Math.round(Math.min(1, Math.max(0, value)) * 255);
}
