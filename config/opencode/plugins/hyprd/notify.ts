// @ts-nocheck -- OpenCode plugin event types are incomplete; keep runtime behavior stable until local event types exist.
import {
  agentNotice,
  cleanSessionTitle,
  cleanText,
  isAgentPart,
  isAssistantText,
  isUserText,
  LIMITS,
  lookup,
  messageID,
  messageRole,
  messageSessionID,
  partMessageID,
  partRole,
} from "./payload.ts";
import {
  applyTodos,
  clearIdleReminder,
  clearStartNotify,
  getSession,
  scheduleComplete,
  scheduleIdleReminder,
  scheduleStartNotify,
  sendNotify,
  trySendStartNotify,
  updateAssistantPartText,
  updateUserMessage,
} from "./state.ts";

const PERMISSION_DEBOUNCE_MS = 1500;

async function handleMessageUpdated({ sessions, messageRoles, messageSessions }, props) {
  const msg = props?.message || props?.info || props;
  const msgID = messageID(msg) || messageID(props);
  const role = messageRole(msg) || partRole(props);
  const id = lookup(props, ["sessionID", "sessionId"]) || messageSessionID(msg) || messageSessionID(props);
  if (msgID && role) messageRoles.set(msgID, role);
  if (msgID && id) messageSessions.set(msgID, id);
  if (!id || role !== "user") return;

  await updateUserMessage(sessions, id, msg);
}

