import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { readAuth, readClaudeCredentials } from "./auth.ts";
import { usageProviders } from "./providers.ts";
import { normalizePercent, record } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

type AnthropicWindow = {
  utilization?: unknown;
  resets_at?: unknown;
};

type AnthropicLimit = {
  kind?: unknown;
  group?: unknown;
  percent?: unknown;
  resets_at?: unknown;
  scope?: unknown;
};

const { id, label, staleAfterMS } = usageProviders.anthropic;
const FETCH_TIMEOUT_MS = 15_000;
const CLAUDE_REFRESH_TIMEOUT_MS = 60_000;
const RECOVER_COOLDOWN_MS = 5 * 60_000;
const execFileAsync = promisify(execFile);

// Share one credential refresh across concurrent provider loads.
let refreshing: Promise<boolean> | null = null;
// Avoid repeating a failed refresh on every poll.
let lastRecoverAt = 0;

function usage(windows: UsageWindow[], note?: string, noteKind?: ProviderUsage["noteKind"]): ProviderUsage {
  return { id, label, windows, note, noteKind };
}

function resetAt(window: AnthropicWindow) {
  return typeof window.resets_at === "string" ? window.resets_at : undefined;
}

function usageWindow(tag: string, window?: AnthropicWindow | null): UsageWindow | undefined {
  if (!window) return undefined;
  const usedPercent = normalizePercent(window.utilization);
  if (usedPercent === undefined) return undefined;
  return { label: tag, usedPercent, resetAt: resetAt(window) };
}

function scopedLabel(displayName: string) {
  const word = displayName
    .trim()
    .split(/\s+/)
    .find((part) => part.toLowerCase() !== "claude");
  const first = Array.from(word ?? "")[0];
  return first?.toUpperCase();
}

function scopedWindow(limit: AnthropicLimit): UsageWindow | undefined {
  if (limit.kind !== "weekly_scoped" || limit.group !== "weekly") {
    return undefined;
  }

  const displayName = record(record(limit.scope)?.model)?.display_name;
  if (typeof displayName !== "string") return undefined;

  const tag = scopedLabel(displayName);
  if (!tag) return undefined;

  const usedPercent = normalizePercent(limit.percent);
  if (usedPercent === undefined) return undefined;

  return {
    label: tag,
    usedPercent,
    resetAt: typeof limit.resets_at === "string" ? limit.resets_at : undefined,
  };
}

function scopedWindows(limits?: AnthropicLimit[] | null) {
  if (!limits) return [];
  return limits.map(scopedWindow).filter((window): window is UsageWindow => Boolean(window));
}

function isExpired(expiresAt: string | number | undefined) {
  const ms = typeof expiresAt === "number" ? expiresAt : Date.parse(expiresAt ?? "");
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
  if (refreshing) return (await refreshing) ? readTokenFromClaude() : undefined;

  if (Date.now() - lastRecoverAt < RECOVER_COOLDOWN_MS) return undefined;

  const recovery = (async () => {
    const credentials = await readClaudeCredentials();
    if (credentials.length === 0) return false;

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
  const credentials = await readClaudeCredentials();
  return credentials.find((candidate) => candidate.accessToken && !isExpired(candidate.expiresAt))?.accessToken;
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

  const payload = record(await response.json());
  const limits = Array.isArray(payload?.limits)
    ? payload.limits.map(record).filter((limit) => limit !== undefined)
    : [];
  const windows = [
    usageWindow("H", record(payload?.five_hour)),
    usageWindow("W", record(payload?.seven_day)),
    ...scopedWindows(limits),
  ].filter((window): window is UsageWindow => Boolean(window));

  if (windows.length === 0) return usage([], "no windows");
  return usage(windows);
}

async function load(): Promise<ProviderUsage> {
  const auth = await readAuth();
  const anthropic = record(auth?.anthropic);

  if (anthropic?.type !== "oauth" || typeof anthropic.access !== "string" || !anthropic.access) {
    return usage([], "no auth");
  }

  const result = await fetchUsage(anthropic.access);
  // Only a 401 triggers credential recovery.
  if (result.note === undefined || result.note !== "401") return result;

  // Refresh once through the Claude CLI, then retry the usage request once.
  const recoveredToken = await tryRecoverAuth();
  if (!recoveredToken) return usage([], "auth recovery failed", "warn");

  const retried = await fetchUsage(recoveredToken);
  return retried;
}

/** Usage adapter for Anthropic OAuth account limits. */
export const anthropicUsage: ProviderAdapter = {
  id,
  label,
  poll: {
    minFetchIntervalMS: 5 * 60_000,
    errorBackoffMS: 5 * 60_000,
    warnBackoffMS: 0,
    rateLimitBackoffMS: 60 * 60_000,
    staleAfterMS,
  },
  load,
};
