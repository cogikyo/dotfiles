import { isLocationNotFoundError } from "@opencode/client/promise";
import type { Plugin } from "@opencode/plugin/tui";

type Limit = { input?: number; context: number };

const reserve = 16_000;
const share = 0.15;
const buffers = new Map<string, Promise<number | string | undefined>>();

// Keep below the native trigger in threshold() when changing opencode.json limits or buffer.
export const ceiling = 200_000;

export function threshold(limit: Limit, buffer: number | string | undefined, root: boolean) {
  const window = limit.input && limit.context ? Math.min(limit.input, limit.context) : limit.input || limit.context;
  const tokens = typeof buffer === "string" ? Math.round((window * Number.parseFloat(buffer)) / 100) : buffer;
  const room =
    tokens !== undefined && tokens < window
      ? tokens
      : Math.max(Math.floor(window * share), window >= 2 * reserve ? reserve : 0);
  const native = window > 0 ? window - room : Number.POSITIVE_INFINITY;
  const value = root ? Math.min(native, ceiling) : native;
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
