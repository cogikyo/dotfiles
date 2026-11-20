import { Plugin, Provider } from "@opencode/plugin";
import { binary } from "./agy.ts";
import { holds, type Owner } from "./gate.ts";
import { Gemini } from "./model.ts";
import { catalog } from "./tools.ts";

const id = Provider.ID.make("gemini");

export default Plugin.define({
  id,
  async setup(ctx) {
    const seen = new Map<string, number>();
    const locate = async (owner: string | undefined, agent: string | undefined): Promise<Owner> => {
      const session =
        owner === undefined ? undefined : await ctx.session.get({ sessionID: owner }).catch(() => undefined);
      return { session: session && owner, agent, directory: session?.location.directory ?? ctx.location.directory };
    };
    await ctx.session.hook(
      "model.request",
      (event) => {
        event.headers["x-opencode-agent"] = event.agent;
      },
      { providerID: id },
    );
    await ctx.permission.hook("evaluate", (event) => {
      const reason = event.source && holds.get(`${event.sessionID} ${event.source.messageID} ${event.source.id}`);
      if (event.action !== "shell" || !reason || event.effect !== "allow") return;
      event.effect = "ask";
      event.message = reason;
    });
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
        event.sdk = {
          languageModel: (model: string) =>
            new Gemini(model, {
              command,
              permission: ctx.permission,
              locate,
              catalog: (owner) => catalog(ctx, owner),
              seen,
            }),
        };
      },
      { providerID: id },
    );
  },
});
