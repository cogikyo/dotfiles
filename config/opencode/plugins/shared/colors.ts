import type { TuiThemeCurrent } from "@opencode-ai/plugin/tui";
import { RGBA } from "@opentui/core";

/** Current OpenCode TUI theme shape. */
export type Theme = TuiThemeCurrent;

const hex = (value: string) => RGBA.fromHex(value);
const defs = {
  blu_2: hex("#7492ef"),
  blu_4: hex("#9db2f4"),
  grn_3: hex("#95cb79"),
  grn_4: hex("#9fd883"),
  sun_4: hex("#f5d599"),
  sun_2: hex("#f5c069"),
  orn_4: hex("#f8b486"),
  rby_3: hex("#f08898"),
  rby_4: hex("#f29ca9"),
  prp_3: hex("#b29ae8"),
  pnk_2: hex("#ea76c0"),
  cyn_3: hex("#50dec8"),
  sky_3: hex("#7cc5ef"),
  fg: hex("#aeb9f8"),
  slt_5: hex("#7b7fb0"),
  prp_2: hex("#a188df"),
} as const;

const palette = {
  blue: defs.blu_2,
  brightBlue: defs.blu_4,
  green: defs.grn_3,
  brightGreen: defs.grn_4,
  brightYellow: defs.sun_4,
  yellow: defs.sun_2,
  orange: defs.orn_4,
  red: defs.rby_3,
  brightRed: defs.rby_4,
  magenta: defs.prp_3,
  pink: defs.pnk_2,
  cyan: defs.cyn_3,
  sky: defs.sky_3,
  text: defs.fg,
  muted: defs.slt_5,
  branch: defs.prp_2,
} as const;

/** Returns the plugin's fixed palette; the current theme is accepted for shared call sites. */
export function colors(_theme: Theme) {
  return palette;
}

/** Clamps a percentage to the inclusive range from 0 to 100. */
export function clampPercent(percent: number) {
  if (!Number.isFinite(percent)) return 0;
  return Math.max(0, Math.min(100, percent));
}

/** Maps context usage to one of nine pressure tiers. */
export function pressureTier(usedPercent: number) {
  const percent = clampPercent(usedPercent);
  if (percent < 15) return 0;
  if (percent < 30) return 1;
  if (percent < 45) return 2;
  if (percent < 60) return 3;
  if (percent < 70) return 4;
  if (percent < 80) return 5;
  if (percent < 90) return 6;
  if (percent < 95) return 7;
  return 8;
}

/** Selects a palette color for the given context pressure. */
export function pressureColor(theme: Theme, usedPercent: number) {
  const c = colors(theme);
  const pressureColors = [
    c.blue,
    c.brightBlue,
    c.green,
    c.brightGreen,
    c.brightYellow,
    c.yellow,
    c.red,
    c.brightRed,
    c.pink,
  ] as const;

  return pressureColors[pressureTier(usedPercent)];
}

/** Returns the color used for provider usage percentages. */
export function usageColor(theme: Theme, usedPercent: number) {
  return pressureColor(theme, usedPercent);
}
