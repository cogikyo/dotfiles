import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { readAuth, readClaudeCredentials } from "./auth.ts";
import { usageProviders } from "./providers.ts";
import { normalizePercent } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

type AuthFile = {
  anthropic?: {
    type?: string;
    access?: string;
  };
};

type AnthropicWindow = {
  utilization?: unknown;
  resets_at?: unknown;
};

type AnthropicUsagePayload = {
  five_hour?: AnthropicWindow | null;
  seven_day?: AnthropicWindow | null;
  limits?: AnthropicLimit[] | null;
};

type AnthropicLimit = {
  kind?: unknown;
  group?: unknown;
  percent?: unknown;
  resets_at?: unknown;
  scope?: AnthropicLimitScope | null;
};

type AnthropicLimitScope = {
  model?: {
    display_name?: unknown;
  } | null;
};

const { id, label, staleAfterMS } = usageProviders.anthropic;
const FETCH_TIMEOUT_MS = 15_000;
const CLAUDE_REFRESH_TIMEOUT_MS = 60_000;
const RECOVER_COOLDOWN_MS = 5 * 60_000;
const execFileAsync = promisify(execFile);

// Single-flight: only one refresh in progress at a time.
let refreshing: Promise<boolean> | null = null;
// Cooldown: suppress repeated recovery attempts within a window.
let lastRecoverAt = 0;

function usage(windows: UsageWindow[], note?: string, noteKind?: ProviderUsage["noteKind"]): ProviderUsage {
  return { id, label, windows, note, noteKind };
}

function resetAt(window: AnthropicWindow) {
  return typeof window.resets_at === "string" ? window.resets_at : undefined;
}

function usageWindow(
  label: string,
  window?: AnthropicWindow | null,
): UsageWindow | undefined {
  if (!window) return undefined;
  const usedPercent = normalizePercent(window.utilization);
  if (usedPercent === undefined) return undefined;
  return { label, usedPercent, resetAt: resetAt(window) };
}

function scopedLabel(displayName: string) {
  const words = displayName
    .trim()
    .split(/\s+/)
    .filter((word) => word.toLowerCase() !== "claude");
  const first = Array.from(words[0] ?? "")[0];
  return first?.toUpperCase();
}

function scopedWindow(limit: AnthropicLimit): UsageWindow | undefined {
  if (limit.kind !== "weekly_scoped" || limit.group !== "weekly") {
    return undefined;
  }

  const displayName = limit.scope?.model?.display_name;
  if (typeof displayName !== "string") return undefined;

  const label = scopedLabel(displayName);
  if (!label) return undefined;

  const usedPercent = normalizePercent(limit.percent);
  if (usedPercent === undefined) return undefined;

  return {
    label,
    usedPercent,
    resetAt: typeof limit.resets_at === "string" ? limit.resets_at : undefined,
  };
}

function scopedWindows(limits?: AnthropicLimit[] | null) {
  if (!limits) return [];
  return limits
    .map(scopedWindow)
    .filter((window): window is UsageWindow => Boolean(window));
}

function isExpired(expiresAt: string | undefined) {
  if (typeof expiresAt !== "string") return true;
  const ms = Date.parse(expiresAt);
  if (!Number.isFinite(ms)) return true;
  return Date.now() >= ms;
}

async function triggerClaudeRefresh(): Promise<boolean> {
  try {
    await execFileAsync("claude", ["-p", ".", "--model", "haiku"], {
      timeout: CLAUDE_REFRESH_TIMEOUT_MS,
      maxBuffer: 16_384,
      cwd: "/tmp",
      shell: false,
    });
    return true;
  } catch {
    return false;
  }
}

async function tryRecoverAuth(): Promise<string | undefined> {
  // Single-flight: deduplicate concurrent calls.
  if (refreshing) return (await refreshing) ? readTokenFromClaude() : undefined;

  // Cooldown: skip if we already tried recently.
  if (Date.now() - lastRecoverAt < RECOVER_COOLDOWN_MS) return undefined;

  const recovery = (async () => {
    // Bail if no Claude credential file exists — nothing to refresh.
    const creds = await readClaudeCredentials();
    if (!creds) return false;

    const ok = await triggerClaudeRefresh();
    lastRecoverAt = Date.now();
    return ok;
  })();

  refreshing = recovery;
  try {
    return (await recovery) ? readTokenFromClaude() : undefined;
  } finally {
    refreshing = null;
  }
}

async function readTokenFromClaude(): Promise<string | undefined> {
  const creds = await readClaudeCredentials();
  if (!creds?.accessToken) return undefined;
  if (isExpired(creds.expiresAt)) return undefined;
  return creds.accessToken;
}

async function fetchUsage(token: string): Promise<ProviderUsage> {
  const response = await fetch("https://api.anthropic.com/api/oauth/usage", {
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: "application/json",
      "User-Agent": "opencode-usage",
      "anthropic-beta": "oauth-2025-04-20",
      "anthropic-version": "2023-06-01",
    },
    signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
  });
  if (!response.ok) return usage([], `${response.status}`);

  const payload = (await response.json()) as AnthropicUsagePayload;
  const windows = [
    usageWindow("H", payload.five_hour),
    usageWindow("W", payload.seven_day),
    ...scopedWindows(payload.limits),
  ].filter((window): window is UsageWindow => Boolean(window));

  if (windows.length === 0) return usage([], "no windows");
  return usage(windows);
}

async function load(): Promise<ProviderUsage> {
  const auth = await readAuth<AuthFile>();
  const anthropic = auth.anthropic;

  if (!anthropic || anthropic.type !== "oauth" || !anthropic.access) {
    return usage([], "no auth");
  }

  const result = await fetchUsage(anthropic.access);
  // Healthy path: got data or a non-401 error. No recovery needed.
  if (result.note === undefined || result.note !== "401") return result;

  // 401: attempt bounded recovery with Claude CLI, then retry once.
  const recoveredToken = await tryRecoverAuth();
  if (!recoveredToken) return usage([], "auth recovery failed", "warn");

  const retried = await fetchUsage(recoveredToken);
  // Post-recovery 401 stays hard; no further retries.
  return retried;
}

export const anthropicUsage: ProviderAdapter = {
  id,
  label,
  poll: {
    minFetchIntervalMS: 2 * 60_000,
    errorBackoffMS: 5 * 60_000,
    warnBackoffMS: 0,
    rateLimitBackoffMS: 60 * 60_000,
    staleAfterMS,
  },
  load,
};
