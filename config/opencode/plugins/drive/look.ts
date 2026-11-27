import type { CliRenderer, RGBA } from "@opentui/core";
import { veil } from "../statusline/light.ts";
import type { Painter } from "../statusline/pace.tsx";
import { colors } from "../statusline/palette.ts";
import type { Prompt } from "../statusline/prompt.ts";
import { arrows } from "./arrows.ts";
import { wheel } from "./wheel.ts";

const idle = 0.45;

export const mark = (hue: RGBA | undefined) => hue ?? colors.brightBlue;

export function look(renderer: CliRenderer, prompt: Prompt): Painter {
  const icon = wheel(renderer);
  const flow = arrows(prompt);

  return {
    draw(buffer, aim, elapsed) {
      const hue = mark(aim.hue);
      icon.draw(buffer, prompt.editor, hue);
      const still = flow(buffer, { hue, peak: aim.hue ? 1 : idle, pacing: aim.pacing }, elapsed);
      if (veil(prompt, still !== undefined)) renderer.requestRender();
      return still ?? true;
    },
    close() {
      icon.close();
      veil(prompt, false);
    },
  };
}
