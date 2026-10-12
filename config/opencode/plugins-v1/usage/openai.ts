import { z } from "zod";
import { readAuth } from "./auth.ts";
import { usageProviders } from "./providers.ts";
import { lenient, normalizePercent } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

const { id, label, staleAfterMS } = usageProviders.openai;
const FETCH_TIMEOUT_MS = 15_000;
const DAY_SECONDS = 24 * 60 * 60;
const WEEK_SECONDS = 7 * DAY_SECONDS;

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ OpenAI usage                                                                                  │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

// ├─ Provider adapter ────────────────────────────────────────────────────────────────────────────┤

/** Loads ChatGPT rate-limit windows; the parent limit supplies fallback reset times, not a separate row. */
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

async function load(): Promise<ProviderUsage> {
  const openai = (await readAuth())?.openai;
  if (!openai) return usage([], "no auth");

  const accountID = openai.accountId || accountIDFromToken(openai.access);
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

  const rateLimit = Payload.parse(await response.json()).rate_limit;
  const windows = rateLimit
    ? [usageWindow(rateLimit.primary_window, rateLimit), usageWindow(rateLimit.secondary_window, rateLimit)].filter(
        (window) => window !== undefined,
      )
    : [];

  if (windows.length === 0) return usage([], "no windows");
  return usage(windows);
}

function usage(windows: UsageWindow[], note?: string): ProviderUsage {
  return { id, label, windows, note };
}

// ├─ Rate-limit windows ──────────────────────────────────────────────────────────────────────────┤

// The account ID comes from an unverified JWT claim when auth.json has none.
function accountIDFromToken(token: string) {
  const parts = token.split(".");
  if (parts.length !== 3) return undefined;

  try {
    const claims = Claims.parse(JSON.parse(Buffer.from(parts[1], "base64url").toString("utf8")));
    return claims["https://api.openai.com/auth"]?.chatgpt_account_id;
  } catch {
    return undefined;
  }
}

const Claims = z.object({
  "https://api.openai.com/auth": lenient(z.object({ chatgpt_account_id: lenient(z.string()) })),
});

function usageWindow(window: Window | undefined, fallback: Window): UsageWindow | undefined {
  if (!window) return undefined;

  const tag = labelFromDuration(window.limit_window_seconds);
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

function usedPercent(window: Window) {
  const used = normalizePercent(window.used_percent);
  if (used !== undefined) return used;
  const remaining = normalizePercent(window.remaining_percent);
  return remaining === undefined ? undefined : 100 - remaining;
}

function resetAtFromWindow(window: Window, fallback: Window) {
  const absolute = resetAt(window.reset_at) ?? resetAt(fallback.reset_at);
  if (absolute) return absolute;

  const resetAfterSeconds = window.reset_after_seconds ?? fallback.reset_after_seconds;
  if (resetAfterSeconds === undefined || resetAfterSeconds < 0) return undefined;
  return new Date(Date.now() + resetAfterSeconds * 1000).toISOString();
}

function resetAt(value: string | number | undefined) {
  if (typeof value === "string") {
    return Number.isNaN(new Date(value).getTime()) ? undefined : value;
  }
  if (value === undefined) return undefined;

  const date = new Date(value * 1000);
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
}

// ├─ Usage payload ───────────────────────────────────────────────────────────────────────────────┤

type Window = z.infer<typeof Window>;

const Window = z.object({
  limit_window_seconds: lenient(z.number()),
  remaining_percent: lenient(z.number()),
  reset_after_seconds: lenient(z.number()),
  reset_at: lenient(z.union([z.string(), z.number()])),
  used_percent: lenient(z.number()),
});

const RateLimit = Window.extend({
  primary_window: lenient(Window),
  secondary_window: lenient(Window),
});

const Payload = z.object({ rate_limit: lenient(RateLimit) });
