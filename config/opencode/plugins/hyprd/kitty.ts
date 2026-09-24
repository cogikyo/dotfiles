import type { TuiPlugin, TuiPluginModule } from "@opencode-ai/plugin/tui";
import fs from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { z } from "zod";
import { readText, writeJson } from "../shared/file.ts";
import {
  ensureKittyContextDir,
  isSocket,
  KITTY_CONTEXT_LOCK_PATH,
  KITTY_CONTEXT_PATH,
  KittyContexts,
  kittySocketPath,
  STALE_CONTEXT_MS,
  type KittyContext,
} from "./context.ts";
import { send } from "./socket.ts";

const WRITE_INTERVAL_MS = 1000;
const FOCUS_ACK_DEBOUNCE_MS = 1000;
const LOCK_RETRY_MS = 25;
const LOCK_TIMEOUT_MS = 1000;

// Directory and generation identify this plugin instance across writes.
const KITTY_PID = Number(process.env.KITTY_PID) || 0;
const KITTY_WINDOW_ID = Number(process.env.KITTY_WINDOW_ID) || 0;
const DIRECTORY = process.cwd();
const GENERATION = Date.now();
const execFileAsync = promisify(execFile);

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Kitty pane sync                                                                               │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

const tui: TuiPlugin = async (api) => {
  let lastFocused: boolean | null = null;
  let lastFocusAckAt = 0;
  let pendingSession = "";
  let syncing = false;
  let disposed = false;
  let syncTask = Promise.resolve();

  const drainSync = async (): Promise<void> => {
    if (disposed || !pendingSession) {
      syncing = false;
      return;
    }

    const next = pendingSession;
    pendingSession = "";
    try {
      await writeContext(next);
    } catch {}
    return drainSync();
  };

  const scheduleSync = (sessionID: string) => {
    if (disposed) return;
    pendingSession = sessionID;
    if (syncing) return;

    syncing = true;
    syncTask = drainSync();
  };

  const sync = () => {
    const current = api.route.current;
    if (current.name !== "session") return;

    const sessionID = current.params?.sessionID;
    if (typeof sessionID === "string" && sessionID !== "") {
      scheduleSync(sessionID);
    }
  };

  const acknowledgeFocusedPane = async () => {
    const pane = await currentPaneState();
    if (pane === null) return;

    const wasFocused = lastFocused;
    lastFocused = pane.focused;
    if (wasFocused !== false || !pane.focused) return;

    const now = Date.now();
    if (now - lastFocusAckAt < FOCUS_ACK_DEBOUNCE_MS) return;

    if (await notifyViewed()) lastFocusAckAt = now;
  };

  const poll = () => {
    sync();
    void acknowledgeFocusedPane();
  };

  poll();
  const timer = setInterval(poll, WRITE_INTERVAL_MS);
  const disposers = [
    api.event.on("session.created", () => sync()),
    api.event.on("tui.session.select", (event) => {
      const sessionID = event.properties?.sessionID;
      if (typeof sessionID !== "string" || sessionID === "") return;

      scheduleSync(sessionID);
    }),
    api.event.on("tui.command.execute", (event) => {
      if (event.properties?.command !== "session.new") return;

      void clearPaneContext();
      setTimeout(sync, 50);
      setTimeout(sync, 250);
    }),
  ];

  api.lifecycle.onDispose(() => {
    disposed = true;
    pendingSession = "";
    clearInterval(timer);
    for (const dispose of disposers) dispose();
    void syncTask.then(clearPaneContext).catch(() => {});
  });
};

/** Syncs the active session to its kitty pane and acknowledges focus when an unfocused pane becomes active. */
export default { id: "hyprd-kitty-context", tui } satisfies TuiPluginModule & { id: string };

// ├─ Context file ────────────────────────────────────────────────────────────────────────────────┤

// Keep one session entry per pane while preserving entries for other panes.
async function writeContext(sessionID: string) {
  if (!sessionID || !KITTY_PID || !KITTY_WINDOW_ID) return;

  await withLock(async () => {
    const contexts = await readContexts();
    await pruneContexts(contexts);

    for (const [id, ctx] of Object.entries(contexts)) {
      if (id !== sessionID && isThisPane(ctx)) delete contexts[id];
    }

    contexts[sessionID] = {
      kitty_pid: KITTY_PID,
      kitty_window_id: KITTY_WINDOW_ID,
      updated_at: Date.now(),
      directory: DIRECTORY,
      generation: GENERATION,
    };

    await writeJson(KITTY_CONTEXT_PATH, contexts, 0o600);
  });
}

