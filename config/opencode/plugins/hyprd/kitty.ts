// @ts-nocheck -- OpenCode plugin event types are incomplete; keep runtime behavior stable until local event types exist.
import fs from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import {
  ensureKittyContextDir,
  isSocket,
  KITTY_CONTEXT_LOCK_PATH,
  KITTY_CONTEXT_PATH,
  kittySocketPath,
  STALE_CONTEXT_MS,
} from "./context.ts";
import { send } from "./socket.ts";

const WRITE_INTERVAL_MS = 1000;
const FOCUS_ACK_DEBOUNCE_MS = 1000;
const LOCK_RETRY_MS = 25;
const LOCK_TIMEOUT_MS = 1000;

const KITTY_PID = Number(process.env.KITTY_PID) || 0;
const KITTY_WINDOW_ID = Number(process.env.KITTY_WINDOW_ID) || 0;
const DIRECTORY = process.cwd();
const GENERATION = Date.now();
const execFileAsync = promisify(execFile);

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function withLock(fn) {
  await ensureKittyContextDir();
  const start = Date.now();
  while (true) {
    try {
      await fs.mkdir(KITTY_CONTEXT_LOCK_PATH, { mode: 0o700 });
      break;
    } catch (error) {
      if (error?.code !== "EEXIST" || Date.now() - start > LOCK_TIMEOUT_MS) return fn();
      await sleep(LOCK_RETRY_MS);
    }
  }

  try {
    return await fn();
  } finally {
    await fs.rm(KITTY_CONTEXT_LOCK_PATH, { recursive: true, force: true });
  }
}

async function readContexts() {
  try {
    const parsed = JSON.parse(await fs.readFile(KITTY_CONTEXT_PATH, "utf8"));
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

async function pruneContexts(contexts) {
  const now = Date.now();
  const sockets = new Map();

  for (const [sessionID, ctx] of Object.entries(contexts)) {
    const pid = Number(ctx?.kitty_pid) || 0;
    const updatedAt = Number(ctx?.updated_at) || 0;
    if (!pid || !updatedAt || now - updatedAt > STALE_CONTEXT_MS) {
      delete contexts[sessionID];
      continue;
    }

    let exists = sockets.get(pid);
    if (exists === undefined) {
      exists = await isSocket(kittySocketPath(pid));
      sockets.set(pid, exists);
    }
    if (!exists) delete contexts[sessionID];
  }
}

function isThisPane(ctx) {
  return Number(ctx?.kitty_pid) === KITTY_PID && Number(ctx?.kitty_window_id) === KITTY_WINDOW_ID;
}

async function currentPaneState() {
  if (!KITTY_PID || !KITTY_WINDOW_ID) return null;

  try {
    const { stdout } = await execFileAsync("kitty", ["@", "--to", `unix:/tmp/kitty-${KITTY_PID}`, "ls"]);
    const windows = JSON.parse(stdout);
    if (!Array.isArray(windows)) return null;

    for (const win of windows) {
      for (const tab of win.tabs || []) {
        const pane = (tab.windows || []).find((entry) => Number(entry?.id) === KITTY_WINDOW_ID);
        if (!pane) continue;
        return { focused: Boolean(win?.is_focused && tab?.is_focused && pane?.is_focused) };
      }
    }
  } catch {
    return null;
  }

  return { focused: false };
}

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

async function clearPaneContext() {
  if (!KITTY_PID || !KITTY_WINDOW_ID) return;

  await withLock(async () => {
    const contexts = await readContexts();
    await pruneContexts(contexts);

    for (const [id, ctx] of Object.entries(contexts)) {
      if (isThisPane(ctx)) delete contexts[id];
    }

    await fs.writeFile(KITTY_CONTEXT_PATH, JSON.stringify(contexts), { mode: 0o600 });
  });
}

async function writeContext(sessionID) {
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

    await fs.writeFile(KITTY_CONTEXT_PATH, JSON.stringify(contexts), { mode: 0o600 });
  });
}

const tui = async (api) => {
  let lastFocused = null;
  let lastFocusAckAt = 0;
  let pendingSession = "";
  let syncing = false;
  let disposed = false;
  let syncTask = Promise.resolve();

  const scheduleSync = (sessionID) => {
    if (disposed) return;
    pendingSession = sessionID;
    if (syncing) return;

    syncing = true;
    syncTask = (async () => {
      while (pendingSession) {
        if (disposed) break;
        const next = pendingSession;
        pendingSession = "";
        try {
          await writeContext(next);
        } catch {}
      }
      syncing = false;
    })();
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

/**
 * TUI plugin that syncs Kitty context on `session.created`, `tui.session.select`, and `tui.command.execute`.
 * It sends hyprd `viewed` when the pane gains focus.
 */
export default { id: "hyprd-kitty-context", tui };
