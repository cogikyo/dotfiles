// @ts-nocheck -- OpenCode plugin event types are incomplete; keep runtime behavior stable until local event types exist.
import { EMPTY_KITTY_CONTEXT, isSocket, KITTY_CONTEXT_PATH, kittySocketPath } from "./context.ts"

const SOCKET_PATH = "/tmp/hyprd.sock"

const LIMITS = {
  id: 128,
  status: 32,
  message: 512,
  patterns: 256,
}

const TODO_COMPLETE_DEBOUNCE_MS = 1500
const COMPLETE_DEBOUNCE_MS = 500
const PERMISSION_DEBOUNCE_MS = 1500
const NOTIFY_DEDUPE_MS = 1000
const START_TITLE_WAIT_MS = 1200
const START_CONTEXT_RETRY_MS = 500
const NEW_SESSION_START_MESSAGE = "New Session started"
const IDLE_REMINDER_MS = 10 * 60 * 1000
const IDLE_CONTEXT_MAX_AGE_MS = 30 * 1000

const recentNotifies = new Map()

function cleanText(value, max = LIMITS.message) {
  if (typeof value !== "string") return ""
  return value.replace(/\s+/g, " ").trim().slice(0, max)
}

function cleanSessionTitle(value) {
  const title = cleanText(value)
  const normalized = cleanText(title.replace(/^New Session\s+-\s*/i, ""), LIMITS.id)
  return isPlaceholderSessionTitle(normalized) ? "" : normalized
}

function isPlaceholderSessionTitle(value) {
  const title = cleanText(value, LIMITS.id).toLowerCase()
  return !title || title === "new session" || title === "session start info here" || isTimestampTitle(title)
}

function isTimestampTitle(value) {
  return /^\d{4}-\d{2}-\d{2}t\d{2}:\d{2}:\d{2}(?:\.\d+)?z$/i.test(cleanText(value, LIMITS.id))
}

function startMessage(state) {
  const subject = state.lastUserMessage || state.title
  return subject ? `Working on "${subject}"` : NEW_SESSION_START_MESSAGE
}

function cleanPromptText(value) {
  return cleanText(String(value || "").replace(/\[Image\s+\d+\]/gi, " "))
}

function messageRole(value) {
  return cleanText(value?.role || value?.metadata?.role || value?.author?.role || value?.type, LIMITS.status).toLowerCase()
}

function partRole(value) {
  return cleanText(value?.role || value?.author?.role, LIMITS.status).toLowerCase()
}

function messageID(value) {
  return value?.messageID || value?.messageId || value?.metadata?.messageID || value?.metadata?.messageId || value?.message?.id || value?.metadata?.id || value?.id || ""
}

function partMessageID(part, props) {
  return props?.messageID || props?.messageId || props?.message?.id || part?.messageID || part?.messageId || part?.message?.id || ""
}

function messageSessionID(value) {
  return value?.sessionID || value?.sessionId || value?.metadata?.sessionID || value?.metadata?.sessionId || value?.session?.id || ""
}

function textFromMessage(value) {
  if (!value) return ""
  if (typeof value === "string") return cleanPromptText(value)
  if (Array.isArray(value)) return cleanPromptText(value.map(textFromMessage).filter(Boolean).join(" "))
  if (typeof value !== "object") return ""

  for (const key of ["text", "message", "prompt", "input"]) {
    if (typeof value[key] === "string") {
      const text = cleanPromptText(value[key])
      if (text) return text
    }
  }

  for (const key of ["parts", "content", "messages"]) {
    const text = textFromMessage(value[key])
    if (text) return text
  }

  return ""
}

function isUntimedUserTextPart(part) {
  return part?.type === "text" && !part.synthetic && !part.ignored && !part.time && textFromMessage(part)
}

async function kittyContext(sessionID, parentFor) {
  if (!sessionID) return EMPTY_KITTY_CONTEXT
  try {
    const contexts = await Bun.file(KITTY_CONTEXT_PATH).json()

    for (let id = sessionID, seen = new Set(); id && !seen.has(id); id = parentFor?.(id)) {
      seen.add(id)
      const ctx = contexts?.[id]
      const kitty_pid = Number(ctx?.kitty_pid) || 0
      const kitty_window_id = Number(ctx?.kitty_window_id) || 0
      const updated_at = Number(ctx?.updated_at) || 0
      if (!kitty_pid || !kitty_window_id) continue
      if (!(await isSocket(kittySocketPath(kitty_pid)))) continue
      return { kitty_pid, kitty_window_id, updated_at }
    }
  } catch {
  }

  return EMPTY_KITTY_CONTEXT
}