// ├─ OpenCode event handlers ─────────────────────────────────────────────────────────────────────┤
const handlers = {
  "message.created": handleMessageUpdated,

  "message.updated": handleMessageUpdated,

  "message.part.delta": async ({ sessions }, { sessionID, partID, field, delta }) => {
    if (!sessionID || !partID || field !== "text" || typeof delta !== "string") return;

    const state = getSession(sessions, sessionID);
    const text = (state.assistantPartText.get(partID) || "") + delta;
    updateAssistantPartText(state, partID, text);
  },

  "message.part.updated": async ({ sessions, messageRoles, messageSessions }, props) => {
    const { sessionID, part } = props;
    const msgID = partMessageID(part, props);
    const id = sessionID || part?.sessionID || messageSessions.get(msgID);
    if (!id || !part) return;

    const state = getSession(sessions, id);
    const role = partRole(part) || messageRole(props?.message) || partRole(props) || messageRoles.get(msgID);

    if (isUserText(part, role)) {
      await updateUserMessage(sessions, id, part);
      return;
    }

    if (isAssistantText(part, role)) {
      updateAssistantPartText(state, part.id, part.text);
      return;
    }

    if (isAgentPart(part) && !state.seenAgentParts.has(part.id)) {
      state.seenAgentParts.add(part.id);
      await sendNotify(sessions, {
        sessionID: id,
        type: "subagent",
        ...agentNotice(part),
      });
    }
  },

  "session.status": async ({ sessions }, { sessionID, status }) => {
    if (!sessionID || typeof status?.type !== "string") return;

    const state = getSession(sessions, sessionID);
    const type = status.type;

    if (type === "busy" || type === "retry") {
      clearTimeout(state.completeTimer);
      state.completeTimer = null;
      clearIdleReminder(state);

      if (!state.active) {
        state.active = true;
        state.startNotified = false;
        if (type !== "retry" && state.lastUserMessageAt <= state.inactiveAt) state.lastUserMessage = "";
        if (type === "retry") await trySendStartNotify(sessions, sessionID, "Retrying");
        else if (state.lastUserMessage) scheduleStartNotify(sessions, sessionID);
        else scheduleStartNotify(sessions, sessionID);
      }
      return;
    }

    if (type === "idle") {
      scheduleComplete(sessions, sessionID);
    }
  },

  "session.idle": async ({ sessions }, { sessionID }) => {
    if (!sessionID) return;
    scheduleComplete(sessions, sessionID);
  },

  "permission.asked": async ({ sessions }, { sessionID, permission, patterns, title, pattern }) => {
    const perm = cleanText(permission || title, LIMITS.id);
    const rawPatterns = patterns ?? pattern;
    const pats = Array.isArray(rawPatterns)
      ? cleanText(rawPatterns.join(", "), LIMITS.patterns)
      : typeof rawPatterns === "string"
        ? cleanText(rawPatterns, LIMITS.patterns)
        : "";
    const message = perm ? (pats ? `${perm}: ${pats}` : perm) : "Permission needed";

    if (sessionID) {
      const state = getSession(sessions, sessionID);
      const now = Date.now();
      if (state.lastPermissionMessage === message && now - state.lastPermissionAt < PERMISSION_DEBOUNCE_MS) return;
      state.lastPermissionMessage = message;
      state.lastPermissionAt = now;
    }

    await sendNotify(sessions, { sessionID, type: "permission", message });
  },

  "permission.updated": async (ctx, props) => {
    await handlers["permission.asked"](ctx, props);
  },

  "question.asked": async ({ sessions }, { sessionID, questions }) => {
    const first = Array.isArray(questions) ? questions[0] : null;
    const header = cleanText(first?.header, LIMITS.id);
    const question = cleanText(first?.question);
    const message = header ? (question ? `${header}: ${question}` : header) : question || "Question asked";
    await sendNotify(sessions, { sessionID, type: "question", message });
  },

  "todo.updated": async ({ sessions }, { sessionID, todos }) => {
    if (!sessionID || !Array.isArray(todos)) return;

    const state = getSession(sessions, sessionID);
    const completed = applyTodos(state, todos);

    if (!state.hasOpenTodos && state.active) scheduleComplete(sessions, sessionID);

    if (completed.length > 0) {
      state.lastTodoCompletedAt = Date.now();
      await Promise.all(
        completed.map((message) => sendNotify(sessions, { sessionID, type: "todo-complete", message })),
      );
    }
  },

  "session.created": async ({ sessions }, { sessionID, info }) => {
    const id = sessionID || info?.id;
    if (!id) return;
    const state = getSession(sessions, id);
    state.parentID = typeof info?.parentID === "string" ? info.parentID : "";
    if (state.parentID) clearIdleReminder(state);
    const title = cleanSessionTitle(info?.title);
    if (title) state.title = title;
    if (state.active && state.title && !state.startNotified) scheduleStartNotify(sessions, id);
  },

  "session.updated": async ({ sessions }, { sessionID, info }) => {
    const id = sessionID || info?.id;
    if (!id) return;
    const state = getSession(sessions, id);
    state.parentID = typeof info?.parentID === "string" ? info.parentID : "";
    if (state.parentID) clearIdleReminder(state);
    const title = cleanSessionTitle(info?.title);
    if (title) state.title = title;
    if (state.active && state.title && !state.startNotified) scheduleStartNotify(sessions, id);
  },

  "session.error": async ({ sessions }, { sessionID, error }) => {
    const message = cleanText(error?.data?.message || error?.name || "Session error");
    if (sessionID) {
      const state = sessions.get(sessionID);
      if (state) {
        clearTimeout(state.completeTimer);
        state.completeTimer = null;
        clearStartNotify(state);
        clearIdleReminder(state);
        state.active = false;
        if (state.parentID) {
          scheduleComplete(sessions, state.parentID);
          scheduleIdleReminder(sessions, state.parentID);
        }
      }
    }
    await sendNotify(sessions, { sessionID, type: "error", message });
  },

  "session.deleted": ({ sessions }, { info }) => {
    if (!info?.id) return;
    const state = sessions.get(info.id);
    if (state) {
      clearTimeout(state.completeTimer);
      clearStartNotify(state);
      clearIdleReminder(state);
      if (state.parentID) {
        scheduleComplete(sessions, state.parentID);
        scheduleIdleReminder(sessions, state.parentID);
      }
    }
    sessions.delete(info.id);
  },
};

const server = async () => {
  const ctx = { sessions: new Map(), messageRoles: new Map(), messageSessions: new Map() };

  return {
    "chat.message": async (input, output) => {
      const sessionID = input?.sessionID || output?.message?.sessionID;
      if (!sessionID) return;

      await updateUserMessage(ctx.sessions, sessionID, output?.parts || output?.message);
    },

    event: async ({ event }) => {
      if (!event || typeof event.type !== "string") return;
      const handler = handlers[event.type];
      if (handler) await handler(ctx, event.properties ?? {});
    },
  };
};

/**
 * Server plugin that maps OpenCode chat and lifecycle events to hyprd notifications.
 * It uses the `chat.message` and `event` hooks for messages, sessions, permissions, questions, and todos.
 */
export default { id: "hyprd-notify", server };
