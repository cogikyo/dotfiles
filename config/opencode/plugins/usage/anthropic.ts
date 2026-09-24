import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { z } from "zod";
import { isExpired, readAuth, readClaudeCredentials } from "./auth.ts";
import { usageProviders } from "./providers.ts";
import { lenient, normalizePercent } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

const { id, label, staleAfterMS } = usageProviders.anthropic;
const FETCH_TIMEOUT_MS = 15_000;
const CLAUDE_REFRESH_TIMEOUT_MS = 60_000;
const RECOVER_COOLDOWN_MS = 5 * 60_000;
const execFileAsync = promisify(execFile);

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Anthropic usage                                                                               │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

// ├─ Provider adapter ────────────────────────────────────────────────────────────────────────────┤

/** Loads Anthropic OAuth usage windows; a 401 triggers one shared Claude credential recovery before retrying. */
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

async function load(): Promise<ProviderUsage> {
  const access = (await readAuth())?.anthropic?.access;
  if (!access) return usage([], "no auth");

  const result = await fetchUsage(access);
  if (result.note !== "401") return result;

  const recoveredToken = await tryRecoverAuth();
  if (!recoveredToken) return usage([], "auth recovery failed", "warn");

  return fetchUsage(recoveredToken);
}

function usage(windows: UsageWindow[], note?: string, noteKind?: ProviderUsage["noteKind"]): ProviderUsage {
  return { id, label, windows, note, noteKind };
}

// ├─ Usage request ───────────────────────────────────────────────────────────────────────────────┤

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

  const payload = Payload.parse(await response.json());
  const windows = [
    usageWindow("H", payload.five_hour),
    usageWindow("W", payload.seven_day),
    ...(payload.limits ?? []).map(scopedWindow),
  ].filter((window) => window !== undefined);

  if (windows.length === 0) return usage([], "no windows");
  return usage(windows);
}

// ├─ Auth recovery ───────────────────────────────────────────────────────────────────────────────┤

let refreshing: Promise<boolean> | null = null;
let lastRecoverAt = 0; // Limit CLI recovery after a credential file was found.

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

// ├─ Usage windows ───────────────────────────────────────────────────────────────────────────────┤

function usageWindow(tag: string, window: Window | undefined): UsageWindow | undefined {
  const usedPercent = normalizePercent(window?.utilization);
  if (usedPercent === undefined) return undefined;
  return { label: tag, usedPercent, resetAt: window?.resets_at };
}

function scopedLabel(displayName: string) {
  const word = displayName
    .trim()
    .split(/\s+/)
    .find((part) => part.toLowerCase() !== "claude");
  const first = Array.from(word ?? "")[0];
  return first?.toUpperCase();
}

function scopedWindow(limit: Limit | undefined): UsageWindow | undefined {
  if (limit?.kind !== "weekly_scoped" || limit.group !== "weekly") {
    return undefined;
  }

  const displayName = limit.scope?.model?.display_name;
  if (displayName === undefined) return undefined;

  const tag = scopedLabel(displayName);
  if (!tag) return undefined;

  const usedPercent = normalizePercent(limit.percent);
  if (usedPercent === undefined) return undefined;

  return { label: tag, usedPercent, resetAt: limit.resets_at };
}

// ├─ Usage payload ───────────────────────────────────────────────────────────────────────────────┤

type Window = z.infer<typeof Window>;
type Limit = z.infer<typeof Limit>;

const Window = z.object({
  utilization: lenient(z.number()),
  resets_at: lenient(z.string()),
});

const Limit = z.object({
  kind: lenient(z.string()),
  group: lenient(z.string()),
  percent: lenient(z.number()),
  resets_at: lenient(z.string()),
  scope: lenient(z.object({ model: lenient(z.object({ display_name: lenient(z.string()) })) })),
});

const Payload = z.object({
  five_hour: lenient(Window),
  seven_day: lenient(Window),
  limits: lenient(z.array(lenient(Limit))),
});
