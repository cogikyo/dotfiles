import { readAuth } from "./auth.ts";
import { usageProviders } from "./providers.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

type AuthFile = {
  "kimi-code"?: {
    type?: string;
    key?: string;
  };
};

type KimiUsagePayload = {
  usage?: KimiUsageRow | null;
  limits?: KimiLimit[] | null;
  totalQuota?: KimiUsageRow | null;
};

type KimiUsageRow = {
  limit?: unknown;
  used?: unknown;
  remaining?: unknown;
  resetTime?: unknown;
};

type KimiLimit = {
  window?: {
    duration?: unknown;
    timeUnit?: unknown;
  } | null;
  detail?: KimiUsageRow | null;
};

const { id, label, staleAfterMS } = usageProviders.kimiCode;
const FETCH_TIMEOUT_MS = 15_000;
const USAGE_URL = "https://api.kimi.com/coding/v1/usages";

function usage(windows: UsageWindow[], note?: string, noteKind?: ProviderUsage["noteKind"]): ProviderUsage {
  return { id, label, windows, note, noteKind };
}

function note(note: string, kind: ProviderUsage["noteKind"] = "warn") {
  return usage([], note, kind);
}

function numeric(value: unknown) {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value !== "string") return undefined;

  const number = Number(value);
  return Number.isFinite(number) ? number : undefined;
}

function usedPercent(row?: KimiUsageRow | null) {
  const limit = numeric(row?.limit);
  if (limit === undefined || limit <= 0) return undefined;

  const used = numeric(row?.used);
  if (used !== undefined) return Math.max(0, Math.min(100, (used / limit) * 100));

  const remaining = numeric(row?.remaining);
  if (remaining === undefined) return undefined;
  return Math.max(0, Math.min(100, 100 - (remaining / limit) * 100));
}

function resetAt(row?: KimiUsageRow | null) {
  if (typeof row?.resetTime !== "string") return undefined;

  const parsed = Date.parse(row.resetTime);
  return Number.isFinite(parsed) ? new Date(parsed).toISOString() : undefined;
}

function limitLabel(limit: KimiLimit) {
  const duration = numeric(limit.window?.duration);
  const unit = typeof limit.window?.timeUnit === "string" ? limit.window.timeUnit : "";

  if (duration === 300 && unit.includes("MINUTE")) return "H";
  if (duration === 7 && unit.includes("DAY")) return "W";
  return undefined;
}

function window(label: string, row?: KimiUsageRow | null): UsageWindow | undefined {
  const used = usedPercent(row);
  const reset = resetAt(row);
  if (used === undefined && !reset) return undefined;

  return { label, usedPercent: used, resetAt: reset };
}

function limitWindows(limits?: KimiLimit[] | null) {
  if (!limits) return [];

  return limits
    .map((limit) => {
      const label = limitLabel(limit);
      return label ? window(label, limit.detail) : undefined;
    })
    .filter((item): item is UsageWindow => Boolean(item));
}

function interpret(payload: KimiUsagePayload) {
  const windows = [
    ...limitWindows(payload.limits),
    window("W", payload.usage),
    window("M", payload.totalQuota),
  ].filter((item): item is UsageWindow => Boolean(item));

  if (windows.length === 0) return note("no windows", "warn");
  return usage(windows);
}

async function load(): Promise<ProviderUsage> {
  const auth: AuthFile = await readAuth<AuthFile>().catch(() => ({}));
  const key = auth["kimi-code"]?.key;
  if (!key) return note("no auth", "warn");

  const response = await fetch(USAGE_URL, {
    headers: {
      Authorization: `Bearer ${key}`,
      Accept: "application/json",
      "User-Agent": "opencode-usage",
    },
    signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
  });

  if (response.status === 401 || response.status === 403) return note("auth", "warn");
  if (response.status === 429) return usage([], "429");
  if (!response.ok) return note(`${response.status}`, "error");

  return interpret((await response.json()) as KimiUsagePayload);
}

export const kimiCodeUsage: ProviderAdapter = {
  id,
  label,
  placeholders: ["H", "W", "M"],
  poll: {
    minFetchIntervalMS: 60_000,
    errorBackoffMS: 3 * 60_000,
    warnBackoffMS: 60_000,
    rateLimitBackoffMS: 10 * 60_000,
    staleAfterMS,
  },
  load,
};
