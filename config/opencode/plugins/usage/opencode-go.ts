import { readAuth } from "./auth.ts";
import { usageProviders } from "./providers.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

const { id, label, staleAfterMS } = usageProviders.opencodeGo;

type AuthFile = {
  "opencode-go"?: { type?: string; key?: string };
};

function note(note: string, noteKind: ProviderUsage["noteKind"] = "error"): ProviderUsage {
  return { id, label, windows: [], note, noteKind };
}

async function load(): Promise<ProviderUsage> {
  const auth = await readAuth<AuthFile>();
  const credential = auth["opencode-go"];
  if (credential?.type !== "api" || !credential.key) return note("no auth", "warn");

  let response: Response;
  try {
    response = await fetch("https://opencode.ai/zen/go/v1/usage", {
      headers: {
        Authorization: `Bearer ${credential.key}`,
        Accept: "application/json",
      },
      redirect: "error",
      signal: AbortSignal.timeout(15_000),
    });
  } catch {
    return note("request failed");
  }

  if (response.status === 401) return note("invalid key");
  if (response.status === 429) return note("429");
  if (!response.ok) return note(`HTTP ${response.status}`);

  const body = (await response.json().catch(() => undefined)) as
    | { usage?: Record<string, { percent?: unknown; resetsAt?: unknown } | undefined> }
    | undefined;
  if (!body || typeof body !== "object" || !body.usage || typeof body.usage !== "object") {
    return note("invalid usage");
  }

  const windows: UsageWindow[] = [];
  for (const [name, label] of [
    ["rolling", "H"],
    ["weekly", "W"],
    ["monthly", "M"],
  ] as const) {
    const value = body.usage[name];
    if (
      !value ||
      typeof value.percent !== "number" ||
      !Number.isFinite(value.percent) ||
      value.percent < 0 ||
      typeof value.resetsAt !== "string" ||
      !Number.isFinite(Date.parse(value.resetsAt))
    ) {
      return note("invalid usage");
    }
    windows.push({
      label,
      usedPercent: Math.min(100, value.percent),
      resetAt: value.resetsAt,
    });
  }

  return { id, label, windows };
}

export const opencodeGoUsage: ProviderAdapter = {
  id,
  label,
  placeholders: ["H", "W", "M"],
  poll: {
    minFetchIntervalMS: 60_000,
    errorBackoffMS: 3 * 60_000,
    warnBackoffMS: 60_000,
    rateLimitBackoffMS: 10 * 60_000,
    staleAfterMS,
  },
  load,
};
