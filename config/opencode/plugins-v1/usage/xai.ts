import { execFile } from "node:child_process";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";
import { z } from "zod";
import { readJson } from "../shared/file.ts";
import { isExpired } from "./auth.ts";
import { usageProviders } from "./providers.ts";
import { lenient, normalizePercent } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

const { id, label, staleAfterMS } = usageProviders.xai;

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ xAI usage                                                                                     │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

// ├─ Provider adapter ────────────────────────────────────────────────────────────────────────────┤

/** Loads xAI weekly credits and monthly usage; expired Grok credentials trigger a CLI refresh. */
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

  const token = auth.token;
  const creditsResult = await fetchBilling(BILLING_CREDITS_URL, token);
  const usageResult = await fetchBilling(BILLING_USAGE_URL, token);

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

// ├─ Billing requests ────────────────────────────────────────────────────────────────────────────┤

const BILLING_USAGE_URL = "https://cli-chat-proxy.grok.com/v1/billing?format=usage";
const BILLING_CREDITS_URL = "https://cli-chat-proxy.grok.com/v1/billing?format=credits";
const FETCH_TIMEOUT_MS = 15_000;

type FetchFailure =
  | { ok: false; kind: "network" }
  | { ok: false; kind: "timeout" }
  | { ok: false; kind: "http"; status: number };

type FetchResult = { ok: true; payload: Billing } | FetchFailure;

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
    if (response.status === 400 && Timeout.safeParse(await response.json().catch(() => undefined)).success) {
      return { ok: false, kind: "timeout" };
    }
    return { ok: false, kind: "http", status: response.status };
  }
  return { ok: true, payload: Billing.parse(await response.json()) };
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

const Timeout = z.object({
  code: z.literal("The operation was cancelled"),
  error: z.literal("Timeout expired"),
});

// ├─ Grok auth ───────────────────────────────────────────────────────────────────────────────────┤

const ISSUER = "https://auth.x.ai";
const GROK_REFRESH_TIMEOUT_MS = 30_000;
const GROK_REFRESH_MAX_OUTPUT = 64 * 1024;
const execFileAsync = promisify(execFile);

// A successful CLI exit does not guarantee fresh credentials.
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
  if (stored?.key && !isExpired(stored.expires_at)) return { ok: true, token: stored.key };

  const failure = await runGrokRefresh();
  if (failure) return { ok: false, reason: failure };

  const refreshed = await grokEntry();
  if (!refreshed?.key || isExpired(refreshed.expires_at)) {
    return { ok: false, reason: "grok login" };
  }
  return { ok: true, token: refreshed.key };
}

async function grokEntry() {
  const file = path.join(os.homedir(), ".grok", "auth.json");
  const auth = await readJson(file, GrokAuth).catch(() => undefined);
  if (!auth) return undefined;
  for (const [key, entry] of Object.entries(auth)) {
    if (entry && (entry.oidc_issuer === ISSUER || key.startsWith(`${ISSUER}::`))) return entry;
  }
  return undefined;
}

type AuthFailure = "no grok cli" | "refresh failed" | "refresh timeout" | "grok login";

type AuthResult = { ok: true; token: string } | { ok: false; reason: AuthFailure };

const Text = lenient(z.string().min(1));

// Grok auth keys include the issuer and client ID.
const GrokAuth = z.record(z.string(), lenient(z.object({ key: Text, expires_at: Text, oidc_issuer: Text })));

// ├─ Usage windows ───────────────────────────────────────────────────────────────────────────────┤

function windowsFromCredits(billing: Billing): UsageWindow[] {
  const end = weeklyReset(billing);
  const monthlyLimit = pick(billing, "monthlyLimit");
  const cap = pick(billing, "onDemandCap");
  const hasPositiveLimit = (monthlyLimit ?? 0) > 0 || (cap ?? 0) > 0;

  let creditPercent = normalizePercent(pick(billing, "creditUsagePercent"));
  // Unified billing can report 0% without a usable limit.
  if (creditPercent === 0 && !hasPositiveLimit && unifiedBilling(billing)) {
    creditPercent = undefined;
  }
  if (creditPercent !== undefined) {
    return [{ label: "W", usedPercent: creditPercent, resetAt: end }];
  }

  if (end) return [{ label: "W", resetAt: end }];
  return [];
}

function windowsFromUsage(billing: Billing): UsageWindow[] {
  const end = pick(billing, "billingPeriodEnd");
  const monthlyLimit = pick(billing, "monthlyLimit");
  const used = pick(billing, "used") ?? pick(billing, "totalUsed") ?? pick(billing, "includedUsed");
  const monthlyPercent = ratioPercent(used, monthlyLimit);
  if (monthlyPercent !== undefined) {
    return [{ label: "M", usedPercent: monthlyPercent, resetAt: end }];
  }

  const cap = pick(billing, "onDemandCap");
  const onDemandPercent = ratioPercent(pick(billing, "onDemandUsed"), cap);
  if (cap !== undefined && cap > 0 && onDemandPercent !== undefined) {
    return [{ label: "M", usedPercent: onDemandPercent, resetAt: end }];
  }
  return [];
}

function unifiedBilling(billing: Billing) {
  return pick(billing, "isUnifiedBillingUser") === true || billing.subscription?.isUnifiedBillingUser === true;
}

function weeklyReset(billing: Billing) {
  return (billing.config?.currentPeriod ?? billing.currentPeriod)?.end ?? pick(billing, "billingPeriodEnd");
}

function pick<K extends keyof Config>(billing: Billing, key: K) {
  return billing[key] ?? billing.config?.[key];
}

function ratioPercent(used: number | undefined, limit: number | undefined) {
  if (used === undefined || limit === undefined || limit <= 0) return undefined;
  return normalizePercent((used / limit) * 100);
}

// ├─ Billing payload ─────────────────────────────────────────────────────────────────────────────┤

// Billing amounts may be nested under `config` or wrapped in `{ val }` or `{ value }`.
const Amount: z.ZodType<number> = z.lazy(() =>
  z.union([
    z.number(),
    z.object({ val: Amount }).transform((wrapped) => wrapped.val),
    z.object({ value: Amount }).transform((wrapped) => wrapped.value),
  ]),
);

const Config = z.object({
  currentPeriod: lenient(z.object({ end: Text })),
  billingPeriodEnd: Text,
  isUnifiedBillingUser: lenient(z.boolean()),
  creditUsagePercent: lenient(Amount),
  monthlyLimit: lenient(Amount),
  used: lenient(Amount),
  includedUsed: lenient(Amount),
  totalUsed: lenient(Amount),
  onDemandCap: lenient(Amount),
  onDemandUsed: lenient(Amount),
});

const Billing = Config.extend({
  config: lenient(Config),
  subscription: lenient(z.object({ isUnifiedBillingUser: lenient(z.boolean()) })),
});

type Config = z.infer<typeof Config>;
type Billing = z.infer<typeof Billing>;
