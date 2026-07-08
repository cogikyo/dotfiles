import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { normalizePercent } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

// xAI usage reads only the Grok CLI auth at ~/.grok/auth.json, never OpenCode's xai OAuth.
// OpenCode's refreshed xai token got 401 on this billing endpoint; the Grok CLI token works.
//
// Two billing shapes share one host path and split by query:
// - `?format=usage` (and bare `/v1/billing`): monthly credit pool `used` / `monthlyLimit` + month end.
// - `?format=credits`: unified weekly period reset, optional `creditUsagePercent`, no stable burn basis.
// Both are polled so the sidebar can show monthly % and weekly reset without inventing a weekly %.
const id = "xai";
const label = "xAI";
const ISSUER = "https://auth.x.ai";
const BILLING_USAGE_URL = "https://cli-chat-proxy.grok.com/v1/billing?format=usage";
const BILLING_CREDITS_URL = "https://cli-chat-proxy.grok.com/v1/billing?format=credits";
// Bound each billing fetch so a hung endpoint cannot hold the shared usage provider lock indefinitely.
const FETCH_TIMEOUT_MS = 15_000;
const GROK_REFRESH_TIMEOUT_MS = 30_000;
const execFileAsync = promisify(execFile);

type GrokAuthEntry = {
  key?: string;
  auth_mode?: string;
  expires_at?: string;
  oidc_issuer?: string;
};

// ~/.grok/auth.json is an object keyed by `<issuer>::<client_id>`.
type GrokAuthFile = Record<string, GrokAuthEntry>;

// Observed live shape nests these under `config`, and money fields arrive wrapped
// as `{ val: number }`; older shapes put the same fields top-level as bare numbers.
// BillingConfig captures the overlapping keys so `pick` can read either location.
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
  // Live usage format uses bare `used`; older/history rows use included/total.
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

function usage(
  windows: UsageWindow[],
  note?: string,
  noteKind?: ProviderUsage["noteKind"],
): ProviderUsage {
  return { id, label, windows, note, noteKind };
}

// Money fields arrive either as a bare number or wrapped as `{ val }` (seen live) or `{ value }`.
function num(value: unknown): number | undefined {
  if (typeof value === "number") return Number.isFinite(value) ? value : undefined;
  if (value && typeof value === "object") {
    const wrapped = value as { val?: unknown; value?: unknown };
    if ("val" in wrapped) return num(wrapped.val);
    if ("value" in wrapped) return num(wrapped.value);
  }
  return undefined;
}

function str(value: unknown) {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

// Fields may sit top-level or under `config`; prefer top-level, fall back to config.
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
    const parsed = JSON.parse(await fs.readFile(file, "utf8")) as unknown;
    if (!parsed || typeof parsed !== "object") return undefined;
    return parsed as GrokAuthFile;
  } catch {
    return undefined;
  }
}

async function refreshGrokAuth() {
  try {
    await execFileAsync(process.env.GROK_CLI || "grok", ["models"], {
      timeout: GROK_REFRESH_TIMEOUT_MS,
      maxBuffer: 16_384,
    });
    return true;
  } catch {
    return false;
  }
}

async function rereadAfterRefresh() {
  if (!(await refreshGrokAuth())) return undefined;
  return readGrokAuth();
}

function xaiEntry(auth: GrokAuthFile) {
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
  return (sub as { isUnifiedBillingUser?: unknown }).isUnifiedBillingUser === true;
}

function weeklyReset(payload: BillingPayload) {
  const period = payload.config?.currentPeriod ?? payload.currentPeriod ?? {};
  return str(period.end) ?? str(pick(payload, "billingPeriodEnd"));
}

function monthlyReset(payload: BillingPayload) {
  return str(pick(payload, "billingPeriodEnd"));
}

