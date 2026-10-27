import type { Renderable, TextRenderable } from "@opentui/core";
import { heat, mix } from "../statusline/working.tsx";
import { clamp, fill, type Frame, type Pen, type Post } from "./draw.ts";

type Span = { from: number; to: number };
type Look = { t?: number; band?: number; glow?: number };
type Typing = { start: number; end: number; blink: number };

const sealed = "\u{f195b}";
const closed = "\u{f01ee}";
const opened = "\u{f01ef}";
const firework = "\u{f0e30}";
const bits = ["\u{f09de}", "\u{f09df}"];
const cell = 28;
const caps = { mail: 1_000, sent: 500 };
const open = 1_000;
const blink = 120;
const stamp = 240;
const segments = new Intl.Segmenter();

export function length({ kind, text }: Post) {
  const typing = Math.min(caps[kind], cells(text) * cell);
  return kind === "mail" ? open + typing + 3 * blink + stamp : typing + 600;
}

export function unseal(pen: Pen, frame: Frame, post: Post, ms: number) {
  const { x, y, tone } = frame;
  const typing = plan(post, open, 3 * blink);
  const shown = typing.end + typing.blink;
  const flash = 1 - phase(ms, shown, shown + stamp);
  const detail = extent(post.detail);
  pen.sweep(frame, (cx) => {
    const look = typed(post.text, typing, ms, cx);
    if (cx < detail.from || cx > detail.to || look.band) return look;
    return ms < shown ? { t: 0 } : { band: 0.35 * flash, glow: flash };
  });
  rows(pen, frame, post.body, phase(ms, typing.end - 100, typing.end + 50));
  if (ms < 480) {
    const shake = ms >= 120 && ms < 400 ? [-1, 1, -1, 1][Math.floor((ms - 120) / 70)] : 0;
    if (shake) pen.stamp(" ", x, y, tone);
    return pen.stamp(sealed, x + shake, y, heat(tone, 0.6));
  }
  if (ms < 800) {
    pen.stamp(closed, x, y, heat(tone, 0.5));
    if (ms < 560) return pen.stamp(firework, x + 1, y, heat(tone, 1));
    const age = phase(ms, 560, 800);
    const spread = 1 + Math.floor(age * 3);
    const ink = mix(heat(tone, 1), mix(pen.base, tone, fill), age);
    const bit = bits[age < 0.5 ? 0 : 1];
    pen.stamp(bit, x + 1 + spread, y, ink);
    pen.stamp(bit, x - spread, y, ink);
    return;
  }
  if (ms < shown) {
    const k = 1 - phase(ms, 800, open);
    pen.shade(x, y, tone, 0.5 * k);
    return pen.stamp(opened, x, y, heat(tone, Math.max(0.3, k)));
  }
  pen.glyph(frame.icon, x, y, heat(tone, flash));
}

export function seal(pen: Pen, frame: Frame, post: Post, ms: number) {
  const { x, y, tone } = frame;
  const typing = plan(post, 0, 0);
  const end = typing.end;
  pen.sweep(frame, (cx) => typed(post.text, typing, ms, cx));
  rows(pen, frame, post.body, phase(ms, end - 100, end + 50));
  if (ms < end) return pen.stamp(opened, x, y, heat(tone, 0.3));
  if (ms < end + 150) return pen.stamp(closed, x, y, heat(tone, 0.4));
  if (ms < end + 400) {
    const k = 1 - phase(ms, end + 150, end + 400);
    for (const at of [x - 1, x, x + 1]) pen.shade(at, y, tone, (at === x ? 0.5 : 0.25) * k);
    return pen.stamp(sealed, x, y, heat(tone, k));
  }
  pen.glyph(frame.icon, x, y, heat(tone, 1 - phase(ms, end + 400, end + 600)));
}

function plan({ kind, text }: Post, start: number, after: number): Typing {
  return { start, end: start + Math.min(caps[kind], cells(text) * cell), blink: after };
}

function typed(text: TextRenderable, { start, end, blink: after }: Typing, ms: number, cx: number): Look {
  const from = Math.trunc(text.screenX);
  const width = Math.min(cells(text), Math.trunc(text.width));
  const at = cx - from;
  if (at < 0 || at > width) return {};
  if (ms < start) return { t: 0 };
  const reached = width * phase(ms, start, end);
  const edge = ms < end ? reach(text.plainText, reached) : width;
  const lit = ms < end || (ms < end + after && Math.floor((ms - end) / blink) % 2 === 0);
  if (at === edge) return lit ? { t: 0, band: 0.85 } : { t: 0 };
  if (at > edge) return { t: 0 };
  const stamped = start + (end - start) * (at / Math.max(1, width));
  return { glow: 0.6 * (1 - phase(ms, stamped, stamped + 250)) };
}

function reach(value: string, reached: number) {
  let edge = 0;
  for (const { segment } of segments.segment(value)) {
    if (edge >= reached) return edge;
    edge += Bun.stringWidth(segment);
  }
  return edge;
}

function cells(text: TextRenderable) {
  return Bun.stringWidth(text.plainText);
}

function rows(pen: Pen, frame: Frame, node: Renderable, t: number) {
  if (t >= 1) return;
  const top = Math.trunc(node.screenY);
  for (let y = top; y < top + Math.trunc(node.height); y++) pen.sweep({ ...frame, y }, () => ({ t }));
}

function extent(node: Renderable): Span {
  const from = Math.trunc(node.screenX);
  return { from, to: from + Math.trunc(node.width) - 1 };
}

function phase(value: number, from: number, to: number) {
  if (to <= from) return value >= to ? 1 : 0;
  return clamp((value - from) / (to - from));
}