function hasFreshIdleContext(ctx) {
  return ctx.updated_at > 0 && Date.now() - ctx.updated_at <= IDLE_CONTEXT_MAX_AGE_MS
}

async function send(command) {
  let resolveDone
  let response = ""
  const done = new Promise((r) => {
    resolveDone = r
  })
  try {
    await Bun.connect({
      unix: SOCKET_PATH,
      socket: {
        open(socket) {
          socket.write(command)
        },
        data(_socket, data) {
          response += new TextDecoder().decode(data)
        },
        close() {
          resolveDone()
        },
        error() {
          resolveDone()
        },
      },
    })
  } catch {
    resolveDone()
    return false
  }
  await done
  return response.trim() === "ok"
}

function notifyKey(req) {
  return JSON.stringify([
    req.source,
    req.event,
    req.message,
    req.last_assistant_message,
    req.agent_type,
    req.kitty_pid,
    req.kitty_window_id,
  ])
}

function recentEnough(key) {
  const previous = recentNotifies.get(key) || 0
  return Date.now() - previous < NOTIFY_DEDUPE_MS
}

function markNotified(key) {
  const now = Date.now()
  recentNotifies.set(key, now)
  for (const [oldKey, seenAt] of recentNotifies) {
    if (now - seenAt > NOTIFY_DEDUPE_MS) recentNotifies.delete(oldKey)
  }
}

async function notify(payload, parentFor) {
  const ctx = await kittyContext(payload.sessionID, parentFor)
  if (!ctx.kitty_pid || !ctx.kitty_window_id) return false
  if (payload.type === "idle" && !hasFreshIdleContext(ctx)) return false

  const req = {
    source: "opencode",
    event: payload.type,
    message: payload.message ?? "",
    last_assistant_message: payload.last_assistant_message ?? "",
    agent_type: payload.agent_type ?? "",
    kitty_pid: ctx.kitty_pid,
    kitty_window_id: ctx.kitty_window_id,
  }
  const key = notifyKey(req)
  if (recentEnough(key)) return false

  const sent = await send("notify " + JSON.stringify(req))
  if (sent) markNotified(key)
  return sent
}

function newSessionState() {
  return {
    active: false,
    seenAgentParts: new Set(),
    assistantPartText: new Map(),
    todoStatuses: null,
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
  }
}

