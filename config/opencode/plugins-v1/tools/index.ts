import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import { diagram } from "./diagram/index.ts";
import { sessions } from "./sessions/index.ts";
import { forget, x } from "./x.ts";

const server: Plugin = async () => ({
  tool: { x, diagram, sessions },
  event: async ({ event }) => {
    if (event.type === "session.deleted") forget(event.properties.info.id);
  },
  dispose: async () => forget(),
});

/** Exposes the permission-gated `x`, `diagram`, and `sessions` tools. */
export default { id: "tools", server } satisfies PluginModule;
