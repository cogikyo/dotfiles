import { RGBA } from "@opentui/core";

export const colors = {
  blue: RGBA.fromHex("#7492ef"),
  brightBlue: RGBA.fromHex("#9db2f4"),
  green: RGBA.fromHex("#95cb79"),
  brightGreen: RGBA.fromHex("#9fd883"),
  yellow: RGBA.fromHex("#f5c069"),
  brightYellow: RGBA.fromHex("#f5d599"),
  orange: RGBA.fromHex("#f8b486"),
  red: RGBA.fromHex("#f08898"),
  brightRed: RGBA.fromHex("#f29ca9"),
  magenta: RGBA.fromHex("#b29ae8"),
  pink: RGBA.fromHex("#ea76c0"),
  sky: RGBA.fromHex("#7cc5ef"),
  muted: RGBA.fromHex("#7b7fb0"),
} as const;

const steps = [15, 30, 45, 60, 70, 80, 90, 95];

const ramp = [
  colors.blue,
  colors.brightBlue,
  colors.green,
  colors.brightGreen,
  colors.brightYellow,
  colors.yellow,
  colors.red,
  colors.brightRed,
  colors.pink,
];

export function tier(percent: number) {
  const index = steps.findIndex((step) => percent < step);
  return index === -1 ? steps.length : index;
}

export function pressure(percent: number) {
  return ramp[tier(percent)];
}
