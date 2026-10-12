import { Plugin } from "@opencode/plugin/effect";
import { Tool } from "@opencode/schema/tool";
import { Effect } from "effect";
import { homedir } from "node:os";
import path from "node:path";
import { commitRejection } from "./commit.ts";
import { denial } from "./policy.ts";

const home = homedir();
const secrets = [
  ".ssh",
  ".gnupg",
  ".password-store",
  ".local/share/keyrings",
  ".local/.owm_api_key",
  ".local/share/dotfiles/porkbun.env",
  ".local/share/dotfiles/vpn",
  ".mozilla/firefox",
  ".claude/.credentials.json",
  ".claude-cogikyo/.credentials.json",
  ".local/share/opencode/auth.json",
  ".local/share/opencode-v2/opencode/auth.json",
].map((secret) => path.join(home, secret));

function text(input: unknown, key: string) {
  if (typeof input !== "object" || input === null) return undefined;
  const value = (input as Record<string, unknown>)[key];
  return typeof value === "string" && value !== "" ? value : undefined;
}

function expand(target: string) {
  if (target === "~") return home;
  return target.startsWith("~/") ? path.join(home, target.slice(2)) : target;
}

function broad(root: string) {
  const prefix = root.endsWith("/") ? root : `${root}/`;
  return root === home || secrets.some((secret) => secret === root || secret.startsWith(prefix));
}

export default Plugin.define({
  id: "guard",
  effect: (ctx) =>
    Effect.gen(function* () {
      const directory = (sessionID: Parameters<typeof ctx.session.get>[0]["sessionID"]) =>
        ctx.session.get({ sessionID }).pipe(
          Effect.map((session) => session.location.directory as string),
          Effect.orDie,
        );

      const resolve = (sessionID: Parameters<typeof directory>[0], target: string | undefined) => {
        const expanded = expand(target ?? ".");
        if (path.isAbsolute(expanded)) return Effect.succeed(path.normalize(expanded));
        return directory(sessionID).pipe(Effect.map((base) => path.resolve(base, expanded)));
      };

      yield* ctx.permission.hook("evaluate", (event) =>
        Effect.sync(() => {
          if (event.action !== "shell") return;
          for (const resource of event.resources) {
            const message = denial(resource, event.agent);
            if (!message) continue;
            event.effect = "deny";
            event.message = message;
            return;
          }
        }),
      );

      yield* ctx.tool.hook("execute.before", (event) =>
        Effect.gen(function* () {
          if (event.tool === "grep" || event.tool === "glob") {
            const root = yield* resolve(event.sessionID, text(event.input, "path"));
            if (!broad(root)) return;
            return yield* new Tool.Error({
              message: `${event.tool} from ${root} would search the home directory or a secret store; use a narrower path`,
            });
          }
          if (event.tool !== "shell") return;
          const command = text(event.input, "command");
          if (!command?.includes("commit")) return;
          const cwd = yield* resolve(event.sessionID, text(event.input, "workdir"));
          const rejection = yield* Effect.promise(() => commitRejection(command, cwd));
          if (rejection) return yield* new Tool.Error({ message: rejection });
        }),
      );
    }),
});
