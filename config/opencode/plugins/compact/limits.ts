import type { Plugin } from "@opencode/plugin/tui";

type Limit = { input?: number; context: number };

const reserve = 16_000;
const buffers = new Map<string, Promise<number | undefined>>();

// Keep below opencode.json's limit.input - compaction.buffer so native compaction does not run first.
export const ceiling = 300_000;

export function threshold(limit: Limit, buffer: number | undefined, root: boolean) {
  const window = limit.input || limit.context;
  const native =
    window > 0
      ? window - (buffer ?? Math.max(Math.floor(window * 0.1), window >= 2 * reserve ? reserve : 0))
      : Number.POSITIVE_INFINITY;
  const value = root ? Math.min(native, ceiling) : native;
  return Number.isFinite(value) && value > 0 ? value : undefined;
}

export function configured(context: Plugin.Context, directory: string) {
  const cached = buffers.get(directory);
  if (cached) return cached;
  const pending = context.client.config.get({ location: { directory } }).then(
    (entries) =>
      entries.reduce<number | undefined>(
        (value, entry) => (entry.type === "document" ? (entry.info.compaction?.buffer ?? value) : value),
        undefined,
      ),
    (error: unknown) => {
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
