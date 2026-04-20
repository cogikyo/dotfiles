const HYPRD = "/home/cullyn/.local/bin/hyprd"

function cleanText(value, max = 512) {
  if (typeof value !== "string") return ""
  const text = value.replace(/\s+/g, " ").trim()
  return text.slice(0, max)
}

async function notify(payload) {
  const proc = Bun.spawn({
    cmd: [HYPRD, "notify", "hook", "opencode", JSON.stringify(payload)],
    env: process.env,
    stdout: "ignore",
    stderr: "ignore",
  })
  await proc.exited
}

export const HyprdNotifyPlugin = async () => {
  const activeSessions = new Set()
  const seenAgentParts = new Map()
  const todoStatuses = new Map()
  const lastAssistantMessages = new Map()
  const lastTodoCompletedAt = new Map()
  let selectedSessionID = ""

  function isSelectedSession(sessionID) {
    return selectedSessionID !== "" && cleanText(sessionID, 128) === selectedSessionID
  }

  async function completeSession(sessionID) {
    if (!isSelectedSession(sessionID)) return
    if (!activeSessions.has(sessionID)) return

    activeSessions.delete(sessionID)

    const lastTodoAt = lastTodoCompletedAt.get(sessionID) || 0
    if (Date.now() - lastTodoAt < 1500) return

    const lastAssistantMessage = cleanText(lastAssistantMessages.get(sessionID) || "")
    await notify({
      type: "complete",
      message: lastAssistantMessage || "Jobs done",
      last_assistant_message: lastAssistantMessage,
    })
  }

  return {
    event: async ({ event }) => {
      if (!event || typeof event.type !== "string") return

      switch (event.type) {
        case "tui.session.select": {
          selectedSessionID = cleanText(event.properties?.sessionID, 128)
          return
        }
        case "message.part.updated": {
          const part = event.properties?.part
          if (!part?.sessionID || !part?.id) return
          if (!isSelectedSession(part.sessionID)) return

          if (part.type === "text" && part?.time?.end) {
            lastAssistantMessages.set(part.sessionID, part.text)
            return
          }

          let seenParts = seenAgentParts.get(part.sessionID)
          if (!seenParts) {
            seenParts = new Set()
            seenAgentParts.set(part.sessionID, seenParts)
          }

          if ((part.type === "subtask" || part.type === "agent") && !seenParts.has(part.id)) {
            seenParts.add(part.id)
            await notify({
              type: "subagent",
              agent_type: cleanText(part.agent || part.name || "Agent", 128),
              message: cleanText(part.description || part.prompt || part.name || "Done"),
            })
          }

          return
        }
        case "session.status": {
          const sessionID = event.properties?.sessionID
          const status = event.properties?.status?.type
          if (!sessionID || typeof status !== "string") return
          if (!isSelectedSession(sessionID)) return

          if ((status === "busy" || status === "retry") && !activeSessions.has(sessionID)) {
            activeSessions.add(sessionID)
            await notify({
              type: "start",
              message: status === "retry" ? "Retrying" : "Working",
            })
            return
          }

          if (status === "idle") {
            await completeSession(sessionID)
          }
          return
        }
        case "session.idle": {
          const sessionID = event.properties?.sessionID
          if (!sessionID) return
          if (!isSelectedSession(sessionID)) return

          await completeSession(sessionID)
          return
        }
        case "permission.asked": {
          const sessionID = event.properties?.sessionID
          if (!isSelectedSession(sessionID)) return
          const permission = cleanText(event.properties?.permission, 128)
          const patterns = Array.isArray(event.properties?.patterns)
            ? cleanText(event.properties.patterns.join(", "), 256)
            : ""
          const message = permission ? (patterns ? `${permission}: ${patterns}` : permission) : "Permission needed"

          await notify({
            type: "permission",
            message,
          })
          return
        }
        case "question.asked": {
          const sessionID = event.properties?.sessionID
          if (!isSelectedSession(sessionID)) return
          const question = cleanText(event.properties?.questions?.[0]?.question)

          await notify({
            type: "question",
            message: question || "Question asked",
          })
          return
        }
        case "todo.updated": {
          const sessionID = event.properties?.sessionID
          const todos = Array.isArray(event.properties?.todos) ? event.properties.todos : null
          if (!sessionID || !todos) return
          if (!isSelectedSession(sessionID)) return

          const previous = todoStatuses.get(sessionID)
          const next = new Map()
          const completed = []

          for (const todo of todos) {
            const content = cleanText(todo?.content)
            const status = cleanText(todo?.status, 32)
            if (!content || !status) continue

            next.set(content, status)
            if (previous && previous.get(content) !== "completed" && status === "completed") {
              completed.push(content)
            }
          }

          todoStatuses.set(sessionID, next)

          for (const content of completed) {
            lastTodoCompletedAt.set(sessionID, Date.now())
            await notify({
              type: "todo-complete",
              message: content,
            })
          }
          return
        }
        case "session.error": {
          const sessionID = event.properties?.sessionID
          if (sessionID && !isSelectedSession(sessionID)) return
          const message = cleanText(event.properties?.error?.message || event.properties?.error?.name || "Session error")

          if (sessionID) {
            activeSessions.delete(sessionID)
          }

          await notify({
            type: "error",
            message,
          })
          return
        }
        case "session.deleted": {
          const sessionID = event.properties?.sessionID
          if (!sessionID) return

          if (isSelectedSession(sessionID)) {
            selectedSessionID = ""
          }
          activeSessions.delete(sessionID)
          seenAgentParts.delete(sessionID)
          todoStatuses.delete(sessionID)
          lastAssistantMessages.delete(sessionID)
          lastTodoCompletedAt.delete(sessionID)
          return
        }
      }
    },
  }
}
