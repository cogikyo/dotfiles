import type { CliRenderer, CursorState, OptimizedBuffer } from "@opentui/core";
import { type Aim, channels, ramp, smooth, tint, toward } from "../statusline/pace.tsx";
import { colors } from "../statusline/palette.ts";
import type { Prompt } from "../statusline/prompt.ts";

type Planes = OptimizedBuffer["buffers"];
type Track = NonNullable<ReturnType<typeof lane>>;

const chevron = 0x276f;
const pitch = 3;
const parked = 3;
const spread = 3.5;
const reach = 2 * spread;
const speed = 0.016;
const slow = 0.5;
const idle = 0.45;
const margin = 4;
const faint = 0.04;
const lull = 0.5;

export function arrows(renderer: CliRenderer, prompt: Prompt) {
  let go = 0;
  let work = 0;
  let head = 0;
  let heads: Float64Array = new Float64Array(0);
  const rgb = channels();
  const rest = colors.brightBlue.toInts();
  const hue = [0, 0, 0];

  const move = (track: Track, rows: number, step: number) => {
    const run = track.to - track.from;
    const span = track.right - track.left;
    heads = fit(heads, rows, span);
    head = go === 0 ? scatter(run) : travel(head, run, step);
    for (let y = 0; y < rows; y++) heads[y] = go === 0 ? scatter(span) : travel(heads[y] ?? 0, span, step);
  };

  return (buffer: OptimizedBuffer, aim: Aim, elapsed: number) => {
    const track = lane(buffer, prompt);
    if (!track) return undefined;
    const want = aim.motion ? 1 : 0;
    go = toward(go, want, elapsed / ramp);
    work = toward(work, aim.hue ? 1 : 0, elapsed / ramp);
    const heat = smooth(work);
    blend(hue, rest, rgb(aim.hue), heat);
    const peak = idle + (1 - idle) * heat;
    move(
      track,
      (track.row - track.top) / buffer.width,
      Number.isFinite(elapsed) ? speed * (slow + (1 - slow) * heat) * elapsed : 0,
    );
    const ease = smooth(go);
    const planes = buffer.buffers;
    for (let slot = 0, x = track.to - 1; x >= track.from; slot++, x -= pitch) {
      const tone = peak * ((slot < parked ? 1 - ease : 0) + ease * flow(x - track.from - head));
      if (tone >= faint) mark(planes, track.row + x, hue, tone);
    }
    if (ease > 0)
      field(buffer, track, renderer.getCursorState(), (cell, x, y) => {
        const tone = peak * ease * flow(x - track.left - (heads[y] ?? 0));
        if (tone >= faint) mark(planes, cell, hue, tone);
      });
    return go === want && want === 0;
  };
}

/** The editor's columns, the input's rows from the editor's top padding down to the model row, and the model row's longest blank stretch trimmed by `margin` where text borders it; undefined when that stretch is too short for the parked chevrons. */
function lane(buffer: OptimizedBuffer, prompt: Prompt) {
  const { width, height } = buffer;
  const y = prompt.cap.screenY - 1;
  const left = Math.max(prompt.editor.screenX, 0);
  const right = Math.min(prompt.editor.screenX + prompt.editor.width, width);
  if (y < 0 || y >= height) return undefined;
  const { char } = buffer.buffers;
  const row = y * width;
  let run = { from: left, to: left };
  for (let x = left, start = left; x <= right; x++) {
    const code = x < right ? char[row + x] : undefined;
    if (code === 0 || code === 32) continue;
    if (x - start > run.to - run.from) run = { from: start, to: x };
    start = x + 1;
  }
  const from = run.from === left ? left : run.from + margin;
  const to = run.to === right ? right : run.to - margin;
  const top = Math.max(prompt.editor.screenY - 1, prompt.frame.screenY, 0) * width;
  return to - from > (parked - 1) * pitch ? { top, row, left, right, from, to } : undefined;
}

function field(
  buffer: OptimizedBuffer,
  track: Track,
  cursor: CursorState,
  visit: (cell: number, x: number, y: number) => void,
) {
  const { width } = buffer;
  const planes = buffer.buffers;
  const { top, row, left, right, from, to } = track;
  const caret = cursor.visible ? (cursor.y - 1) * width + cursor.x - 1 : -1;
  for (let line = top, y = 0; line < row; line += width, y++) {
    for (let x = left, start = left; x <= right; x++) {
      if (x < right && line + x !== caret && open(planes, line + x, row + from)) continue;
      const end = x === right ? right : x - margin;
      for (let at = start === left ? left : start + margin; at < end; at++)
        if ((to - 1 - at) % pitch === 0) visit(line + at, at, y);
      start = x + 1;
    }
  }
}

function fit(heads: Float64Array, rows: number, span: number) {
  return heads.length === rows ? heads : new Float64Array(rows).map((_, y) => heads[y] ?? enter(span));
}

function scatter(span: number) {
  return Math.random() * (span + 2 * reach) - reach;
}

function travel(head: number, span: number, step: number) {
  const next = head + step;
  return next > span + reach ? enter(span) : next;
}

function enter(span: number) {
  return -reach - Math.random() * span * lull;
}

function blend(into: number[], from: readonly number[], to: readonly number[], amount: number) {
  for (let channel = 0; channel < 3; channel++) {
    const base = from[channel] ?? 0;
    into[channel] = base + ((to[channel] ?? base) - base) * amount;
  }
}

function flow(offset: number) {
  return Math.exp(-((offset / spread) ** 2));
}

function open({ char, bg }: Planes, cell: number, base: number) {
  const code = char[cell];
  if (code !== 0 && code !== 32) return false;
  for (let channel = 0; channel < 4; channel++) if (bg[cell * 4 + channel] !== bg[base * 4 + channel]) return false;
  return true;
}

function mark({ char, fg, bg }: Planes, cell: number, hue: readonly number[], tone: number) {
  char[cell] = chevron;
  for (let channel = 0; channel < 4; channel++) fg[cell * 4 + channel] = bg[cell * 4 + channel] ?? 0;
  tint(fg, cell, hue, tone);
}
