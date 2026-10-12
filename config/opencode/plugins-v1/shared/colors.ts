import { RGBA } from "@opentui/core";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Sidebar colors                                                                                │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

// ├─ Palette ─────────────────────────────────────────────────────────────────────────────────────┤

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

/** Fixed sidebar palette, independent of the active theme. */
export const colors = {
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

// ├─ Pressure ────────────────────────────────────────────────────────────────────────────────────┤

/** Clamps a percentage to 0–100, treating non-finite values as zero. */
export function clampPercent(percent: number) {
  if (!Number.isFinite(percent)) return 0;
  return Math.max(0, Math.min(100, percent));
}

/** Maps a used percentage to one of nine sidebar pressure tiers. */
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

/** Colors context pressure from blue to pink as usage increases. */
export function pressureColor(usedPercent: number) {
  const pressureColors = [
    colors.blue,
    colors.brightBlue,
    colors.green,
    colors.brightGreen,
    colors.brightYellow,
    colors.yellow,
    colors.red,
    colors.brightRed,
    colors.pink,
  ] as const;

  return pressureColors[pressureTier(usedPercent)];
}

/** Uses the context-pressure palette for provider usage. */
export function usageColor(usedPercent: number) {
  return pressureColor(usedPercent);
}
