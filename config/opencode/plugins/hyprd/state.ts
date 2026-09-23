// @ts-nocheck -- OpenCode plugin event types are incomplete; keep runtime behavior stable until local event types exist.
import { cleanText, LIMITS, startMessage, textFromMessage } from "./payload.ts";
import { notify } from "./route.ts";

const TODO_COMPLETE_DEBOUNCE_MS = 1500;
const COMPLETE_DEBOUNCE_MS = 500;
const START_TITLE_WAIT_MS = 1200;
const START_CONTEXT_RETRY_MS = 500;
const IDLE_REMINDER_MS = 10 * 60 * 1000;

function newSessionState() {
  return {
    active: false,
    seenAgentParts: new Set(),
    assistantPartText: new Map(),
    todoStatuses: null,
    hasOpenTodos: false,
    lastAssistantMessage: "",
    lastUserMessage: "",
    lastUserMessageAt: 0,
    inactiveAt: 0,
    lastTodoCompletedAt: 0,
    completeTimer: null,
    startTimer: null,
    startNotified: false,
    idleTimer: null,
    lastPermissionAt: 0,
    lastPermissionMessage: "",
    parentID: "",
    title: "",
  };
}

export function clearIdleReminder(state) {
  clearTimeout(state.idleTimer);
  state.idleTimer = null;
}

export function clearStartNotify(state) {
  clearTimeout(state.startTimer);
  state.startTimer = null;
}

export function updateAssistantPartText(state, partID, text) {
  if (!partID) return;

  const message = cleanText(text);
  if (!message) return;

  state.assistantPartText.set(partID, message);
  state.lastAssistantMessage = message;
}

export function applyTodos(state, todos) {
  const previous = state.todoStatuses;
  const next = new Map();
  const completed = [];
  let open = 0;

  for (const todo of todos) {
    const content = cleanText(todo?.content);
    const status = cleanText(todo?.status, LIMITS.status);
    if (!content || !status) continue;

    next.set(content, status);
    if (status === "pending" || status === "in_progress") open++;
    if (previous && previous.get(content) !== "completed" && status === "completed") {
      completed.push(content);
    }
  }

  state.todoStatuses = next;
  state.hasOpenTodos = open > 0;
  return completed;
}

export function getSession(sessions, sessionID) {
  let state = sessions.get(sessionID);
  if (!state) {
    state = newSessionState();
    sessions.set(sessionID, state);
  }
  return state;
}

function parentFor(sessions, sessionID) {
  return sessions.get(sessionID)?.parentID || "";
}

function hasActiveDescendant(sessions, sessionID, seen = new Set()) {
  if (!sessionID || seen.has(sessionID)) return false;
  seen.add(sessionID);

  for (const [id, state] of sessions) {
    if (state.parentID !== sessionID) continue;
    if (state.active || state.completeTimer) return true;
    if (hasActiveDescendant(sessions, id, seen)) return true;
  }

  return false;
}

export async function sendNotify(sessions, payload) {
  return await notify(payload, (sessionID) => parentFor(sessions, sessionID));
}

export async function trySendStartNotify(sessions, sessionID, message) {
  const state = sessions.get(sessionID);
  if (!state?.active || state.startNotified) return false;

  clearStartNotify(state);
  const sent = await sendNotify(sessions, {
    sessionID,
    type: "start",
    message: message || startMessage(state),
  });
  if (sent) {
    state.startNotified = true;
    return true;
  }
  if (state.active) scheduleStartNotify(sessions, sessionID, message, START_CONTEXT_RETRY_MS);
  return false;
}

export function scheduleStartNotify(sessions, sessionID, message, delay = START_TITLE_WAIT_MS) {
  const state = sessions.get(sessionID);
  if (!state?.active || state.startNotified || state.startTimer) return;

  state.startTimer = setTimeout(() => {
    state.startTimer = null;
    void trySendStartNotify(sessions, sessionID, message);
  }, delay);
}

export function scheduleIdleReminder(sessions, sessionID) {
  const state = sessions.get(sessionID);
  if (!state || state.parentID || state.active || state.idleTimer || hasActiveDescendant(sessions, sessionID)) return;

  state.idleTimer = setTimeout(() => {
    state.idleTimer = null;
    void remindIdle(sessions, sessionID, state);
  }, IDLE_REMINDER_MS);
}

async function remindIdle(sessions, sessionID, state) {
  if (state.parentID || state.active || hasActiveDescendant(sessions, sessionID)) return;

  const message = state.lastUserMessage || state.title || state.lastAssistantMessage || "Still idle";
  await sendNotify(sessions, {
    sessionID,
    type: "idle",
    message,
  });
  scheduleIdleReminder(sessions, sessionID);
}

export function scheduleComplete(sessions, sessionID) {
  const state = sessions.get(sessionID);
  if (!state?.active || state.completeTimer) return;

  const inactiveAt = Date.now();
  state.completeTimer = setTimeout(() => {
    state.completeTimer = null;
    void complete(sessions, sessionID, state, inactiveAt);
  }, COMPLETE_DEBOUNCE_MS);
}

async function complete(sessions, sessionID, state, inactiveAt) {
  if (!state.active) return;

  if (state.hasOpenTodos || hasActiveDescendant(sessions, sessionID)) return;

  state.active = false;
  state.inactiveAt = inactiveAt;
  clearStartNotify(state);

  if (Date.now() - state.lastTodoCompletedAt < TODO_COMPLETE_DEBOUNCE_MS) {
    scheduleIdleReminder(sessions, sessionID);
    if (state.parentID) {
      scheduleComplete(sessions, state.parentID);
      scheduleIdleReminder(sessions, state.parentID);
    }
    return;
  }

  const isSubagent = state.parentID !== "";
  const message = state.lastAssistantMessage || state.title;
  await sendNotify(sessions, {
    sessionID,
    type: isSubagent ? "subagent" : "complete",
    agent_type: isSubagent ? state.title : "",
    message: message || (isSubagent ? "Done" : "Jobs done"),
    last_assistant_message: message,
  });
  scheduleIdleReminder(sessions, sessionID);
  if (state.parentID) {
    scheduleComplete(sessions, state.parentID);
    scheduleIdleReminder(sessions, state.parentID);
  }
}

export async function updateUserMessage(sessions, sessionID, message) {
  const text = textFromMessage(message);
  if (!text) return;

  const state = getSession(sessions, sessionID);
  state.lastUserMessage = text;
  state.lastUserMessageAt = Date.now();
  if (state.active && !state.startNotified) scheduleStartNotify(sessions, sessionID);
}
