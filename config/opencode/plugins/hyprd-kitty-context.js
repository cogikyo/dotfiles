import fs from "node:fs/promises"

const CONTEXT_PATH = "/tmp/opencode-kitty-context.json"
const LOCK_PATH = "/tmp/opencode-kitty-context.lock"
const WRITE_INTERVAL_MS = 1000
const LOCK_RETRY_MS = 25
const LOCK_TIMEOUT_MS = 1000
const STALE_CONTEXT_MS = 24 * 60 * 60 * 1000

const KITTY_PID = Number(process.env.KITTY_PID) || 0
const KITTY_WINDOW_ID = Number(process.env.KITTY_WINDOW_ID) || 0

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function isSocket(path) {
  try {
    return (await fs.stat(path)).isSocket()
  } catch {
    return false
  }
}

async function withLock(fn) {
  const start = Date.now()
  while (true) {
    try {
      await fs.mkdir(LOCK_PATH, { mode: 0o700 })
      break
    } catch (error) {
      if (error?.code !== "EEXIST" || Date.now() - start > LOCK_TIMEOUT_MS) return fn()
      await sleep(LOCK_RETRY_MS)
    }
  }

  try {
    return await fn()
  } finally {
    await fs.rm(LOCK_PATH, { recursive: true, force: true })
  }
}

async function readContexts() {
  try {
    const parsed = JSON.parse(await fs.readFile(CONTEXT_PATH, "utf8"))
    return parsed && typeof parsed === "object" ? parsed : {}
  } catch {
    return {}
  }
}

async function pruneContexts(contexts) {
  const now = Date.now()
  const sockets = new Map()

  for (const [sessionID, ctx] of Object.entries(contexts)) {
    const pid = Number(ctx?.kitty_pid) || 0
    const updatedAt = Number(ctx?.updated_at) || 0
    if (!pid || !updatedAt || now - updatedAt > STALE_CONTEXT_MS) {
      delete contexts[sessionID]
      continue
    }

    let exists = sockets.get(pid)
    if (exists === undefined) {
      exists = await isSocket(`/tmp/kitty-${pid}`)
      sockets.set(pid, exists)
    }
    if (!exists) delete contexts[sessionID]
  }
}

function isThisPane(ctx) {
  return Number(ctx?.kitty_pid) === KITTY_PID && Number(ctx?.kitty_window_id) === KITTY_WINDOW_ID
}

async function clearPaneContext() {
  if (!KITTY_PID || !KITTY_WINDOW_ID) return

  await withLock(async () => {
    const contexts = await readContexts()
    await pruneContexts(contexts)

    for (const [id, ctx] of Object.entries(contexts)) {
      if (isThisPane(ctx)) delete contexts[id]
    }

    await fs.writeFile(CONTEXT_PATH, JSON.stringify(contexts), { mode: 0o600 })
  })
}

async function writeContext(sessionID) {
  if (!sessionID || !KITTY_PID || !KITTY_WINDOW_ID) return

  await withLock(async () => {
    const contexts = await readContexts()
    await pruneContexts(contexts)

    for (const [id, ctx] of Object.entries(contexts)) {
      if (id !== sessionID && isThisPane(ctx)) delete contexts[id]
    }

    contexts[sessionID] = {
      kitty_pid: KITTY_PID,
      kitty_window_id: KITTY_WINDOW_ID,
      updated_at: Date.now(),
    }

    await fs.writeFile(CONTEXT_PATH, JSON.stringify(contexts), { mode: 0o600 })
  })
}

const tui = async (api) => {
  const sync = () => {
    const current = api.route.current
    if (current.name !== "session") return

    const sessionID = current.params?.sessionID
    if (typeof sessionID !== "string" || sessionID === "") return

    void writeContext(sessionID)
  }

  sync()
  const timer = setInterval(sync, WRITE_INTERVAL_MS)
  const disposers = [
    api.event.on("session.status", () => sync()),
    api.event.on("session.created", () => sync()),
    api.event.on("tui.session.select", (event) => {
      const sessionID = event.properties?.sessionID
      if (typeof sessionID !== "string" || sessionID === "") return

      void writeContext(sessionID)
    }),
    api.event.on("tui.command.execute", (event) => {
      if (event.properties?.command !== "session.new") return

      void clearPaneContext()
      setTimeout(sync, 50)
      setTimeout(sync, 250)
    }),
  ]

  api.lifecycle.onDispose(() => {
    clearInterval(timer)
    for (const dispose of disposers) dispose()
    void clearPaneContext()
  })
}

export default { id: "hyprd-kitty-context", tui }
