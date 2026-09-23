import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { usageProviders } from "./providers.ts";
import { normalizePercent, record } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

// Loads credentials from the Grok CLI auth file for xAI billing requests.
const { id, label, staleAfterMS } = usageProviders.xai;
const ISSUER = "https://auth.x.ai";
const BILLING_USAGE_URL = "https://cli-chat-proxy.grok.com/v1/billing?format=usage";
const BILLING_CREDITS_URL = "https://cli-chat-proxy.grok.com/v1/billing?format=credits";
// Bound requests so an unresponsive endpoint cannot hold the provider lock indefinitely.
const FETCH_TIMEOUT_MS = 15_000;
const GROK_REFRESH_TIMEOUT_MS = 30_000;
// Limit command output because only the refreshed auth entry is used.
const GROK_REFRESH_MAX_OUTPUT = 64 * 1024;
const execFileAsync = promisify(execFile);

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ xAI usage                                                                                     │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

type GrokAuthEntry = {
  key?: string;
  auth_mode?: string;
  expires_at?: string;
  oidc_issuer?: string;
};

// The auth file maps `<issuer>::<client_id>` keys to credentials.
type GrokAuthFile = Record<string, GrokAuthEntry>;

// ├─ Billing payload formats ─────────────────────────────────────────────────────────────────────┤
// Billing fields may be nested under config and wrapped as `{ val }` or `{ value }`.
type BillingConfig = {
  currentPeriod?: {
    type?: unknown;
    start?: unknown;
    end?: unknown;
  };
  billingPeriodEnd?: unknown;
  subscriptionTier?: unknown;
  isUnifiedBillingUser?: unknown;
  creditUsagePercent?: unknown;
  monthlyLimit?: unknown;
  // Older billing responses use these usage field names.
  used?: unknown;
  includedUsed?: unknown;
  totalUsed?: unknown;
  onDemandCap?: unknown;
  onDemandUsed?: unknown;
};

type BillingPayload = BillingConfig & {
  config?: BillingConfig;
  subscription?: unknown;
};

// ├─ Usage windows ───────────────────────────────────────────────────────────────────────────────┤
function usage(windows: UsageWindow[], note?: string, noteKind?: ProviderUsage["noteKind"]): ProviderUsage {
  return { id, label, windows, note, noteKind };
}

// Billing amounts may be numbers or wrapped in val/value objects.
function num(value: unknown): number | undefined {
  if (typeof value === "number") return Number.isFinite(value) ? value : undefined;
  if (value && typeof value === "object") {
    const wrapped = record(value);
    if (!wrapped) return undefined;
    if ("val" in wrapped) return num(wrapped.val);
    if ("value" in wrapped) return num(wrapped.value);
  }
  return undefined;
}