const server = async () => {
  const sessions = new Map()
  const messageRoles = new Map()
  const messageSessions = new Map()

  function getSession(sessionID) {
    let state = sessions.get(sessionID)
    if (!state) {
      state = newSessionState()
      sessions.set(sessionID, state)
    }
    return state
  }

  function parentFor(sessionID) {
    return sessions.get(sessionID)?.parentID || ""
  }

  function hasActiveDescendant(sessionID, seen = new Set()) {
    if (!sessionID || seen.has(sessionID)) return false
    seen.add(sessionID)

    for (const [id, state] of sessions) {
      if (state.parentID !== sessionID) continue
      if (state.active || state.completeTimer) return true
      if (hasActiveDescendant(id, seen)) return true
    }

    return false
  }

  async function sendNotify(payload) {
    return await notify(payload, parentFor)
  }

  function clearIdleReminder(state) {
    clearTimeout(state.idleTimer)
    state.idleTimer = null
  }

  function clearStartNotify(state) {
    clearTimeout(state.startTimer)
    state.startTimer = null
  }

  async function trySendStartNotify(sessionID, message) {
    const state = sessions.get(sessionID)
    if (!state?.active || state.startNotified) return false

    clearStartNotify(state)
    const sent = await sendNotify({
      sessionID,
      type: "start",
      message: message || startMessage(state),
    })
    if (sent) {
      state.startNotified = true
      return true
    }
    if (state.active) scheduleStartNotify(sessionID, message, START_CONTEXT_RETRY_MS)
    return false
  }

  function scheduleStartNotify(sessionID, message, delay = START_TITLE_WAIT_MS) {
    const state = sessions.get(sessionID)
    if (!state?.active || state.startNotified || state.startTimer) return

    state.startTimer = setTimeout(async () => {
      state.startTimer = null
      await trySendStartNotify(sessionID, message)
    }, delay)
  }

  function scheduleIdleReminder(sessionID) {
    const state = sessions.get(sessionID)
    if (!state || state.parentID || state.active || state.idleTimer || hasActiveDescendant(sessionID)) return

    state.idleTimer = setTimeout(async () => {
      state.idleTimer = null
      if (state.parentID || state.active || hasActiveDescendant(sessionID)) return

      const message = state.lastUserMessage || state.title || state.lastAssistantMessage || "Still idle"
      await sendNotify({
        sessionID,
        type: "idle",
        message,
      })
      scheduleIdleReminder(sessionID)
    }, IDLE_REMINDER_MS)
  }

  function scheduleComplete(sessionID) {
    const state = sessions.get(sessionID)
    if (!state?.active || state.completeTimer) return

    const inactiveAt = Date.now()
    state.completeTimer = setTimeout(async () => {
      state.completeTimer = null
      if (!state.active) return

      if (hasActiveDescendant(sessionID)) return

      state.active = false
      state.inactiveAt = inactiveAt
      clearStartNotify(state)

      if (Date.now() - state.lastTodoCompletedAt < TODO_COMPLETE_DEBOUNCE_MS) {
        scheduleIdleReminder(sessionID)
        if (state.parentID) {
          scheduleComplete(state.parentID)
          scheduleIdleReminder(state.parentID)
        }
        return
      }

      const isSubagent = state.parentID !== ""
      const message = state.lastAssistantMessage || state.title
      await sendNotify({
        sessionID,
        type: isSubagent ? "subagent" : "complete",
        agent_type: isSubagent ? state.title : "",
        message: message || (isSubagent ? "Done" : "Jobs done"),
        last_assistant_message: message,
      })
      scheduleIdleReminder(sessionID)
      if (state.parentID) {
        scheduleComplete(state.parentID)
        scheduleIdleReminder(state.parentID)
      }
    }, COMPLETE_DEBOUNCE_MS)
  }

  function updateAssistantPartText(state, partID, text) {
    if (!partID) return

    const message = cleanText(text)
    if (!message) return

    state.assistantPartText.set(partID, message)
    state.lastAssistantMessage = message
  }

  async function updateUserMessage(sessionID, message) {
    const text = textFromMessage(message)
    if (!text) return

    const state = getSession(sessionID)
    state.lastUserMessage = text
    state.lastUserMessageAt = Date.now()
    if (state.active && !state.startNotified) scheduleStartNotify(sessionID)
  }

  async function handleMessageUpdated(props) {
    const msg = props?.message || props?.info || props
    const msgID = messageID(msg) || messageID(props)
    const role = messageRole(msg) || partRole(props)
    const id = props?.sessionID || props?.sessionId || messageSessionID(msg) || messageSessionID(props)
    if (msgID && role) messageRoles.set(msgID, role)
    if (msgID && id) messageSessions.set(msgID, id)
    if (!id || role !== "user") return

    await updateUserMessage(id, msg)
  }

  const handlers = {
    "message.created": handleMessageUpdated,

    "message.updated": handleMessageUpdated,

    "message.part.delta": async ({ sessionID, partID, field, delta }) => {
      if (!sessionID || !partID || field !== "text" || typeof delta !== "string") return

      const state = getSession(sessionID)
      const text = (state.assistantPartText.get(partID) || "") + delta
      updateAssistantPartText(state, partID, text)
    },

    "message.part.updated": async (props) => {
      const { sessionID, part } = props
      const msgID = partMessageID(part, props)
      const id = sessionID || part?.sessionID || messageSessions.get(msgID)
      if (!id || !part) return

      const state = getSession(id)
      const role = partRole(part) || messageRole(props?.message) || partRole(props) || messageRoles.get(msgID)

      if (part.type === "text" && (role === "user" || (!role && isUntimedUserTextPart(part)))) {
        await updateUserMessage(id, part)
        return
      }

      if (part.id && part.type === "text" && (role === "assistant" || (!role && part?.time?.end))) {
        updateAssistantPartText(state, part.id, part.text)
        return
      }

      if (part.id && (part.type === "subtask" || part.type === "agent") && !state.seenAgentParts.has(part.id)) {
        state.seenAgentParts.add(part.id)
        await sendNotify({
          sessionID: id,
          type: "subagent",
          agent_type: cleanText(part.agent || part.name || "Agent", LIMITS.id),
          message: cleanText(part.description || part.prompt || part.name || "Done"),
        })
      }
    },

    "session.status": async ({ sessionID, status }) => {
      if (!sessionID || typeof status?.type !== "string") return

      const state = getSession(sessionID)
      const type = status.type

      if (type === "busy" || type === "retry") {
        clearTimeout(state.completeTimer)
        state.completeTimer = null
        clearIdleReminder(state)

        if (!state.active) {
          state.active = true
          state.startNotified = false
          if (type !== "retry" && state.lastUserMessageAt <= state.inactiveAt) state.lastUserMessage = ""
          if (type === "retry") await trySendStartNotify(sessionID, "Retrying")
          else if (state.lastUserMessage) scheduleStartNotify(sessionID)
          else scheduleStartNotify(sessionID)
        }
        return
      }

      if (type === "idle") {
        scheduleComplete(sessionID)
      }
    },

    "session.idle": async ({ sessionID }) => {
      if (!sessionID) return
      scheduleComplete(sessionID)
    },

    "permission.asked": async ({ sessionID, permission, patterns, title, pattern }) => {
      const perm = cleanText(permission || title, LIMITS.id)
      const rawPatterns = patterns ?? pattern
      const pats = Array.isArray(rawPatterns)
        ? cleanText(rawPatterns.join(", "), LIMITS.patterns)
        : typeof rawPatterns === "string"
          ? cleanText(rawPatterns, LIMITS.patterns)
          : ""
      const message = perm ? (pats ? `${perm}: ${pats}` : perm) : "Permission needed"

      if (sessionID) {
        const state = getSession(sessionID)
        const now = Date.now()
        if (state.lastPermissionMessage === message && now - state.lastPermissionAt < PERMISSION_DEBOUNCE_MS) return
        state.lastPermissionMessage = message
        state.lastPermissionAt = now
      }

      await sendNotify({ sessionID, type: "permission", message })
    },

    "permission.updated": async (props) => {
      await handlers["permission.asked"](props)
    },

    "question.asked": async ({ sessionID, questions }) => {
      const first = Array.isArray(questions) ? questions[0] : null
      const header = cleanText(first?.header, LIMITS.id)
      const question = cleanText(first?.question)
      const message = header ? (question ? `${header}: ${question}` : header) : question || "Question asked"
      await sendNotify({ sessionID, type: "question", message })
    },

    "todo.updated": async ({ sessionID, todos }) => {
      if (!sessionID || !Array.isArray(todos)) return

      const state = getSession(sessionID)
      const previous = state.todoStatuses
      const next = new Map()
      const completed = []

      for (const todo of todos) {
        const content = cleanText(todo?.content)
        const status = cleanText(todo?.status, LIMITS.status)
        if (!content || !status) continue

        next.set(content, status)
        if (previous && previous.get(content) !== "completed" && status === "completed") {
          completed.push(content)
        }
      }

      state.todoStatuses = next

      if (completed.length > 0) {
        state.lastTodoCompletedAt = Date.now()
        await Promise.all(completed.map((message) => sendNotify({ sessionID, type: "todo-complete", message })))
      }
    },

    "session.created": async ({ sessionID, info }) => {
      const id = sessionID || info?.id
      if (!id) return
      const state = getSession(id)
      state.parentID = typeof info?.parentID === "string" ? info.parentID : ""
      if (state.parentID) clearIdleReminder(state)
      const title = cleanSessionTitle(info?.title)
      if (title) state.title = title
      if (state.active && state.title && !state.startNotified) scheduleStartNotify(id)
    },

    "session.updated": async ({ sessionID, info }) => {
      const id = sessionID || info?.id
      if (!id) return
      const state = getSession(id)
      state.parentID = typeof info?.parentID === "string" ? info.parentID : ""
      if (state.parentID) clearIdleReminder(state)
      const title = cleanSessionTitle(info?.title)
      if (title) state.title = title
      if (state.active && state.title && !state.startNotified) scheduleStartNotify(id)
    },

    "session.error": async ({ sessionID, error }) => {
      const message = cleanText(error?.data?.message || error?.name || "Session error")
      if (sessionID) {
        const state = sessions.get(sessionID)
        if (state) {
          clearTimeout(state.completeTimer)
          state.completeTimer = null
          clearStartNotify(state)
          clearIdleReminder(state)
          state.active = false
          if (state.parentID) {
            scheduleComplete(state.parentID)
            scheduleIdleReminder(state.parentID)
          }
        }
      }
      await sendNotify({ sessionID, type: "error", message })
    },

    "session.deleted": ({ info }) => {
      if (!info?.id) return
      const state = sessions.get(info.id)
      if (state) {
        clearTimeout(state.completeTimer)
        clearStartNotify(state)
        clearIdleReminder(state)
        if (state.parentID) {
          scheduleComplete(state.parentID)
          scheduleIdleReminder(state.parentID)
        }
      }
      sessions.delete(info.id)
    },
  }

  return {
    "chat.message": async (input, output) => {
      const sessionID = input?.sessionID || output?.message?.sessionID
      if (!sessionID) return

      await updateUserMessage(sessionID, output?.parts || output?.message)
    },

    event: async ({ event }) => {
      if (!event || typeof event.type !== "string") return
      const handler = handlers[event.type]
      if (handler) await handler(event.properties ?? {})
    },
  }
}

export default { id: "hyprd-notify", server }
