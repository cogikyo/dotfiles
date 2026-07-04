import { readAuth } from "./auth.ts";
import { normalizePercent } from "./types.ts";
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
const label = "Anthropic";
const FETCH_TIMEOUT_MS = 15_000;

function usage(windows: UsageWindow[], note?: string): ProviderUsage {
  return { id, label, windows, note };
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
    return usage([], "no auth");
  }

  const response = await fetch("https://api.anthropic.com/api/oauth/usage", {
    headers: {
      Authorization: `Bearer ${anthropic.access}`,
      Accept: "application/json",
      "User-Agent": "opencode-usage",
      "anthropic-beta": "oauth-2025-04-20",
      "anthropic-version": "2023-06-01",
    },
    signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
  });
  if (!response.ok) return usage([], `${response.status}`);

  const payload = (await response.json()) as AnthropicUsagePayload;
  const windows = [
    usageWindow("H", payload.five_hour),
    usageWindow("W", payload.seven_day),
  ].filter((window): window is UsageWindow => Boolean(window));

  if (windows.length === 0) return usage([], "no windows");
  return usage(windows);
}

export const anthropicUsage: ProviderAdapter = {
  id,
  label,
  poll: {
    minFetchIntervalMS: 60_000,
    errorBackoffMS: 2 * 60_000,
    warnBackoffMS: 0,
    rateLimitBackoffMS: 15 * 60_000,
    staleAfterMS: 2 * 60_000,
  },
  load,
};
