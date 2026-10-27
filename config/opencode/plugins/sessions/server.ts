import { Plugin } from "@opencode/plugin";
import { Option, Schema } from "effect";
import { homedir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { flags, retag, states, tag, title } from "./tags.ts";

const cap = 20;
const timeout = 30_000;
const store = path.join(
  process.env.XDG_DATA_HOME || path.join(homedir(), ".local", "share"),
  "opencode",
  "opencode.db",
);

const text = (description: string) => Schema.optional(Schema.String.annotate({ description }));
const flag = (description: string) => Schema.optional(Schema.Boolean.annotate({ description }));
const count = (description: string, maximum = Number.MAX_SAFE_INTEGER) =>
  Schema.optional(Schema.Int.check(Schema.isBetween({ minimum: 0, maximum })).annotate({ description }));

const Input = Schema.Struct({
  query: Schema.Literals(["list", "text", "reasoning", "tools", "part", "sql"]),
  session: text("Session ID; required for text, reasoning, and tools."),
  part: text(
    "Part address `<message ID>#<content index>`, or a bare message ID for non-assistant text; required for part.",
  ),
  directory: text("list: exact session directory."),
  agent: text("list: exact session agent, such as `collab`; sessions left on the default agent store none."),
  parent: text("list: children of this session ID."),
  nested: flag("list: include child sessions at every level."),
  since: text("list: earliest time_updated, ISO 8601."),
  until: text("list: latest time_updated, ISO 8601."),
  role: Schema.optional(
    Schema.Literals(["user", "assistant"]).annotate({ description: "text, reasoning: message role." }),
  ),
  synthetic: flag("text: include harness-injected text (synthetic, system, skill, compaction summary); default false."),
  match: text("text, reasoning: case-insensitive substring."),
  tool: text("tools: exact tool name, such as `shell` or `skill`."),
  from: count("part: output character offset for paging long output."),
  sql: text("sql: one SELECT or WITH statement with `?` placeholders."),
  params: Schema.optional(
    Schema.Array(Schema.Union([Schema.String, Schema.Finite, Schema.Null])).annotate({
      description: "sql: placeholder values.",
    }),
  ),
  limit: Schema.optional(
    Schema.Int.check(Schema.isBetween({ minimum: 1, maximum: cap })).annotate({
      description: `Row limit, at most ${cap}; list defaults to 12.`,
    }),
  ),
  offset: count("Rows to skip for paging."),
});

export type Query = typeof Input.Type & { readonly limit: number };

const Reply = Schema.Union([Schema.Struct({ output: Schema.String }), Schema.Struct({ error: Schema.String })]);

const description = [
  `Read-only queries over the local OpenCode session store. Each call returns at most ${cap} JSON rows; text is capped at 800 characters.`,
  "- list: sessions by directory, agent, parent, and time, newest first; top-level only unless `parent` or `nested` is set.",
  "- text, reasoning: one session's text or reasoning excerpts, oldest first.",
  "- tools: one session's tool calls with status and a short input excerpt, without outputs.",
  "- part: one item's input, output, and error excerpts; page long output with `from`. Row IDs `<message ID>#<n>` address item n of an assistant message's content.",
  "- sql: one read-only SELECT over session_v2, session_message (type, seq, data JSON; assistant data.content[] holds text, reasoning, and tool items; tool input is at `$.state.input`), and project; json_each is allowed; bind every value through `params`.",
].join("\n");

async function query(input: Query, signal: AbortSignal) {
  const deadline = AbortSignal.timeout(timeout);
  const proc = Bun.spawn([process.execPath, fileURLToPath(new URL("./query.ts", import.meta.url)), store], {
    env: { ...process.env, BUN_BE_BUN: "1" },
    stdin: new Blob([JSON.stringify(input)]),
    stdout: "pipe",
    stderr: "pipe",
    killSignal: "SIGKILL",
    signal: AbortSignal.any([signal, deadline]),
  });
  const [stdout, stderr] = await Promise.all([
    new Response(proc.stdout).text(),
    new Response(proc.stderr).text(),
    proc.exited,
  ]);
  if (proc.signalCode && signal.aborted) throw new Error("sessions query aborted by the caller");
  if (proc.signalCode && deadline.aborted) throw new Error(`sessions query timed out after ${timeout / 1000}s`);
  if (proc.signalCode) throw new Error(`sessions query killed by ${proc.signalCode}`);
  const reply = Schema.decodeUnknownOption(Schema.fromJsonString(Reply))(stdout);
  if (Option.isNone(reply)) throw new Error(`sessions query failed: ${stderr.trim().slice(-1000) || "no output"}`);
  if ("error" in reply.value) throw new Error(reply.value.error);
  return reply.value.output;
}

const TagInput = Schema.Struct({
  sessionID: Schema.optional(
    Schema.String.annotate({ description: "Top-level session to tag; omit for this session." }),
  ),
  state: Schema.Literals(states).annotate({ description: "Why the session is finished." }),
  flags: Schema.optional(
    Schema.Array(Schema.Literals(flags)).annotate({ description: "Follow-up markers for search and ranking." }),
  ),
});

const TagOutput = Schema.Struct({ sessionID: Schema.String, title: Schema.String });

const tagDescription = [
  "Mark a finished top-level session by replacing its title tag with one state and optional flags, keeping the rest of the title.",
  "The state marks the session closed. Repeating a call changes nothing.",
  "Only top-level sessions can call it.",
].join(" ");

export default Plugin.define({
  id: "sessions",
  async setup(ctx) {
    await ctx.tool.transform((editor) => {
      editor.add({
        name: "sessions",
        description,
        input: Input,
        options: { codemode: false },
        execute: async (input, context) => ({
          content: await query({ ...input, limit: input.limit ?? (input.query === "list" ? 12 : cap) }, context.signal),
        }),
      });
      editor.add({
        name: "session_tag",
        description: tagDescription,
        input: TagInput,
        output: TagOutput,
        options: { namespace: "opencode", codemode: true, pinned: true },
        execute: async (input, context) => {
          const caller = await ctx.session.get({ sessionID: context.sessionID });
          if (caller.parentID) throw new Error("session_tag: only top-level sessions can tag sessions");
          const sessionID = input.sessionID ?? context.sessionID;
          const session = sessionID === caller.id ? caller : await ctx.session.get({ sessionID });
          if (session.parentID)
            throw new Error(`session_tag: ${sessionID} is a child session; only top-level sessions take tags`);
          // This tool cannot clear REFUSALS; the TUI toggle can.
          const sticky = tag(session.title)?.flags.filter((name) => name === "refusals") ?? [];
          if (sticky.length && session.id !== caller.id)
            throw new Error(`session_tag: ${sessionID} is hidden from agents by the refusals flag`);
          const next = retag(title(session), { state: input.state, flags: [...(input.flags ?? []), ...sticky] });
          const changed = next !== session.title;
          if (changed) await ctx.session.update({ sessionID, title: next });
          return {
            output: { sessionID, title: next },
            content: `${changed ? "Tagged" : "Already tagged"} session ${sessionID}: ${next}`,
          };
        },
      });
    });
  },
});
