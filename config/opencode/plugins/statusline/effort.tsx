import { usePlugin } from "@opencode/plugin/tui";
import { RGBA, TextAttributes, type OptimizedBuffer, type Renderable } from "@opentui/core";
import { onCleanup, onMount } from "solid-js";
import { colors } from "./palette.ts";
import { cell, locate, overlay, type Prompt } from "./prompt.ts";

type Text = Renderable & { readonly plainText: string };
type Level = { readonly icon: string; readonly fg: RGBA };

const hidden = "xhigh";

const levels: Partial<Record<string, Level>> = {
  low: { icon: "\u{f08fe}", fg: colors.muted },
  medium: { icon: "\u{f08f4}", fg: colors.green },
  high: { icon: "\u{f08f6}", fg: colors.blue },
  max: { icon: "\u{f09d1}", fg: colors.pink },
};

const unset: Level & { readonly word: string } = { icon: "\u{f08f5}", fg: colors.brightGreen, word: "default" };

/** OpenCode V2 has no model/variant-row slot, so the footer hosts a post-render overlay. */
export function Effort(props: { mode: "normal" | "shell" }) {
  const context = usePlugin();
  let host: Renderable | undefined;

  onMount(() => {
    const prompt = host && locate(host);
    if (!prompt) return;
    onCleanup(
      overlay(context, prompt.row, (buffer) => {
        if (props.mode !== "normal") return;
        const row = metadata(prompt);
        if (!row) return;
        const variant = context.ui.model.current()?.variant;
        if (variant) paint(buffer, flatten(row), variant);
        else if (context.ui.model.variant.list().length > 0) append(buffer, flatten(row), row.screenX + row.width);
      }),
    );
  });

  return <box position="absolute" ref={(box) => (host = box)} />;
}

function metadata(prompt: Prompt) {
  const siblings = prompt.editor.parent?.getChildren() ?? [];
  return siblings[siblings.indexOf(prompt.editor) + 1];
}

function paint(buffer: OptimizedBuffer, texts: Text[], variant: string) {
  const index = texts.findLastIndex((text, at) => text.plainText === variant && texts[at - 1]?.plainText === "·");
  const word = texts[index];
  const dot = texts[index - 1];
  if (!word || !dot) return;
  if (variant === hidden) return erase(buffer, dot.screenX - 1, word.screenX + word.width, word.screenY);
  const level = levels[variant];
  if (!level) return;
  buffer.drawText(level.icon, dot.screenX, dot.screenY, level.fg);
  buffer.drawText(variant, word.screenX, word.screenY, level.fg, undefined, TextAttributes.BOLD);
}

function append(buffer: OptimizedBuffer, texts: Text[], edge: number) {
  const last = texts.at(-1);
  if (!last) return;
  const x = last.screenX + last.width + 1;
  const y = last.screenY;
  const end = x + 2 + Bun.stringWidth(unset.word);
  if (end > edge) return;
  for (let column = x - 1; column < end; column++) if (cell(buffer, column, y) !== undefined) return;
  buffer.drawText(unset.icon, x, y, unset.fg);
  buffer.drawText(unset.word, x + 2, y, unset.fg, undefined, TextAttributes.BOLD);
}

function erase(buffer: OptimizedBuffer, from: number, to: number, y: number) {
  if (y < 0 || y >= buffer.height) return;
  const { bg } = buffer.buffers;
  for (let x = Math.max(from, 0); x < Math.min(to, buffer.width); x++) {
    const index = (y * buffer.width + x) * 4;
    const color = RGBA.fromArray(bg.slice(index, index + 4));
    buffer.setCell(x, y, " ", color, color);
  }
}

function flatten(node: Renderable): Text[] {
  return textual(node) ? [node] : node.getChildren().flatMap(flatten);
}

function textual(node: Renderable): node is Text {
  return "plainText" in node && typeof node.plainText === "string";
}
