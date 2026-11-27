import type { CliRenderer } from "@opentui/core";
import { veil } from "../statusline/light.ts";
import type { Painter } from "../statusline/pace.tsx";
import { colors } from "../statusline/palette.ts";
import type { Prompt } from "../statusline/prompt.ts";
import { arrows } from "./arrows.ts";
import { wheel } from "./wheel.ts";

export function look(renderer: CliRenderer, prompt: Prompt): Painter {
  const icon = wheel(renderer);
  const flow = arrows(renderer, prompt);

  return {
    draw(buffer, aim, elapsed) {
      icon.draw(buffer, prompt.editor, aim.hue ?? colors.brightBlue);
      const still = flow(buffer, aim, elapsed);
      // The chevrons replace the edge light only while they have a lane to draw in.
      if (veil(prompt, still !== undefined)) renderer.requestRender();
      return still ?? true;
    },
    close() {
      icon.close();
      veil(prompt, false);
    },
  };
}
