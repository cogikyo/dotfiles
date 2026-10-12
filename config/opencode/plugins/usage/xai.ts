import { execFile } from "node:child_process";
import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";
import { usageProviders } from "./providers.ts";
import { normalizePercent, num, record, text, type Json } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

const { id, label, staleAfterMS } = usageProviders.xai;

export const xaiUsage: ProviderAdapter = {
  id,
  label,
  placeholders: ["W", "M"],
  poll: {
    minFetchIntervalMS: 5 * 60_000,
    errorBackoffMS: 5 * 60_000,
    warnBackoffMS: 0,
    rateLimitBackoffMS: 15 * 60_000,
    staleAfterMS,
  },
  load,
};

async function load(): Promise<ProviderUsage> {
  const auth = await liveAuth();
  if (!auth.ok) return usage([], auth.reason, "warn");

  const creditsResult = await fetchBilling(BILLING_CREDITS_URL, auth.token);
  const usageResult = await fetchBilling(BILLING_USAGE_URL, auth.token);

  const failed = statusNote([usageResult, creditsResult]);
  if (failed) return failed;

  const windows = [
    ...(creditsResult.ok ? windowsFromCredits(creditsResult.payload) : []),
    ...(usageResult.ok ? windowsFromUsage(usageResult.payload) : []),
  ];
  if (windows.length === 0) return usage([], "no usage", "warn");
  return usage(windows);
}

function usage(windows: UsageWindow[], note?: string, noteKind?: ProviderUsage["noteKind"]): ProviderUsage {
  return { id, label, windows, note, noteKind };
}

const BILLING_USAGE_URL = "https://cli-chat-proxy.grok.com/v1/billing?format=usage";
const BILLING_CREDITS_URL = "https://cli-chat-proxy.grok.com/v1/billing?format=credits";
const FETCH_TIMEOUT_MS = 15_000;

type FetchFailure =
  | { ok: false; kind: "network" }
  | { ok: false; kind: "timeout" }
  | { ok: false; kind: "http"; status: number };

type FetchResult = { ok: true; payload: Json } | FetchFailure;

async function fetchBillingOnce(url: string, token: string): Promise<FetchResult> {
  let response: Response;
  try {
    response = await fetch(url, {
      headers: {
        Authorization: `Bearer ${token}`,
        "X-XAI-Token-Auth": "xai-grok-cli",
        Accept: "application/json",
        "User-Agent": "opencode-usage",
      },
      signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
    });
  } catch {
    return { ok: false, kind: "network" };
  }
  if (!response.ok) {
    const body = record(await response.json().catch(() => undefined));
    if (response.status === 400 && body?.code === "The operation was cancelled" && body.error === "Timeout expired") {
      return { ok: false, kind: "timeout" };
    }
    return { ok: false, kind: "http", status: response.status };
  }
  const payload = record(await response.json());
  if (!payload) throw new Error("invalid billing");
  return { ok: true, payload };
}

async function fetchBilling(url: string, token: string): Promise<FetchResult> {
  const first = await fetchBillingOnce(url, token);
  return !first.ok && first.kind === "timeout" ? fetchBillingOnce(url, token) : first;
}

function statusNote(results: FetchResult[]): ProviderUsage | undefined {
  const failures = results.filter((result): result is FetchFailure => !result.ok);
  if (failures.length !== results.length) return undefined;
  if (failures.some((result) => result.kind === "http" && result.status === 429)) {
    return usage([], "429", "error");
  }
  const http = failures.find((result) => result.kind === "http");
  if (http?.kind === "http") return usage([], `${http.status}`, "error");
  if (failures.some((result) => result.kind === "timeout")) return usage([], "timeout", "error");
  return usage([], "unavailable", "error");
}

const ISSUER = "https://auth.x.ai";
const GROK_REFRESH_TIMEOUT_MS = 30_000;
const GROK_REFRESH_MAX_OUTPUT = 64 * 1024;
const execFileAsync = promisify(execFile);

type AuthFailure = "no grok cli" | "refresh failed" | "refresh timeout" | "grok login";

type AuthResult = { ok: true; token: string } | { ok: false; reason: AuthFailure };

async function runGrokRefresh(): Promise<AuthFailure | undefined> {
  try {
    await execFileAsync(process.env.GROK_CLI || "grok", ["models"], {
      timeout: GROK_REFRESH_TIMEOUT_MS,
      maxBuffer: GROK_REFRESH_MAX_OUTPUT,
    });
    return undefined;
  } catch (error) {
    if (!(error instanceof Error)) return "refresh failed";
    if ("code" in error && error.code === "ENOENT") return "no grok cli";
    if ("killed" in error && error.killed === true) return "refresh timeout";
    return "refresh failed";
  }
}

