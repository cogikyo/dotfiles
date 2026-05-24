import type { TuiThemeCurrent } from "@opencode-ai/plugin/tui";
import { RGBA } from "@opentui/core";

export type Theme = TuiThemeCurrent;

const c = (hex: string) => RGBA.fromHex(hex);
const defs = {
  blu_2: c("#7492ef"),
  blu_4: c("#9db2f4"),
  grn_3: c("#95cb79"),
  grn_4: c("#9fd883"),
  sun_4: c("#f5d599"),
  sun_2: c("#f5c069"),
  orn_4: c("#f8b486"),
  rby_3: c("#f08898"),
  rby_4: c("#f29ca9"),
  prp_3: c("#b29ae8"),
  pnk_2: c("#ea76c0"),
  cyn_3: c("#50dec8"),
  sky_3: c("#7cc5ef"),
  fg: c("#aeb9f8"),
  slt_5: c("#7b7fb0"),
  prp_2: c("#a188df"),
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

export function colors(_theme: Theme) {
  return palette;
}

export function clampPercent(percent: number) {
  if (!Number.isFinite(percent)) return 0;
  return Math.max(0, Math.min(100, percent));
}

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

export function usageColor(theme: Theme, usedPercent: number) {
  return pressureColor(theme, usedPercent);
}
