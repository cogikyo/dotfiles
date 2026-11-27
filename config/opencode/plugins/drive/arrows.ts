import { RGBA, type OptimizedBuffer } from "@opentui/core";
import { colors } from "../statusline/palette.ts";
import { cell, type Prompt } from "../statusline/prompt.ts";
import { mix } from "../statusline/working.tsx";

type Point = { readonly x: number; readonly y: number };

const boost = [
  { weight: 3, tone: 0.2 },
  { weight: 2, tone: 0.16 },
  { weight: 1, tone: 0.12 },
];
const tallest = 4;
const margin = 6;

function chevron(origin: Point, height: number, weight: number) {
  return Array.from({ length: height }, (_, row) => {
    const fold = height - 1 - row;
    const [tail, head] = row < fold ? "◥◣" : row > fold ? "◢◤" : "🭨🭬";
    const glyphs = [tail, ...Array.from({ length: weight - 1 }, () => "█"), head];
    const x = origin.x + Math.min(row, fold);
    return glyphs.map((glyph, column) => ({ x: x + column, y: origin.y + row, glyph }));
  }).flat();
}

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
  const clear = ({ x, y }: Point) =>
    x >= left &&
    !(cursor && cursor.y === y && Math.abs(cursor.x - x) <= 1) &&
    [x - 1, x, x + 1].every((column) => cell(buffer, column, y) === undefined) &&
    shade(x, y).toInts().slice(0, 3).join() === base;
  const height = Math.min(bottom - top, tallest);
  const fits = [];
  let tip = right - margin - 1;
  for (const { weight, tone } of boost) {
    const tall = Math.min(2 * weight - (height % 2), height);
    const origin = { x: tip - weight - Math.floor((tall - 1) / 2), y: bottom - (height + tall) / 2 };
    const marks = chevron(origin, tall, weight);
    if (!marks.every(clear)) break;
    fits.push({ marks, tone });
    tip -= 2 * weight;
  }
  for (const { marks, tone } of fits)
    for (const mark of marks)
      buffer.drawText(mark.glyph, mark.x, mark.y, mix(shade(mark.x, mark.y), colors.blue, tone));
}
