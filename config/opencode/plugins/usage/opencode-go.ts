import { z } from "zod";
import { readAuth } from "./auth.ts";
import { usageProviders } from "./providers.ts";
import type { ProviderAdapter, ProviderUsage } from "./types.ts";

const { id, label, staleAfterMS } = usageProviders.opencodeGo;

/** Loads OpenCode Go rolling, weekly, and monthly limits; invalid usage returns an error note. */
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

const Window = z.object({
  percent: z.number().min(0),
  resetsAt: z.string().refine((value) => Number.isFinite(Date.parse(value))),
});

const Body = z.object({
  usage: z.object({ rolling: Window, weekly: Window, monthly: Window }),
});

async function load(): Promise<ProviderUsage> {
  const key = (await readAuth())?.["opencode-go"]?.key;
  if (!key) return note("no auth", "warn");

  let response: Response;
  try {
    response = await fetch("https://opencode.ai/zen/go/v1/usage", {
      headers: {
        Authorization: `Bearer ${key}`,
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

  const body = Body.safeParse(await response.json().catch(() => undefined));
  if (!body.success) return note("invalid usage");

  const { rolling, weekly, monthly } = body.data.usage;
  const windows = [
    { tag: "H", window: rolling },
    { tag: "W", window: weekly },
    { tag: "M", window: monthly },
  ].map(({ tag, window }) => ({ label: tag, usedPercent: Math.min(100, window.percent), resetAt: window.resetsAt }));

  return { id, label, windows };
}

function note(text: string, noteKind: ProviderUsage["noteKind"] = "error"): ProviderUsage {
  return { id, label, windows: [], note: text, noteKind };
}
