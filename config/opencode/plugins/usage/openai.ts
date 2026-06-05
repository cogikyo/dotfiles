import { readAuth } from "./auth.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

type AuthFile = {
  openai?: {
    type?: string;
    access?: string;
    accountId?: string;
  };
};

type OpenAIUsagePayload = {
  rate_limit?: {
    reset_at?: string;
    reset_after_seconds?: number;
    primary_window?: Record<string, unknown>;
    secondary_window?: Record<string, unknown>;
  };
};

const id = "openai";
const label = "OpenAI";
const MIN_FETCH_INTERVAL_MS = 60_000;
const ERROR_BACKOFF_MS = 60_000;
const RATE_LIMIT_BACKOFF_MS = 10 * 60_000;
const STALE_AFTER_MS = 2 * 60_000;

function usage(windows: UsageWindow[], note?: string): ProviderUsage {
  return { id, label, windows, note };
}

function decodeJwtPayload(token: string) {
  const parts = token.split(".");
  if (parts.length !== 3) return undefined;

  try {
    return JSON.parse(Buffer.from(parts[1], "base64url").toString("utf8")) as {
      "https://api.openai.com/auth"?: {
        chatgpt_account_id?: string;
      };
    };
  } catch {
    return undefined;
  }
}

function accountIDFromToken(token: string) {
  return decodeJwtPayload(token)?.["https://api.openai.com/auth"]
    ?.chatgpt_account_id;
}

function normalizePercent(value: unknown) {
  if (typeof value !== "number" || Number.isNaN(value)) return undefined;
  const expanded = value > 0 && value < 1 ? value * 100 : value;
  return Math.max(0, Math.min(100, expanded));
}

function resetAtFromWindow(
  window: Record<string, unknown>,
  fallback?: Record<string, unknown>,
) {
  const absolute =
    typeof window.reset_at === "string"
      ? window.reset_at
      : typeof fallback?.reset_at === "string"
        ? fallback.reset_at
        : undefined;
  if (absolute) return absolute;

  const resetAfterSeconds =
    typeof window.reset_after_seconds === "number"
      ? window.reset_after_seconds
      : typeof fallback?.reset_after_seconds === "number"
        ? fallback.reset_after_seconds
        : undefined;
  if (resetAfterSeconds === undefined || resetAfterSeconds < 0)
    return undefined;
  return new Date(Date.now() + resetAfterSeconds * 1000).toISOString();
}

function usedPercent(window: Record<string, unknown>) {
  return (
    normalizePercent(window.used_percent) ??
    (() => {
      const remaining = normalizePercent(window.remaining_percent);
      return remaining === undefined ? undefined : 100 - remaining;
    })()
  );
}

async function load(): Promise<ProviderUsage> {
  const auth = await readAuth<AuthFile>();
  const openai = auth.openai;

  if (!openai || openai.type !== "oauth" || !openai.access) {
    return usage([], "OAuth not found");
  }

  const accountID = openai.accountId || accountIDFromToken(openai.access);
  const headers = new Headers({
    Authorization: `Bearer ${openai.access}`,
    Accept: "application/json",
    "User-Agent": "opencode-usage",
  });
  if (accountID) headers.set("ChatGPT-Account-Id", accountID);

  const response = await fetch("https://chatgpt.com/backend-api/wham/usage", {
    headers,
  });
  if (!response.ok) return usage([], `HTTP ${response.status}`);

  const payload = (await response.json()) as OpenAIUsagePayload;
  const rateLimit = payload.rate_limit ?? {};
  const primaryWindow = rateLimit.primary_window ?? {};
  const secondaryWindow = rateLimit.secondary_window;

  const windows: UsageWindow[] = [];
  const primaryUsed = usedPercent(primaryWindow);
  if (primaryUsed !== undefined) {
    windows.push({
      label: "H",
      usedPercent: primaryUsed,
      resetAt: resetAtFromWindow(primaryWindow, rateLimit),
    });
  }

  if (secondaryWindow) {
    const secondaryUsed = usedPercent(secondaryWindow);
    if (secondaryUsed !== undefined) {
      windows.push({
        label: "W",
        usedPercent: secondaryUsed,
        resetAt: resetAtFromWindow(secondaryWindow, rateLimit),
      });
    }
  }

  if (windows.length === 0) return usage([], "Usage windows unavailable");
  return usage(windows);
}

export const openaiUsage: ProviderAdapter = {
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
