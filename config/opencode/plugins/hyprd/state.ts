import { cleanText, LIMITS, promptText, startMessage } from "./payload.ts";
import { notify, type Notice } from "./route.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Session state                                                                                 │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

/** Process-local notification state for one session. */
export type State = {
  active: boolean;
  seenAgentParts: Set<string>;
  /** Assistant text by part ID, including incoming deltas. */
  assistantPartText: Map<string, string>;
  /** Missing until the first todo update, which does not emit completions. */
  todoStatuses?: Map<string, string>;
  hasOpenTodos: boolean;
  lastAssistantMessage: string;
  lastUserMessage: string;
  /** Preserves user text that arrives after the previous run was scheduled to end. */
  lastUserMessageAt: number;
  inactiveAt: number;
  /** Recent todo completion suppresses a redundant complete notice. */
  lastTodoCompletedAt: number;
  completeTimer?: Timer;
  startTimer?: Timer;
  startNotified: boolean;
  idleTimer?: Timer;
  /** Together with the message, suppresses duplicate permission notices. */
  lastPermissionAt: number;
  lastPermissionMessage: string;
  parentID: string;
  title: string;
};

export type Sessions = Map<string, State>;

type Timer = ReturnType<typeof setTimeout>;

/** Gets or creates notification state for a session. */
export function getSession(sessions: Sessions, sessionID: string) {
  let state = sessions.get(sessionID);
  if (!state) {
    state = newSessionState();
    sessions.set(sessionID, state);
  }
  return state;
}

function newSessionState(): State {
  return {
    active: false,
    seenAgentParts: new Set(),
    assistantPartText: new Map(),
    hasOpenTodos: false,
    lastAssistantMessage: "",
    lastUserMessage: "",
    lastUserMessageAt: 0,
    inactiveAt: 0,
    lastTodoCompletedAt: 0,
    startNotified: false,
    lastPermissionAt: 0,
    lastPermissionMessage: "",
    parentID: "",
    title: "",
  };
}

// ├─ Start notices ───────────────────────────────────────────────────────────────────────────────┤

const START_TITLE_WAIT_MS = 1200;
const START_CONTEXT_RETRY_MS = 500;

/** Schedules one start notice for an active, unnotified session. */
export function scheduleStartNotify(
  sessions: Sessions,
  sessionID: string,
  message?: string,
  delay = START_TITLE_WAIT_MS,
) {
  const state = sessions.get(sessionID);
  if (!state?.active || state.startNotified || state.startTimer) return;

  state.startTimer = setTimeout(() => {
    state.startTimer = undefined;
    void trySendStartNotify(sessions, sessionID, message);
  }, delay);
}

/** Sends the start notice and retries after 500ms if hyprd cannot accept it while the session is active. */
export async function trySendStartNotify(sessions: Sessions, sessionID: string, message?: string) {
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

export function clearStartNotify(state: State) {
  clearTimeout(state.startTimer);
  state.startTimer = undefined;
}

/** Saves user text for notices and schedules a start when needed. */
export function updateUserMessage(sessions: Sessions, sessionID: string, message: string) {
  const text = promptText(message);
  if (!text) return;

  const state = getSession(sessions, sessionID);
  state.lastUserMessage = text;
  state.lastUserMessageAt = Date.now();
  if (state.active && !state.startNotified) scheduleStartNotify(sessions, sessionID);
}

/** Keeps the latest usable assistant text for a part and the session. */
export function updateAssistantPartText(state: State, partID: string, text: string) {
  if (!partID) return;

  const message = cleanText(text);
  if (!message) return;

  state.assistantPartText.set(partID, message);
  state.lastAssistantMessage = message;
}

// ├─ Completion ──────────────────────────────────────────────────────────────────────────────────┤

const TODO_COMPLETE_DEBOUNCE_MS = 1500;
const COMPLETE_DEBOUNCE_MS = 500;

/** Debounces completion for an active session, leaving any existing timer in place. */
export function scheduleComplete(sessions: Sessions, sessionID: string) {
  const state = sessions.get(sessionID);
  if (!state?.active || state.completeTimer) return;

  const inactiveAt = Date.now();
  state.completeTimer = setTimeout(() => {
    state.completeTimer = undefined;
    void complete(sessions, sessionID, state, inactiveAt);
  }, COMPLETE_DEBOUNCE_MS);
}

// Open todos or running descendants defer completion; recent todo notices suppress the complete notice.
async function complete(sessions: Sessions, sessionID: string, state: State, inactiveAt: number) {
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

/** Tracks todos by content and returns newly completed items; the first update establishes a baseline. */
export function applyTodos(state: State, todos: { content: string; status: string }[]) {
  const previous = state.todoStatuses;
  const next = new Map<string, string>();
  const completed: string[] = [];
  let open = 0;

  for (const todo of todos) {
    const content = cleanText(todo.content);
    const status = cleanText(todo.status, LIMITS.status);
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

// ├─ Idle reminders ──────────────────────────────────────────────────────────────────────────────┤

const IDLE_REMINDER_MS = 10 * 60 * 1000;

/** Repeats idle reminders for an inactive root without running descendants. */
export function scheduleIdleReminder(sessions: Sessions, sessionID: string) {
  const state = sessions.get(sessionID);
  if (!state || state.parentID || state.active || state.idleTimer || hasActiveDescendant(sessions, sessionID)) return;

  state.idleTimer = setTimeout(() => {
    state.idleTimer = undefined;
    void remindIdle(sessions, sessionID, state);
  }, IDLE_REMINDER_MS);
}

export function clearIdleReminder(state: State) {
  clearTimeout(state.idleTimer);
  state.idleTimer = undefined;
}

async function remindIdle(sessions: Sessions, sessionID: string, state: State) {
  if (state.parentID || state.active || hasActiveDescendant(sessions, sessionID)) return;

  const message = state.lastUserMessage || state.title || state.lastAssistantMessage || "Still idle";
  await sendNotify(sessions, {
    sessionID,
    type: "idle",
    message,
  });
  scheduleIdleReminder(sessions, sessionID);
}

// Completion waits for descendants still active or waiting on their completion timer.
function hasActiveDescendant(sessions: Sessions, sessionID: string, seen = new Set<string>()): boolean {
  if (!sessionID || seen.has(sessionID)) return false;
  seen.add(sessionID);

  for (const [id, state] of sessions) {
    if (state.parentID !== sessionID) continue;
    if (state.active || state.completeTimer) return true;
    if (hasActiveDescendant(sessions, id, seen)) return true;
  }

  return false;
}

// ├─ Delivery ────────────────────────────────────────────────────────────────────────────────────┤

export async function sendNotify(sessions: Sessions, payload: Notice) {
  return await notify(payload, (sessionID) => parentFor(sessions, sessionID));
}

function parentFor(sessions: Sessions, sessionID: string) {
  return sessions.get(sessionID)?.parentID || "";
}
