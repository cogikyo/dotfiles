import type { SessionPromptAsyncData } from "@opencode-ai/sdk/v2";
import { record } from "../shared/record.ts";
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
import { type Client, errorMessage, string, unwrap } from "./sdk.ts";

export type ChildWait = {
  assistant?: Record<string, unknown>;
  messages: unknown[];
  limit?: ContextLimit;
  interruption?: string;
};

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

const STATUS_POLL_MS = 300;
const STARTUP_TIMEOUT_MS = 120_000;

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
    const { status, running, turnMessages } = await readTurn(client, sessionID, initialMessageIDs, waitSignal);
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

    if (!limit && running && !finalAssistant(lastAssistantMessage(turnMessages))) {
      await deliverWarning(delivery, turnMessages);
    }

    if (status && status.type !== "idle" && !running) return poll();
    if (running) return poll();
    if (!active) return poll();
    return { assistant: lastAssistantMessage(turnMessages), messages: turnMessages, limit };
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

async function readTurn(client: Client, sessionID: string, initialMessageIDs: Set<string>, signal: AbortSignal) {
  const [statuses, messages] = await Promise.all([
    unwrap<Record<string, unknown>>(client.session.status({ signal }), `read child session ${sessionID} status`),
    readChildMessages(client, sessionID, signal),
  ]);
  const status = record(statuses[sessionID]);
  const running = status?.type === "busy" || status?.type === "retry";
  const turnMessages = messages.filter((message) => {
    const id = messageID(message);
    return !!id && !initialMessageIDs.has(id);
  });
  return { status, running, turnMessages };
}

async function deliverWarning(input: Delivery, messages: unknown[]) {
  const { client, sessionID, prepared, limits, spent, requested, notes, signal } = input;
  const tokens = maxContextTokens(messages);
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
    `send context ${level} warning to child session ${sessionID}`,
  );
}

export async function readChildMessages(client: Client, sessionID: string, signal: AbortSignal) {
  return unwrap<unknown[]>(
    client.session.messages({ path: { id: sessionID }, signal }),
    `read child session ${sessionID} messages`,
  );
}

function lastAssistantMessage(messages: unknown[]) {
  for (let index = messages.length - 1; index >= 0; index--) {
    const message = record(messages[index]);
    if (record(message?.info)?.role === "assistant") return message;
  }
  return undefined;
}

export function messageID(message: unknown) {
  return string(record(record(message)?.info)?.id);
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
