import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import { tool } from "@opencode-ai/plugin";
import { renderUsageStatus } from "./status.ts";

const id = "usage-status";

const server: Plugin = async () => ({
  tool: {
    usage_status: tool({
      description: "Read the local usage-sidebar cache and report per-provider remaining headroom, freshness, and reset timing. Fast cache read; stale or unknown values are never presented as current capacity.",
      args: {},
      async execute(_args, context) {
        await context.ask({
          permission: "usage_status",
          patterns: ["*"],
          always: [],
          metadata: {},
        });
        return renderUsageStatus();
      },
    }),
  },
});

export default { id, server } satisfies PluginModule;
