import type { SessionPromptAsyncData } from "@opencode-ai/sdk/v2";
import { errorMessage } from "../shared/error.ts";
import { type Client, type Message, messages, type Reply, statuses, unwrap } from "../shared/opencode.ts";
import type { PreparedTask } from "./args.ts";
import {
  type ContextLimit,
  type ContextLimits,
  type ContextWarning,
  contextLimitedSessions,
  contextWarningPrompt,
  finalAssistant,
  maxContextTokens,
  observeContextLimit,
  observeContextWarnings,
  pendingContextWarning,
} from "./context.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Child wait                                                                                    │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

const STATUS_POLL_MS = 300;
const STARTUP_TIMEOUT_MS = 120_000;

/** Child turn outcome, including any startup interruption or context limit. */
export type ChildWait = {
  assistant?: Reply;
  messages: Message[];
  limit?: ContextLimit;
  interruption?: string;
};

/** Waits for child activity and completion, warning as context fills and aborting on a context limit.
 * A child with no startup activity is aborted after 120 seconds; caller aborts still throw. */
export async function waitForChild(input: {
  client: Client;
  sessionID: string;
  initialMessageIDs: Set<string>;
  limits: ContextLimits;
  prepared: PreparedTask;
  notes: string[];
  signal: AbortSignal;
  abortChild: () => void;
}): Promise<ChildWait> {
  const { client, sessionID, initialMessageIDs, limits, prepared, notes, signal, abortChild } = input;
  let active = false;
  let limit: ContextLimit | undefined;
  const spent = new Set<ContextWarning>();
  const requested = new Set<ContextWarning>();
  const observed = new Set<ContextWarning>();
  const startup = new AbortController();
  const startupTimer = setTimeout(
    () => startup.abort(new Error("delegate child startup timed out")),
    STARTUP_TIMEOUT_MS,
  );
  const waitSignal = AbortSignal.any([signal, startup.signal]);
  const delivery: Delivery = { client, sessionID, prepared, limits, spent, requested, notes, signal: waitSignal };

  const poll = async (): Promise<ChildWait> => {
    await abortableDelay(STATUS_POLL_MS, waitSignal);
    const { running, turnMessages } = await readTurn(client, sessionID, initialMessageIDs, waitSignal);
    observeContextWarnings(turnMessages, spent, observed);
    if (running) {
      active = true;
      clearTimeout(startupTimer);
    }
    if (!active && turnMessages.length) {
      active = true;
      clearTimeout(startupTimer);
      return poll();
    }

    const limitObservation = observeContextLimit(turnMessages, limits);
    if (!limit && limitObservation) {
      limit = limitObservation;
      contextLimitedSessions.add(sessionID);
      if (running) abortChild();
    }

    if (!limit && running && !finalAssistant(lastReply(turnMessages))) {
      await deliverWarning(delivery, turnMessages);
    }

    if (running || !active) return poll();
    return { assistant: lastReply(turnMessages), messages: turnMessages, limit };
  };

  try {
    return await poll();
  } catch (error) {
    if (startup.signal.aborted && !signal.aborted) {
      abortChild();
      return { interruption: "child showed no activity within 120 seconds", messages: [] };
    }
    throw error;
  } finally {
    for (const warning of requested) {
      if (!observed.has(warning)) {
        notes.push(`context ${warning} warning delivery was not confirmed before monitoring ended`);
      }
    }
    clearTimeout(startupTimer);
  }
}

export function readChildMessages(client: Client, sessionID: string, signal: AbortSignal) {
  return messages(client, sessionID, { label: `delegate read child session ${sessionID} messages`, signal });
}

// ├─ Poll ────────────────────────────────────────────────────────────────────────────────────────┤

async function readTurn(client: Client, sessionID: string, initialMessageIDs: Set<string>, signal: AbortSignal) {
  const [live, all] = await Promise.all([
    statuses(client, { label: `delegate read child session ${sessionID} status`, signal }),
    readChildMessages(client, sessionID, signal),
  ]);
  const type = live[sessionID]?.type;
  const running = type === "busy" || type === "retry";
  const turnMessages = all.filter((message) => !initialMessageIDs.has(message.info.id));
  return { running, turnMessages };
}

function lastReply(turn: Message[]): Reply | undefined {
  for (const { info, parts } of turn.toReversed()) if (info.role === "assistant") return { info, parts };
}

function abortableDelay(milliseconds: number, signal: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    if (signal.aborted) {
      reject(signal.reason ?? new Error("delegate task aborted"));
      return;
    }

    const timer = setTimeout(done, milliseconds);
    signal.addEventListener("abort", aborted, { once: true });

    function done() {
      signal.removeEventListener("abort", aborted);
      resolve();
    }

    function aborted() {
      clearTimeout(timer);
      reject(signal.reason ?? new Error("delegate task aborted"));
    }
  });
}

// ├─ Warning delivery ────────────────────────────────────────────────────────────────────────────┤

type Delivery = {
  client: Client;
  sessionID: string;
  prepared: PreparedTask;
  limits: ContextLimits;
  spent: Set<ContextWarning>;
  requested: Set<ContextWarning>;
  notes: string[];
  signal: AbortSignal;
};

async function deliverWarning(input: Delivery, turn: Message[]) {
  const { client, sessionID, prepared, limits, spent, requested, notes, signal } = input;
  const tokens = maxContextTokens(turn);
  const warning = pendingContextWarning(tokens, limits, spent);
  if (!warning) return;
  spent.add(warning);
  requested.add(warning);
  if (warning === "final") spent.add("medium");
  if (warning !== "soft") spent.add("soft");
  try {
    await sendContextWarning({ client, sessionID, prepared, level: warning, tokens, limits, signal });
  } catch (error) {
    requested.delete(warning);
    notes.push(`context ${warning} warning was not delivered: ${errorMessage(error)}`);
  }
}

async function sendContextWarning(input: {
  client: Client;
  sessionID: string;
  prepared: PreparedTask;
  level: ContextWarning;
  tokens: number | undefined;
  limits: ContextLimits;
  signal: AbortSignal;
}) {
  const { client, sessionID, prepared, level, tokens, limits, signal } = input;
  const body = {
    model: prepared.model,
    ...(prepared.variant ? { variant: prepared.variant } : {}),
    agent: prepared.agent.name,
    parts: [{ type: "text", text: contextWarningPrompt(level, tokens, limits) }],
  } satisfies NonNullable<SessionPromptAsyncData["body"]>;
  await unwrap(
    client.session.promptAsync({ path: { id: sessionID }, body, signal }),
    `delegate send context ${level} warning to child session ${sessionID}`,
  );
}
