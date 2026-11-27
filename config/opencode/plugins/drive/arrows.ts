import type { OptimizedBuffer, RGBA } from "@opentui/core";
import { ramp, smooth, tint, toward } from "../statusline/pace.tsx";
import type { Prompt } from "../statusline/prompt.ts";

type Aim = { readonly hue: RGBA; readonly peak: number; readonly pacing: boolean };

const chevron = 0x276f;
const pitch = 3;
const parked = 3;
const spread = 3.5;
const reach = 2 * spread;
const speed = 0.016;
const margin = 4;
const faint = 0.04;

export function arrows(prompt: Prompt) {
  let go = 0;
  let head = -reach;

  return (buffer: OptimizedBuffer, aim: Aim, elapsed: number) => {
    const track = lane(buffer, prompt);
    if (!track) return undefined;
    const want = aim.pacing ? 1 : 0;
    go = toward(go, want, elapsed / ramp);
    if (go === 0) head = -reach;
    else if (Number.isFinite(elapsed)) head += speed * elapsed;
    if (head > track.to - track.from + reach) head = -reach;
    const ease = smooth(go);
    const hue = aim.hue.toInts();
    const { char, fg, bg } = buffer.buffers;
    for (let slot = 0, x = track.to - 1; x >= track.from; slot++, x -= pitch) {
      const flow = Math.exp(-(((x - track.from - head) / spread) ** 2));
      const tone = aim.peak * ((slot < parked ? 1 - ease : 0) + ease * flow);
      if (tone < faint) continue;
      const cell = track.row + x;
      char[cell] = chevron;
      fg.set(bg.subarray(cell * 4, cell * 4 + 4), cell * 4);
      tint(fg, cell, hue, tone);
    }
    return go === want && want === 0;
  };
}

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
  return to - from > (parked - 1) * pitch ? { row, from, to } : undefined;
}
