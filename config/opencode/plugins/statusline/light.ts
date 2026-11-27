import type { OptimizedBuffer, Renderable } from "@opentui/core";
import { channels, ramp, smooth, tint, toward, type Painter } from "./pace.tsx";
import type { Prompt } from "./prompt.ts";

type Pose = { readonly glyph: number; readonly centre: number; readonly wide: number; readonly peak: number };

const lip = 0x2580;
const rail = { slim: 0x1fb83, fine: 0x1fb82 };
const rest = { wide: 0.2, peak: 0.5 };
const pace = { wide: 0.09, peak: 0.95 };
const swing = 0.33;
const blur = 0.5;
const grain = 8;
const period = 4000;

const key = Symbol.for("statusline.light");
const scope = globalThis as typeof globalThis & { [key]?: WeakSet<Renderable> };
const veiled = (scope[key] ??= new WeakSet());

export function veil(prompt: Prompt, on: boolean) {
  if (veiled.has(prompt.cap) === on) return false;
  if (on) veiled.add(prompt.cap);
  else veiled.delete(prompt.cap);
  return true;
}

export function light(prompt: Prompt): Painter {
  let on = 0;
  let go = 0;
  let phase = 0;
  const rgb = channels();
  let edge: Renderable | undefined;

  return {
    draw(buffer, aim, elapsed) {
      const hidden = veiled.has(prompt.cap);
      const lit = aim.hue !== undefined && !hidden;
      const want = { on: lit ? 1 : 0, go: lit && aim.pacing ? 1 : 0 };
      const hue = rgb(aim.hue);
      on = hidden ? 0 : toward(on, want.on, elapsed / ramp);
      go = on === 0 ? 0 : toward(go, want.go, elapsed / ramp);
      if (go === 0) phase = 0;
      else if (Number.isFinite(elapsed)) phase += (2 * Math.PI * elapsed) / period;
      if (edge?.parent !== prompt.cap) edge = prompt.cap.getChildren()[0];
      if (edge && on > 0) paint(buffer, edge, pose(on, go, phase, edge.width), hue);
      return on === want.on && go === want.go && want.go === 0;
    },
  };
}

function pose(on: number, go: number, phase: number, cells: number): Pose {
  const ease = smooth(go);
  const stretch = 1 + ease * blur * Math.abs(Math.cos(phase));
  return {
    glyph: on < 0.5 ? rail.slim : rail.fine,
    centre: 0.5 + ease * swing * Math.sin(phase),
    wide: Math.max((rest.wide + (pace.wide - rest.wide) * ease) * stretch, grain / cells),
    peak: (smooth(on) * (rest.peak + (pace.peak - rest.peak) * ease)) / Math.sqrt(stretch),
  };
}

function paint(buffer: OptimizedBuffer, edge: Renderable, { glyph, centre, wide, peak }: Pose, hue: readonly number[]) {
  const { width, height } = buffer;
  const { screenX: left, screenY: y, width: cells } = edge;
  if (y < 0 || y >= height) return;
  const { char, fg } = buffer.buffers;
  const to = Math.min(left + cells, width);
  for (let x = Math.max(left, 0); x < to; x++) {
    const cell = y * width + x;
    if (char[cell] !== lip) continue;
    char[cell] = glyph;
    tint(fg, cell, hue, peak * Math.exp(-((((x - left + 0.5) / cells - centre) / wide) ** 2)));
  }
}
