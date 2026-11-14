import { Plugin } from "@opencode/plugin/effect";
import { Tool } from "@opencode/schema/tool";
import { Effect, Schema } from "effect";
import { statSync } from "node:fs";
import { mkdir, open, readFile, rm } from "node:fs/promises";
import path from "node:path";
import { stripVTControlCharacters } from "node:util";
import { expand, temp, tilde } from "../guard/paths.ts";
import { flags, launches } from "../guard/shell.ts";

const root = path.join(temp, "terminal");
const runner = path.join(import.meta.dirname, "run.sh");
const limit = 6000;
const chunk = 64 * 1024;
const failures = ["sudo: a terminal is required", "sudo: a password is required"];
const redirect =
  "`sudo` without `-n` prompts for a password the shell cannot supply; run the command with the `terminal` tool so the user can type it.";
const hint =
  "sudo needed a password the shell cannot supply; run the command with the `terminal` tool so the user can type it, using `ssh -t` inside `terminal` for remote sudo.";
const controls = [
  [
    "command",
    /(?![\n\t])\p{Cc}/u,
    "only newline and tab are allowed; write escapes in shell syntax, such as $'\\e' or printf '\\033', instead of raw characters",
  ],
  ["reason", /\p{Cc}/u, "none are allowed"],
  ["workdir", /\p{Cc}/u, "none are allowed"],
] as const;

const Input = Schema.Struct({
  command: Schema.String.check(Schema.isMinLength(1)).annotate({ description: "Command to run in the window." }),
  reason: Schema.String.check(Schema.isMinLength(1)).annotate({
    description: "One sentence shown in the window: what the command does and what the user must type or answer.",
  }),
  workdir: Schema.optionalKey(Schema.String).annotate({
    description: "Working directory to execute the command in. Defaults to the current working directory.",
  }),
});

const description = [
  "Run a command in a new terminal window on the user's desktop, where the user types passwords and answers prompts directly.",
  "Use it only when the command needs the user's keyboard: sudo or other passwords and passphrases, [Y/n] prompts, interactive logins, and `dctl` root stages; run everything else in the shell.",
  "Only top-level sessions can call it; a child session returns the command to its parent as a blocker.",
  "The window covers the session, so before calling, write your results, show the command in a bash fence, and say what it does and what the user will type.",
  "Get any required approval first, including the repository's ask-first root stages, then call terminal without a Ready popup; the call asks the user to Run it or Run manually.",
  "Never ask the user in prose to run a command; call this tool instead.",
  "One command per call; the call blocks until the window closes and returns the exit code, an output tail, and the log path.",
].join(" ");

const choices = [
  { label: "Run it", description: "Open a terminal window and run it now" },
  { label: "Run manually", description: "Get the command to copy and run yourself" },
] as const;

const Answered = Schema.Struct({ answers: Schema.Array(Schema.Array(Schema.String)) });

type Question = Tool.Info & { readonly id: string };

function ask(question: Question, input: typeof Input.Type, cd: string | undefined, context: Tool.Context) {
  const text = [input.reason, ...(cd ? ["", `in ${tilde(cd)}`] : []), "", input.command].join("\n");
  return question
    .execute({ questions: [{ header: "Terminal", question: text, options: choices }] }, context)
    .pipe(Effect.map(({ metadata }) => (Schema.is(Answered)(metadata) ? (metadata.answers[0]?.[0] ?? "") : "")));
}

