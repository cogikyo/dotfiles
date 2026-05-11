import type { TuiThemeCurrent } from "@opencode-ai/plugin/tui";

export type Theme = TuiThemeCurrent;

export function colors(theme: Theme) {
  return {
    blue: theme.secondary,
    brightBlue: theme.markdownEmph,
    green: theme.success,
    brightGreen: theme.diffHighlightAdded,
    brightYellow: theme.primary,
    yellow: theme.warning,
    orange: theme.primary,
    red: theme.error,
    brightRed: theme.diffHighlightRemoved,
    magenta: theme.syntaxKeyword,
    pink: theme.syntaxNumber,
    cyan: theme.info,
    sky: theme.accent,
    text: theme.text,
    muted: theme.textMuted,
    branch: theme.syntaxKeyword,
  };
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
