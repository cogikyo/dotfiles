import { binary, environment } from "../gemini/agy.ts";
import { usageProviders } from "./providers.ts";
import { num, record, str, type Json } from "./types.ts";
import type { ProviderAdapter, ProviderUsage, UsageWindow } from "./types.ts";

const { id, label, staleAfterMS } = usageProviders.gemini;
const buckets = [
  ["gemini-5h", "H"],
  ["gemini-weekly", "W"],
] as const;

export const geminiUsage: ProviderAdapter = {
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
  let proc: Bun.Subprocess<"ignore", "pipe", "ignore">;
  try {
    proc = Bun.spawn([binary, "--print", "/usage", "--output-format", "json"], {
      env: environment(),
      stdin: "ignore",
      stdout: "pipe",
      stderr: "ignore",
      timeout: 30_000,
    });
  } catch {
    return note(`no ${binary}`);
  }
  const [output, code] = await Promise.all([new Response(proc.stdout).text(), proc.exited]);
  if (code !== 0) return note(`exit ${code}`);

  const reply = record(parse(output));
  if (reply?.status !== "SUCCESS") return note(str(reply?.status)?.toLowerCase() ?? "unreadable");
  const groups = record(record(reply.command)?.data)?.groups;
  const all = (Array.isArray(groups) ? groups : []).flatMap((group) => record(group)?.buckets).map(record);
  const byID = new Map(all.map((item) => [item?.id, item]));
  const windows = buckets.map(([bucket, tag]) => window(tag, byID.get(bucket))).filter((item) => item !== undefined);
  return windows.length ? { id, label, windows } : note("no windows");
}

function parse(output: string): unknown {
  try {
    return JSON.parse(output);
  } catch {
    return undefined;
  }
}

function window(tag: string, bucket: Json | undefined): UsageWindow | undefined {
  const remaining = num(bucket?.remaining_fraction);
  if (remaining === undefined) return undefined;
  const used = Math.max(0, Math.min(100, (1 - remaining) * 100));
  return { label: tag, usedPercent: used > 0 && used < 1 ? 1 : used, resetAt: time(bucket?.reset_time) };
}

function time(value: unknown) {
  if (typeof value !== "string") return undefined;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString();
}

function note(text: string): ProviderUsage {
  return { id, label, windows: [], note: text, noteKind: "error" };
}
