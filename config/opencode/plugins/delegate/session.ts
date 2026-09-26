import type { ToolContext } from "@opencode-ai/plugin";
import type { SessionPromptAsyncData, SessionSummarizeData } from "@opencode-ai/sdk/v2";
import { errorMessage } from "../shared/error.ts";
import { type Client, create, session, type Status, statuses, unwrap } from "../shared/opencode.ts";
import { CONTEXT_PRESSURE } from "../shared/session.ts";
import {
  applyDisplayArgs,
  parseModel,
  type PreparedTask,
  readAgent,
  readCurrentAssistantMessage,
  type TaskArgs,
  taskArgs,
  validateVariant,
} from "./args.ts";
import { contextLimitedSessions, sealContextLimited } from "./context.ts";
import { assertLaneOpen, closeBlocked, laneChild, readExistingChild, sameExecution, sessionExecution } from "./lane.ts";
import { delegate } from "./metadata.ts";
import { deriveChildPermission, type Execution } from "./permission.ts";
import {
  blockedResult,
  contextLimitedResult,
  interruptedResult,
  isContentFilterBlock,
  lastTextPart,
  renderOutput,
  withNotes,
} from "./result.ts";
import { type ChildWait, readChildMessages, waitForChild } from "./wait.ts";

const COLLAB = "collab";
const GIT = "build/git";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Child sessions                                                                                │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

/** Validates a task call and resolves the child's model, permissions, and execution mode after approval.
 * `build/git` requires an attended primary Collab parent and explicit unattended execution. */
export async function prepareTask(client: Client, ctx: ToolContext, input: TaskArgs): Promise<PreparedTask> {
  const args = taskArgs(input);
  const effort = args.effort;
  applyDisplayArgs(input, args);
  const agent = await readAgent(client, args.subagent_type);
  const parent = await session(client, ctx.sessionID, { label: `delegate read parent session ${ctx.sessionID}` });
  validateTaskTarget(parent.agent ?? "", agent.name);
  const parentExecution = sessionExecution(delegate(parent));
  if (agent.name === GIT) {
    if (ctx.agent !== COLLAB || parent.parentID || parentExecution.unattended) {
      throw new Error("delegate refuses build/git without an attended primary collab parent");
    }
    if (args.unattended !== true) {
      throw new Error("delegate build/git requires explicit unattended true");
    }
  }
  const execution = resolveExecution(parentExecution, args);
  if (args.compact && !args.lane) throw new Error("delegate compact requires a lane");

  await askTaskPermission(ctx, args, execution);

  const parentMessage = await readCurrentAssistantMessage(client, ctx);
  const model = args.model ? parseModel(args.model) : (agent.model ?? parentMessage.model);
  const variant = effort ?? (args.model ? undefined : agent.model ? agent.variant : parentMessage.variant);

  await validateVariant(client, model, variant);

  const { permission, envelope } = await deriveChildPermission(client, parent, agent, execution);

  return {
    args,
    agent,
    model,
    variant,
    permission,
    envelope,
    execution,
  };
}

/** Runs a child turn, resuming eligible lanes or creating new children for closed or context-limited lanes.
 * Context-limited children cannot resume in this process even if sealing their permissions fails. */
export async function runChildTask(input: {
  client: Client;
  ctx: ToolContext;
  prepared: PreparedTask;
  notes: string[];
}) {
  const args = input.prepared.args;
  const child = await openChild(input.client, input.ctx, args, input.prepared);

  const metadata = { sessionId: child.id };
  const notes = [...input.notes];

  try {
    await updateToolMetadata(input.ctx, metadata);
  } catch (error) {
    notes.push(`delegate metadata update failed: ${errorMessage(error)}`);
  }

  const childAbort = createChildAbort(input.client, child.id);
  const abort = () => childAbort.start();

  input.ctx.abort.addEventListener("abort", abort);
  try {
    if (input.ctx.abort.aborted) throw new Error("delegate task aborted before child prompt");
    if (args.compact) {
      const body: NonNullable<SessionSummarizeData["body"]> = { ...input.prepared.model, auto: false };
      const summarized = await unwrap(
        input.client.session.summarize({ path: { id: child.id }, body, signal: input.ctx.abort }),
        `delegate compact lane ${args.lane}`,
      );
      if (!summarized) throw new Error(`delegate compaction failed for lane ${args.lane}`);
    }
    let completion: ChildWait;
    try {
      completion = await promptChild({
        client: input.client,
        sessionID: child.id,
        prompt: args.prompt,
        resumedLane: child.resumed ? args.lane : undefined,
        prepared: input.prepared,
        notes,
        signal: input.ctx.abort,
        abortChild: childAbort.start,
      });
    } catch (error) {
      if (isContentFilterBlock(error)) {
        return blockedChild({
          client: input.client,
          args,
          metadata,
          sessionID: child.id,
          notes,
          signal: input.ctx.abort,
        });
      }
      throw error;
    }

    return await completionResult({
      client: input.client,
      args,
      prepared: input.prepared,
      metadata,
      sessionID: child.id,
      notes,
      completion,
      signal: input.ctx.abort,
    });
  } finally {
    input.ctx.abort.removeEventListener("abort", abort);
    childAbort.stop();
  }
}

