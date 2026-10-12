import fs from "node:fs/promises";
import path from "node:path";
import { claudeAccounts } from "../anthropic/accounts.ts";
import { usageProviders, type UsageProviderSpec } from "./providers.ts";
import { normalizePercent, num, record, str, text, type Json } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

const FETCH_TIMEOUT_MS = 15_000;
const HOURLY = "H";
const WEEKLY = "W";

export const anthropicUsage = createUsage(usageProviders.anthropic, claudeAccounts.trend.configDir);
export const anthropicPersonalUsage = createUsage(usageProviders.anthropicPersonal, claudeAccounts.cogikyo.configDir);

function createUsage({ id, label, staleAfterMS }: UsageProviderSpec, configDir: string): ProviderAdapter {
  return {
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
    const credentials = await readCredentials(configDir);
    if (!credentials?.accessToken) return usage([], "no auth");
    if (isExpired(credentials.expiresAt)) return usage([], "expired");
    return fetchUsage(credentials.accessToken);
  }

  function usage(windows: UsageWindow[], note?: string): ProviderUsage {
    return { id, label, windows, note };
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

    const payload = record(await response.json()) ?? {};
    const limits = Array.isArray(payload.limits) ? payload.limits : [];
    const windows = [
      usageWindow(HOURLY, record(payload.five_hour)),
      usageWindow(WEEKLY, record(payload.seven_day)),
      ...limits.map((limit) => scopedWindow(record(limit))),
    ].filter((window) => window !== undefined);

    if (windows.length === 0) return usage([], "no windows");
    return usage(windows);
  }
}

async function readCredentials(configDir: string) {
  const file = path.join(configDir, ".credentials.json");
  const root = await fs
    .readFile(file, "utf8")
    .then((content) => record(JSON.parse(content)))
    .catch(() => undefined);
  if (!root) return undefined;
  const oauth = record(root.claudeAiOauth) ?? root;
  const expiresAt = oauth.expiresAt;
  return {
    accessToken: str(oauth.accessToken),
    expiresAt: typeof expiresAt === "string" || typeof expiresAt === "number" ? expiresAt : undefined,
  };
}

function isExpired(expiresAt: string | number | undefined) {
  const ms = typeof expiresAt === "number" ? expiresAt : Date.parse(expiresAt ?? "");
  return !Number.isFinite(ms) || Date.now() >= ms;
}

function usageWindow(tag: string, window: Json | undefined): UsageWindow | undefined {
  const usedPercent = normalizePercent(num(window?.utilization));
  if (usedPercent === undefined) return undefined;
  return { label: tag, usedPercent, resetAt: str(window?.resets_at) };
}

function scopedLabel(displayName: string) {
  const word = displayName
    .trim()
    .split(/\s+/)
    .find((part) => part.toLowerCase() !== "claude");
  const first = Array.from(word ?? "")[0];
  return first?.toUpperCase();
}

function scopedWindow(limit: Json | undefined): UsageWindow | undefined {
  if (limit?.kind !== "weekly_scoped" || limit.group !== "weekly") return undefined;

  const displayName = text(record(record(limit.scope)?.model)?.display_name);
  if (displayName === undefined) return undefined;

  const tag = scopedLabel(displayName);
  if (!tag) return undefined;

  const usedPercent = normalizePercent(num(limit.percent));
  if (usedPercent === undefined) return undefined;

  return { label: tag, usedPercent, resetAt: str(limit.resets_at) };
}