function str(value: unknown) {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

// Prefer top-level fields and use config fields when they are absent.
function pick<K extends keyof BillingConfig>(payload: BillingPayload, key: K) {
  return payload[key] ?? payload.config?.[key];
}

function ratioPercent(used: number | undefined, limit: number | undefined) {
  if (used === undefined || limit === undefined || limit <= 0) return undefined;
  return normalizePercent((used / limit) * 100);
}

function isExpired(expiresAt: string | undefined) {
  if (typeof expiresAt !== "string") return true;
  const ms = Date.parse(expiresAt);
  if (!Number.isFinite(ms)) return true;
  return Date.now() >= ms;
}

async function readGrokAuth(): Promise<GrokAuthFile | undefined> {
  const file = path.join(os.homedir(), ".grok", "auth.json");
  try {
    const parsed = record(JSON.parse(await fs.readFile(file, "utf8")));
    if (!parsed) return undefined;
    return Object.fromEntries(
      Object.entries(parsed)
        .map(([key, value]) => {
          const entry = record(value);
          if (!entry) return undefined;
          return [
            key,
            {
              key: str(entry.key),
              auth_mode: str(entry.auth_mode),
              expires_at: str(entry.expires_at),
              oidc_issuer: str(entry.oidc_issuer),
            },
          ] as const;
        })
        .filter((entry) => entry !== undefined),
    );
  } catch {
    return undefined;
  }
}

type AuthFailure = "no grok cli" | "refresh failed" | "refresh timeout" | "grok login";

type AuthResult = { ok: true; token: string } | { ok: false; reason: AuthFailure };

// Check auth.json after the command because `grok models` can exit successfully without refreshing.
async function runGrokRefresh(): Promise<AuthFailure | undefined> {
  try {
    await execFileAsync(process.env.GROK_CLI || "grok", ["models"], {
      timeout: GROK_REFRESH_TIMEOUT_MS,
      maxBuffer: GROK_REFRESH_MAX_OUTPUT,
    });
    return undefined;
  } catch (error) {
    const failure = record(error);
    if (failure?.code === "ENOENT") return "no grok cli";
    if (failure?.killed === true) return "refresh timeout";
    return "refresh failed";
  }
}

async function liveAuth(): Promise<AuthResult> {
  const stored = xaiEntry(await readGrokAuth());
  if (stored?.key && !isExpired(stored.expires_at)) return { ok: true, token: stored.key };

  const failure = await runGrokRefresh();
  if (failure) return { ok: false, reason: failure };

  const refreshed = xaiEntry(await readGrokAuth());
  // A successful command without a live token still requires the user to log in.
  if (!refreshed?.key || isExpired(refreshed.expires_at)) {
    return { ok: false, reason: "grok login" };
  }
  return { ok: true, token: refreshed.key };
}

function xaiEntry(auth: GrokAuthFile | undefined) {
  if (!auth) return undefined;
  for (const [key, entry] of Object.entries(auth)) {
    if (!entry || typeof entry !== "object") continue;
    if (entry.oidc_issuer === ISSUER || key.startsWith(`${ISSUER}::`)) return entry;
  }
  return undefined;
}

function unifiedBilling(payload: BillingPayload) {
  if (pick(payload, "isUnifiedBillingUser") === true) return true;
  const sub = payload.subscription;
  if (!sub || typeof sub !== "object") return false;
  return record(sub)?.isUnifiedBillingUser === true;
}

function weeklyReset(payload: BillingPayload) {
  const period = payload.config?.currentPeriod ?? payload.currentPeriod ?? {};
  return str(period.end) ?? str(pick(payload, "billingPeriodEnd"));
}

function monthlyReset(payload: BillingPayload) {
  return str(pick(payload, "billingPeriodEnd"));
}

// ├─ Weekly and monthly calculations ─────────────────────────────────────────────────────────────┤
// Credits responses provide a weekly reset and may provide a weekly percentage.
function windowsFromCredits(payload: BillingPayload): UsageWindow[] {
  const end = weeklyReset(payload);
  const monthlyLimit = num(pick(payload, "monthlyLimit"));
  const cap = num(pick(payload, "onDemandCap"));
  const hasPositiveLimit = (monthlyLimit ?? 0) > 0 || (cap ?? 0) > 0;

  let creditPercent = normalizePercent(num(pick(payload, "creditUsagePercent")));
  // A zero percent without a positive unified-billing limit has no measurable basis.
  if (creditPercent === 0 && !hasPositiveLimit && unifiedBilling(payload)) {
    creditPercent = undefined;
  }
  if (creditPercent !== undefined) {
    return [{ label: "W", usedPercent: creditPercent, resetAt: end }];
  }

  // Preserve the weekly reset even when the response has no usable percentage.
  if (end) return [{ label: "W", resetAt: end }];
  return [];
}

// Usage responses describe the monthly credit pool.
function windowsFromUsage(payload: BillingPayload): UsageWindow[] {
  const end = monthlyReset(payload);
  const monthlyLimit = num(pick(payload, "monthlyLimit"));
  const used = num(pick(payload, "used")) ?? num(pick(payload, "totalUsed")) ?? num(pick(payload, "includedUsed"));
  const monthlyPercent = ratioPercent(used, monthlyLimit);
  if (monthlyPercent !== undefined) {
    return [{ label: "M", usedPercent: monthlyPercent, resetAt: end }];
  }

  const cap = num(pick(payload, "onDemandCap"));
  const onDemandPercent = ratioPercent(num(pick(payload, "onDemandUsed")), cap);
  if (cap !== undefined && cap > 0 && onDemandPercent !== undefined) {
    return [{ label: "M", usedPercent: onDemandPercent, resetAt: end }];
  }
  return [];
}

function mergeWindows(parts: UsageWindow[][]): UsageWindow[] {
  const byLabel = new Map<string, UsageWindow>();
  // Keep the first known percentage and fill missing fields from later responses.
  for (const windows of parts) {
    for (const window of windows) {
      const prev = byLabel.get(window.label);
      if (!prev) {
        byLabel.set(window.label, { ...window });
        continue;
      }
      if (prev.usedPercent === undefined && window.usedPercent !== undefined) {
        prev.usedPercent = window.usedPercent;
      }
      if (!prev.resetAt && window.resetAt) prev.resetAt = window.resetAt;
    }
  }
  // Keep weekly and monthly windows in sidebar order.
  const order = ["W", "M"];
  const ordered: UsageWindow[] = [];
  for (const key of order) {
    const window = byLabel.get(key);
    if (window) ordered.push(window);
  }
  for (const [key, window] of byLabel) {
    if (!order.includes(key)) ordered.push(window);
  }
  return ordered;
}

// ├─ Billing requests ────────────────────────────────────────────────────────────────────────────┤
type FetchFailure =
  | { ok: false; kind: "network" }
  | { ok: false; kind: "timeout" }
  | { ok: false; kind: "http"; status: number };

type FetchResult = { ok: true; payload: BillingPayload } | FetchFailure;

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
    if (response.status === 400) {
      const body = record(await response.json().catch(() => undefined));
      if (body?.code === "The operation was cancelled" && body.error === "Timeout expired") {
        return { ok: false, kind: "timeout" };
      }
    }
    return { ok: false, kind: "http", status: response.status };
  }
  const raw = record(await response.json());
  const config = record(raw?.config);
  const period = record(config?.currentPeriod);
  const currentPeriod = record(raw?.currentPeriod);
  return {
    ok: true,
    payload: {
      ...raw,
      currentPeriod: currentPeriod && { ...currentPeriod },
      config: config && { ...config, currentPeriod: period && { ...period } },
    },
  };
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

async function load(): Promise<ProviderUsage> {
  const auth = await liveAuth();
  if (!auth.ok) return usage([], auth.reason, "warn");

  const token = auth.token;
  const creditsResult = await fetchBilling(BILLING_CREDITS_URL, token);
  const usageResult = await fetchBilling(BILLING_USAGE_URL, token);

  const failed = statusNote([usageResult, creditsResult]);
  if (failed) return failed;

  const windows = mergeWindows([
    usageResult.ok ? windowsFromUsage(usageResult.payload) : [],
    creditsResult.ok ? windowsFromCredits(creditsResult.payload) : [],
  ]);
  if (windows.length === 0) return usage([], "no usage", "warn");
  return usage(windows);
}

// ├─ Provider adapter ────────────────────────────────────────────────────────────────────────────┤
/** Usage adapter for xAI monthly credits and weekly reset data. */
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
