// @ts-nocheck -- OpenCode plugin event types are incomplete; keep runtime behavior stable until local event types exist.
import fs from "node:fs/promises"
import {
  ensureKittyContextDir,
  isSocket,
  KITTY_CONTEXT_LOCK_PATH,
  KITTY_CONTEXT_PATH,
  kittySocketPath,
  STALE_CONTEXT_MS,
} from "./context.ts"

const WRITE_INTERVAL_MS = 1000
const STALE_BUSY_STATUS_MS = 30_000
const LOCK_RETRY_MS = 25
const LOCK_TIMEOUT_MS = 1000

const KITTY_PID = Number(process.env.KITTY_PID) || 0
const KITTY_WINDOW_ID = Number(process.env.KITTY_WINDOW_ID) || 0

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function withLock(fn) {
  await ensureKittyContextDir()
  const start = Date.now()
  while (true) {
    try {
      await fs.mkdir(KITTY_CONTEXT_LOCK_PATH, { mode: 0o700 })
      break
    } catch (error) {
      if (error?.code !== "EEXIST" || Date.now() - start > LOCK_TIMEOUT_MS) return fn()
      await sleep(LOCK_RETRY_MS)
    }
  }

  try {
    return await fn()
  } finally {
    await fs.rm(KITTY_CONTEXT_LOCK_PATH, { recursive: true, force: true })
  }
}

async function readContexts() {
  try {
    const parsed = JSON.parse(await fs.readFile(KITTY_CONTEXT_PATH, "utf8"))
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
      exists = await isSocket(kittySocketPath(pid))
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

    await fs.writeFile(KITTY_CONTEXT_PATH, JSON.stringify(contexts), { mode: 0o600 })
  })
}

const sessionStatuses = new Map()

function currentAgent(api, sessionID) {
  const messages = api.state.session.messages(sessionID)
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index]
    if ("agent" in message && message.agent) return message.agent
  }
  return undefined
}

function normalizeStatus(status) {
  let type = ""
  if (typeof status?.type === "string") type = status.type
  else if (typeof status === "string") type = status

  if (type === "busy" || type === "retry") return "busy"
  if (type === "idle") return "idle"
  return type
}

function rememberStatus(sessionID, status, now = Date.now()) {
  if (typeof sessionID !== "string" || sessionID === "") return undefined

  const type = normalizeStatus(status)
  if (!type) return undefined

  const entry = { type, updatedAt: now }
  sessionStatuses.set(sessionID, entry)
  return entry
}

function stateStatus(api, sessionID) {
  try {
    return normalizeStatus(api.state?.session?.status?.(sessionID))
  } catch {
    return ""
  }
}

function currentStatus(api, sessionID) {
  const now = Date.now()
  const observed = stateStatus(api, sessionID)
  if (observed) return rememberStatus(sessionID, observed, now)

  const remembered = sessionStatuses.get(sessionID)
  if (!remembered) return { type: "idle", updatedAt: now }
  if (remembered.type === "busy" && now - remembered.updatedAt > STALE_BUSY_STATUS_MS) {
    return rememberStatus(sessionID, "idle", now)
  }
  return remembered
}

async function writeContext(api, sessionID) {
  if (!sessionID || !KITTY_PID || !KITTY_WINDOW_ID) return

  await withLock(async () => {
    const contexts = await readContexts()
    await pruneContexts(contexts)

    for (const [id, ctx] of Object.entries(contexts)) {
      if (id !== sessionID && isThisPane(ctx)) delete contexts[id]
    }

    const agent = currentAgent(api, sessionID)
    const status = currentStatus(api, sessionID)
    contexts[sessionID] = {
      kitty_pid: KITTY_PID,
      kitty_window_id: KITTY_WINDOW_ID,
      updated_at: Date.now(),
      status_updated_at: status.updatedAt,
      status: status.type,
      ...(agent ? { agent } : {}),
    }

    await fs.writeFile(KITTY_CONTEXT_PATH, JSON.stringify(contexts), { mode: 0o600 })
  })
}

const tui = async (api) => {
  const sync = () => {
    const current = api.route.current
    if (current.name !== "session") return

    const sessionID = current.params?.sessionID
    if (typeof sessionID !== "string" || sessionID === "") return

    void writeContext(api, sessionID)
  }

  sync()
  const timer = setInterval(sync, WRITE_INTERVAL_MS)
  const disposers = [
    api.event.on("session.status", (event) => {
      rememberStatus(event.properties?.sessionID, event.properties?.status)
      sync()
    }),
    api.event.on("session.idle", (event) => {
      rememberStatus(event.properties?.sessionID, "idle")
      sync()
    }),
    api.event.on("session.created", () => sync()),
    api.event.on("tui.session.select", (event) => {
      const sessionID = event.properties?.sessionID
      if (typeof sessionID !== "string" || sessionID === "") return

      void writeContext(api, sessionID)
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
