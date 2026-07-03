import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

// xAI usage reads only the Grok CLI auth at ~/.grok/auth.json, never OpenCode's xai OAuth.
// OpenCode's refreshed xai token got 401 on this billing endpoint; the Grok CLI token works.
// This adapter surfaces subscription tier and current-period reset. A true burn percent
// only appears in live inference SSE `rate_limits.updated`, which is deliberately not tapped here.
const id = "xai";
const label = "xAI";
const ISSUER = "https://auth.x.ai";
const BILLING_URL = "https://cli-chat-proxy.grok.com/v1/billing?format=credits";
const MIN_FETCH_INTERVAL_MS = 5 * 60_000;
const ERROR_BACKOFF_MS = 5 * 60_000;
const RATE_LIMIT_BACKOFF_MS = 15 * 60_000;
const STALE_AFTER_MS = 10 * 60_000;
// Bound the billing fetch so a hung endpoint cannot hold the shared usage provider lock indefinitely.
const FETCH_TIMEOUT_MS = 15_000;

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
  subscriptionTier?: unknown;
  creditUsagePercent?: unknown;
  monthlyLimit?: unknown;
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

function normalizePercent(value: number | undefined) {
  if (value === undefined || Number.isNaN(value)) return undefined;
  const expanded = value > 0 && value < 1 ? value * 100 : value;
  return Math.max(0, Math.min(100, expanded));
}

function ratioPercent(used: number | undefined, limit: number | undefined) {
  if (used === undefined || limit === undefined || limit <= 0) return undefined;
  return normalizePercent((used / limit) * 100);
}

function tierFrom(payload: BillingPayload) {
  const direct = str(pick(payload, "subscriptionTier"));
  if (direct) return direct;
  const subscription = payload.subscription;
  if (typeof subscription === "string") return str(subscription);
  if (subscription && typeof subscription === "object") {
    return str((subscription as { tier?: unknown }).tier);
  }
  return undefined;
}

function untilDuration(end: string) {
  const ms = Date.parse(end) - Date.now();
  if (!Number.isFinite(ms) || ms <= 0) return undefined;
  const totalMinutes = Math.ceil(ms / 60_000);
  const days = Math.floor(totalMinutes / (24 * 60));
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  const minutes = totalMinutes % 60;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

function resetNote(tier: string | undefined, end: string | undefined) {
  const reset = end ? untilDuration(end) : undefined;
  if (tier && reset) return `${tier} · W resets ${reset}`;
  if (tier) return tier;
  if (reset) return `weekly reset ${reset}`;
  return "subscription active; no usage percent";
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

function xaiEntry(auth: GrokAuthFile) {
  for (const [key, entry] of Object.entries(auth)) {
    if (!entry || typeof entry !== "object") continue;
    if (entry.oidc_issuer === ISSUER || key.startsWith(`${ISSUER}::`)) return entry;
  }
  return undefined;
}

function interpret(payload: BillingPayload): ProviderUsage {
  const period = payload.config?.currentPeriod ?? payload.currentPeriod ?? {};
  const end = str(period.end);
  const tier = tierFrom(payload);

  const creditPercent = normalizePercent(num(pick(payload, "creditUsagePercent")));
  if (creditPercent !== undefined) {
    return usage([{ label: "W", usedPercent: creditPercent, resetAt: end }]);
  }

  const monthlyLimit = num(pick(payload, "monthlyLimit"));
  const includedPercent = ratioPercent(
    num(pick(payload, "totalUsed")) ?? num(pick(payload, "includedUsed")),
    monthlyLimit,
  );
  if (includedPercent !== undefined) {
    return usage([{ label: "W", usedPercent: includedPercent, resetAt: end }]);
  }

  const cap = num(pick(payload, "onDemandCap"));
  const onDemandPercent = ratioPercent(num(pick(payload, "onDemandUsed")), cap);
  if (cap !== undefined && cap > 0 && onDemandPercent !== undefined) {
    return usage([{ label: "W", usedPercent: onDemandPercent, resetAt: end }]);
  }

  // No numeric signal (expected on this unified subscription): keep it informational, never fake 0%.
  return usage([], resetNote(tier, end), "info");
}

async function load(): Promise<ProviderUsage> {
  const auth = await readGrokAuth();
  if (!auth) return usage([], "Grok CLI auth unavailable", "warn");

  const entry = xaiEntry(auth);
  if (!entry?.key) return usage([], "Grok CLI auth unavailable", "warn");
  if (isExpired(entry.expires_at)) return usage([], "Grok CLI token expired", "warn");

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
  let response: Response;
  try {
    response = await fetch(BILLING_URL, {
      headers: {
        Authorization: `Bearer ${entry.key}`,
        "X-XAI-Token-Auth": "xai-grok-cli",
        Accept: "application/json",
        "User-Agent": "opencode-usage",
      },
      signal: controller.signal,
    });
  } catch {
    // Timeout or network failure: degrade coarsely without leaking the token or endpoint details.
    return usage([], "usage unavailable", "error");
  } finally {
    clearTimeout(timeout);
  }
  if (response.status === 429) return usage([], "HTTP 429", "error");
  if (!response.ok) return usage([], `HTTP ${response.status}`, "error");

  const payload = (await response.json()) as BillingPayload;
  return interpret(payload);
}

export const xaiUsage: ProviderAdapter = {
  id,
  label,
  poll: {
    minFetchIntervalMS: MIN_FETCH_INTERVAL_MS,
    errorBackoffMS: ERROR_BACKOFF_MS,
    rateLimitBackoffMS: RATE_LIMIT_BACKOFF_MS,
    staleAfterMS: STALE_AFTER_MS,
  },
  load,
};
