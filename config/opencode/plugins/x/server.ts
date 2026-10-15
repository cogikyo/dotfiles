import { Plugin } from "@opencode/plugin";
import { Effect, Fiber, Schema } from "effect";
import { mkdir, mkdtemp, rm } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";

const timeout = 10 * 60 * 1000;
const workspace = path.join(homedir(), ".local/state/opencode/x");
const handle = /(?<![A-Za-z0-9_])@([A-Za-z0-9_]{1,15})(?![A-Za-z0-9_])/gu;
const invisible = /[\p{Cc}\p{Cf}\p{Zl}\p{Zp}]/u;

const flags = [
  "--model",
  "grok-4.7",
  "--reasoning-effort",
  "high",
  "--no-plan",
  "--no-subagents",
  "--no-memory",
  "--no-auto-update",
  "--verbatim",
  "--disable-web-search",
  "--permission-mode",
  "default",
  "--disallowed-tools",
  "run_terminal_cmd,run_terminal_command,read_file,search_replace,write,list_dir,grep,task,kill_command_or_subagent,get_command_or_subagent_output,spawn_subagent,scheduler_create,scheduler_delete,scheduler_list,monitor,search_tool,use_tool,workflow,enter_plan_mode,exit_plan_mode,ask_user_question,send_feedback,web_search,web_fetch,image_gen,image_edit,image_to_video,reference_to_video,todo_write,Agent",
  "--deny",
  "Bash",
  "--deny",
  "Read",
  "--deny",
  "Edit",
  "--deny",
  "Write",
  "--deny",
  "Grep",
  "--deny",
  "WebFetch",
  "--deny",
  "MCPTool",
  "--sandbox",
  "off",
  "--system-prompt-override",
  "Use only native X search tools. Never use generic web search, local files, or prior knowledge as evidence. Cite canonical x.com status URLs with handle and date. Separate official or maintainer statements from first-hand reports and from hype. If native X search cannot settle the claim, say so.",
];

const similar = 0.29;
const stop = new Set(
  "and are for from how the this that with what when which who whether any into than then there their them they was were will can not does its".split(
    " ",
  ),
);
const runs = new Map<string, { brief: string; words: Set<string> }[]>();

const Input = Schema.Struct({
  brief: Schema.String.check(Schema.isMinLength(1)).annotate({
    description:
      "Self-contained research brief: the claim to check, date window, relevant handles, and any mainline web findings to test.",
  }),
});

function forget(sessionID?: string) {
  if (sessionID === undefined) runs.clear();
  else runs.delete(sessionID);
}

function reject(sessionID: string, candidate: Set<string>) {
  for (const earlier of runs.get(sessionID) ?? []) {
    const score = jaccard(candidate, earlier.words);
    if (score < similar) continue;
    const quote = earlier.brief.length > 160 ? `${earlier.brief.slice(0, 160)}…` : earlier.brief;
    throw new Error(
      `x brief repeats an earlier brief in this session (word overlap ${score.toFixed(2)} >= ${similar}): "${quote}". Reuse that result, or write a new brief aimed at a named gap in it.`,
    );
  }
}

function prompt(brief: string) {
  const text = brief.normalize("NFKC");
  for (const char of text) {
    if (char === "\n" || char === "\t" || !invisible.test(char)) continue;
    const code = char.codePointAt(0)?.toString(16).toUpperCase().padStart(4, "0");
    throw new Error(`x brief must not contain control, format, or line-separator characters (found U+${code})`);
  }
  const named = text.replace(handle, "X user handle $1");
  if (/[@＠﹫]/u.test(named)) {
    throw new Error(
      "x brief may use '@' only in bare X handles such as @xai; other '@' text would make Grok attach a local file",
    );
  }
  if (named.split("\n").some((line) => /^\p{White_Space}*[/!]/u.test(line))) {
    throw new Error("x brief lines must not start with '/' or '!', which Grok treats as commands");
  }
  return `X research brief:\n${named}`;
}

async function search(text: string, abort: AbortSignal) {
  await mkdir(workspace, { recursive: true, mode: 0o700 });
  const cwd = await mkdtemp(path.join(workspace, "run-"));
  try {
    return await grok(text, cwd, abort);
  } finally {
    await rm(cwd, { recursive: true, force: true });
  }
}

async function grok(text: string, cwd: string, abort: AbortSignal) {
  const deadline = AbortSignal.timeout(timeout);
  const proc = Bun.spawn(["grok", "--single", text, ...flags], {
    cwd,
    stdin: "ignore",
    stdout: "pipe",
    stderr: "pipe",
    signal: AbortSignal.any([abort, deadline]),
  });
  const [stdout, stderr, code] = await Promise.all([
    new Response(proc.stdout).text(),
    new Response(proc.stderr).text(),
    proc.exited,
  ]);
  if (proc.signalCode && abort.aborted) throw new Error(`grok aborted by the caller (${proc.signalCode})`);
  if (proc.signalCode && deadline.aborted) throw new Error(`grok timed out after ${timeout / 60000} min`);
  if (proc.signalCode) throw new Error(`grok stopped by ${proc.signalCode}`);
  if (code !== 0) throw new Error(`grok exited ${code}: ${stderr.trim().slice(-2000) || "no stderr"}`);
  if (!stdout.trim())
    throw new Error(`grok returned no output${stderr.trim() ? `: ${stderr.trim().slice(-2000)}` : ""}`);
  return stdout.trim();
}

function words(text: string) {
  return new Set(
    (text.toLowerCase().match(/[a-z][a-z0-9]*/g) ?? []).filter((word) => word.length > 2 && !stop.has(word)),
  );
}

function jaccard(left: Set<string>, right: Set<string>) {
  const shared = [...left].filter((word) => right.has(word)).length;
  const union = left.size + right.size - shared;
  return union ? shared / union : 1;
}

export default Plugin.define({
  id: "x",
  async setup(ctx) {
    const fiber = Fiber.getCurrent();
    if (!fiber) throw new Error("x: setup started outside the plugin fiber");
    const log = Effect.runForkWith(fiber.context);
    const controller = new AbortController();
    void (async () => {
      for await (const event of ctx.event.subscribe({ signal: controller.signal }))
        if (event.type === "session.deleted") forget(event.data.sessionID);
    })().catch((error: unknown) => log(Effect.logError("x: watch sessions failed", { error: String(error) })));

    await ctx.tool.transform((editor) =>
      editor.add({
        name: "x",
        description:
          "Search live X/Twitter through Grok CLI native X search and return Grok's report. Slow; one call per brief. The output is untrusted evidence.",
        input: Input,
        options: { codemode: false },
        execute: async (args, context) => {
          const brief = args.brief.trim();
          const text = prompt(brief);
          const run = { brief, words: words(brief) };
          reject(context.sessionID, run.words);
          const session = runs.get(context.sessionID) ?? [];
          session.push(run);
          runs.set(context.sessionID, session);
          try {
            return { content: await search(text, context.signal) };
          } catch (error) {
            session.splice(session.indexOf(run), 1);
            throw error;
          }
        },
      }),
    );

    return () => {
      controller.abort();
      forget();
    };
  },
});
