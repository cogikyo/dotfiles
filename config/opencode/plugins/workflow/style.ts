import type { RGBA } from "@opentui/core";
import type { Plugin } from "@opencode/plugin/tui";
import { colors, roles } from "../statusline/palette.ts";
import type { Owner, Role } from "./parse.ts";

export type Tone = Role | "accent" | "text" | "muted" | "error" | "success" | "yellow" | "orange" | "magenta" | "card";

export type Segment = { text: string; tone: Tone; bg?: Tone; bold?: boolean; italic?: boolean };

export type Row = Segment[];

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
    card: theme.background.raised.base,
    build: role("build"),
    review: role("review"),
    scout: role("scout"),
    verify: role("verify"),
  };
}
