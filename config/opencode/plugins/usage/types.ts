/** A provider usage window, with an optional percentage and reset time. */
export type UsageWindow = {
  label: string;
  // Some providers report a reset period without a usage percentage.
  usedPercent?: number;
  resetAt?: string;
};

/** Converts a provider percentage or fraction to a value from 0 through 100. */
export function normalizePercent(value: unknown): number | undefined {
  if (value == null || typeof value !== "number") return undefined;
  if (!Number.isFinite(value)) return undefined;
  const expanded = value > 0 && value < 1 ? value * 100 : value;
  return Math.max(0, Math.min(100, expanded));
}

/** Removes cached windows whose labels are no longer declared by the adapter. */
export function declaredWindows(usage: ProviderUsage, placeholders?: string[]) {
  if (!placeholders) return usage.windows;
  return usage.windows.filter((window) => placeholders.includes(window.label));
}

/** Severity used to color a provider status note in the dashboard. */
export type NoteKind = "info" | "warn" | "error";

export type ProviderUsage = {
  id: string;
  label: string;
  windows: UsageWindow[];
  note?: string;
  // Missing severity keeps the UI's legacy stale-versus-error coloring.
  noteKind?: NoteKind;
  // Window labels used for placeholder rows when no windows are available.
  placeholders?: string[];
};

/** Loads one provider's usage and defines its polling and display policy. */
export type ProviderAdapter = {
  id: string;
  label: string;
  // Window labels rendered as placeholders when a fetch returns no windows.
  placeholders?: string[];
  poll: {
    minFetchIntervalMS: number;
    errorBackoffMS: number;
    warnBackoffMS: number;
    rateLimitBackoffMS: number;
    staleAfterMS: number;
  };
  load(): Promise<ProviderUsage>;
};
