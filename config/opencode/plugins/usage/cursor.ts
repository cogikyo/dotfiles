import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { usageProviders } from "./providers.ts";
import { num, record, str } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

const { id, label, staleAfterMS } = usageProviders.cursor;
const origin = "https://api2.cursor.sh";
const keyFile = path.join(os.homedir(), ".local", "share", "dotfiles", "cursor-api-key");

let token: string | undefined;

export const cursorUsage: ProviderAdapter = {
  id,
  label,
  placeholders: ["C", "O", "G"],
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
  const key = await fs.readFile(keyFile, "utf8").then(
    (content) => content.trim(),
    () => "",
  );
  if (!key) return note("no auth", "warn");

  if (!token) {
    const access = await exchange(key);
    if (typeof access !== "string") return access;
    token = access;
  }

  const [plan, grok] = await Promise.all([
    post("/aiserver.v1.DashboardService/GetCurrentPeriodUsage", token),
    post("/aiserver.v1.DashboardService/GetSandUsageStatus", token),
  ]);
  if (!plan) return note("request failed");
  if (plan.status === 401) token = undefined;
  if (!plan.ok) return note(`${plan.status}`);

  const windows = await parse(plan, grok);
  return windows.length ? { id, label, windows } : note("no windows");
}

async function exchange(key: string): Promise<string | ProviderUsage> {
  const response = await post("/auth/exchange_user_api_key", key);
  if (!response) return note("request failed");
  if (response.status === 401 || response.status === 403) return note("invalid key");
  if (!response.ok) return note(`${response.status}`);
  return str(record(await response.json().catch(() => undefined))?.accessToken) ?? note("invalid token");
}

async function parse(plan: Response, grok: Response | undefined) {
  const usage = record(await plan.json().catch(() => undefined));
  const quota = record(usage?.planUsage);
  const cycleEnd = time(usage?.billingCycleEnd);
  const bot = grok?.ok ? record(await grok.json().catch(() => undefined)) : undefined;
  return [
    window("C", num(quota?.autoPercentUsed), cycleEnd),
    window("O", num(quota?.apiPercentUsed), cycleEnd),
    window("G", num(bot?.usagePercent), time(bot?.nextResetTimestampUtc)),
  ].filter((item) => item !== undefined);
}

function post(route: string, bearer: string) {
  return fetch(`${origin}${route}`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${bearer}`,
      Accept: "application/json",
      "Content-Type": "application/json",
      "User-Agent": "opencode-usage",
    },
    body: "{}",
    redirect: "error",
    signal: AbortSignal.timeout(15_000),
  }).catch(() => undefined);
}

function window(tag: string, percent: number | undefined, resetAt: string | undefined): UsageWindow | undefined {
  if (percent === undefined) return undefined;
  const used = Math.max(0, Math.min(100, percent));
  return { label: tag, usedPercent: used > 0 && used < 1 ? 1 : used, resetAt };
}

function time(value: unknown) {
  if (typeof value !== "string" && typeof value !== "number") return undefined;
  const date = new Date(typeof value === "string" && /^\d+$/.test(value) ? Number(value) : value);
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
}

function note(text: string, noteKind: ProviderUsage["noteKind"] = "error"): ProviderUsage {
  return { id, label, windows: [], note: text, noteKind };
}
