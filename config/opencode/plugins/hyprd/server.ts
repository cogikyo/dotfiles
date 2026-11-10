import { Plugin } from "@opencode/plugin";
import { api } from "./rpc.ts";

export default Plugin.define({
  id: "hyprd",
  async setup(ctx) {
    const rpc = await ctx.rpc.register(api, {
      focus: async (input) => {
        await rpc.events.emit("focus", input);
        return input;
      },
    });
  },
});
