import { RGBA, type OptimizedBuffer } from "@opentui/core";
import { colors } from "../statusline/palette.ts";
import { cell, type Prompt } from "../statusline/prompt.ts";
import { mix } from "../statusline/working.tsx";

type Point = { readonly x: number; readonly y: number };

const glyph = "\u{203a}";
const step = 6;
const tone = 0.27;

export function arrows(buffer: OptimizedBuffer, prompt: Prompt, cursor?: Point) {
  const inner = prompt.frame.getChildren()[0];
  if (!inner) return;
  const { bg } = buffer.buffers;
  const shade = (x: number, y: number) => {
    const index = (y * buffer.width + x) * 4;
    return RGBA.fromArray(bg.slice(index, index + 4));
  };
  const left = Math.max(inner.screenX, 0);
  const right = Math.min(inner.screenX + inner.width, buffer.width);
  const top = Math.max(prompt.editor.screenY - 1, inner.screenY, 0);
  const bottom = Math.min(inner.screenY + inner.height, buffer.height);
  if (left >= right || top >= bottom) return;
  const base = shade(left, top).toInts().slice(0, 3).join();
  for (let y = top; y < bottom; y++) {
    for (let x = left; x < right; x++) {
      if ((right - 1 - x + (y - top) * (step / 2)) % step !== 0) continue;
      if (cursor && cursor.y === y && Math.abs(cursor.x - x) <= 1) continue;
      if ([x - 1, x, x + 1].some((column) => cell(buffer, column, y) !== undefined)) continue;
      const here = shade(x, y);
      if (here.toInts().slice(0, 3).join() !== base) continue;
      buffer.drawText(glyph, x, y, mix(here, colors.blue, tone));
    }
  }
}
