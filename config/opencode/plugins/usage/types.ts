export type UsageWindow = {
  label: string;
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

export type Credential =
  | { type: "oauth"; methodID: string; access: string; metadata?: Readonly<Record<string, unknown>> }
  | { type: "key"; key: string }
  | { type: "external" };

export type Credentials = (integrationID: string) => Promise<Credential | undefined>;

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

export function normalizePercent(value: number | undefined): number | undefined {
  if (value === undefined || !Number.isFinite(value)) return undefined;
  const expanded = value > 0 && value < 1 ? value * 100 : value;
  return Math.max(0, Math.min(100, expanded));
}

export function declaredWindows(usage: ProviderUsage, placeholders?: string[]) {
  if (!placeholders) return usage.windows;
  return usage.windows.filter((window) => placeholders.includes(window.label));
}

export type Json = Readonly<Record<string, unknown>>;

export function record(value: unknown): Json | undefined {
  return typeof value === "object" && value !== null && !Array.isArray(value) ? (value as Json) : undefined;
}

export function num(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

export function str(value: unknown) {
  return typeof value === "string" ? value : undefined;
}

export function text(value: unknown) {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}
