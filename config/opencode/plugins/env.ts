import { Plugin } from "@opencode/plugin";

const names = ["XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"];

export default Plugin.define({
  id: "env",
  async setup(ctx) {
    await ctx.shell.hook("create.before", (shell) => {
      for (const name of names) {
        const host = shell.env[`HOST_${name}`];
        if (host === undefined) continue;
        delete shell.env[`HOST_${name}`];
        if (host) shell.env[name] = host;
        else delete shell.env[name];
      }
    });
  },
});
