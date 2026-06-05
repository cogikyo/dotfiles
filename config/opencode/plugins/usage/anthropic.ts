import { readAuth } from "./auth.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

type AuthFile = {
  anthropic?: {
    type?: string;
    access?: string;
  };
};

type AnthropicWindow = {
  utilization?: unknown;
  resets_at?: unknown;
};

type AnthropicUsagePayload = {
  five_hour?: AnthropicWindow | null;
  seven_day?: AnthropicWindow | null;
};

const id = "anthropic";
const label = "Claude";
const MIN_FETCH_INTERVAL_MS = 60_000;
const ERROR_BACKOFF_MS = 2 * 60_000;
const RATE_LIMIT_BACKOFF_MS = 15 * 60_000;
const STALE_AFTER_MS = 2 * 60_000;

function usage(windows: UsageWindow[], note?: string): ProviderUsage {
  return { id, label, windows, note };
}

function normalizePercent(value: unknown) {
  if (typeof value !== "number" || Number.isNaN(value)) return undefined;
  const expanded = value > 0 && value < 1 ? value * 100 : value;
  return Math.max(0, Math.min(100, expanded));
}

function resetAt(window: AnthropicWindow) {
  return typeof window.resets_at === "string" ? window.resets_at : undefined;
}

function usageWindow(
  label: string,
  window?: AnthropicWindow | null,
): UsageWindow | undefined {
  if (!window) return undefined;
  const usedPercent = normalizePercent(window.utilization);
  if (usedPercent === undefined) return undefined;
  return { label, usedPercent, resetAt: resetAt(window) };
}

async function load(): Promise<ProviderUsage> {
  const auth = await readAuth<AuthFile>();
  const anthropic = auth.anthropic;

  if (!anthropic || anthropic.type !== "oauth" || !anthropic.access) {
    return usage([], "OAuth not found");
  }

  const response = await fetch("https://api.anthropic.com/api/oauth/usage", {
    headers: {
      Authorization: `Bearer ${anthropic.access}`,
      Accept: "application/json",
      "User-Agent": "opencode-usage",
      "anthropic-beta": "oauth-2025-04-20",
      "anthropic-version": "2023-06-01",
    },
  });
  if (!response.ok) return usage([], `HTTP ${response.status}`);

  const payload = (await response.json()) as AnthropicUsagePayload;
  const windows = [
    usageWindow("H", payload.five_hour),
    usageWindow("W", payload.seven_day),
  ].filter((window): window is UsageWindow => Boolean(window));

  if (windows.length === 0) return usage([], "Usage windows unavailable");
  return usage(windows);
}

export const anthropicUsage: ProviderAdapter = {
  id,
  label,
  poll: {
    minFetchIntervalMS: MIN_FETCH_INTERVAL_MS,
    errorBackoffMS: ERROR_BACKOFF_MS,
    rateLimitBackoffMS: RATE_LIMIT_BACKOFF_MS,
    staleAfterMS: STALE_AFTER_MS,
  },
  load,
};
