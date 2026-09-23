import fs from "node:fs/promises";
import path from "node:path";

const uid = typeof process.getuid === "function" ? process.getuid() : "user";

/** Private runtime directory shared by the TUI and server Kitty context readers. */
export const KITTY_CONTEXT_DIR = process.env.XDG_RUNTIME_DIR
  ? path.join(process.env.XDG_RUNTIME_DIR, "opencode")
  : path.join("/tmp", `opencode-${uid}`);

/** JSON file mapping session ids to Kitty process and window ids. */
export const KITTY_CONTEXT_PATH = path.join(KITTY_CONTEXT_DIR, "kitty-context.json");
export const KITTY_CONTEXT_LOCK_PATH = path.join(KITTY_CONTEXT_DIR, "kitty-context.lock");
export const STALE_CONTEXT_MS = 24 * 60 * 60 * 1000;

/** Entry shape stored in the shared kitty-context JSON file. */
export type KittyContext = {
  kitty_pid: number;
  kitty_window_id: number;
  updated_at: number;
  directory?: string;
  generation?: number;
};

/** Maps OpenCode session ids to their most recent Kitty pane. */
export type KittyContexts = Record<string, KittyContext>;

export const EMPTY_KITTY_CONTEXT: KittyContext = {
  kitty_pid: 0,
  kitty_window_id: 0,
  updated_at: 0,
};

/** Creates and validates the private runtime directory used for Kitty context. */
export async function ensureKittyContextDir() {
  await fs.mkdir(KITTY_CONTEXT_DIR, { recursive: true, mode: 0o700 });
  const stat = await fs.lstat(KITTY_CONTEXT_DIR);
  if (!stat.isDirectory() || stat.isSymbolicLink()) {
    throw new Error(`unsafe kitty context directory: ${KITTY_CONTEXT_DIR}`);
  }

  if (typeof process.getuid === "function" && stat.uid !== process.getuid()) {
    throw new Error(`kitty context directory is not owned by this user: ${KITTY_CONTEXT_DIR}`);
  }

  await fs.chmod(KITTY_CONTEXT_DIR, 0o700);
  const mode = (await fs.lstat(KITTY_CONTEXT_DIR)).mode & 0o777;
  if (mode !== 0o700) {
    throw new Error(`kitty context directory has unsafe mode: ${KITTY_CONTEXT_DIR}`);
  }
}

/** Returns the Kitty remote-control socket path for a process id. */
export function kittySocketPath(pid: number) {
  return `/tmp/kitty-${pid}`;
}

/** Checks whether a filesystem path is a Unix socket. */
export async function isSocket(socketPath: string) {
  try {
    return (await fs.stat(socketPath)).isSocket();
  } catch {
    return false;
  }
}
