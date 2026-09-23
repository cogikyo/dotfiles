import { readAuth } from "./auth.ts";
import { usageProviders } from "./providers.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

type AuthFile = {
  cursor?: {
    type?: string;
    access?: string;
  };
};

type CursorPlanUsage = {
  autoPercentUsed?: unknown;
  apiPercentUsed?: unknown;
};

type CursorUsagePayload = {
  billingCycleEnd?: unknown;
  planUsage?: CursorPlanUsage | null;
};

// These fields are already percentages; values between 0 and 1 mean less than 1%, not fractions.
const { id, label, staleAfterMS } = usageProviders.cursor;
const USAGE_URL = "https://api2.cursor.sh/aiserver.v1.DashboardService/GetCurrentPeriodUsage";
const FETCH_TIMEOUT_MS = 15_000;

function usage(windows: UsageWindow[], note?: string): ProviderUsage {
  return { id, label, windows, note };
}

function resetAt(value: unknown) {
  const ms =
    typeof value === "number" ? value : typeof value === "string" && value.length > 0 ? Number(value) : undefined;
  if (ms === undefined || !Number.isFinite(ms)) return undefined;
  const date = new Date(ms);
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
}

function cursorPercent(value: unknown) {
  if (typeof value !== "number" || !Number.isFinite(value)) return undefined;
  const pct = Math.max(0, Math.min(100, value));
  if (pct === 0) return 0;
  // The dashboard rounds nonzero usage below 1% up to 1%.
  if (pct < 1) return 1;
  return pct;
}

function usageWindow(windowLabel: string, percent: unknown, cycleEnd: string | undefined): UsageWindow | undefined {
  const usedPercent = cursorPercent(percent);
  if (usedPercent === undefined) return undefined;
  return { label: windowLabel, usedPercent, resetAt: cycleEnd };
}

async function load(): Promise<ProviderUsage> {
  const auth = await readAuth<AuthFile>();
  const cursor = auth.cursor;

  if (!cursor || cursor.type !== "oauth" || !cursor.access) {
    return usage([], "no auth");
  }

  const response = await fetch(USAGE_URL, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${cursor.access}`,
      Accept: "application/json",
      "Content-Type": "application/json",
      "User-Agent": "opencode-usage",
    },
    body: "{}",
    signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
  });
  if (!response.ok) return usage([], `${response.status}`);

  const payload = (await response.json()) as CursorUsagePayload;
  const cycleEnd = resetAt(payload.billingCycleEnd);
  const windows = [
    usageWindow("C", payload.planUsage?.autoPercentUsed, cycleEnd),
    usageWindow("O", payload.planUsage?.apiPercentUsed, cycleEnd),
  ].filter((window): window is UsageWindow => Boolean(window));

  if (windows.length === 0) return usage([], "no windows");
  return usage(windows);
}

/** Usage adapter for Cursor plan limits. */
export const cursorUsage: ProviderAdapter = {
  id,
  label,
  placeholders: ["C", "O"],
  poll: {
    minFetchIntervalMS: 60_000,
    errorBackoffMS: 60_000,
    warnBackoffMS: 0,
    rateLimitBackoffMS: 10 * 60_000,
    staleAfterMS,
  },
  load,
};
