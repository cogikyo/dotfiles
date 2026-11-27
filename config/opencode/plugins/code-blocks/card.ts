import { BoxRenderable, CodeRenderable, type MarkdownCodeBlockRenderer, type SyntaxStyle } from "@opentui/core";
import type { Plugin } from "@opencode/plugin/tui";
import type { Change, Vagari } from "./vagari.ts";

const inset = 2;

export function card(
  context: Pick<Plugin.Context, "theme" | "renderer">,
  vagari: Vagari,
  filetype: string | undefined,
  style: SyntaxStyle,
): MarkdownCodeBlockRenderer {
  return (token, render) => {
    const label = token.lang?.trim().split(/\s+/, 1)[0] ?? "";
    const code = new CodeRenderable(context.renderer, {
      content: token.text,
      filetype,
      syntaxStyle: style,
      treeSitterClient: render.treeSitterClient,
      conceal: render.concealCode,
      fg: vagari.fg,
      bg: "transparent",
      drawUnstyledText: true, // The host rebuilds cards on each streaming chunk; keep text visible until highlighting returns.
      width: "auto",
      flexShrink: 0,
      marginTop: 0,
      marginBottom: 0,
    });

    const bg = context.theme.background.raised.base;
    const muted = context.theme.text.muted;

    const tint = filetype === "diff" ? vagari.tint(bg) : undefined;
    const kinds = tint ? changes(token.text) : [];
    const box = new BoxRenderable(context.renderer, {
      width: "100%",
      flexShrink: 0,
      marginTop: 0, // An unset OpenTUI margin becomes NaN and loses the blank row above the card.
      backgroundColor: bg,
      paddingTop: 1,
      paddingBottom: 1,
      paddingLeft: inset,
      paddingRight: inset,
      renderAfter(buffer) {
        const x = this.screenX + this.width - inset - Bun.stringWidth(label);
        if (x >= this.screenX + inset) buffer.drawText(label, x, this.screenY, muted, bg);
        if (!tint) return;
        for (const [row, source] of code.getLineSources(0, code.height).entries()) {
          const kind = kinds[source];
          if (kind) buffer.fillRect(this.screenX, code.screenY + row, this.width, 1, tint[kind]);
        }
      },
    });
    box.add(code);
    return box;
  };
}

function changes(content: string): (Change | undefined)[] {
  const lines = content.split("\n");
  return lines.map((line, index) => {
    // Recognize headers by adjacency because model-written hunk counts are unreliable.
    if (line.startsWith("--- ") && lines[index + 1]?.startsWith("+++ ")) return undefined;
    if (line.startsWith("+++ ") && lines[index - 1]?.startsWith("--- ")) return undefined;
    if (line.startsWith("+")) return "add";
    if (line.startsWith("-")) return "delete";
    if (line.startsWith("!")) return "change";
    return undefined;
  });
}
