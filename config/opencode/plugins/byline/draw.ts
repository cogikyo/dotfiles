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
type Frame = { x: number; y: number; left: number; icon: string; tone: RGBA; tint: RGBA; p: number };
type Effect = (pen: Pen, frame: Frame) => void;
type Seen = { first: number; revealing: boolean; live: boolean; settled?: number };

const fill = 0.1;
const reveal = 600;
const settle = 300;
const fresh = 3_000;
const rings = ["·", "◦", "○", "◎"];
const pooled = 0x8000_0000;

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
    for (const entry of scene.lanes) this.#icon(pen, entry, left);
  }

  #icon(pen: Pen, { lane: { tone, role }, reply, mark }: Scene["lanes"][number], left: number) {
    const seen = reply && this.#seen.get(reply.id);
    if (!reply || !seen || !mark || mark.node.isDestroyed) return;
    const { node, x: offset, glyph: icon } = mark;
    const x = Math.trunc(node.screenX) + offset;
    const y = Math.trunc(node.screenY);
    if (!pen.claim(icon, x, y)) return;
    const now = performance.now();
    const p = reveals(seen, now);
    if (p !== undefined) return effects[role]?.(pen, { x, y, left, icon, tone, tint: mix(pen.base, tone, fill), p });
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
  readonly #tints = new Set<number>();
  #claimed = -1;
  readonly clip: Clip;
  readonly base: RGBA;

  private constructor(buffer: OptimizedBuffer, planes: { bg: Uint16Array; char: Uint32Array }, clip: Clip, base: RGBA) {
    this.#buffer = buffer;
    this.#bg = planes.bg;
    this.#chars = planes.char;
    this.#width = buffer.width;
    this.clip = clip;
    this.base = base;
  }

  static open(buffer: OptimizedBuffer, clip: Clip, base: RGBA) {
    const { bg, char }: { bg: unknown; char: unknown } = buffer.buffers;
    if (!(bg instanceof Uint16Array) || !(char instanceof Uint32Array)) return;
    const inside = {
      left: Math.max(0, clip.left),
      top: Math.max(0, clip.top),
      right: Math.min(buffer.width - 1, clip.right),
      bottom: Math.min(buffer.height - 1, clip.bottom),
    };
    return new Pen(buffer, { bg, char }, inside, base);
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
    return this.#plain(x, y) || this.#tints.has(pack(this.#read(x, y)));
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

const effects: Partial<Record<string, Effect>> = {
  build: (pen, { x, y, icon, tone, tint, p }) => {
    if (p < 0.3) {
      pen.glyph(icon, x, y - 1, mix(tint, tone, p / 0.3));
      pen.glyph(icon, x, y, tint);
      return;
    }
    const k = p < 0.5 ? 1 : 1 - (p - 0.5) / 0.5;
    pen.glyph(icon, x, y, heat(tone, k));
    if (p >= 0.7) return;
    const spark = p < 0.5 ? "✦" : "✧";
    pen.glyph(spark, x - 1, y, heat(tone, 0.8 * k));
    pen.glyph(spark, x + 1, y, heat(tone, 0.8 * k));
  },
  review: (pen, { x, y, left, icon, tone, tint, p }) => {
    if (p < 0.6) {
      const column = left + Math.round((x - left) * (p / 0.6));
      for (const [offset, amount] of [0.45, 0.25, 0.12].entries()) {
        if (column - offset >= left) pen.shade(column - offset, y, tone, amount);
      }
      pen.glyph(icon, x, y, tint);
      return;
    }
    const k = 1 - (p - 0.6) / 0.4;
    pen.shade(x, y, tone, 0.35 * k);
    pen.glyph(icon, x, y, heat(tone, k));
  },
  scout: (pen, { x, y, icon, tone, p }) => {
    if (p < 0.6) return pen.glyph(rings[Math.min(rings.length - 1, Math.floor(p / 0.15))], x, y, heat(tone, 0.4));
    pen.glyph(icon, x, y, heat(tone, 0.6 * (1 - (p - 0.6) / 0.4)));
  },
  verify: (pen, { x, y, icon, tone, tint, p }) => {
    if (p < 0.2) return pen.glyph(icon, x, y, tint);
    const k = p < 0.35 ? 1 : 1 - (p - 0.35) / 0.65;
    pen.shade(x, y, tone, 0.5 * k);
    pen.glyph(icon, x, y, heat(tone, k));
    if (p >= 0.35 && p < 0.8) pen.glyph("✓", x - 1, y, heat(tone, k));
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

function wave(since: number) {
  return (1 - Math.cos((Math.max(0, since) / period) * 2 * Math.PI)) / 2;
}

function near(channel: number, value: number) {
  return Math.abs((channel & 0xff) - byte(value)) <= 1;
}

function byte(value: number) {
  return Math.round(Math.min(1, Math.max(0, value)) * 255);
}
