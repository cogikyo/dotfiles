export type UsageWindow = {
  label: string;
  // Undefined when the provider exposes a window/reset period but no burn percent
  // (e.g. xAI's weekly subscription). The UI renders it as a muted "--" cell.
  usedPercent?: number;
  resetAt?: string;
};

export function normalizePercent(value: unknown): number | undefined {
  if (value == null || typeof value !== "number") return undefined;
  if (!Number.isFinite(value)) return undefined;
  const expanded = value > 0 && value < 1 ? value * 100 : value;
  return Math.max(0, Math.min(100, expanded));
}

export type NoteKind = "info" | "warn" | "error";

export type ProviderUsage = {
  id: string;
  label: string;
  windows: UsageWindow[];
  note?: string;
  // noteKind colors the inline note: "info" muted, "warn" amber, "error" red.
  // Undefined keeps the legacy split — muted when windows exist (stale), red when windowless.
  noteKind?: NoteKind;
  // Expected window labels for placeholder rows when this provider has no windows;
  // stamped from the adapter so the UI shows the right shape per provider.
  placeholders?: string[];
};

export type ProviderAdapter = {
  id: string;
  label: string;
  // Window labels rendered as placeholder rows when a fetch yields no windows.
  // Defaults to ["H", "W"]; xAI overrides to ["W", "M"] since it has no hourly window.
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
