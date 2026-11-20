import { Plugin } from "@opencode/plugin/effect";
import type { Permission } from "@opencode/schema/permission";
import { Tool } from "@opencode/schema/tool";
import { Effect } from "effect";
import { appendFile, mkdir } from "node:fs/promises";
import path from "node:path";
import { check, fetchPublic, type Request } from "./fetch.ts";
import { scratch } from "./paths.ts";
import { readOnly } from "./policy.ts";
import { review, text } from "./review.ts";

const ttl = 24 * 60 * 60 * 1000;
const pushes = new Map<string, { reason: string; time: number }>();
const fetches = new Set<string>();
const runs = path.join(scratch, "runs.jsonl");

type Assert = (
  input: Pick<Permission.Request, "sessionID" | "action" | "resources" | "save" | "metadata" | "source"> & {
    agent?: Tool.Context["agent"];
  },
) => Effect.Effect<void, Tool.Error>;

function record(run: Record<string, unknown>) {
  return Effect.tryPromise({
    try: async () => {
      await mkdir(scratch, { recursive: true, mode: 0o700 });
      await appendFile(runs, `${JSON.stringify(run)}\n`);
    },
    catch: (error) => new Tool.Error({ message: `guard could not append ${runs}: ${String(error)}` }),
  });
}

function prune(now: number) {
  for (const [key, held] of pushes) if (now - held.time > ttl) pushes.delete(key);
}

function describe(event: { agent: string; tools: Record<string, { description: string }> }) {
  return Effect.sync(() => {
    const { shell, webfetch } = event.tools;
    const restricted = readOnly(event.agent);
    if (webfetch && restricted) {
      webfetch.description = `${webfetch.description} To fetch several URLs, batch them as parallel webfetch calls in one message.`;
    }
    if (!shell) return;
    shell.description = `${shell.description} Calls in one message run in parallel, so read a command's output file in a later message.`;
    if (!restricted) {
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
          const key = `${context.sessionID} ${context.messageID} ${context.id}`;
          fetches.add(key);
          yield* assert({
            sessionID: context.sessionID,
            agent: context.agent,
            action: "webfetch",
            resources: [input.url],
            save: ["*"],
            metadata: { ...input },
            source: { type: "tool", messageID: context.messageID, id: context.id },
          }).pipe(Effect.ensuring(Effect.sync(() => fetches.delete(key))));
          const result = yield* Effect.tryPromise({
            try: (signal) => fetchPublic(input, signal),
            catch: (error) => new Tool.Error({ message: `Unable to fetch ${input.url}: ${(error as Error).message}` }),
          });
          return { output: result, content: result.output, metadata: { contentType: result.contentType } };
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

      yield* ctx.tool.hook("execute.before", (event) =>
        Effect.gen(function* () {
          const base = yield* directory(event.sessionID);
          const result = yield* Effect.promise(() =>
            review({
              tool: event.tool,
              input: event.input,
              agent: event.agent,
              directory: base,
              session: event.sessionID,
            }),
          );
          if ("problem" in result) return yield* new Tool.Error({ message: result.problem });
          if (result.input) Object.assign(event.input as Record<string, unknown>, result.input);
          const restricted = readOnly(event.agent);
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
          if (event.tool !== "shell" || !text(event.input, "command")) return;
          if (!restricted) {
            const now = Date.now();
            prune(now);
            if (result.push) {
              pushes.set(`${event.sessionID} ${event.messageID} ${event.id}`, { reason: result.push, time: now });
            }
            return;
          }
          if (!result.cage) return;
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
            workdir: result.cage.workdir,
            scratch: result.cage.scratch,
            requested: result.cage.command,
          });
        }),
      );

      yield* ctx.permission.hook("evaluate", (event) =>
        Effect.sync(() => {
          prune(Date.now());
          const key = event.source && `${event.sessionID} ${event.source.messageID} ${event.source.id}`;
          if (event.action === "webfetch" && readOnly(event.agent) && !(key && fetches.has(key))) {
            event.effect = "deny";
            event.message = "read-only agents fetch only through the `webfetch` tool, not from Code Mode scripts";
            return;
          }
          // Keep the record: an "always" reply re-evaluates pending requests.
          const pending = key && pushes.get(key);
          if (event.action !== "shell" || !pending || event.effect !== "allow") return;
          event.effect = "ask";
          event.message = pending.reason;
        }),
      );

      yield* ctx.tool.hook("execute.after", (event) => {
        if (event.tool === "shell") pushes.delete(`${event.sessionID} ${event.messageID} ${event.id}`);
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
