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
    const code = render.defaultRender();
    if (!(code instanceof CodeRenderable)) return code;

    const label = token.lang?.trim().split(/\s+/, 1)[0] ?? "";
    code.filetype = filetype;
    code.syntaxStyle = style;
    code.fg = vagari.fg;

    const bg = context.theme.background.raised.base;
    const muted = context.theme.text.muted;
    code.bg = "transparent";
    code.drawUnstyledText = true; // The host rebuilds cards on each streaming chunk; keep text visible until highlighting returns.
    code.width = "auto";
    code.marginTop = 0;
    code.marginBottom = 0;

    const tint = filetype === "diff" ? vagari.tint(bg) : undefined;
    let content = "";
    let kinds: (Change | undefined)[] = [];
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
        if (code.content !== content) {
          content = code.content;
          kinds = changes(content);
        }
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
