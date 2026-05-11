import fs from "node:fs/promises"
import path from "node:path"

const uid = typeof process.getuid === "function" ? process.getuid() : "user"

export const KITTY_CONTEXT_DIR = process.env.XDG_RUNTIME_DIR
  ? path.join(process.env.XDG_RUNTIME_DIR, "opencode")
  : path.join("/tmp", `opencode-${uid}`)

export const KITTY_CONTEXT_PATH = path.join(KITTY_CONTEXT_DIR, "kitty-context.json")
export const KITTY_CONTEXT_LOCK_PATH = path.join(KITTY_CONTEXT_DIR, "kitty-context.lock")
export const STALE_CONTEXT_MS = 24 * 60 * 60 * 1000

export type KittyContext = {
  kitty_pid: number
  kitty_window_id: number
  updated_at: number
}

export type KittyContexts = Record<string, KittyContext>

export const EMPTY_KITTY_CONTEXT: KittyContext = {
  kitty_pid: 0,
  kitty_window_id: 0,
  updated_at: 0,
}

export async function ensureKittyContextDir() {
  await fs.mkdir(KITTY_CONTEXT_DIR, { recursive: true, mode: 0o700 })
  const stat = await fs.lstat(KITTY_CONTEXT_DIR)
  if (!stat.isDirectory() || stat.isSymbolicLink()) {
    throw new Error(`unsafe kitty context directory: ${KITTY_CONTEXT_DIR}`)
  }

  if (typeof process.getuid === "function" && stat.uid !== process.getuid()) {
    throw new Error(`kitty context directory is not owned by this user: ${KITTY_CONTEXT_DIR}`)
  }

  await fs.chmod(KITTY_CONTEXT_DIR, 0o700)
  const mode = (await fs.lstat(KITTY_CONTEXT_DIR)).mode & 0o777
  if (mode !== 0o700) {
    throw new Error(`kitty context directory has unsafe mode: ${KITTY_CONTEXT_DIR}`)
  }
}

export function kittySocketPath(pid: number) {
  return `/tmp/kitty-${pid}`
}

export async function isSocket(socketPath: string) {
  try {
    return (await fs.stat(socketPath)).isSocket()
  } catch {
    return false
  }
}
