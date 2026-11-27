import { OptimizedBuffer, type CliRenderer, type RGBA, type Renderable } from "@opentui/core";
import { edge, glyph } from "../statusline/delivery.tsx";
import { cell } from "../statusline/prompt.ts";

const icon = "\u{f04d4}";
const span = Bun.stringWidth(glyph.steer);

export function wheel(renderer: CliRenderer) {
  let sample: OptimizedBuffer | undefined;
  let steer = 0;

  const draw = (buffer: OptimizedBuffer, editor: Renderable, color: RGBA) => {
    // Render the steer glyph once to learn the code the buffer stores for it, so the wheel overwrites that icon and nothing else.
    if (!sample) {
      sample = OptimizedBuffer.create(span, 1, renderer.widthMethod, {
        owner: renderer.nativeScene,
      });
      sample.drawText(glyph.steer, 0, 0, color);
      steer = sample.withBuffers((cells) => cells.char[0] ?? 0);
    }
    const x = editor.screenX + editor.width - span;
    const y = editor.screenY;
    const from = edge(editor);
    if (from < editor.screenX || y < 0 || x >= buffer.width || y >= buffer.height) return;
    for (let column = from; column < x; column++) if (cell(buffer, column, y) !== undefined) return;
    const code = buffer.buffers.char[y * buffer.width + x] ?? 0;
    if (code !== 0 && code !== 32 && code !== steer) return;
    buffer.drawText(icon, x, y, color);
  };

  return { draw, close: () => sample?.destroy() };
}
