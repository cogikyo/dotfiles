import { usageProviders } from "./providers.ts";
import { num, record, str } from "./types.ts";
import type { Credentials, ProviderAdapter, ProviderUsage } from "./types.ts";

const { id, label, staleAfterMS } = usageProviders.opencodeGo;

export function opencodeGoUsage(credentials: Credentials): ProviderAdapter {
  return {
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
    load: () => load(credentials),
  };
}

function window(value: unknown) {
  const item = record(value);
  const percent = num(item?.percent);
  const resetsAt = str(item?.resetsAt);
  if (percent === undefined || percent < 0 || !resetsAt || !Number.isFinite(Date.parse(resetsAt))) return undefined;
  return { percent, resetsAt };
}

async function authHeaders(credentials: Credentials): Promise<Record<string, string> | undefined> {
  // Console sign-in authenticates Go; Go API keys are the older service-account path.
  const signIn = await credentials("opencode");
  if (signIn?.type === "oauth") {
    const orgID = str(signIn.metadata?.orgID);
    return { Authorization: `Bearer ${signIn.access}`, ...(orgID ? { "x-org-id": orgID } : {}) };
  }
  const goKey = await credentials(id);
  if (goKey?.type === "key" && goKey.key) return { Authorization: `Bearer ${goKey.key}` };
  return undefined;
}

async function load(credentials: Credentials): Promise<ProviderUsage> {
  const auth = await authHeaders(credentials);
  if (!auth) return note("no auth", "warn");

  let response: Response;
  try {
    response = await fetch("https://opencode.ai/zen/go/v1/usage", {
      headers: {
        ...auth,
        Accept: "application/json",
      },
      redirect: "error",
      signal: AbortSignal.timeout(15_000),
    });
  } catch {
    return note("request failed");
  }

  if (response.status === 401) return note("invalid auth");
  if (response.status === 429) return note("429");
  if (!response.ok) return note(`HTTP ${response.status}`);

  const body = record(record(await response.json().catch(() => undefined))?.usage);
  const tagged = [
    { tag: "H", window: window(body?.rolling) },
    { tag: "W", window: window(body?.weekly) },
    { tag: "M", window: window(body?.monthly) },
  ];
  const windows = tagged.flatMap(({ tag, window: item }) =>
    item ? [{ label: tag, usedPercent: Math.min(100, item.percent), resetAt: item.resetsAt }] : [],
  );
  if (windows.length !== tagged.length) return note("invalid usage");

  return { id, label, windows };
}

function note(text: string, noteKind: ProviderUsage["noteKind"] = "error"): ProviderUsage {
  return { id, label, windows: [], note: text, noteKind };
}
