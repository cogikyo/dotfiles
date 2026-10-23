import { Plugin } from "@opencode/plugin/effect";
import type { Permission } from "@opencode/schema/permission";
import { Tool } from "@opencode/schema/tool";
import { Effect } from "effect";
import { appendFile, mkdir } from "node:fs/promises";
import path from "node:path";
import { commitRejection } from "./commit.ts";
import { check, fetchPublic, type Request } from "./fetch.ts";
import { confine, expand, home, scratch, secrets } from "./paths.ts";
import { denial, readOnly } from "./policy.ts";
import { launches } from "./shell.ts";

const cage = path.join(home, ".local/bin/cage");
const runs = path.join(scratch, "runs.jsonl");
const navigators = new Set(["cd", "pushd", "popd"]);
const allowed = ["shell", "read", "grep", "glob", "webfetch", "websearch", "skill", "sessions", "x", "diagram"];
const files: Record<string, { key: string; recursive: boolean; required: boolean }> = {
  read: { key: "path", recursive: false, required: true },
  grep: { key: "path", recursive: true, required: false },
  glob: { key: "path", recursive: true, required: false },
  diagram: { key: "file", recursive: false, required: true },
};

type Assert = (
  input: Pick<Permission.Request, "sessionID" | "action" | "resources" | "save" | "metadata" | "source"> & {
    agent?: Tool.Context["agent"];
  },
) => Effect.Effect<void, Tool.Error>;

function text(input: unknown, key: string) {
  if (typeof input !== "object" || input === null) return undefined;
  const value = (input as Record<string, unknown>)[key];
  return typeof value === "string" && value !== "" ? value : undefined;
}

