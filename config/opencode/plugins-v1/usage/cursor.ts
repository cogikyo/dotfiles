import { z } from "zod";
import { readAuth } from "./auth.ts";
import { usageProviders } from "./providers.ts";
import { lenient } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

const { id, label, staleAfterMS } = usageProviders.cursor;
const DASHBOARD_URL = "https://api2.cursor.sh/aiserver.v1.DashboardService";
const FETCH_TIMEOUT_MS = 15_000;

export const GROK_BOT = "G";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Cursor usage                                                                                  │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

// ├─ Provider adapter ────────────────────────────────────────────────────────────────────────────┤

/**
 * Loads Cursor plan usage and the Grok Bot `G` window; sub-1% values are percentages, not fractions.
 * Plan reset inputs are epoch milliseconds; `G` reset inputs are ISO timestamps.
 */
export const cursorUsage: ProviderAdapter = {
  id,
  label,
  placeholders: ["C", "O", GROK_BOT],
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

  const [plan, grokBot] = await Promise.all([
    dashboard("GetCurrentPeriodUsage", access),
    dashboard("GetSandUsageStatus", access),
  ]);
  if (!plan.ok) return usage([], `${plan.status}`);

  const payload = Payload.parse(await plan.json());
  const cycleEnd = resetAt(payload.billingCycleEnd);
  const grokBotUsage = grokBot.ok ? GrokBotPayload.parse(await grokBot.json()) : undefined;
  const windows = [
    usageWindow("C", payload.planUsage?.autoPercentUsed, cycleEnd),
    usageWindow("O", payload.planUsage?.apiPercentUsed, cycleEnd),
    usageWindow(GROK_BOT, grokBotUsage?.usagePercent, isoTime(grokBotUsage?.nextResetTimestampUtc)),
  ].filter((window) => window !== undefined);

  if (windows.length === 0) return usage([], "no windows");
  return usage(windows);
}

function dashboard(method: string, access: string) {
  return fetch(`${DASHBOARD_URL}/${method}`, {
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

function usageWindow(windowLabel: string, percent: number | undefined, end: string | undefined) {
  const usedPercent = cursorPercent(percent);
  if (usedPercent === undefined) return undefined;
  return { label: windowLabel, usedPercent, resetAt: end };
}

function resetAt(value: number | string | undefined) {
  const ms = value === "" ? undefined : Number(value);
  if (ms === undefined || !Number.isFinite(ms)) return undefined;
  const date = new Date(ms);
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
}

function isoTime(value: string | undefined) {
  const date = new Date(value ?? "");
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

const GrokBotPayload = z.object({
  usagePercent: lenient(z.number()),
  nextResetTimestampUtc: lenient(z.string()),
});
