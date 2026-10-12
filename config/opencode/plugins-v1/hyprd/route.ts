import {
  EMPTY_KITTY_CONTEXT,
  isSocket,
  KITTY_CONTEXT_PATH,
  KittyContexts,
  kittySocketPath,
  type KittyContext,
} from "./context.ts";
import { send } from "./socket.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Notice routing                                                                                │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

/** Notice data for hyprd; `sessionID` selects a pane but is omitted from the socket payload. */
export type Notice = {
  sessionID?: string;
  type: string;
  message?: string;
  last_assistant_message?: string;
  agent_type?: string;
};

const NOTIFY_DEDUPE_MS = 1000;
const IDLE_CONTEXT_MAX_AGE_MS = 30 * 1000;

type Request = {
  source: string;
  event: string;
  message: string;
  last_assistant_message: string;
  agent_type: string;
  kitty_pid: number;
  kitty_window_id: number;
};

/** Sends a notice to the nearest live session or parent pane, suppressing duplicates and stale idle notices. */
export async function notify(payload: Notice, parentFor: (id: string) => string) {
  const ctx = await kittyContext(payload.sessionID, parentFor);
  if (!ctx.kitty_pid || !ctx.kitty_window_id) return false;
  if (payload.type === "idle" && !hasFreshIdleContext(ctx)) return false;

  const req: Request = {
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

// ├─ Pane lookup ─────────────────────────────────────────────────────────────────────────────────┤

async function kittyContext(sessionID: string | undefined, parentFor: (id: string) => string) {
  if (!sessionID) return EMPTY_KITTY_CONTEXT;
  try {
    const contexts = KittyContexts.parse(await Bun.file(KITTY_CONTEXT_PATH).json());
    const candidates: KittyContext[] = [];

    for (let id = sessionID, seen = new Set<string>(); id && !seen.has(id); id = parentFor(id)) {
      seen.add(id);
      const pane = contexts[id];
      if (pane?.kitty_pid && pane.kitty_window_id) {
        candidates.push({
          kitty_pid: pane.kitty_pid,
          kitty_window_id: pane.kitty_window_id,
          updated_at: pane.updated_at,
        });
      }
    }

    const live = await Promise.all(candidates.map((ctx) => isSocket(kittySocketPath(ctx.kitty_pid))));
    const ctx = candidates.find((_, index) => live[index]);
    if (ctx) return ctx;
  } catch {}

  return EMPTY_KITTY_CONTEXT;
}

function hasFreshIdleContext(ctx: KittyContext) {
  return ctx.updated_at > 0 && Date.now() - ctx.updated_at <= IDLE_CONTEXT_MAX_AGE_MS;
}

// ├─ Duplicate suppression ───────────────────────────────────────────────────────────────────────┤

const recentNotifies = new Map<string, number>();

function notifyKey(req: Request) {
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

function recentEnough(key: string) {
  const previous = recentNotifies.get(key) || 0;
  return Date.now() - previous < NOTIFY_DEDUPE_MS;
}

function markNotified(key: string) {
  const now = Date.now();
  recentNotifies.set(key, now);
  for (const [oldKey, seenAt] of recentNotifies) {
    if (now - seenAt > NOTIFY_DEDUPE_MS) recentNotifies.delete(oldKey);
  }
}