function quote(word: string) {
  return `'${word.replaceAll("'", `'\\''`)}'`;
}

function caged(command: string, dir: string) {
  const hides = secrets().flatMap((secret) => ["--hide", quote(secret)]);
  return [quote(cage), ...hides, quote(dir), "--", "bash", "-c", quote(command)].join(" ");
}

function record(run: Record<string, unknown>) {
  return Effect.tryPromise({
    try: async () => {
      await mkdir(scratch, { recursive: true, mode: 0o700 });
      await appendFile(runs, `${JSON.stringify(run)}\n`);
    },
    catch: (error) => new Tool.Error({ message: `guard could not append ${runs}: ${String(error)}` }),
  });
}

function broad(root: string) {
  const prefix = root.endsWith("/") ? root : `${root}/`;
  return root === home || secrets().some((secret) => secret === root || secret.startsWith(prefix));
}

function describe(event: { agent: string; tools: Record<string, { description: string }> }) {
  return Effect.sync(() => {
    const shell = event.tools.shell;
    if (!shell) return;
    shell.description = `${shell.description} Set the working directory with \`workdir\`; commands that run \`cd\`, \`pushd\`, or \`popd\` are rejected.`;
    if (!readOnly(event.agent)) {
      shell.description = `${shell.description} The shell is zsh: an unquoted \`$var\` expands to one word, so pass lists as arrays.`;
      return;
    }
    shell.description = `${shell.description} Commands run offline in a sandbox; write only under \`$OPENCODE_SCRATCH\`, where \`tmp/\` and \`cache/\` are mutable and files at the root cannot be deleted.`;
  });
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

      const assert = (ctx.permission as unknown as { assert?: Assert }).assert;

      const fetch = (input: Request, context: Tool.Context) =>
        Effect.gen(function* () {
          yield* Effect.try({
            try: () => check(input.url),
            catch: (error) => new Tool.Error({ message: `Unable to fetch ${input.url}: ${(error as Error).message}` }),
          });
          if (!assert) {
            return yield* new Tool.Error({ message: "guard cannot ask webfetch permission on this OpenCode host" });
          }
          yield* assert({
            sessionID: context.sessionID,
            agent: context.agent,
            action: "webfetch",
            resources: [input.url],
            save: ["*"],
            metadata: { ...input },
            source: { type: "tool", messageID: context.messageID, id: context.id },
          });
          const result = yield* Effect.tryPromise({
            try: (signal) => fetchPublic(input, signal),
            catch: (error) => new Tool.Error({ message: `Unable to fetch ${input.url}: ${(error as Error).message}` }),
          });
          return { output: result, content: result.output, metadata: { contentType: result.contentType } };
        });

      const confined = (event: { tool: string; sessionID: Parameters<typeof directory>[0]; input: unknown }) =>
        Effect.gen(function* () {
          const file = files[event.tool];
          const input = event.input as Record<string, unknown>;
          if (typeof input !== "object" || input === null) return;
          const raw = input[file.key];
          const sentinel = event.tool === "glob" && (raw === "undefined" || raw === "null");
          const target = typeof raw === "string" && !sentinel ? raw : undefined;
          if (target === undefined && file.required) return;
          const base = yield* directory(event.sessionID);
          input[file.key] = yield* Effect.tryPromise({
            try: () => confine(target ?? ".", base, file.recursive),
            catch: (error) =>
              new Tool.Error({
                message: `${event.tool} cannot use ${target ?? base}: ${(error as Error).message}; read-only agents can read it only through the sandboxed shell, if at all`,
              }),
          });
        });

      yield* ctx.tool.transform((editor) => {
        editor.update("webfetch", (tool) => {
          const execute = tool.execute;
          tool.execute = (input, context) =>
            readOnly(context.agent)
              ? fetch({ ...(input as Request), format: (input as Partial<Request>).format ?? "markdown" }, context)
              : execute(input, context);
        });
      });

      yield* ctx.session.hook("context", describe);
      yield* ctx.session.hook("compaction", describe);
      yield* ctx.session.hook("generate", describe);

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
          const restricted = readOnly(event.agent);
          if (restricted && !allowed.includes(event.tool)) {
            return yield* new Tool.Error({
              message: `read-only agents cannot call \`${event.tool}\`; they may call only ${allowed.map((name) => `\`${name}\``).join(", ")}`,
            });
          }
          if (restricted && event.tool in files) return yield* confined(event);
          if (restricted && event.tool === "x") {
            const session = yield* ctx.session.get({ sessionID: event.sessionID }).pipe(Effect.orDie);
            return yield* record({
              time: new Date().toISOString(),
              phase: "admitted",
              tool: "x",
              callID: event.id,
              messageID: event.messageID,
              sessionID: event.sessionID,
              parentID: session.parentID,
              agent: event.agent,
              requested: text(event.input, "brief"),
            });
          }
          if (event.tool === "grep" || event.tool === "glob") {
            const root = yield* resolve(event.sessionID, text(event.input, "path"));
            if (!broad(root)) return;
            return yield* new Tool.Error({
              message: `${event.tool} from ${root} would search the home directory or a secret store; use a narrower path`,
            });
          }
          if (event.tool !== "shell") return;
          const command = text(event.input, "command");
          if (!command) return;
          // The host asks external_directory before evaluating shell permissions, so reject navigation here.
          if (launches(command).some(([name]) => navigators.has(name))) {
            return yield* new Tool.Error({
              message:
                "`cd`, `pushd`, and `popd` are disabled; set the shell tool's `workdir` parameter instead, and use absolute paths, `git -C`, or a separate call for other directories",
            });
          }
          if (!restricted && command.includes("commit")) {
            const cwd = yield* resolve(event.sessionID, text(event.input, "workdir"));
            const rejection = yield* Effect.promise(() => commitRejection(command, cwd));
            if (rejection) return yield* new Tool.Error({ message: rejection });
          }
          if (!restricted) return;
          const cwd = yield* resolve(event.sessionID, text(event.input, "workdir"));
          const dir = path.join(scratch, event.sessionID);
          (event.input as Record<string, unknown>).command = caged(command, dir);
          const session = yield* ctx.session.get({ sessionID: event.sessionID }).pipe(Effect.orDie);
          yield* record({
            time: new Date().toISOString(),
            phase: "caged",
            tool: "shell",
            callID: event.id,
            messageID: event.messageID,
            sessionID: event.sessionID,
            parentID: session.parentID,
            agent: event.agent,
            workdir: cwd,
            scratch: dir,
            requested: command,
          });
        }),
      );

      yield* ctx.tool.hook("execute.after", (event) => {
        if ((event.tool !== "shell" && event.tool !== "x") || !readOnly(event.agent)) return Effect.void;
        return record({
          time: new Date().toISOString(),
          phase: "returned",
          tool: event.tool,
          callID: event.id,
          sessionID: event.sessionID,
          status: event.status,
        }).pipe(Effect.ignore);
      });
    }),
});
