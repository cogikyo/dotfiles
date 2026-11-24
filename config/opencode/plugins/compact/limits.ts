import { isLocationNotFoundError } from "@opencode/client/promise";
import type { Plugin } from "@opencode/plugin/tui";

type Limit = { input?: number; context: number };

const reserve = 16_000;
const share = 0.15;
const buffers = new Map<string, Promise<number | string | undefined>>();

const levels = [
  { tokens: 300_000, kind: "soft" },
  { tokens: 350_000, kind: "soft" },
  { tokens: 400_000, kind: "strong" },
] as const;

export const floor = levels[0].tokens;
export const finish = levels[levels.length - 1].tokens;

export type Level = (typeof levels)[number];

export function due(total: number, sent?: { level?: number; id?: string }) {
  const last = sent && (sent.level ?? floor);
  const retry = levels.find((level) => level.tokens === last);
  if (sent?.id !== undefined && retry) return { level: retry, id: sent.id };
  const next = levels.findLast((level) => level.tokens <= total && level.tokens > (last ?? 0));
  return next && { level: next };
}

export function threshold(limit: Limit, buffer: number | string | undefined) {
  const window = limit.input && limit.context ? Math.min(limit.input, limit.context) : limit.input || limit.context;
  const tokens = typeof buffer === "string" ? Math.round((window * Number.parseFloat(buffer)) / 100) : buffer;
  const room =
    tokens !== undefined && tokens < window
      ? tokens
      : Math.max(Math.floor(window * share), window >= 2 * reserve ? reserve : 0);
  const value = window > 0 ? window - room : Number.POSITIVE_INFINITY;
  return Number.isFinite(value) && value > 0 ? value : undefined;
}

export function configured(context: Plugin.Context, directory: string) {
  const cached = buffers.get(directory);
  if (cached) return cached;
  const pending = context.client.config.get({ location: { directory } }).then(
    (entries) =>
      entries.reduce<number | string | undefined>(
        (value, entry) => (entry.type === "document" ? (entry.info.compaction?.buffer ?? value) : value),
        undefined,
      ),
    (error: unknown) => {
      if (isLocationNotFoundError(error)) return undefined;
      buffers.delete(directory);
      context.ui.toast.show({
        message: `Read compaction config: ${error instanceof Error ? error.message : String(error)}`,
        variant: "error",
      });
      throw error;
    },
  );
  buffers.set(directory, pending);
  return pending;
}
