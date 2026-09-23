import { record } from "../shared/record.ts";
import { readAuth } from "./auth.ts";
import { usageProviders } from "./providers.ts";
import { normalizePercent } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

type OpenAIWindow = {
  limit_window_seconds?: unknown;
  remaining_percent?: unknown;
  reset_after_seconds?: unknown;
  reset_at?: unknown;
  used_percent?: unknown;
};

type OpenAIRateLimit = OpenAIWindow & {
  primary_window?: OpenAIWindow | null;
  secondary_window?: OpenAIWindow | null;
};

const { id, label, staleAfterMS } = usageProviders.openai;
const FETCH_TIMEOUT_MS = 15_000;
const DAY_SECONDS = 24 * 60 * 60;
const WEEK_SECONDS = 7 * DAY_SECONDS;

function usage(windows: UsageWindow[], note?: string): ProviderUsage {
  return { id, label, windows, note };
}

function decodeJwtPayload(token: string) {
  const parts = token.split(".");
  if (parts.length !== 3) return undefined;

  try {
    return record(JSON.parse(Buffer.from(parts[1], "base64url").toString("utf8")));
  } catch {
    return undefined;
  }
}

function accountIDFromToken(token: string) {
  const claims = record(decodeJwtPayload(token)?.["https://api.openai.com/auth"]);
  return typeof claims?.chatgpt_account_id === "string" ? claims.chatgpt_account_id : undefined;
}

function resetAtFromWindow(window: OpenAIWindow, fallback?: OpenAIWindow) {
  const absolute = resetAt(window.reset_at) ?? resetAt(fallback?.reset_at);
  if (absolute) return absolute;

  const resetAfterSeconds =
    typeof window.reset_after_seconds === "number"
      ? window.reset_after_seconds
      : typeof fallback?.reset_after_seconds === "number"
        ? fallback.reset_after_seconds
        : undefined;
  if (resetAfterSeconds === undefined || !Number.isFinite(resetAfterSeconds) || resetAfterSeconds < 0) {
    return undefined;
  }
  return new Date(Date.now() + resetAfterSeconds * 1000).toISOString();
}

function resetAt(value: unknown) {
  if (typeof value === "string") {
    return Number.isNaN(new Date(value).getTime()) ? undefined : value;
  }
  if (typeof value !== "number" || !Number.isFinite(value)) return undefined;

  const date = new Date(value * 1000);
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
}

function usedPercent(window: OpenAIWindow) {
  return (
    normalizePercent(window.used_percent) ??
    (() => {
      const remaining = normalizePercent(window.remaining_percent);
      return remaining === undefined ? undefined : 100 - remaining;
    })()
  );
}

function labelFromDuration(value: unknown) {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    return undefined;
  }
  const isWeekly = value >= WEEK_SECONDS - DAY_SECONDS / 2 && value <= WEEK_SECONDS + DAY_SECONDS / 2;
  return isWeekly ? "W" : "H";
}

function usageWindow(window: OpenAIWindow | null | undefined, fallback: OpenAIRateLimit): UsageWindow | undefined {
  if (!window) return undefined;

  const tag = labelFromDuration(window.limit_window_seconds);
  if (!tag) return undefined;

  const used = usedPercent(window);
  const reset = resetAtFromWindow(window, fallback);
  if (used === undefined && !reset) return undefined;

  return { label: tag, usedPercent: used, resetAt: reset };
}

/** Converts the API rate-limit windows into sidebar windows. */
export function parseOpenAIWindows(rateLimit?: OpenAIRateLimit): UsageWindow[] {
  if (!rateLimit) return [];

  return [usageWindow(rateLimit.primary_window, rateLimit), usageWindow(rateLimit.secondary_window, rateLimit)].filter(
    (window): window is UsageWindow => Boolean(window),
  );
}

async function load(): Promise<ProviderUsage> {
  const auth = await readAuth();
  const openai = record(auth?.openai);

  if (openai?.type !== "oauth" || typeof openai.access !== "string" || !openai.access) {
    return usage([], "no auth");
  }

  const accountID = (typeof openai.accountId === "string" && openai.accountId) || accountIDFromToken(openai.access);
  const headers = new Headers({
    Authorization: `Bearer ${openai.access}`,
    Accept: "application/json",
    "User-Agent": "opencode-usage",
  });
  if (accountID) headers.set("ChatGPT-Account-Id", accountID);

  const response = await fetch("https://chatgpt.com/backend-api/wham/usage", {
    headers,
    signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
  });
  if (!response.ok) return usage([], `${response.status}`);

  const payload = record(await response.json());
  const rateLimit = record(payload?.rate_limit);
  const windows = parseOpenAIWindows(
    rateLimit && {
      ...rateLimit,
      primary_window: record(rateLimit.primary_window),
      secondary_window: record(rateLimit.secondary_window),
    },
  );

  if (windows.length === 0) return usage([], "no windows");
  return usage(windows);
}

/** Usage adapter for ChatGPT account rate limits. */
export const openaiUsage: ProviderAdapter = {
  id,
  label,
  poll: {
    minFetchIntervalMS: 60_000,
    errorBackoffMS: 60_000,
    warnBackoffMS: 0,
    rateLimitBackoffMS: 10 * 60_000,
    staleAfterMS,
  },
  load,
};
