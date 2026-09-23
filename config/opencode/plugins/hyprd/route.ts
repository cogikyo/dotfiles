// @ts-nocheck -- OpenCode plugin event types are incomplete; keep runtime behavior stable until local event types exist.
import { EMPTY_KITTY_CONTEXT, isSocket, KITTY_CONTEXT_PATH, kittySocketPath } from "./context.ts";
import { send } from "./socket.ts";

const NOTIFY_DEDUPE_MS = 1000;
const IDLE_CONTEXT_MAX_AGE_MS = 30 * 1000;

const recentNotifies = new Map();

function kittyPane(ctx) {
  const kitty_pid = Number(ctx?.kitty_pid) || 0;
  const kitty_window_id = Number(ctx?.kitty_window_id) || 0;
  const updated_at = Number(ctx?.updated_at) || 0;
  if (!kitty_pid || !kitty_window_id) return null;
  return { kitty_pid, kitty_window_id, updated_at };
}

async function kittyContext(sessionID, parentFor) {
  if (!sessionID) return EMPTY_KITTY_CONTEXT;
  try {
    const contexts = await Bun.file(KITTY_CONTEXT_PATH).json();
    const candidates = [];

    for (let id = sessionID, seen = new Set(); id && !seen.has(id); id = parentFor?.(id)) {
      seen.add(id);
      const pane = kittyPane(contexts?.[id]);
      if (pane) candidates.push(pane);
    }

    const live = await Promise.all(candidates.map((ctx) => isSocket(kittySocketPath(ctx.kitty_pid))));
    const ctx = candidates.find((_, index) => live[index]);
    if (ctx) return ctx;
  } catch {}

  return EMPTY_KITTY_CONTEXT;
}

function hasFreshIdleContext(ctx) {
  return ctx.updated_at > 0 && Date.now() - ctx.updated_at <= IDLE_CONTEXT_MAX_AGE_MS;
}

function notifyKey(req) {
  return JSON.stringify([
    req.source,
    req.event,
    req.message,
    req.last_assistant_message,
    req.agent_type,
    req.kitty_pid,
    req.kitty_window_id,
  ]);
}

function recentEnough(key) {
  const previous = recentNotifies.get(key) || 0;
  return Date.now() - previous < NOTIFY_DEDUPE_MS;
}

function markNotified(key) {
  const now = Date.now();
  recentNotifies.set(key, now);
  for (const [oldKey, seenAt] of recentNotifies) {
    if (now - seenAt > NOTIFY_DEDUPE_MS) recentNotifies.delete(oldKey);
  }
}

export async function notify(payload, parentFor) {
  const ctx = await kittyContext(payload.sessionID, parentFor);
  if (!ctx.kitty_pid || !ctx.kitty_window_id) return false;
  if (payload.type === "idle" && !hasFreshIdleContext(ctx)) return false;

  const req = {
    source: "opencode",
    event: payload.type,
    message: payload.message ?? "",
    last_assistant_message: payload.last_assistant_message ?? "",
    agent_type: payload.agent_type ?? "",
    kitty_pid: ctx.kitty_pid,
    kitty_window_id: ctx.kitty_window_id,
  };
  const key = notifyKey(req);
  if (recentEnough(key)) return false;

  const sent = await send("notify " + JSON.stringify(req));
  if (sent) markNotified(key);
  return sent;
}
