import { StyledText, TextAttributes, type RGBA, type TextChunk } from "@opentui/core";
import type { Plugin } from "@opencode/plugin/tui";
import { glyphs } from "../lanes/label.ts";
import { colors, roles } from "../statusline/palette.ts";
import type { Owner, Role } from "./parse.ts";
import type { Phase } from "./progress.ts";

export type Tone =
  | Role
  | "accent"
  | "text"
  | "muted"
  | "error"
  | "success"
  | "yellow"
  | "orange"
  | "magenta"
  | "sky"
  | "card";

export type Segment = { text: string; tone: Tone; bg?: Tone; bold?: boolean; italic?: boolean };

export type Row = Segment[];

const icons = { self: "◇", gate: "◆", you: "\u{f007}" } as const;

export function ink(owner: Owner): Omit<Segment, "text"> {
  switch (owner.kind) {
    case "self":
      return { tone: "muted" };
    case "gate":
      return { tone: "orange" };
    case "you":
      return { tone: "yellow" };
    case "lane":
      return owner.lane ? { tone: owner.role ?? "muted", bold: true } : { tone: owner.role ?? "muted", italic: true };
  }
}

export function icon(owner: Owner) {
  if (owner.kind !== "lane") return icons[owner.kind];
  if (!owner.role) return "?";
  const glyph = glyphs(`${owner.role}/${owner.scope}`);
  return glyph.scope ?? glyph.role ?? "?";
}

export const shades: Record<Phase, Tone> = {
  done: "success",
  skipped: "muted",
  failed: "error",
  running: "sky",
  returned: "yellow",
  ready: "accent",
  waiting: "muted",
};

export function pill(text: string, tone: Tone): Segment[] {
  return [{ text: ` ${text} `, tone: "card", bg: tone, bold: true }];
}

export function palette(theme: Plugin.Context["theme"], mode: Plugin.Context["themeMode"]): Record<Tone, RGBA> {
  const role = (name: Role) => roles[name] ?? theme.text.muted;
  return {
    accent: theme.hue.accent[mode === "light" ? 300 : 200],
    text: theme.text.base,
    muted: theme.text.muted,
    error: theme.text.feedback.error.base,
    success: theme.text.feedback.success.base,
    yellow: colors.yellow,
    orange: colors.orange,
    magenta: colors.magenta,
    sky: colors.sky,
    card: theme.background.raised.base,
    build: role("build"),
    review: role("review"),
    scout: role("scout"),
    verify: role("verify"),
  };
}

export function styled(rows: readonly Row[], tones: Record<Tone, RGBA>) {
  return new StyledText(
    rows.flatMap((row, index): TextChunk[] => [
      ...(index ? [{ __isChunk: true as const, text: "\n" }] : []),
      ...row.map(({ text, tone, bg, bold, italic }) => ({
        __isChunk: true as const,
        text,
        fg: tones[tone],
        bg: bg && tones[bg],
        attributes: (bold ? TextAttributes.BOLD : 0) | (italic ? TextAttributes.ITALIC : 0),
      })),
    ]),
  );
}