function manual(cd: string | undefined) {
  return [
    "The user chose to run this command themselves when ready; no window opened and nothing ran.",
    "Users usually pick this because they want to understand or discuss the command first.",
    `Reply with the exact command in a bash fence they can copy${cd ? `, with \`cd ${cd}\` on its own line first` : ""}, then a few plain sentences on what it does, what they will be asked to type, and what output or risk to expect.`,
    "Invite questions and end your turn; do not call `terminal` again for this command unless the user asks.",
  ].join(" ");
}

function typed(answer: string) {
  return [
    `The user typed their own answer instead of choosing Run it or Run manually: "${answer}"`,
    "No window opened and nothing ran.",
    "If the answer asks something or voices doubt, answer it; otherwise act on it.",
    "Do not call `terminal` again for this command unless the user asks.",
  ].join(" ");
}

export function unsafe(input: { command: string; reason: string; workdir?: string }) {
  for (const [field, pattern, allowed] of controls) {
    const char = input[field]?.match(pattern)?.[0];
    if (char === undefined) continue;
    const code = char.codePointAt(0)!.toString(16).toUpperCase().padStart(4, "0");
    return `${field} contains control character U+${code}; ${allowed}.`;
  }
}

function shell() {
  const own = process.env.SHELL;
  const usable =
    own && !["fish", "nu"].includes(path.basename(own)) && statSync(own, { throwIfNoEntry: false })?.isFile();
  return usable ? own : (Bun.which("bash") ?? "/bin/sh");
}

function prompts(command: string) {
  return launches(command).some(
    ({ command: launch }) =>
      launch[0] === "sudo" && !flags(launch).some(({ flag }) => flag === "n" || flag === "non-interactive"),
  );
}

function clean(raw: string) {
  const lines = stripVTControlCharacters(raw)
    .split("\n")
    .map((line) => line.replace(/\r+$/u, "").split("\r").at(-1) ?? "")
    .filter((line) => !/^Script (?:started|done) on /u.test(line));
  return lines.join("\n").trim();
}

async function last(file: string) {
  const handle = await open(file, "r").catch(() => undefined);
  if (!handle) return undefined;
  try {
    const { size } = await handle.stat();
    const start = Math.max(0, size - chunk);
    const { buffer, bytesRead } = await handle.read(Buffer.alloc(size - start), 0, size - start, start);
    const text = buffer.toString("utf8", 0, bytesRead);
    return { text, cut: start > 0 };
  } finally {
    await handle.close();
  }
}

function tail(text: string, cut: boolean) {
  if (text.length <= limit && !cut) return { text, cut };
  const rest = text.slice(-limit).replace(/^[\uDC00-\uDFFF]/u, "");
  const line = rest.indexOf("\n");
  return { text: line < 0 ? rest : rest.slice(line + 1), cut: true };
}

type Run = { kitty: string; cwd: string; session: string; reason: string; command: string };

async function popup(dir: string, run: Run, signal: AbortSignal) {
  await rm(dir, { recursive: true, force: true });
  await mkdir(dir, { recursive: true, mode: 0o700 });
  const env = Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith("KITTY_")));
  const proc = Bun.spawn(
    [
      run.kitty,
      "--class",
      "opencode-terminal",
      "--title",
      `${run.session} — ${run.reason}`,
      "-o",
      "allow_remote_control=no",
      "-o",
      "listen_on=none",
      "-e",
      "bash",
      runner,
      dir,
      shell(),
      run.session,
      run.reason,
      run.command,
    ],
    { cwd: run.cwd, env, stdin: "ignore", stdout: "ignore", stderr: "pipe", signal },
  );
  const [stderr, code] = await Promise.all([new Response(proc.stderr).text(), proc.exited]);
  return report(dir, code, stderr);
}

async function report(dir: string, code: number, stderr: string) {
  const result = (await readFile(path.join(dir, "result"), "utf8").catch(() => undefined))?.trim();
  if (result === undefined && code !== 0) {
    const detail = stderr.trim().slice(-2000);
    throw new Error(`kitty exited ${code} before the command finished${detail ? `: ${detail}` : ""}`);
  }
  if (result?.startsWith("failed ")) {
    const detail = (await readFile(path.join(dir, "runner.err"), "utf8").catch(() => "")).trim().slice(-2000);
    throw new Error(`the runner exited ${result.slice(7)} before the command started${detail ? `: ${detail}` : ""}`);
  }
  const exit = result?.match(/^exit (\d+)$/u)?.[1];
  if (exit !== undefined) return summary(dir, `Exit code: ${exit}`, { exit: Number(exit) });
  if (result === "started") {
    return summary(dir, "The window closed while the command was running, so no exit code was recorded.", {});
  }
  if (result === undefined) return summary(dir, "The window closed before the command started, so nothing ran.", {});
  throw new Error(`unrecognized result record: ${result}`);
}

async function summary(dir: string, status: string, metadata: { exit?: number }) {
  const log = path.join(dir, "output.log");
  const read = await last(log);
  if (!read) return { content: status, metadata };
  const output = tail(clean(read.text), read.cut);
  const body = output.text ? [output.cut ? "Output (end only):" : "Output:", output.text] : ["No output."];
  return { content: [status, `Log: ${log}`, "", ...body].join("\n"), metadata: { ...metadata, log } };
}

export default Plugin.define({
  id: "terminal",
  effect: (ctx) =>
    Effect.gen(function* () {
      yield* ctx.tool.transform((editor) => {
        editor.add({
          name: "terminal",
          description,
          input: Input,
          options: { codemode: false },
          execute: (input, context) =>
            Effect.gen(function* () {
              const invalid = unsafe(input);
              if (invalid) return yield* new Tool.Error({ message: invalid });
              const session = yield* ctx.session.get({ sessionID: context.sessionID }).pipe(Effect.orDie);
              if (session.parentID) {
                return yield* new Tool.Error({
                  message: "terminal is for top-level sessions only; return the command to your parent as a blocker",
                });
              }
              const cwd = path.resolve(session.location.directory, expand(input.workdir ?? "."));
              if (!statSync(cwd, { throwIfNoEntry: false })?.isDirectory()) {
                return yield* new Tool.Error({ message: `workdir ${cwd} is not a directory` });
              }
              if (!process.env.WAYLAND_DISPLAY && !process.env.DISPLAY) {
                return yield* new Tool.Error({
                  message: "the OpenCode server has no WAYLAND_DISPLAY or DISPLAY, so it cannot open a terminal window",
                });
              }
              const kitty = Bun.which("kitty");
              if (!kitty) {
                return yield* new Tool.Error({
                  message: "kitty is not on the OpenCode server's PATH, so it cannot open a terminal window",
                });
              }
              yield* ctx.permission.assert({
                sessionID: context.sessionID,
                agent: context.agent,
                action: "terminal",
                resources: [input.command],
                source: { type: "tool", messageID: context.messageID, id: context.id },
              });
              const question = (yield* ctx.tool.list()).find((tool) => tool.id === "question");
              if (!question) {
                return yield* new Tool.Error({
                  message: "the question tool is not registered, so terminal cannot ask",
                });
              }
              const cd = cwd === path.resolve(session.location.directory) ? undefined : cwd;
              const answer = yield* ask(question, input, cd, context);
              if (answer === "Run manually") return { content: manual(cd), metadata: { choice: "manual" } };
              if (answer !== "Run it") return { content: typed(answer), metadata: { choice: "typed" } };
              const dir = path.join(root, context.id.replace(/[^\w.-]/gu, "_"));
              const label = (session.title ?? "").replace(/\p{Cc}+/gu, " ").trim() || "OpenCode";
              const result = yield* Effect.tryPromise({
                try: (signal) => popup(dir, { ...input, kitty, cwd, session: label }, signal),
                catch: (error) => new Tool.Error({ message: `terminal failed: ${(error as Error).message}` }),
              });
              return { ...result, metadata: { ...result.metadata, choice: "run" } };
            }),
        });
      });

      yield* ctx.tool.hook("execute.before", (event) => {
        const command = (event.input as { command?: unknown } | undefined)?.command;
        if (event.tool !== "shell" || typeof command !== "string" || !prompts(command)) return Effect.void;
        return Effect.fail(new Tool.Error({ message: redirect }));
      });

      yield* ctx.tool.hook("execute.after", (event) =>
        Effect.sync(() => {
          if (event.tool !== "shell" || event.status !== "completed") return;
          const { content } = event.result;
          const items = typeof content === "string" ? [{ type: "text" as const, text: content }] : (content ?? []);
          const text = items.map((item) => (item.type === "text" ? item.text : "")).join("\n");
          if (!failures.some((failure) => text.includes(failure))) return;
          event.result = { ...event.result, content: [...items, { type: "text", text: hint }] };
        }),
      );
    }),
});
