import { z } from "zod";
import type { Message, Reply, Rule } from "../shared/opencode.ts";
import type { TaskArgs } from "./args.ts";
import type { ContextLimit } from "./context.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Task results                                                                                  │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

const CONTEXT_ADVICE =
  "re-brief narrower work; the next call to this lane creates a fresh child, never resume this context-limited session";
const CONTENT_FILTER_ADVICE =
  "child unrecoverable; re-brief a fresh child (reword the brief first, switch provider as last resort); never resume this session";
const INTERRUPTED_ADVICE =
  "completion unknown; reconcile durable state before re-running because the child may have edited files";

export function lastTextPart(reply: Reply) {
  return reply.parts.findLast((part) => part.type === "text")?.text ?? "";
}

/** Adds task notes ahead of the result text when present. */
export function withNotes(text: string, notes: string[]) {
  if (!notes.length) return text;
  return [`[${notes.join("; ")}]`, text].filter(Boolean).join("\n\n");
}

/** Reports a context-limited turn with recovered text and a warning when writes may have occurred. */
export function contextLimitedResult(input: {
  args: TaskArgs;
  metadata: Record<string, unknown>;
  sessionID: string;
  notes: string[];
  limit: ContextLimit;
  messages: Message[];
  permission: Rule[];
}) {
  const text = recoverableText(input.messages);
  const lines = [
    `context_limit: ${input.limit.level}`,
    `context_tokens: ${input.limit.tokens ?? "unknown"}`,
    `child_session_id: ${input.sessionID}`,
  ];
  if (input.limit.level === "compaction") {
    lines.push(
      "warning: automatic child compaction was observed; it may have started before the next poll, so any compacted continuation is untrusted",
    );
  }
  lines.push(
    !hasWriteAccess(input.permission)
      ? "durable_state: no writes expected from the child permission envelope"
      : "durable_state: uncertain; reconcile the tree and Git before continuing because this child had write-capable permissions",
    `advice: ${CONTEXT_ADVICE}`,
    "",
    "partial_recovered_text:",
    text || "(no recoverable assistant text)",
  );
  return {
    title: input.args.description,
    metadata: input.metadata,
    output: renderOutput({
      sessionID: input.sessionID,
      state: "context_limited",
      text: withNotes(lines.join("\n"), input.notes),
    }),
  };
}

/** Reports a content-filter block as an error; this result does not seal the lane. */
export function blockedResult(args: TaskArgs, metadata: Record<string, unknown>, sessionID: string, notes: string[]) {
  const text = withNotes(
    [`blocked: content_filter`, `child_session_id: ${sessionID}`, `advice: ${CONTENT_FILTER_ADVICE}`].join("\n"),
    notes,
  );
  return {
    title: args.description,
    metadata,
    output: renderOutput({ sessionID, state: "error", text }),
  };
}

/** Reports an unfinished child as an error without sealing its lane. */
export function interruptedResult(input: {
  args: TaskArgs;
  metadata: Record<string, unknown>;
  sessionID: string;
  notes: string[];
  reason?: string;
}) {
  const { args, metadata, sessionID, notes, reason = "child became idle without assistant output" } = input;
  const text = withNotes(
    [`interrupted: ${reason}`, `child_session_id: ${sessionID}`, `advice: ${INTERRUPTED_ADVICE}`].join("\n"),
    notes,
  );
  return {
    title: args.description,
    metadata,
    output: renderOutput({ sessionID, state: "error", text }),
  };
}

/** Wraps task text in a state-labeled task element, using `task_error` for errors. */
export function renderOutput(input: {
  sessionID: string;
  state: "completed" | "context_limited" | "error";
  text: string;
}) {
  const tag = input.state === "error" ? "task_error" : "task_result";
  return [`<task id="${input.sessionID}" state="${input.state}">`, `<${tag}>`, input.text, `</${tag}>`, "</task>"].join(
    "\n",
  );
}

const Failure = z.object({
  name: z.string().optional().catch(undefined),
  message: z.string().optional().catch(undefined),
  data: z
    .object({ message: z.string().optional().catch(undefined) })
    .optional()
    .catch(undefined),
});

/** Recognizes content-filter or refusal errors in strings and SDK error objects. */
export function isContentFilterBlock(error: unknown) {
  if (typeof error === "string") return isContentFilterText(error);
  const { data } = Failure.safeParse(error);
  const message = data?.message || data?.data?.message || (error instanceof Error ? String(error) : undefined);
  return isContentFilterText(data?.name) || isContentFilterText(message);
}

// ├─ Recovered text ──────────────────────────────────────────────────────────────────────────────┤

function recoverableText(messages: Message[]) {
  return messages
    .filter((message) => message.info.role === "assistant")
    .flatMap((message) => message.parts)
    .flatMap((part) => (part.type === "text" && part.text.trim() ? [part.text.trim()] : []))
    .join("\n\n");
}

function hasWriteAccess(rules: Rule[]) {
  const writePermissions = new Set(["*", "bash", "edit", "task", "write"]);
  return rules.some((rule) => rule.action === "allow" && writePermissions.has(rule.permission));
}

function isContentFilterText(value: string | undefined) {
  if (!value) return false;
  const compact = value.toLowerCase().replace(/[^a-z]/gu, "");
  return compact.includes("contentfilter") || compact.includes("refusal");
}
