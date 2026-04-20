const SOCKET_PATH = "/tmp/hyprd.sock"

const LIMITS = {
  id: 128,
  status: 32,
  message: 512,
  patterns: 256,
}

const TODO_COMPLETE_DEBOUNCE_MS = 1500

const KITTY_PID = Number(process.env.KITTY_PID) || 0
const KITTY_WINDOW_ID = Number(process.env.KITTY_WINDOW_ID) || 0

function cleanText(value, max = LIMITS.message) {
  if (typeof value !== "string") return ""
  return value.replace(/\s+/g, " ").trim().slice(0, max)
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

async function notify(payload) {
  const req = {
    source: "opencode",
    event: payload.type,
    message: payload.message ?? "",
    last_assistant_message: payload.last_assistant_message ?? "",
    agent_type: payload.agent_type ?? "",
    kitty_pid: KITTY_PID,
    kitty_window_id: KITTY_WINDOW_ID,
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
  }
}

export const HyprdNotifyPlugin = async () => {
  const sessions = new Map()
  let selectedSessionID = ""
  let hasExplicitSelection = false

  function getSession(sessionID) {
    let state = sessions.get(sessionID)
    if (!state) {
      state = newSessionState()
      sessions.set(sessionID, state)
    }
    return state
  }

  function isSelected(sessionID) {
    if (selectedSessionID === "" || !hasExplicitSelection) return true
    return sessionID === selectedSessionID
  }

  async function completeSession(sessionID) {
    if (!isSelected(sessionID)) return
    const state = sessions.get(sessionID)
    if (!state?.active) return

    state.active = false
    if (Date.now() - state.lastTodoCompletedAt < TODO_COMPLETE_DEBOUNCE_MS) return

    const message = state.lastAssistantMessage
    await notify({
      type: "complete",
      message: message || "Jobs done",
      last_assistant_message: message,
    })
  }

  const handlers = {
    "tui.session.select": ({ sessionID }) => {
      selectedSessionID = cleanText(sessionID, LIMITS.id)
      hasExplicitSelection = selectedSessionID !== ""
    },

    "message.part.updated": async ({ part }) => {
      if (!part?.sessionID || !part?.id) return
      if (!isSelected(part.sessionID)) return

      const state = getSession(part.sessionID)

      if (part.type === "text" && part?.time?.end) {
        state.lastAssistantMessage = cleanText(part.text)
        return
      }

      if ((part.type === "subtask" || part.type === "agent") && !state.seenAgentParts.has(part.id)) {
        state.seenAgentParts.add(part.id)
        await notify({
          type: "subagent",
          agent_type: cleanText(part.agent || part.name || "Agent", LIMITS.id),
          message: cleanText(part.description || part.prompt || part.name || "Done"),
        })
      }
    },

    "session.status": async ({ sessionID, status }) => {
      if (!sessionID || typeof status?.type !== "string") return
      if (!isSelected(sessionID)) return

      const state = getSession(sessionID)
      const type = status.type

      if ((type === "busy" || type === "retry") && !state.active) {
        state.active = true
        await notify({
          type: "start",
          message: type === "retry" ? "Retrying" : "Working",
        })
        return
      }

      if (type === "idle") {
        await completeSession(sessionID)
      }
    },

    "session.idle": async ({ sessionID }) => {
      if (!sessionID || !isSelected(sessionID)) return
      await completeSession(sessionID)
    },

    "permission.asked": async ({ sessionID, permission, patterns }) => {
      if (!isSelected(sessionID)) return
      const perm = cleanText(permission, LIMITS.id)
      const pats = Array.isArray(patterns) ? cleanText(patterns.join(", "), LIMITS.patterns) : ""
      const message = perm ? (pats ? `${perm}: ${pats}` : perm) : "Permission needed"
      await notify({ type: "permission", message })
    },

    "question.asked": async ({ sessionID, questions }) => {
      if (!isSelected(sessionID)) return
      const question = cleanText(questions?.[0]?.question)
      await notify({ type: "question", message: question || "Question asked" })
    },

    "todo.updated": async ({ sessionID, todos }) => {
      if (!sessionID || !Array.isArray(todos) || !isSelected(sessionID)) return

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
        await Promise.all(completed.map((message) => notify({ type: "todo-complete", message })))
      }
    },

    "session.error": async ({ sessionID, error }) => {
      if (sessionID && !isSelected(sessionID)) return
      const message = cleanText(error?.message || error?.name || "Session error")
      if (sessionID) {
        const state = sessions.get(sessionID)
        if (state) state.active = false
      }
      await notify({ type: "error", message })
    },

    "session.deleted": ({ sessionID }) => {
      if (!sessionID) return
      if (selectedSessionID === sessionID) {
        selectedSessionID = ""
        hasExplicitSelection = false
      }
      sessions.delete(sessionID)
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
