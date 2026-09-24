import { z } from "zod";

export type UsageWindow = {
  label: string;
  // A provider can report a reset without a percentage.
  usedPercent?: number;
  resetAt?: string;
};

export type NoteKind = "info" | "warn" | "error";

export type ProviderUsage = {
  id: string;
  label: string;
  windows: UsageWindow[];
  note?: string;
  noteKind?: NoteKind;
  placeholders?: string[];
};

/** Loads provider usage under a polling policy; the sidebar keeps cached data on failures. */
export type ProviderAdapter = {
  id: string;
  label: string;
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

/** Normalizes percentages and fractional values to 0–100; `1` stays 1%. */
export function normalizePercent(value: number | undefined): number | undefined {
  if (value === undefined || !Number.isFinite(value)) return undefined;
  const expanded = value > 0 && value < 1 ? value * 100 : value;
  return Math.max(0, Math.min(100, expanded));
}

/** Treats invalid optional fields as absent without rejecting the parent object. */
export function lenient<T extends z.ZodType>(schema: T) {
  return schema.optional().catch(undefined);
}

/** Keeps only cached windows declared by the adapter, or all windows if no labels are declared. */
export function declaredWindows(usage: ProviderUsage, placeholders?: string[]) {
  if (!placeholders) return usage.windows;
  return usage.windows.filter((window) => placeholders.includes(window.label));
}
