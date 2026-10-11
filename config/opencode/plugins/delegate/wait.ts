import type { SessionPromptAsyncData } from "@opencode-ai/sdk/v2";
import { errorMessage } from "../shared/error.ts";
import { type Client, type Message, messages, type Reply, statuses, unwrap } from "../shared/opencode.ts";
import { STEP_BUDGET } from "../shared/session.ts";
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
const SETTLE_POLLS = 10;
const STEP_WARNING_MARKER = "[DELEGATE STEP BUDGET: WARNING]";

/** Child turn outcome, including any startup interruption, context limit, or step limit. */
export type ChildWait = {
  assistant?: Reply;
  messages: Message[];
  limit?: ContextLimit;
  steps?: number;
  interruption?: string;
};

/** Waits for child activity and completion, warning as context fills and aborting on a context limit.
 * When the provider reports no tokens, assistant steps stand in for context and abort the child at the step budget.
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
  let unsettled = 0;
  const stop: Stop = {};
  const spent = new Set<ContextWarning>();
  const requested = new Set<ContextWarning>();
  const observed = new Set<ContextWarning>();
  const startup = new AbortController();
  const startupTimer = setTimeout(
    () => startup.abort(new Error("delegate child startup timed out")),
    STARTUP_TIMEOUT_MS,
  );
  const waitSignal = AbortSignal.any([signal, startup.signal]);
  const delivery: Delivery = {
    client,
    sessionID,
    prepared,
    limits,
    spent,
    requested,
    stepWarned: false,
    notes,
    signal: waitSignal,
    abortChild,
  };

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

    await govern(delivery, stop, running, turnMessages);

    if (running || !active) return poll();
    const assistant = lastReply(turnMessages);
    if (!settled(assistant) && unsettled++ < SETTLE_POLLS) return poll();
    return { assistant, messages: turnMessages, ...stop };
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

function settled(reply: Reply | undefined) {
  return !reply || !!reply.info.finish || !!reply.info.error;
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
  stepWarned: boolean;
  notes: string[];
  signal: AbortSignal;
  abortChild: () => void;
};

type Stop = Pick<ChildWait, "limit" | "steps">;

async function govern(input: Delivery, stop: Stop, running: boolean, turn: Message[]) {
  if (stop.limit || stop.steps) return;
  const steps = tokenlessSteps(turn);
  stop.limit = observeContextLimit(turn, input.limits);
  if (stop.limit) contextLimitedSessions.add(input.sessionID);
  else if (running && steps >= STEP_BUDGET.hard) stop.steps = steps;
  if (stop.limit || stop.steps) {
    if (running) input.abortChild();
    return;
  }
  if (running && !finalAssistant(lastReply(turn))) await deliverWarning(input, turn, steps);
}

async function deliverWarning(input: Delivery, turn: Message[], steps: number) {
  const { limits, spent, requested, notes } = input;
  if (!input.stepWarned && steps >= STEP_BUDGET.warning) {
    input.stepWarned = true;
    try {
      await send(input, stepWarningPrompt(steps), "step budget warning");
    } catch (error) {
      notes.push(`step budget warning was not delivered: ${errorMessage(error)}`);
    }
  }
  const tokens = maxContextTokens(turn);
  const warning = pendingContextWarning(tokens, limits, spent);
  if (!warning) return;
  spent.add(warning);
  requested.add(warning);
  if (warning === "final") spent.add("medium");
  try {
    await send(input, contextWarningPrompt(warning, tokens, limits), `context ${warning} warning`);
  } catch (error) {
    requested.delete(warning);
    notes.push(`context ${warning} warning was not delivered: ${errorMessage(error)}`);
  }
}

async function send(input: Delivery, text: string, label: string) {
  const { client, sessionID, prepared, signal } = input;
  const body = {
    model: prepared.model,
    ...(prepared.variant ? { variant: prepared.variant } : {}),
    agent: prepared.agent.name,
    parts: [{ type: "text", text }],
  } satisfies NonNullable<SessionPromptAsyncData["body"]>;
  await unwrap(
    client.session.promptAsync({ path: { id: sessionID }, body, signal }),
    `delegate send ${label} to child session ${sessionID}`,
  );
}

// ├─ Step budget ─────────────────────────────────────────────────────────────────────────────────┤

function tokenlessSteps(turn: Message[]) {
  if (maxContextTokens(turn) !== undefined) return 0;
  return turn.filter((message) => message.info.role === "assistant").length;
}

function stepWarningPrompt(steps: number) {
  return [
    STEP_WARNING_MARKER,
    "This provider reports no token counts, so the delegate counts assistant steps instead.",
    `Observed steps: ${steps}; the child is stopped at ${STEP_BUDGET.hard} steps.`,
    "Wrap up: finish the current step and return a concise final report, including any unfinished work.",
  ].join("\n");
}
