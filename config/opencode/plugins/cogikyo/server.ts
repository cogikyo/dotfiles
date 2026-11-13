import { Plugin } from "@opencode/plugin/effect";
import type { Tool } from "@opencode/schema/tool";
import { Effect, Schema } from "effect";
import path from "node:path";
import { expand, home, located, tilde, within } from "../guard/paths.ts";

const Pathed = Schema.Struct({ path: Schema.String });
const Patched = Schema.Struct({ applied: Schema.Array(Schema.Struct({ target: Schema.String })) });

export function notice(files: readonly string[]) {
  return `Public dotfiles: this write landed in ${files.map(tilde).join(", ")}; follow ~/dotfiles/AGENTS.md. Homelab, apartment, device, and app content belongs in ~/cogikyo, even on costello; cogikyo-only packages go in ~/dotfiles/packages/cogikyo.lst.`;
}

function written(tool: string, input: unknown, output: unknown) {
  if ((tool === "edit" || tool === "write") && Schema.is(Pathed)(input)) return [input.path];
  if (tool === "patch" && Schema.is(Patched)(output)) return output.applied.map(({ target }) => target);
  return [];
}

function append(content: Tool.Result["content"], text: string): Tool.Result["content"] {
  if (typeof content === "string") return `${content}\n\n${text}`;
  return [...(content ?? []), { type: "text", text }];
}

export default Plugin.define({
  id: "cogikyo",
  effect: (ctx) =>
    Effect.gen(function* () {
      const base = ctx.location.directory;
      const scoped = yield* Effect.try({
        try: () => within(located(base), located(path.join(home, "cogikyo"))),
        catch: (error) => error,
      }).pipe(Effect.orElseSucceed(() => false));
      if (!scoped) return;
      const seen = new Set<string>();

      yield* ctx.tool.hook("execute.after", (event) =>
        Effect.sync(() => {
          if (event.status !== "completed") return;
          const dotfiles = located(path.join(home, "dotfiles"));
          const files = written(event.tool, event.input, event.result.output)
            .map((file) => located(path.resolve(base, expand(file))))
            .filter((file) => within(file, dotfiles));
          const fresh = files.filter((file) => {
            const key = `${event.sessionID} ${path.relative(dotfiles, file).split(path.sep)[0]}`;
            if (seen.has(key)) return false;
            seen.add(key);
            return true;
          });
          if (fresh.length === 0) return;
          event.result = { ...event.result, content: append(event.result.content, notice(fresh)) };
        }).pipe(Effect.catchCause((cause) => Effect.logWarning("cogikyo: dotfiles notice skipped", cause))),
      );
    }),
});