async function clearPaneContext() {
  if (!KITTY_PID || !KITTY_WINDOW_ID) return;

  await withLock(async () => {
    const contexts = await readContexts();
    await pruneContexts(contexts);

    for (const [id, ctx] of Object.entries(contexts)) {
      if (isThisPane(ctx)) delete contexts[id];
    }

    await writeJson(KITTY_CONTEXT_PATH, contexts, 0o600);
  });
}

async function readContexts() {
  try {
    const text = await readText(KITTY_CONTEXT_PATH);
    return text === undefined ? {} : KittyContexts.parse(JSON.parse(text));
  } catch {
    return {};
  }
}

// Prune expired panes and panes whose kitty socket no longer exists.
async function pruneContexts(contexts: KittyContexts) {
  const now = Date.now();
  const live = new Map<string, number>();

  for (const [sessionID, ctx] of Object.entries(contexts)) {
    const pid = ctx.kitty_pid;
    const updatedAt = ctx.updated_at;
    if (!pid || !updatedAt || now - updatedAt > STALE_CONTEXT_MS) {
      delete contexts[sessionID];
      continue;
    }
    live.set(sessionID, pid);
  }

  const pids = new Set(live.values());
  const sockets = new Map(
    await Promise.all(
      Array.from(pids, async (pid): Promise<[number, boolean]> => [pid, await isSocket(kittySocketPath(pid))]),
    ),
  );
  for (const [sessionID, pid] of live) {
    if (!sockets.get(pid)) delete contexts[sessionID];
  }
}

function isThisPane(ctx: KittyContext) {
  return ctx.kitty_pid === KITTY_PID && ctx.kitty_window_id === KITTY_WINDOW_ID;
}

// ├─ Focus ───────────────────────────────────────────────────────────────────────────────────────┤

// Kitty's `ls` output nests panes under tabs and OS windows.
const focus = z.boolean().optional().catch(undefined);
const Panes = z.array(
  z.object({
    is_focused: focus,
    tabs: z
      .array(
        z.object({
          is_focused: focus,
          windows: z.array(z.object({ id: z.coerce.number().catch(0), is_focused: focus })).catch([]),
        }),
      )
      .catch([]),
  }),
);

// An absent pane is unfocused; invalid kitty output leaves focus unknown.
async function currentPaneState() {
  if (!KITTY_PID || !KITTY_WINDOW_ID) return null;

  try {
    const { stdout } = await execFileAsync("kitty", ["@", "--to", `unix:/tmp/kitty-${KITTY_PID}`, "ls"]);
    const windows = Panes.safeParse(JSON.parse(stdout));
    if (!windows.success) return null;

    for (const win of windows.data) {
      for (const tab of win.tabs) {
        const pane = tab.windows.find((entry) => entry.id === KITTY_WINDOW_ID);
        if (!pane) continue;
        return { focused: Boolean(win.is_focused && tab.is_focused && pane.is_focused) };
      }
    }
  } catch {
    return null;
  }

  return { focused: false };
}

// Hyprd treats `viewed` as a pane acknowledgment, not a visible notice.
async function notifyViewed() {
  if (!KITTY_PID || !KITTY_WINDOW_ID) return false;

  return send(
    "notify " +
      JSON.stringify({
        source: "opencode",
        event: "viewed",
        kitty_pid: KITTY_PID,
        kitty_window_id: KITTY_WINDOW_ID,
      }),
  );
}

// ├─ Lock ────────────────────────────────────────────────────────────────────────────────────────┤

// Context writes proceed unlocked if the directory lock times out after one second.
async function withLock(fn: () => Promise<void>) {
  await ensureKittyContextDir();
  if (!(await acquireLock(Date.now()))) return fn();

  try {
    return await fn();
  } finally {
    await fs.rm(KITTY_CONTEXT_LOCK_PATH, { recursive: true, force: true });
  }
}

async function acquireLock(start: number): Promise<boolean> {
  try {
    await fs.mkdir(KITTY_CONTEXT_LOCK_PATH, { mode: 0o700 });
    return true;
  } catch (error) {
    if (!(error instanceof Error && "code" in error && error.code === "EEXIST")) return false;
    if (Date.now() - start > LOCK_TIMEOUT_MS) return false;
    await Bun.sleep(LOCK_RETRY_MS);
    return acquireLock(start);
  }
}
