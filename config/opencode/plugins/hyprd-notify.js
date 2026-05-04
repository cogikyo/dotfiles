import fs from "node:fs/promises"

const SOCKET_PATH = "/tmp/hyprd.sock"
const KITTY_CONTEXT_PATH = "/tmp/opencode-kitty-context.json"

const LIMITS = {
  id: 128,
  status: 32,
  message: 512,
  patterns: 256,
}

const TODO_COMPLETE_DEBOUNCE_MS = 1500
const COMPLETE_DEBOUNCE_MS = 500
const IDLE_REMINDER_MS = 10 * 60 * 1000

function cleanText(value, max = LIMITS.message) {
  if (typeof value !== "string") return ""
  return value.replace(/\s+/g, " ").trim().slice(0, max)
}

async function kittyContext(sessionID, parentFor) {
  if (!sessionID) return { kitty_pid: 0, kitty_window_id: 0 }
  try {
    const contexts = await Bun.file(KITTY_CONTEXT_PATH).json()

    for (let id = sessionID, seen = new Set(); id && !seen.has(id); id = parentFor?.(id)) {
      seen.add(id)
      const ctx = contexts?.[id]
      const kitty_pid = Number(ctx?.kitty_pid) || 0
      const kitty_window_id = Number(ctx?.kitty_window_id) || 0
      if (!kitty_pid || !kitty_window_id) continue
      if (!(await isSocket(`/tmp/kitty-${kitty_pid}`))) continue
      return { kitty_pid, kitty_window_id }
    }
  } catch {
  }

  return { kitty_pid: 0, kitty_window_id: 0 }
}

async function isSocket(path) {
  try {
    return (await fs.stat(path)).isSocket()
  } catch {
    return false
  }
}

async function send(command) {
  let resolveDone
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
        data() {},
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
    return
  }
  await done
}

async function notify(payload, parentFor) {
  const ctx = await kittyContext(payload.sessionID, parentFor)
  if (payload.type === "idle" && (!ctx.kitty_pid || !ctx.kitty_window_id)) return

  const req = {
    source: "opencode",
    event: payload.type,
    message: payload.message ?? "",
    last_assistant_message: payload.last_assistant_message ?? "",
    agent_type: payload.agent_type ?? "",
    kitty_pid: ctx.kitty_pid,
    kitty_window_id: ctx.kitty_window_id,
  }
  await send("notify " + JSON.stringify(req))
}

function newSessionState() {
  return {
    active: false,
    seenAgentParts: new Set(),
    todoStatuses: null,
    lastAssistantMessage: "",
    lastTodoCompletedAt: 0,
    completeTimer: null,
    idleTimer: null,
    parentID: "",
    title: "",
  }
}

const server = async () => {
  const sessions = new Map()

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

  async function sendNotify(payload) {
    await notify(payload, parentFor)
  }

  function clearIdleReminder(state) {
    clearTimeout(state.idleTimer)
    state.idleTimer = null
  }

  function scheduleIdleReminder(sessionID) {
    const state = sessions.get(sessionID)
    if (!state || state.active || state.idleTimer) return

    state.idleTimer = setTimeout(async () => {
      state.idleTimer = null
      if (state.active) return

      const message = state.lastAssistantMessage || state.title || "Still idle"
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

    state.completeTimer = setTimeout(async () => {
      state.completeTimer = null
      if (!state.active) return
      state.active = false

      if (Date.now() - state.lastTodoCompletedAt < TODO_COMPLETE_DEBOUNCE_MS) {
        scheduleIdleReminder(sessionID)
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
    }, COMPLETE_DEBOUNCE_MS)
  }

  const handlers = {
    "message.part.updated": async ({ part }) => {
      if (!part?.sessionID || !part?.id) return

      const state = getSession(part.sessionID)

      if (part.type === "text" && part?.time?.end) {
        state.lastAssistantMessage = cleanText(part.text)
        return
      }

      if ((part.type === "subtask" || part.type === "agent") && !state.seenAgentParts.has(part.id)) {
        state.seenAgentParts.add(part.id)
        await sendNotify({
          sessionID: part.sessionID,
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
          await sendNotify({
            sessionID,
            type: "start",
            message: type === "retry" ? "Retrying" : "Working",
          })
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

    "session.created": ({ sessionID, info }) => {
      const id = sessionID || info?.id
      if (!id) return
      const state = getSession(id)
      state.parentID = typeof info?.parentID === "string" ? info.parentID : ""
      state.title = cleanText(info?.title, LIMITS.id)
    },

    "session.updated": ({ sessionID, info }) => {
      const id = sessionID || info?.id
      if (!id) return
      const state = getSession(id)
      state.parentID = typeof info?.parentID === "string" ? info.parentID : ""
      state.title = cleanText(info?.title, LIMITS.id)
    },

    "session.error": async ({ sessionID, error }) => {
      const message = cleanText(error?.data?.message || error?.name || "Session error")
      if (sessionID) {
        const state = sessions.get(sessionID)
        if (state) {
          clearTimeout(state.completeTimer)
          state.completeTimer = null
          clearIdleReminder(state)
          state.active = false
        }
      }
      await sendNotify({ sessionID, type: "error", message })
    },

    "session.deleted": ({ info }) => {
      if (!info?.id) return
      const state = sessions.get(info.id)
      if (state) {
        clearTimeout(state.completeTimer)
        clearIdleReminder(state)
      }
      sessions.delete(info.id)
    },
  }

  return {
    event: async ({ event }) => {
      if (!event || typeof event.type !== "string") return
      const handler = handlers[event.type]
      if (handler) await handler(event.properties ?? {})
    },
  }
}

export default { id: "hyprd-notify", server }
