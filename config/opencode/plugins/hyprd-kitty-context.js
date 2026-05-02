import fs from "node:fs/promises"

const CONTEXT_PATH = "/tmp/opencode-kitty-context.json"
const WRITE_INTERVAL_MS = 1000

const KITTY_PID = Number(process.env.KITTY_PID) || 0
const KITTY_WINDOW_ID = Number(process.env.KITTY_WINDOW_ID) || 0

async function writeContext(sessionID) {
  if (!sessionID || !KITTY_PID || !KITTY_WINDOW_ID) return

  let contexts = {}
  try {
    contexts = JSON.parse(await fs.readFile(CONTEXT_PATH, "utf8"))
  } catch {}

  contexts[sessionID] = {
    kitty_pid: KITTY_PID,
    kitty_window_id: KITTY_WINDOW_ID,
    updated_at: Date.now(),
  }

  await fs.writeFile(CONTEXT_PATH, JSON.stringify(contexts), { mode: 0o600 })
}

const tui = async (api) => {
  let lastSessionID = ""

  const sync = () => {
    const current = api.route.current
    if (current.name !== "session") return

    const sessionID = current.params?.sessionID
    if (typeof sessionID !== "string" || sessionID === "") return

    lastSessionID = sessionID
    void writeContext(sessionID)
  }

  sync()
  const timer = setInterval(sync, WRITE_INTERVAL_MS)
  api.lifecycle.onDispose(() => clearInterval(timer))

  api.event.on("session.status", (event) => {
    const sessionID = event?.properties?.sessionID ?? event?.sessionID
    if (typeof sessionID === "string" && sessionID !== lastSessionID) {
      void writeContext(sessionID)
    }
  })
}

export default { id: "hyprd-kitty-context", tui }