// Credits shape: weekly period, optional creditUsagePercent. Never invent a weekly % from monthly used.
function windowsFromCredits(payload: BillingPayload): UsageWindow[] {
  const end = weeklyReset(payload);
  const monthlyLimit = num(pick(payload, "monthlyLimit"));
  const cap = num(pick(payload, "onDemandCap"));
  const hasPositiveLimit = (monthlyLimit ?? 0) > 0 || (cap ?? 0) > 0;

  let creditPercent = normalizePercent(num(pick(payload, "creditUsagePercent")));
  // Unified subscriptions with no positive cap or limit carry no percent basis; a constant
  // `creditUsagePercent: 0` in that shape is meaningless, so treat it as unknown, never 0%.
  if (creditPercent === 0 && !hasPositiveLimit && unifiedBilling(payload)) {
    creditPercent = undefined;
  }
  if (creditPercent !== undefined) {
    return [{ label: "W", usedPercent: creditPercent, resetAt: end }];
  }

  // Weekly reset alone is still useful; percent stays unknown rather than faking 0%.
  if (end) return [{ label: "W", resetAt: end }];
  return [];
}

// Usage shape: monthly credit pool. Prefer live `used`, then total/included history aliases.
function windowsFromUsage(payload: BillingPayload): UsageWindow[] {
  const end = monthlyReset(payload);
  const monthlyLimit = num(pick(payload, "monthlyLimit"));
  const used =
    num(pick(payload, "used")) ?? num(pick(payload, "totalUsed")) ?? num(pick(payload, "includedUsed"));
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
  // Prefer first non-empty percent per label; later rows only fill missing reset/percent.
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
  // Stable sidebar order: weekly before monthly.
  const order = ["W", "M"];
  const ordered: UsageWindow[] = [];
  for (const label of order) {
    const window = byLabel.get(label);
    if (window) ordered.push(window);
  }
  for (const [label, window] of byLabel) {
    if (!order.includes(label)) ordered.push(window);
  }
  return ordered;
}

type FetchResult =
  | { ok: true; payload: BillingPayload }
  | { ok: false; status?: number; kind: "network" | "http" };

async function fetchBilling(url: string, token: string): Promise<FetchResult> {
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
  if (!response.ok) return { ok: false, kind: "http", status: response.status };
  return { ok: true, payload: (await response.json()) as BillingPayload };
}

function statusNote(results: FetchResult[]): ProviderUsage | undefined {
  if (results.some((result) => result.ok)) return undefined;
  if (results.some((result) => result.kind === "http" && result.status === 429)) {
    return usage([], "429", "error");
  }
  const http = results.find((result) => result.kind === "http" && result.status !== undefined);
  if (http && http.kind === "http" && http.status !== undefined) {
    return usage([], `${http.status}`, "error");
  }
  return usage([], "unavailable", "error");
}

async function load(): Promise<ProviderUsage> {
  let auth = await readGrokAuth();
  if (!auth) auth = await rereadAfterRefresh();
  if (!auth) return usage([], "no auth", "warn");

  let entry = xaiEntry(auth);
  if (!entry?.key || isExpired(entry.expires_at)) {
    auth = (await rereadAfterRefresh()) ?? auth;
    entry = xaiEntry(auth);
  }
  if (!entry?.key) return usage([], "no auth", "warn");
  if (isExpired(entry.expires_at)) return usage([], "expired", "warn");

  const token = entry.key;
  const [usageResult, creditsResult] = await Promise.all([
    fetchBilling(BILLING_USAGE_URL, token),
    fetchBilling(BILLING_CREDITS_URL, token),
  ]);

  const failed = statusNote([usageResult, creditsResult]);
  if (failed) return failed;

  const windows = mergeWindows([
    usageResult.ok ? windowsFromUsage(usageResult.payload) : [],
    creditsResult.ok ? windowsFromCredits(creditsResult.payload) : [],
  ]);
  if (windows.length === 0) return usage([], "no usage", "warn");
  return usage(windows);
}

export const xaiUsage: ProviderAdapter = {
  id,
  label,
  placeholders: ["W", "M"],
  poll: {
    minFetchIntervalMS: 5 * 60_000,
    errorBackoffMS: 5 * 60_000,
    warnBackoffMS: 0,
    rateLimitBackoffMS: 15 * 60_000,
    staleAfterMS: 10 * 60_000,
  },
  load,
};
