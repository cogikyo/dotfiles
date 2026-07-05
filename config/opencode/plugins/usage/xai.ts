import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { normalizePercent } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

// xAI usage reads only the Grok CLI auth at ~/.grok/auth.json, never OpenCode's xai OAuth.
// OpenCode's refreshed xai token got 401 on this billing endpoint; the Grok CLI token works.
// This adapter surfaces the current-period reset, and a burn percent only when the billing
// payload exposes one. A true weekly burn percent otherwise appears only in live inference SSE
// `rate_limits.updated`, which is deliberately not tapped here, so the weekly row stays unknown.
const id = "xai";
const label = "xAI";
const ISSUER = "https://auth.x.ai";
const BILLING_URL = "https://cli-chat-proxy.grok.com/v1/billing?format=credits";
// Bound the billing fetch so a hung endpoint cannot hold the shared usage provider lock indefinitely.
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
  subscriptionTier?: unknown;
  isUnifiedBillingUser?: unknown;
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

function interpret(payload: BillingPayload): ProviderUsage {
  const period = payload.config?.currentPeriod ?? payload.currentPeriod ?? {};
  const end = str(period.end);

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
    return usage([{ label: "W", usedPercent: creditPercent, resetAt: end }]);
  }

  const includedPercent = ratioPercent(
    num(pick(payload, "totalUsed")) ?? num(pick(payload, "includedUsed")),
    monthlyLimit,
  );
  if (includedPercent !== undefined) {
    return usage([{ label: "W", usedPercent: includedPercent, resetAt: end }]);
  }

  const onDemandPercent = ratioPercent(num(pick(payload, "onDemandUsed")), cap);
  if (cap !== undefined && cap > 0 && onDemandPercent !== undefined) {
    return usage([{ label: "W", usedPercent: onDemandPercent, resetAt: end }]);
  }

  // No numeric signal (expected on this unified subscription): render one weekly row with a
  // real reset but an unknown percent, so the UI shows a muted "--" cell instead of faking 0%.
  return usage([{ label: "W", resetAt: end }]);
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

  let response: Response;
  try {
    response = await fetch(BILLING_URL, {
      headers: {
        Authorization: `Bearer ${entry.key}`,
        "X-XAI-Token-Auth": "xai-grok-cli",
        Accept: "application/json",
        "User-Agent": "opencode-usage",
      },
      signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
    });
  } catch {
    // Timeout or network failure: degrade coarsely without leaking the token or endpoint details.
    return usage([], "unavailable", "error");
  }
  if (response.status === 429) return usage([], "429", "error");
  if (!response.ok) return usage([], `${response.status}`, "error");

  const payload = (await response.json()) as BillingPayload;
  return interpret(payload);
}

export const xaiUsage: ProviderAdapter = {
  id,
  label,
  placeholders: ["W"],
  poll: {
    minFetchIntervalMS: 5 * 60_000,
    errorBackoffMS: 5 * 60_000,
    warnBackoffMS: 0,
    rateLimitBackoffMS: 15 * 60_000,
    staleAfterMS: 10 * 60_000,
  },
  load,
};
