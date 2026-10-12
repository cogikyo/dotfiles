import { usageProviders } from "./providers.ts";
import { normalizePercent, num, record, str, type Json } from "./types.ts";
import type { Credentials, ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

const { id, label, staleAfterMS } = usageProviders.openai;
const FETCH_TIMEOUT_MS = 15_000;
const DAY_SECONDS = 24 * 60 * 60;
const WEEK_SECONDS = 7 * DAY_SECONDS;

export function openaiUsage(credentials: Credentials): ProviderAdapter {
  return {
    id,
    label,
    poll: {
      minFetchIntervalMS: 60_000,
      errorBackoffMS: 60_000,
      warnBackoffMS: 0,
      rateLimitBackoffMS: 10 * 60_000,
      staleAfterMS,
    },
    load: () => load(credentials),
  };
}

async function load(credentials: Credentials): Promise<ProviderUsage> {
  const credential = await credentials("openai");
  if (credential?.type !== "oauth") return usage([], "no auth");

  const accountID = str(credential.metadata?.accountID) || accountIDFromToken(credential.access);
  const headers = new Headers({
    Authorization: `Bearer ${credential.access}`,
    Accept: "application/json",
    "User-Agent": "opencode-usage",
  });
  if (accountID) headers.set("ChatGPT-Account-Id", accountID);

  const response = await fetch("https://chatgpt.com/backend-api/wham/usage", {
    headers,
    signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
  });
  if (!response.ok) return usage([], `${response.status}`);

  const rateLimit = record(record(await response.json())?.rate_limit);
  const windows = rateLimit
    ? [
        usageWindow(record(rateLimit.primary_window), rateLimit),
        usageWindow(record(rateLimit.secondary_window), rateLimit),
      ].filter((window) => window !== undefined)
    : [];

  if (windows.length === 0) return usage([], "no windows");
  return usage(windows);
}

function usage(windows: UsageWindow[], note?: string): ProviderUsage {
  return { id, label, windows, note };
}

function accountIDFromToken(token: string) {
  const parts = token.split(".");
  if (parts.length !== 3) return undefined;

  try {
    const claims = record(JSON.parse(Buffer.from(parts[1], "base64url").toString("utf8")));
    return str(record(claims?.["https://api.openai.com/auth"])?.chatgpt_account_id);
  } catch {
    return undefined;
  }
}

function usageWindow(window: Json | undefined, fallback: Json): UsageWindow | undefined {
  if (!window) return undefined;

  const tag = labelFromDuration(num(window.limit_window_seconds));
  if (!tag) return undefined;

  const used = usedPercent(window);
  const reset = resetAtFromWindow(window, fallback);
  if (used === undefined && !reset) return undefined;

  return { label: tag, usedPercent: used, resetAt: reset };
}

function labelFromDuration(seconds: number | undefined) {
  if (seconds === undefined || seconds <= 0) return undefined;
  const isWeekly = seconds >= WEEK_SECONDS - DAY_SECONDS / 2 && seconds <= WEEK_SECONDS + DAY_SECONDS / 2;
  return isWeekly ? "W" : "H";
}

function usedPercent(window: Json) {
  const used = normalizePercent(num(window.used_percent));
  if (used !== undefined) return used;
  const remaining = normalizePercent(num(window.remaining_percent));
  return remaining === undefined ? undefined : 100 - remaining;
}

function resetAtFromWindow(window: Json, fallback: Json) {
  const absolute = resetAt(window.reset_at) ?? resetAt(fallback.reset_at);
  if (absolute) return absolute;

  const resetAfterSeconds = num(window.reset_after_seconds) ?? num(fallback.reset_after_seconds);
  if (resetAfterSeconds === undefined || resetAfterSeconds < 0) return undefined;
  return new Date(Date.now() + resetAfterSeconds * 1000).toISOString();
}

function resetAt(value: unknown) {
  if (typeof value === "string") {
    return Number.isNaN(new Date(value).getTime()) ? undefined : value;
  }
  const seconds = num(value);
  if (seconds === undefined) return undefined;

  const date = new Date(seconds * 1000);
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
}
