import { Plugin, Provider } from "@opencode/plugin";
import { binary } from "./agy.ts";
import { Gemini } from "./model.ts";

const id = Provider.ID.make("gemini");

export default Plugin.define({
  id,
  async setup(ctx) {
    const seen = new Map<string, number>();
    const locate = async (owner: string | undefined) => {
      if (owner === undefined) return ctx.location.directory;
      const session = await ctx.session.get({ sessionID: owner }).catch(() => undefined);
      return session?.location.directory ?? ctx.location.directory;
    };
    await ctx.provider.transform((editor) =>
      editor.add({
        info: { id, name: "Gemini", activation: "auto", package: "aisdk:@cogikyo/gemini" },
        models: [],
      }),
    );
    await ctx.aisdk.hook(
      "sdk",
      (event) => {
        const command = typeof event.options.command === "string" ? event.options.command : binary;
        event.sdk = { languageModel: (model: string) => new Gemini(model, { command, locate, seen }) };
      },
      { providerID: id },
    );
  },
});
