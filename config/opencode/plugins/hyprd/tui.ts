import { Plugin } from "@opencode/plugin/tui";
import context from "./context.ts";
import notify from "./notify.ts";

export default Plugin.define({
  id: "hyprd",
  async setup(input) {
    const cleanups = await Promise.all([notify, context].map(async (plugin) => plugin.setup(input)));
    return async () => {
      await Promise.all(cleanups.map(async (cleanup) => cleanup?.()));
    };
  },
});
