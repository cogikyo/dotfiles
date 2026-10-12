import { Plugin } from "@opencode/plugin";
import { anthropicPersonalUsage, anthropicUsage } from "./anthropic.ts";
import { cursorUsage } from "./cursor.ts";
import { opencodeGoUsage } from "./opencode-go.ts";
import { openaiUsage } from "./openai.ts";
import { refresh } from "./poll.ts";
import { Definition, type Snapshot } from "./rpc.ts";
import { renderUsageStatus } from "./status.ts";
import type { Credentials, ProviderAdapter } from "./types.ts";
import { xaiUsage } from "./xai.ts";

const POLL_MS = 60_000;

export default Plugin.define({
  id: "usage",
  async setup(ctx) {
    const credentials: Credentials = async (integrationID) => {
      const connection = await ctx.integration.connection.active(integrationID);
      if (!connection) return undefined;
      return ctx.integration.connection.resolve(connection);
    };
    const adapters: ProviderAdapter[] = [
      openaiUsage(credentials),
      anthropicUsage,
      anthropicPersonalUsage,
      xaiUsage,
      cursorUsage,
      opencodeGoUsage(credentials),
    ];

    let latest: Snapshot | undefined;
    let running: Promise<Snapshot> | undefined;
    const poll = (providerID?: string) => {
      if (running && !providerID) return running;
      const task = Promise.all(
        adapters.map((adapter) =>
          refresh(adapter, adapter.id === providerID).catch(() => ({
            id: adapter.id,
            label: adapter.label,
            placeholders: adapter.placeholders,
            windows: [],
            note: "unavailable",
            noteKind: "error" as const,
          })),
        ),
      ).then(async (providers) => {
        latest = { providers };
        await rpc.events.emit("updated", latest).catch(() => undefined);
        return latest;
      });
      running = task.finally(() => {
        if (running === task) running = undefined;
      });
      return task;
    };

    const rpc = await ctx.rpc.register(Definition, {
      list: async () => latest ?? poll(),
      refresh: async ({ providerID }) => poll(providerID),
    });

    await ctx.tool.transform((editor) =>
      editor.add({
        name: "usage_status",
        description:
          "Read the local usage-sidebar cache and report per-provider remaining headroom, freshness, and reset timing. Fast cache read; stale or unknown values are never presented as current capacity.",
        input: { type: "object", properties: {}, additionalProperties: false },
        execute: async () => ({ content: await renderUsageStatus() }),
      }),
    );

    void poll();
    const timer = setInterval(() => void poll(), POLL_MS);
    return () => clearInterval(timer);
  },
});
