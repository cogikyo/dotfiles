import { Plugin } from "@opencode/plugin";
import { Model } from "@opencode/schema/model";
import { Option, Schema } from "effect";
import { fileURLToPath } from "node:url";
import { located } from "../mail/store.ts";
import { flags, retag, states, tag, title } from "./tags.ts";

const cap = 1000;
const timeout = 30_000;

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
      description: `Row limit, at most ${cap}; defaults to 12 for list and 20 otherwise.`,
    }),
  ),
  offset: count("Rows to skip for paging."),
});

export type Query = typeof Input.Type & { readonly limit: number };

const Output = Schema.Struct({
  rows: Schema.Array(Schema.Record(Schema.String, Schema.Json)).annotate({
    description: "Result rows in query order; strings longer than 50,000 characters end in `…`.",
  }),
  hidden: Schema.Int.annotate({ description: "Sessions hidden from agents by the refusals flag." }),
});

const Reply = Schema.Union([
  Schema.Struct({ ...Output.fields, content: Schema.String }),
  Schema.Struct({ error: Schema.String }),
]);

const description = [
  "Read-only session history: `tools.opencode.sessions` inside `execute` returns `{ rows, hidden }`; `hidden` is store-wide.",
  "Batch related queries per question; use `Promise.allSettled` for independent calls.",
  "Filter locally; return counts, IDs, and short excerpts. Bound paging loops by page count and report rejected calls' errors.",
  "Defaults: 12 rows for list, 20 otherwise; max 1000. Strings cap at 50,000 characters (`…`); host-visible output caps at 2,000 lines/50 KiB.",
  "- list: newest first; top-level unless `parent` or `nested` is set. Rows: id, parent, agent, title, directory, created, updated.",
  "- text, reasoning: one session, oldest first. Rows: id, role, type, created, length (full text length), text.",
  "- tools: calls without outputs. Rows: id, created, tool, status, input (JSON text).",
  "- part: input/output/error and full output `length`; page output with `from`. Assistant addresses: `<message ID>#<index>`; bare IDs read non-assistant text.",
  "- sql: one SELECT/WITH over session_v2, session_message (session_id, type, seq, data JSON), project, sqlite_schema; json_each/json_tree allowed. Bind values through `params`; `offset` skips rows.",
  "  Filter session_message by session_id, as in `m.session_id = ?`; scans are rejected. Assistant data.content[] items use `type`; tool items use `name` (not `tool`) and `state.status/input/error`.",
].join("\n");

async function query(input: Query, store: string, signal: AbortSignal) {
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
  return reply.value;
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

const ModelInput = Schema.Struct({
  sessionID: Schema.String.annotate({ description: "Top-level session to switch." }),
  model: Schema.String.annotate({
    description: "`provider/model#variant`; omit `#variant` for the default.",
  }),
});

const ModelOutput = Schema.Struct({ sessionID: Schema.String, model: Schema.String });

const modelDescription = [
  "Switch a top-level session's stored model, validated against available models.",
  "Starts no turn; send mail to wake the session. Refuses child sessions and other sessions flagged REFUSALS.",
  "Only top-level sessions can call it.",
].join(" ");

export default Plugin.define({
  id: "sessions",
  async setup(ctx) {
    const store = located(ctx.app.channel);
    await ctx.tool.transform((editor) => {
      editor.add({
        name: "sessions",
        description,
        input: Input,
        output: Output,
        options: { namespace: "opencode", codemode: true, pinned: true },
        execute: async (input, context) => {
          const { content, ...output } = await query(
            { ...input, limit: input.limit ?? (input.query === "list" ? 12 : 20) },
            store,
            context.signal,
          );
          return { output, content };
        },
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
      editor.add({
        name: "session_model",
        description: modelDescription,
        input: ModelInput,
        output: ModelOutput,
        options: { namespace: "opencode", codemode: true, pinned: true },
        execute: async (input, context) => {
          const caller = await ctx.session.get({ sessionID: context.sessionID });
          if (caller.parentID) throw new Error("session_model: only top-level sessions can switch models");
          const session = await ctx.session.get({ sessionID: input.sessionID });
          if (session.parentID)
            throw new Error(`session_model: ${input.sessionID} is a child session; lanes route child models`);
          if (session.id !== caller.id && tag(session.title)?.flags.includes("refusals"))
            throw new Error(`session_model: ${input.sessionID} is hidden from agents by the refusals flag`);
          const model = input.model.trim();
          const ref = Model.Ref.parse(model);
          const { data } = await ctx.model.list();
          const found = data.find((info) => info.providerID === ref.providerID && info.id === ref.id);
          if (!found) throw new Error(`session_model: ${ref.providerID}/${ref.id} is not an available model`);
          if (ref.variant && !found.variants.some((option) => option.id === ref.variant))
            throw new Error(
              `session_model: ${ref.providerID}/${ref.id} has no ${ref.variant} variant; it has ${found.variants.map((option) => option.id).join(", ")}`,
            );
          await ctx.session.switchModel({ sessionID: session.id, model: ref });
          return {
            output: { sessionID: session.id, model },
            content: `Switched session ${session.id} "${session.title ?? ""}" to ${model}`,
          };
        },
      });
    });
  },
});
