import { Plugin } from "@opencode/plugin/effect";
import type { Permission } from "@opencode/schema/permission";
import { Tool } from "@opencode/schema/tool";
import { Effect } from "effect";
import { statSync } from "node:fs";
import { appendFile, mkdir } from "node:fs/promises";
import path from "node:path";
import { commitRejection } from "./commit.ts";
import { relocated } from "./dirs.ts";
import { check, fetchPublic, type Request } from "./fetch.ts";
import { confine, expand, home, scratch, secrets } from "./paths.ts";
import { inspect, readOnly } from "./policy.ts";
import { enter } from "./shell.ts";

const cage = path.join(home, ".local/bin/cage");
const ttl = 24 * 60 * 60 * 1000;
const pushes = new Map<string, { reason: string; time: number }>();
const runs = path.join(scratch, "runs.jsonl");
const allowed = [
  "shell",
  "read",
  "grep",
  "glob",
  "webfetch",
  "websearch",
  "skill",
  "sessions",
  "x",
  "diagram",
  "workflow",
  "subagent",
];
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

function prune(now: number) {
  for (const [key, held] of pushes) if (now - held.time > ttl) pushes.delete(key);
}

function broad(root: string) {
  const prefix = root.endsWith("/") ? root : `${root}/`;
  return root === home || secrets().some((secret) => secret === root || secret.startsWith(prefix));
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
            catch: (error) => {
              const code = (error as NodeJS.ErrnoException).code;
              const missing = code === "ENOENT" || code === "ENOTDIR";
              return new Tool.Error({
                message: missing
                  ? `${event.tool} cannot use ${target ?? base}: no such file or directory`
                  : `${event.tool} cannot use ${target ?? base}: ${(error as Error).message}; read-only agents can read it only through the sandboxed shell, if at all`,
              });
            },
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

      yield* ctx.tool.hook("execute.before", (event) =>
        Effect.gen(function* () {
          const stray = relocated(event.tool, event.input);
          if (stray) return yield* new Tool.Error({ message: stray });
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
          const requested = text(event.input, "command");
          if (!requested) return;
          const base = yield* resolve(event.sessionID, text(event.input, "workdir"));
          const { source: command, dir: cwd } = enter(requested, base);
          if (command !== requested) {
            if (!statSync(cwd, { throwIfNoEntry: false })?.isDirectory()) {
              return yield* new Tool.Error({ message: `cd: ${cwd} is not a directory` });
            }
            Object.assign(event.input as Record<string, unknown>, { command, workdir: cwd });
          }
          const known: Record<string, string> = restricted
            ? { OPENCODE_SCRATCH: path.join(scratch, event.sessionID) }
            : {};
          const review = inspect(command, event.agent, cwd, known);
          const { problems } = review;
          if (!restricted && review.commits.includes(undefined)) {
            problems.push("the guard can't tell which repo this commit runs in; set `workdir` to the repo");
          } else if (!restricted) {
            for (const dir of new Set(review.commits)) {
              const rejection = yield* Effect.promise(() => commitRejection(command, dir ?? cwd));
              if (!rejection) continue;
              problems.push(rejection);
              break;
            }
          }
          if (problems.length === 1) return yield* new Tool.Error({ message: problems[0] });
          if (problems.length > 1) {
            const list = problems.map((problem) => `- ${problem.replaceAll("\n", "\n  ")}`).join("\n");
            return yield* new Tool.Error({
              message: `the guard rejected this command for ${problems.length} reasons:\n${list}`,
            });
          }
          if (!restricted) {
            const now = Date.now();
            prune(now);
            if (review.push) {
              pushes.set(`${event.sessionID} ${event.messageID} ${event.id}`, { reason: review.push, time: now });
            }
            return;
          }
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

      yield* ctx.permission.hook("evaluate", (event) =>
        Effect.sync(() => {
          prune(Date.now());
          // Keep the record: an "always" reply re-evaluates pending requests.
          const pending = event.source && pushes.get(`${event.sessionID} ${event.source.messageID} ${event.source.id}`);
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