// ├─ Setup checks ────────────────────────────────────────────────────────────────────────────────┤

async function askTaskPermission(ctx: ToolContext, args: TaskArgs, execution: Execution) {
  await ctx.ask({
    permission: "task",
    patterns: [args.subagent_type],
    always: args.subagent_type === GIT ? [] : ["*"],
    metadata: {
      ...(args.subagent_type === GIT ? { prompt: args.prompt } : {}),
      description: args.description,
      subagent_type: args.subagent_type,
      model: args.model?.trim(),
      effort: args.effort,
      unattended: execution.unattended,
      lane: args.lane,
    },
  });
}

function validateTaskTarget(parentAgent: string, target: string) {
  if (target === COLLAB) {
    throw new Error("delegate refuses collab as a child; collab is attended-primary only");
  }
  if (target === GIT && parentAgent !== COLLAB) {
    throw new Error("delegate refuses build/git; only attended primary collab may launch it");
  }
}

function resolveExecution(parent: Execution, requested: TaskArgs): Execution {
  if (parent.unattended && requested.unattended === false) {
    throw new Error("delegate refuses attended child under unattended parent");
  }

  return { unattended: parent.unattended || requested.unattended !== false };
}

// ├─ Open and prompt ─────────────────────────────────────────────────────────────────────────────┤

async function openChild(client: Client, ctx: ToolContext, args: TaskArgs, prepared: PreparedTask) {
  const current = args.lane ? await laneChild(client, ctx.sessionID, args.lane, ctx.abort) : undefined;
  const closed = !!current?.delegate.closed;
  if (current && !closed && current.agent !== prepared.agent.name) {
    throw new Error(`delegate lane ${args.lane} is pinned to ${current.agent}; requested ${prepared.agent.name}`);
  }
  const limited = current && (contextLimitedSessions.has(current.id) || current.delegate.context);
  const resumable = current && !closed && !limited;
  if (args.compact && !resumable) {
    throw new Error(`delegate cannot compact lane ${args.lane}: no resumable child`);
  }
  if (!resumable) return { ...(await createChild(client, ctx, args, prepared)), resumed: false };
  const child = await readExistingChild({
    client,
    child: current,
    permission: prepared.permission,
    envelope: prepared.envelope,
    execution: prepared.execution,
    signal: ctx.abort,
  });
  return { ...child, resumed: true };
}

async function createChild(client: Client, ctx: ToolContext, args: TaskArgs, prepared: PreparedTask) {
  const metadata = {
    delegate: {
      unattended: prepared.execution.unattended,
      ...(args.lane ? { lane: args.lane, basis: prepared.envelope.basis } : {}),
    },
  };
  const created = await create(
    client,
    {
      parentID: ctx.sessionID,
      title: `${args.lane ? `[${args.lane}] ` : ""}${args.description} (@${prepared.agent.name} subagent)`,
      agent: prepared.agent.name,
      permission: prepared.permission,
      metadata,
    },
    { label: `delegate create child session for ${prepared.agent.name}` },
  );
  const meta = delegate(created);
  if (!sameExecution(sessionExecution(meta), prepared.execution)) {
    throw new Error("delegate child session create lost or mismatched execution metadata");
  }
  if (meta.lane !== args.lane) throw new Error("delegate child session create lost or mismatched lane metadata");
  return { id: created.id };
}