async function liveAuth(): Promise<AuthResult> {
  const stored = await grokEntry();
  if (stored?.key && !isExpired(stored.expiresAt)) return { ok: true, token: stored.key };

  const failure = await runGrokRefresh();
  if (failure) return { ok: false, reason: failure };

  const refreshed = await grokEntry();
  if (!refreshed?.key || isExpired(refreshed.expiresAt)) {
    return { ok: false, reason: "grok login" };
  }
  return { ok: true, token: refreshed.key };
}

async function grokEntry() {
  const file = path.join(os.homedir(), ".grok", "auth.json");
  const auth = await fs
    .readFile(file, "utf8")
    .then((content) => record(JSON.parse(content)))
    .catch(() => undefined);
  if (!auth) return undefined;
  for (const [key, value] of Object.entries(auth)) {
    const entry = record(value);
    if (!entry) continue;
    if (text(entry.oidc_issuer) === ISSUER || key.startsWith(`${ISSUER}::`)) {
      return { key: text(entry.key), expiresAt: text(entry.expires_at) };
    }
  }
  return undefined;
}

function isExpired(expiresAt: string | undefined) {
  const ms = Date.parse(expiresAt ?? "");
  return !Number.isFinite(ms) || Date.now() >= ms;
}

function amount(value: unknown): number | undefined {
  const direct = num(value);
  if (direct !== undefined) return direct;
  const wrapped = record(value);
  if (!wrapped) return undefined;
  return "val" in wrapped ? amount(wrapped.val) : "value" in wrapped ? amount(wrapped.value) : undefined;
}

function windowsFromCredits(billing: Json): UsageWindow[] {
  const end = weeklyReset(billing);
  const monthlyLimit = pickAmount(billing, "monthlyLimit");
  const cap = pickAmount(billing, "onDemandCap");
  const hasPositiveLimit = (monthlyLimit ?? 0) > 0 || (cap ?? 0) > 0;

  let creditPercent = normalizePercent(pickAmount(billing, "creditUsagePercent"));
  if (creditPercent === 0 && !hasPositiveLimit && unifiedBilling(billing)) {
    creditPercent = undefined;
  }
  if (creditPercent !== undefined) {
    return [{ label: "W", usedPercent: creditPercent, resetAt: end }];
  }

  if (end) return [{ label: "W", resetAt: end }];
  return [];
}

function windowsFromUsage(billing: Json): UsageWindow[] {
  const end = pickText(billing, "billingPeriodEnd");
  const monthlyLimit = pickAmount(billing, "monthlyLimit");
  const used = pickAmount(billing, "used") ?? pickAmount(billing, "totalUsed") ?? pickAmount(billing, "includedUsed");
  const monthlyPercent = ratioPercent(used, monthlyLimit);
  if (monthlyPercent !== undefined) {
    return [{ label: "M", usedPercent: monthlyPercent, resetAt: end }];
  }

  const cap = pickAmount(billing, "onDemandCap");
  const onDemandPercent = ratioPercent(pickAmount(billing, "onDemandUsed"), cap);
  if (cap !== undefined && cap > 0 && onDemandPercent !== undefined) {
    return [{ label: "M", usedPercent: onDemandPercent, resetAt: end }];
  }
  return [];
}

function unifiedFlag(value: Json | undefined) {
  return value?.isUnifiedBillingUser === true;
}

function unifiedBilling(billing: Json) {
  return unifiedFlag(billing) || unifiedFlag(record(billing.config)) || unifiedFlag(record(billing.subscription));
}

function weeklyReset(billing: Json) {
  const period = record(record(billing.config)?.currentPeriod) ?? record(billing.currentPeriod);
  return text(period?.end) ?? pickText(billing, "billingPeriodEnd");
}

function pickAmount(billing: Json, key: string) {
  return amount(billing[key]) ?? amount(record(billing.config)?.[key]);
}

function pickText(billing: Json, key: string) {
  return text(billing[key]) ?? text(record(billing.config)?.[key]);
}

function ratioPercent(used: number | undefined, limit: number | undefined) {
  if (used === undefined || limit === undefined || limit <= 0) return undefined;
  return normalizePercent((used / limit) * 100);
}
