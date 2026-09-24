import type * as v2 from "@opencode-ai/sdk/v2";
import { z } from "zod";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Notify payloads                                                                               │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

/** Text limits for hyprd notices. */
export const LIMITS = {
  id: 128,
  status: 32,
  message: 512,
  patterns: 256,
};

// ├─ Event views ─────────────────────────────────────────────────────────────────────────────────┤

// Invalid event payloads are ignored by notify.ts.

type Text = Pick<v2.TextPart, "id" | "messageID" | "type" | "text" | "synthetic" | "ignored"> & {
  time?: Pick<NonNullable<v2.TextPart["time"]>, "end">;
};
type Subtask = Pick<v2.SubtaskPart, "id" | "messageID" | "type" | "agent" | "description" | "prompt">;
type Agent = Pick<v2.AgentPart, "id" | "messageID" | "type" | "name">;
type Part = Text | Subtask | Agent;

const Part: z.ZodType<Part> = z.discriminatedUnion("type", [
  z.object({
    id: z.string(),
    messageID: z.string(),
    type: z.literal("text"),
    text: z.string(),
    synthetic: z.boolean().optional(),
    ignored: z.boolean().optional(),
    time: z.object({ end: z.number().optional() }).optional(),
  }),
  z.object({
    id: z.string(),
    messageID: z.string(),
    type: z.literal("subtask"),
    agent: z.string(),
    description: z.string(),
    prompt: z.string(),
  }),
  z.object({ id: z.string(), messageID: z.string(), type: z.literal("agent"), name: z.string() }),
]);

type MessageUpdated = Pick<v2.EventMessageUpdated["properties"], "sessionID"> & {
  info: Pick<v2.Message, "id" | "role">;
};

export const MessageUpdated: z.ZodType<MessageUpdated> = z.object({
  sessionID: z.string(),
  info: z.object({ id: z.string(), role: z.enum(["user", "assistant"]) }),
});

type PartUpdated = Pick<v2.EventMessagePartUpdated["properties"], "sessionID"> & { part: Part };

/** Accepts only text, subtask, and agent parts from `message.part.updated`. */
export const PartUpdated: z.ZodType<PartUpdated> = z.object({ sessionID: z.string(), part: Part });

type PartDelta = Pick<v2.EventMessagePartDelta["properties"], "sessionID" | "partID" | "field" | "delta">;

export const PartDelta: z.ZodType<PartDelta> = z.object({
  sessionID: z.string(),
  partID: z.string(),
  field: z.string(),
  delta: z.string(),
});

type SessionStatus = Pick<v2.EventSessionStatus["properties"], "sessionID"> & {
  status: Pick<v2.SessionStatus, "type">;
};

export const SessionStatus: z.ZodType<SessionStatus> = z.object({
  sessionID: z.string(),
  status: z.object({ type: z.enum(["idle", "busy", "retry"]) }),
});

type SessionIdle = Pick<v2.EventSessionIdle["properties"], "sessionID">;

export const SessionIdle: z.ZodType<SessionIdle> = z.object({ sessionID: z.string() });

type Session = Pick<v2.Session, "id" | "parentID" | "title">;
type SessionInfo = { info: Session };

/** Validates session info shared by creation, update, and deletion events. */
export const SessionInfo: z.ZodType<SessionInfo> = z.object({
  info: z.object({ id: z.string(), parentID: z.string().optional(), title: z.string() }),
});

type Failure = { name: string; data?: { message?: string } };
type SessionError = Pick<v2.EventSessionError["properties"], "sessionID"> & { error?: Failure };

/** Accepts session errors without an ID or a usable error message. */
export const SessionError: z.ZodType<SessionError> = z.object({
  sessionID: z.string().optional(),
  error: z
    .object({
      name: z.string(),
      data: z.object({ message: z.string().optional().catch(undefined) }).optional(),
    })
    .optional(),
});

type PermissionAsked = Pick<v2.EventPermissionAsked["properties"], "sessionID" | "permission" | "patterns">;

export const PermissionAsked: z.ZodType<PermissionAsked> = z.object({
  sessionID: z.string(),
  permission: z.string(),
  patterns: z.array(z.string()),
});

type QuestionAsked = Pick<v2.EventQuestionAsked["properties"], "sessionID"> & {
  questions: Pick<v2.QuestionInfo, "header" | "question">[];
};

export const QuestionAsked: z.ZodType<QuestionAsked> = z.object({
  sessionID: z.string(),
  questions: z.array(z.object({ header: z.string(), question: z.string() })),
});

type TodoUpdated = Pick<v2.EventTodoUpdated["properties"], "sessionID"> & {
  todos: Pick<v2.Todo, "content" | "status">[];
};

export const TodoUpdated: z.ZodType<TodoUpdated> = z.object({
  sessionID: z.string(),
  todos: z.array(z.object({ content: z.string(), status: z.string() })),
});

// ├─ Notice text ─────────────────────────────────────────────────────────────────────────────────┤

export type Role = v2.Message["role"];

/** Cleans and truncates notice text. */
export function cleanText(value: string | undefined, max = LIMITS.message) {
  if (value === undefined) return "";
  return value.replace(/\s+/g, " ").trim().slice(0, max);
}

/** Removes generic or timestamp-based session titles so they do not become notice text. */
export function cleanSessionTitle(value: string) {
  const title = cleanText(value);
  const normalized = cleanText(title.replace(/^New Session\s+-\s*/i, ""), LIMITS.id);
  return isPlaceholderSessionTitle(normalized) ? "" : normalized;
}

function isPlaceholderSessionTitle(value: string) {
  const title = cleanText(value, LIMITS.id).toLowerCase();
  return !title || title === "new session" || title === "session start info here" || isTimestampTitle(title);
}

function isTimestampTitle(value: string) {
  return /^\d{4}-\d{2}-\d{2}t\d{2}:\d{2}:\d{2}(?:\.\d+)?z$/i.test(cleanText(value, LIMITS.id));
}

const NEW_SESSION_START_MESSAGE = "New Session started";

/** Uses the last user message or title for a start notice, with a generic fallback. */
export function startMessage({ lastUserMessage, title }: { lastUserMessage: string; title: string }) {
  const subject = lastUserMessage || title;
  return subject ? `Working on "${subject}"` : NEW_SESSION_START_MESSAGE;
}

/** Omits image placeholders from user-facing prompt text. */
export function promptText(value: string) {
  return cleanText(value.replace(/\[Image\s+\d+\]/gi, " "));
}

/** Recognizes user text by message role, or by part metadata when the role is unavailable. */
export function isUserText(part: Text, role: Role | undefined) {
  if (role) return role === "user";
  return !part.synthetic && !part.ignored && !part.time && promptText(part.text) !== "";
}

/** Recognizes assistant text by message role, or by its end time when the role is unavailable. */
export function isAssistantText(part: Text, role: Role | undefined) {
  if (!part.id) return false;
  if (role) return role === "assistant";
  return Boolean(part.time?.end);
}

/** Builds a subagent notice from an agent name or subtask description. */
export function agentNotice(part: Subtask | Agent) {
  if (part.type === "agent") {
    return { agent_type: cleanText(part.name || "Agent", LIMITS.id), message: cleanText(part.name || "Done") };
  }
  return {
    agent_type: cleanText(part.agent || "Agent", LIMITS.id),
    message: cleanText(part.description || part.prompt || "Done"),
  };
}