async function promptChild(input: {
  client: Client;
  sessionID: string;
  prompt: string;
  resumedLane?: string;
  prepared: PreparedTask;
  notes: string[];
  signal: AbortSignal;
  abortChild: () => void;
}) {
  const { client, sessionID, prepared, signal } = input;
  const initialMessages = await readChildMessages(client, sessionID, signal);
  const initialMessageIDs = new Set(initialMessages.map((message) => message.info.id));
  if (input.resumedLane) await assertLaneOpen(client, sessionID, input.resumedLane, signal);
  const body = {
    model: prepared.model,
    ...(prepared.variant ? { variant: prepared.variant } : {}),
    agent: prepared.agent.name,
    parts: [{ type: "text", text: input.prompt }],
  } satisfies NonNullable<SessionPromptAsyncData["body"]>;
  await unwrap(
    client.session.promptAsync({ path: { id: sessionID }, body, signal }),
    `delegate prompt child session ${sessionID}`,
  );
  return waitForChild({
    client,
    sessionID,
    initialMessageIDs,
    limits: CONTEXT_PRESSURE,
    prepared,
    notes: input.notes,
    signal,
    abortChild: input.abortChild,
  });
}

async function completionResult(input: {
  client: Client;
  args: TaskArgs;
  prepared: PreparedTask;
  metadata: Record<string, unknown>;
  sessionID: string;
  notes: string[];
  completion: ChildWait;
  signal: AbortSignal;
}) {
  const { client, args, prepared, metadata, sessionID, notes, completion, signal } = input;
  if (completion.interruption) {
    return interruptedResult({ args, metadata, sessionID, notes, reason: completion.interruption });
  }
  const completionError = completion.assistant?.info.error;
  if (completionError && isContentFilterBlock(completionError)) {
    return blockedChild({ client, args, metadata, sessionID, notes, signal });
  }
  if (completion.limit) {
    try {
      await sealContextLimited(client, sessionID, completion.limit, signal);
    } catch (error) {
      notes.push(`context-limit seal failed: ${errorMessage(error)}`);
    }
    return contextLimitedResult({
      args,
      metadata,
      sessionID,
      notes,
      limit: completion.limit,
      messages: completion.messages,
      permission: prepared.permission,
    });
  }
  const response = completion.assistant;
  if (!response) return interruptedResult({ args, metadata, sessionID, notes });
  if (response.info.error) throw new Error(`delegate child failed: ${errorMessage(response.info.error)}`);

  const text = withNotes(lastTextPart(response), notes);
  return {
    title: args.description,
    metadata,
    output: renderOutput({ sessionID, state: "completed", text }),
  };
}

/** Attempts to close a blocked child before reporting it, retaining close failures as notes. */
async function blockedChild(input: {
  client: Client;
  args: TaskArgs;
  metadata: Record<string, unknown>;
  sessionID: string;
  notes: string[];
  signal: AbortSignal;
}) {
  const { client, args, metadata, sessionID, notes, signal } = input;
  try {
    await closeBlocked(client, sessionID, signal);
  } catch (error) {
    notes.push(`content-filter close failed: ${errorMessage(error)}`);
  }
  return blockedResult(args, metadata, sessionID, notes);
}

async function updateToolMetadata(ctx: ToolContext, metadata: Record<string, unknown>) {
  const result: unknown = ctx.metadata({ metadata });
  const { Context, Effect } = await import("effect");
  await (Effect.isEffect(result) ? Effect.runPromiseWith(Context.makeUnsafe<unknown>(new Map()))(result) : result);
}

// ├─ Child cancellation ──────────────────────────────────────────────────────────────────────────┤

function createChildAbort(client: Client, sessionID: string) {
  const timers = new Set<ReturnType<typeof setTimeout>>();
  let stopped = false;
  let started = false;

  const stop = () => {
    stopped = true;
    for (const timer of timers) clearTimeout(timer);
    timers.clear();
  };

  const attempt = async () => {
    if (stopped) return;
    try {
      const aborted = await unwrap(
        client.session.abort({ path: { id: sessionID } }),
        `delegate abort child session ${sessionID}`,
      );
      if (aborted) stop();
    } catch {
      // Scheduled checks keep trying after an abort request fails.
    }
  };

  const retry = async () => {
    if (stopped) return;
    try {
      const live = await statuses(client, { label: `delegate read child session ${sessionID} status after abort` });
      if (stopped) return;
      const status: Status | undefined = live[sessionID];
      if (!status || status.type === "idle") {
        stop();
        return;
      }
      await attempt();
    } catch {
      // Keep the next check after a transient status error.
    }
  };

  const schedule = (delay: number) => {
    const timer = setTimeout(() => {
      timers.delete(timer);
      void retry();
    }, delay);
    timers.add(timer);
  };

  const start = () => {
    if (started || stopped) return;
    started = true;
    void attempt();
    schedule(1_000);
    schedule(2_500);
  };

  return { start, stop };
}
