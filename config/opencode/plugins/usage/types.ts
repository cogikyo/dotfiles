export type UsageWindow = {
  label: string;
  usedPercent: number;
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
};

export type ProviderAdapter = {
  id: string;
  label: string;
  poll?: {
    minFetchIntervalMS?: number;
    errorBackoffMS?: number;
    rateLimitBackoffMS?: number;
    staleAfterMS?: number;
  };
  load(): Promise<ProviderUsage>;
};
