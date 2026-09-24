import { z } from "zod";
import { readAuth } from "./auth.ts";
import { usageProviders } from "./providers.ts";
import { lenient } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

const { id, label, staleAfterMS } = usageProviders.cursor;
const USAGE_URL = "https://api2.cursor.sh/aiserver.v1.DashboardService/GetCurrentPeriodUsage";
const FETCH_TIMEOUT_MS = 15_000;

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Cursor usage                                                                                  │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

// ├─ Provider adapter ────────────────────────────────────────────────────────────────────────────┤

/** Loads Cursor plan usage; sub-1% values are percentages, not fractions, and reset times are epoch milliseconds. */
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

async function load(): Promise<ProviderUsage> {
  const access = (await readAuth())?.cursor?.access;
  if (!access) return usage([], "no auth");

  const response = await fetch(USAGE_URL, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${access}`,
      Accept: "application/json",
      "Content-Type": "application/json",
      "User-Agent": "opencode-usage",
    },
    body: "{}",
    signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
  });
  if (!response.ok) return usage([], `${response.status}`);

  const payload = Payload.parse(await response.json());
  const cycleEnd = resetAt(payload.billingCycleEnd);
  const windows = [
    usageWindow("C", payload.planUsage?.autoPercentUsed, cycleEnd),
    usageWindow("O", payload.planUsage?.apiPercentUsed, cycleEnd),
  ].filter((window) => window !== undefined);

  if (windows.length === 0) return usage([], "no windows");
  return usage(windows);
}

function usage(windows: UsageWindow[], note?: string): ProviderUsage {
  return { id, label, windows, note };
}

// ├─ Percent and reset ───────────────────────────────────────────────────────────────────────────┤

function cursorPercent(value: number | undefined) {
  if (value === undefined) return undefined;
  const pct = Math.max(0, Math.min(100, value));
  if (pct === 0) return 0;
  if (pct < 1) return 1;
  return pct;
}

function usageWindow(windowLabel: string, percent: number | undefined, cycleEnd: string | undefined) {
  const usedPercent = cursorPercent(percent);
  if (usedPercent === undefined) return undefined;
  return { label: windowLabel, usedPercent, resetAt: cycleEnd };
}

function resetAt(value: number | string | undefined) {
  const ms = value === "" ? undefined : Number(value);
  if (ms === undefined || !Number.isFinite(ms)) return undefined;
  const date = new Date(ms);
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
}

// ├─ Usage payload ───────────────────────────────────────────────────────────────────────────────┤

const Payload = z.object({
  planUsage: lenient(
    z.object({
      autoPercentUsed: lenient(z.number()),
      apiPercentUsed: lenient(z.number()),
    }),
  ),
  billingCycleEnd: lenient(z.union([z.number(), z.string()])),
});
