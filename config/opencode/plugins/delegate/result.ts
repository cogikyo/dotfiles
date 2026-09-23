import { record } from "../shared/record.ts";
import type { TaskArgs } from "./args.ts";
import type { ContextLimit } from "./context.ts";
import type { Rule } from "./permission.ts";
import { string } from "./sdk.ts";

const CONTENT_FILTER_ADVICE =
  "child unrecoverable; re-brief a fresh child (reword the brief first, switch provider as last resort); never resume this session";
const INTERRUPTED_ADVICE =
  "completion unknown; reconcile durable state before re-running because the child may have edited files";
const CONTEXT_ADVICE =
  "re-brief narrower work; the next call to this lane creates a fresh child, never resume this context-limited session";

export function lastTextPart(value: unknown) {
  const parts = record(value)?.parts;
  if (!Array.isArray(parts)) return "";
  for (let index = parts.length - 1; index >= 0; index--) {
    const part = record(parts[index]);
    if (part?.type === "text" && typeof part.text === "string") return part.text;
  }
  return "";
}

export function withNotes(text: string, notes: string[]) {
  if (!notes.length) return text;
  return [`[${notes.join("; ")}]`, text].filter(Boolean).join("\n\n");
}

export function contextLimitedResult(input: {
  args: TaskArgs;
  metadata: Record<string, unknown>;
  sessionID: string;
  notes: string[];
  limit: ContextLimit;
  messages: unknown[];
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

function recoverableText(messages: unknown[]) {
  return messages
    .flatMap((message) => {
      const root = record(message);
      if (record(root?.info)?.role !== "assistant") return [];
      const parts = root?.parts;
      if (!Array.isArray(parts)) return [];
      return parts.flatMap((value) => {
        const part = record(value);
        return part?.type === "text" && typeof part.text === "string" && part.text.trim() ? [part.text.trim()] : [];
      });
    })
    .join("\n\n");
}

function hasWriteAccess(rules: Rule[]) {
  const writePermissions = new Set(["*", "bash", "edit", "task", "write"]);
  return rules.some((rule) => rule.action === "allow" && writePermissions.has(rule.permission));
}

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

export function isContentFilterBlock(error: unknown) {
  const root = record(error);
  const name = string(root?.name) ?? (error instanceof Error ? error.name : undefined);
  if (isContentFilterText(name)) return true;

  const data = record(root?.data);
  const message =
    string(root?.message) ??
    string(data?.message) ??
    (error instanceof Error || typeof error === "string" ? String(error) : undefined);
  return isContentFilterText(message);
}

function isContentFilterText(value: string | undefined) {
  if (!value) return false;
  const compact = value.toLowerCase().replace(/[^a-z]/gu, "");
  return compact.includes("contentfilter") || compact.includes("refusal");
}
