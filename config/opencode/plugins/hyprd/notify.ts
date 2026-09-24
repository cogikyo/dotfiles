import type { Plugin } from "@opencode-ai/plugin";
import type { Part as V1Part } from "@opencode-ai/sdk";
import type { z } from "zod";
import {
  agentNotice,
  cleanSessionTitle,
  cleanText,
  isAssistantText,
  isUserText,
  LIMITS,
  MessageUpdated,
  PartDelta,
  PartUpdated,
  PermissionAsked,
  QuestionAsked,
  type Role,
  SessionError,
  SessionIdle,
  SessionInfo,
  SessionStatus,
  TodoUpdated,
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
  type Sessions,
  trySendStartNotify,
  updateAssistantPartText,
  updateUserMessage,
} from "./state.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Hyprland notifications                                                                        │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

const PERMISSION_DEBOUNCE_MS = 1500;

type Context = { sessions: Sessions; roles: Map<string, Role> };

const server: Plugin = async () => {
  const ctx: Context = { sessions: new Map(), roles: new Map() };

  return {
    "chat.message": async (input, output) => {
      updateUserMessage(ctx.sessions, input.sessionID, output.parts.map(partText).join(" "));
    },

    event: async ({ event }) => {
      await handlers.get(event.type)?.(ctx, event.properties);
    },
  };
};

/** Turns valid OpenCode chat and session events into notices for live kitty panes. */
export default { id: "hyprd-notify", server };

// ├─ OpenCode event handlers ─────────────────────────────────────────────────────────────────────┤

const sessionInfo = on(SessionInfo, ({ sessions }, { info }) => {
  const state = getSession(sessions, info.id);
  state.parentID = info.parentID ?? "";
  if (state.parentID) clearIdleReminder(state);
  const title = cleanSessionTitle(info.title);
  if (title) state.title = title;
  if (state.active && state.title && !state.startNotified) scheduleStartNotify(sessions, info.id);
});

const handlers = new Map([
  [
    "message.updated",
    // Message roles remain cached after session deletion.
    on(MessageUpdated, ({ roles }, { info }) => {
      roles.set(info.id, info.role);
    }),
  ],

  [
    "message.part.delta",
    on(PartDelta, ({ sessions }, { sessionID, partID, field, delta }) => {
      if (field !== "text") return;

      const state = getSession(sessions, sessionID);
      const text = (state.assistantPartText.get(partID) || "") + delta;
      updateAssistantPartText(state, partID, text);
    }),
  ],

  [
    "message.part.updated",
    on(PartUpdated, async ({ sessions, roles }, { sessionID, part }) => {
      const state = getSession(sessions, sessionID);
      const role = roles.get(part.messageID);

      if (part.type === "text") {
        if (isUserText(part, role)) updateUserMessage(sessions, sessionID, part.text);
        else if (isAssistantText(part, role)) updateAssistantPartText(state, part.id, part.text);
        return;
      }

      // Each agent or subtask part generates at most one notice per session state.
      if (part.id && !state.seenAgentParts.has(part.id)) {
        state.seenAgentParts.add(part.id);
        await sendNotify(sessions, { sessionID, type: "subagent", ...agentNotice(part) });
      }
    }),
  ],

  [
    "session.status",
    on(SessionStatus, async ({ sessions }, { sessionID, status }) => {
      const state = getSession(sessions, sessionID);
      const type = status.type;

      // Idle status waits for the completion debounce before ending the run.
      if (type === "idle") {
        scheduleComplete(sessions, sessionID);
        return;
      }

      clearTimeout(state.completeTimer);
      state.completeTimer = undefined;
      clearIdleReminder(state);
      if (state.active) return;

      state.active = true;
      state.startNotified = false;
      // A retry or a message that arrived during completion remains the start subject.
      if (type !== "retry" && state.lastUserMessageAt <= state.inactiveAt) state.lastUserMessage = "";
      if (type === "retry") await trySendStartNotify(sessions, sessionID, "Retrying");
      else scheduleStartNotify(sessions, sessionID);
    }),
  ],

  [
    "session.idle",
    on(SessionIdle, ({ sessions }, { sessionID }) => {
      scheduleComplete(sessions, sessionID);
    }),
  ],

  [
    "permission.asked",
    on(PermissionAsked, async ({ sessions }, { sessionID, permission, patterns }) => {
      const perm = cleanText(permission, LIMITS.id);
      const pats = cleanText(patterns.join(", "), LIMITS.patterns);
      const message = perm ? (pats ? `${perm}: ${pats}` : perm) : "Permission needed";

      const state = getSession(sessions, sessionID);
      const now = Date.now();
      // Permission duplicates are suppressed by text across panes for 1500ms.
      if (state.lastPermissionMessage === message && now - state.lastPermissionAt < PERMISSION_DEBOUNCE_MS) return;
      state.lastPermissionMessage = message;
      state.lastPermissionAt = now;

      await sendNotify(sessions, { sessionID, type: "permission", message });
    }),
  ],

  [
    "question.asked",
    on(QuestionAsked, async ({ sessions }, { sessionID, questions }) => {
      const first = questions[0];
      const header = cleanText(first?.header, LIMITS.id);
      const question = cleanText(first?.question);
      const message = header ? (question ? `${header}: ${question}` : header) : question || "Question asked";
      await sendNotify(sessions, { sessionID, type: "question", message });
    }),
  ],

  [
    "todo.updated",
    on(TodoUpdated, async ({ sessions }, { sessionID, todos }) => {
      const state = getSession(sessions, sessionID);
      const completed = applyTodos(state, todos);

      if (!state.hasOpenTodos && state.active) scheduleComplete(sessions, sessionID);

      if (completed.length > 0) {
        state.lastTodoCompletedAt = Date.now();
        await Promise.all(
          completed.map((message) => sendNotify(sessions, { sessionID, type: "todo-complete", message })),
        );
      }
    }),
  ],

  ["session.created", sessionInfo],

  ["session.updated", sessionInfo],

  [
    "session.error",
    on(SessionError, async ({ sessions }, { sessionID, error }) => {
      const message = cleanText(error?.data?.message || error?.name || "Session error");
      const state = sessionID ? sessions.get(sessionID) : undefined;
      if (state) {
        clearTimeout(state.completeTimer);
        state.completeTimer = undefined;
        clearStartNotify(state);
        clearIdleReminder(state);
        state.active = false;
        if (state.parentID) {
          scheduleComplete(sessions, state.parentID);
          scheduleIdleReminder(sessions, state.parentID);
        }
      }
      await sendNotify(sessions, { sessionID, type: "error", message });
    }),
  ],

  [
    "session.deleted",
    on(SessionInfo, ({ sessions }, { info }) => {
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
    }),
  ],
]);

function on<T>(schema: z.ZodType<T>, handle: (ctx: Context, props: T) => Promise<void> | void) {
  return async (ctx: Context, properties: unknown) => {
    const result = schema.safeParse(properties);
    if (result.success) await handle(ctx, result.data);
  };
}

function partText(part: V1Part) {
  if (part.type === "text") return part.text;
  if (part.type === "subtask") return part.prompt;
  return "";
}
