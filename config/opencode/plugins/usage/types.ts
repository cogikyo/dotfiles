export type UsageWindow = {
  label: string;
  // Undefined when the provider exposes a window/reset period but no burn percent
  // (e.g. xAI's weekly subscription). The UI renders it as a muted "--" cell.
  usedPercent?: number;
  resetAt?: string;
};

export type NoteKind = "info" | "warn" | "error";

export type ProviderUsage = {
  id: string;
  label: string;
  windows: UsageWindow[];
  note?: string;
  // noteKind classifies a windowless note: "info"/"warn" are benign states the
  // sidebar keeps visible, while "error" (or undefined) keeps the legacy red error path.
  noteKind?: NoteKind;
  // Expected window labels for placeholder rows when this provider has no windows;
  // stamped from the adapter so the UI shows the right shape per provider.
  placeholders?: string[];
};

export type ProviderAdapter = {
  id: string;
  label: string;
  // Window labels rendered as placeholder rows when a fetch yields no windows.
  // Defaults to ["H", "W"]; xAI overrides to ["W"] since it has no hourly window.
  placeholders?: string[];
  poll?: {
    minFetchIntervalMS?: number;
    errorBackoffMS?: number;
    warnBackoffMS?: number;
    rateLimitBackoffMS?: number;
    staleAfterMS?: number;
  };
  load(): Promise<ProviderUsage>;
};
