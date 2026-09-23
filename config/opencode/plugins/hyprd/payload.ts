// @ts-nocheck -- OpenCode plugin event types are incomplete; keep runtime behavior stable until local event types exist.
export const LIMITS = {
  id: 128,
  status: 32,
  message: 512,
  patterns: 256,
};

const NEW_SESSION_START_MESSAGE = "New Session started";
const MESSAGE_ID_PATHS = [
  "messageID",
  "messageId",
  "metadata.messageID",
  "metadata.messageId",
  "message.id",
  "metadata.id",
  "id",
];
const PART_MESSAGE_ID_PATHS = ["messageID", "messageId", "message.id"];
const SESSION_ID_PATHS = ["sessionID", "sessionId", "metadata.sessionID", "metadata.sessionId", "session.id"];

export function cleanText(value, max = LIMITS.message) {
  if (typeof value !== "string") return "";
  return value.replace(/\s+/g, " ").trim().slice(0, max);
}

export function cleanSessionTitle(value) {
  const title = cleanText(value);
  const normalized = cleanText(title.replace(/^New Session\s+-\s*/i, ""), LIMITS.id);
  return isPlaceholderSessionTitle(normalized) ? "" : normalized;
}

function isPlaceholderSessionTitle(value) {
  const title = cleanText(value, LIMITS.id).toLowerCase();
  return !title || title === "new session" || title === "session start info here" || isTimestampTitle(title);
}

function isTimestampTitle(value) {
  return /^\d{4}-\d{2}-\d{2}t\d{2}:\d{2}:\d{2}(?:\.\d+)?z$/i.test(cleanText(value, LIMITS.id));
}

export function startMessage(state) {
  const subject = state.lastUserMessage || state.title;
  return subject ? `Working on "${subject}"` : NEW_SESSION_START_MESSAGE;
}

function cleanPromptText(value) {
  return cleanText(String(value || "").replace(/\[Image\s+\d+\]/gi, " "));
}

export function messageRole(value) {
  return cleanText(
    value?.role || value?.metadata?.role || value?.author?.role || value?.type,
    LIMITS.status,
  ).toLowerCase();
}

export function partRole(value) {
  return cleanText(value?.role || value?.author?.role, LIMITS.status).toLowerCase();
}

export function lookup(value, paths) {
  for (const path of paths) {
    const found = path.split(".").reduce((node, key) => node?.[key], value);
    if (found) return found;
  }
  return "";
}

export function messageID(value) {
  return lookup(value, MESSAGE_ID_PATHS);
}

export function partMessageID(part, props) {
  return lookup(props, PART_MESSAGE_ID_PATHS) || lookup(part, PART_MESSAGE_ID_PATHS);
}

export function messageSessionID(value) {
  return lookup(value, SESSION_ID_PATHS);
}

export function textFromMessage(value) {
  if (!value) return "";
  if (typeof value === "string") return cleanPromptText(value);
  if (Array.isArray(value)) return cleanPromptText(value.map(textFromMessage).filter(Boolean).join(" "));
  if (typeof value !== "object") return "";

  for (const key of ["text", "message", "prompt", "input"]) {
    if (typeof value[key] === "string") {
      const text = cleanPromptText(value[key]);
      if (text) return text;
    }
  }

  for (const key of ["parts", "content", "messages"]) {
    const text = textFromMessage(value[key]);
    if (text) return text;
  }

  return "";
}

function isUntimedUserTextPart(part) {
  return part?.type === "text" && !part.synthetic && !part.ignored && !part.time && textFromMessage(part);
}

export function isUserText(part, role) {
  return part.type === "text" && (role === "user" || (!role && isUntimedUserTextPart(part)));
}

export function isAssistantText(part, role) {
  return part.id && part.type === "text" && (role === "assistant" || (!role && part?.time?.end));
}

export function isAgentPart(part) {
  return part.id && (part.type === "subtask" || part.type === "agent");
}

export function agentNotice(part) {
  return {
    agent_type: cleanText(part.agent || part.name || "Agent", LIMITS.id),
    message: cleanText(part.description || part.prompt || part.name || "Done"),
  };
}
