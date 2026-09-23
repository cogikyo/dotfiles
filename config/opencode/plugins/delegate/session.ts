import type { ToolContext } from "@opencode-ai/plugin";
import type { SessionCreateData, SessionPromptAsyncData, SessionSummarizeData } from "@opencode-ai/sdk/v2";
import { record } from "../shared/record.ts";
import { CONTEXT_PRESSURE } from "../shared/session.ts";
import {
  applyDisplayArgs,
  parseEffort,
  parseModel,
  type PreparedTask,
  readAgent,
  readCurrentAssistantMessage,
  type TaskArgs,
  taskArgs,
  validateVariant,
} from "./args.ts";
import { contextLimitedSessions, sealContextLimited } from "./context.ts";
import {
  laneChild,
  readExistingChild,
  sameExecution,
  sessionAgent,
  sessionContextLimit,
  sessionExecution,
  sessionLane,
  sessionParentID,
} from "./lane.ts";
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
import { type Client, errorMessage, string, unwrap } from "./sdk.ts";
import { type ChildWait, messageID, readChildMessages, waitForChild } from "./wait.ts";

const COLLAB = "collab";
const GIT = "build/git";

// ├─ Child task setup ────────────────────────────────────────────────────────────────────────────┤
export async function prepareTask(client: Client, ctx: ToolContext, input: unknown): Promise<PreparedTask> {
  const args = taskArgs(input);
  const effort = parseEffort(args);
  applyDisplayArgs(input, args, effort);
  const agent = await readAgent(client, args.subagent_type);
  const parent = await unwrap<Record<string, unknown>>(
    client.session.get({ path: { id: ctx.sessionID } }),
    `read parent session ${ctx.sessionID}`,
  );
  const parentAgent = sessionAgent(parent) ?? "";
  validateTaskTarget(parentAgent, agent.name);
  const parentExecution = sessionExecution(parent);
  if (agent.name === GIT) {
    if (ctx.agent !== COLLAB || sessionParentID(parent) || parentExecution.unattended) {
      throw new Error("delegate refuses build/git without an attended primary collab parent");
    }
    if (args.unattended !== true) {
      throw new Error("delegate build/git requires explicit unattended true");
    }
  }
  const execution = resolveExecution(parentExecution, args);
  if (args.compact && !args.lane) throw new Error("delegate compact requires a lane");

  await askTaskPermission(ctx, { ...args, effort }, execution);

  const parentMessage = await readCurrentAssistantMessage(client, ctx);
  const model = args.model ? parseModel(args.model) : (agent.model ?? parentMessage.model);
  const variant = effort ?? (args.model ? undefined : agent.model ? agent.variant : parentMessage.variant);

  await validateVariant(client, model, variant);

  const permission = await deriveChildPermission(client, parent, agent, execution);

  return {
    args,
    agent,
    model,
    variant,
    permission,
    execution,
  };
}

export async function runChildTask(input: {
  client: Client;
  ctx: ToolContext;
  args: TaskArgs;
  prepared: PreparedTask;
  notes: string[];
}) {
  const child = await openChild(input.client, input.ctx, input.args, input.prepared);

  const metadata = { sessionId: child.id };
  const notes = [...input.notes];

  try {
    await updateToolMetadata(input.ctx, { metadata });
  } catch (error) {
    notes.push(`delegate metadata update failed: ${errorMessage(error)}`);
  }

  const childAbort = createChildAbort(input.client, child.id);
  const abort = () => childAbort.start();

  input.ctx.abort.addEventListener("abort", abort);
  try {
    if (input.ctx.abort.aborted) throw new Error("delegate task aborted before child prompt");
    if (input.args.compact) {
      const body: NonNullable<SessionSummarizeData["body"]> = { ...input.prepared.model, auto: false };
      const summarized = await unwrap<boolean>(
        input.client.session.summarize({ path: { id: child.id }, body, signal: input.ctx.abort }),
        `compact lane ${input.args.lane}`,
      );
      if (!summarized) throw new Error(`delegate compaction failed for lane ${input.args.lane}`);
    }
    let completion: ChildWait;
    try {
      completion = await promptChild({
        client: input.client,
        sessionID: child.id,
        prompt: input.args.prompt,
        prepared: input.prepared,
        notes,
        signal: input.ctx.abort,
        abortChild: childAbort.start,
      });
    } catch (error) {
      if (isContentFilterBlock(error)) return blockedResult(input.args, metadata, child.id, notes);
      throw error;
    }

    return await completionResult({
      client: input.client,
      args: input.args,
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

async function openChild(client: Client, ctx: ToolContext, args: TaskArgs, prepared: PreparedTask) {
  const current = args.lane ? await laneChild(client, ctx.sessionID, args.lane, ctx.abort) : undefined;
  if (current && sessionAgent(current) !== prepared.agent.name) {
    throw new Error(
      `delegate lane ${args.lane} is pinned to ${sessionAgent(current)}; requested ${prepared.agent.name}`,
    );
  }
  const limited = current && (contextLimitedSessions.has(String(current.id)) || sessionContextLimit(current));
  if (args.compact && (!current || limited)) {
    throw new Error(`delegate cannot compact lane ${args.lane}: no resumable child`);
  }
  return current && !limited
    ? await readExistingChild({
        client,
        session: current,
        permission: prepared.permission,
        execution: prepared.execution,
        signal: ctx.abort,
      })
    : await createChild(client, ctx, args, prepared);
}

async function promptChild(input: {
  client: Client;
  sessionID: string;
  prompt: string;
  prepared: PreparedTask;
  notes: string[];
  signal: AbortSignal;
  abortChild: () => void;
}) {
  const { client, sessionID, prepared, signal } = input;
  const initialMessages = await readChildMessages(client, sessionID, signal);
  const initialMessageIDs = new Set(initialMessages.map(messageID).filter((id): id is string => !!id));
  const body = {
    model: prepared.model,
    ...(prepared.variant ? { variant: prepared.variant } : {}),
    agent: prepared.agent.name,
    parts: [{ type: "text", text: input.prompt }],
  } satisfies NonNullable<SessionPromptAsyncData["body"]>;
  await unwrap(
    client.session.promptAsync({ path: { id: sessionID }, body, signal }),
    `prompt child session ${sessionID}`,
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
  const completionError = record(completion.assistant?.info)?.error;
  if (completionError && isContentFilterBlock(completionError)) {
    return blockedResult(args, metadata, sessionID, notes);
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
  const info = record(response.info);
  if (info?.error) {
    if (isContentFilterBlock(info.error)) return blockedResult(args, metadata, sessionID, notes);
    throw new Error(`delegate child failed: ${errorMessage(info.error)}`);
  }

  const text = withNotes(lastTextPart(response), notes);
  return {
    title: args.description,
    metadata,
    output: renderOutput({ sessionID, state: "completed", text }),
  };
}

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

async function createChild(client: Client, ctx: ToolContext, args: TaskArgs, prepared: PreparedTask) {
  const metadata = {
    delegate: { unattended: prepared.execution.unattended, ...(args.lane ? { lane: args.lane } : {}) },
  };
  const body: NonNullable<SessionCreateData["body"]> = {
    parentID: ctx.sessionID,
    title: `${args.lane ? `[${args.lane}] ` : ""}${args.description} (@${prepared.agent.name} subagent)`,
    agent: prepared.agent.name,
    permission: prepared.permission,
    metadata,
  };
  const session = await unwrap<Record<string, unknown>>(
    client.session.create({ body }),
    `create child session for ${prepared.agent.name}`,
  );
  const id = string(session.id);
  if (!id) throw new Error("delegate child session create response did not include an id");
  if (!sameExecution(sessionExecution(session), prepared.execution)) {
    throw new Error("delegate child session create lost or mismatched execution metadata");
  }
  if (sessionLane(session) !== args.lane)
    throw new Error("delegate child session create lost or mismatched lane metadata");
  return { id };
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
      const aborted = await unwrap<boolean>(
        client.session.abort({ path: { id: sessionID } }),
        `abort child session ${sessionID}`,
      );
      if (aborted) stop();
    } catch {
      // The scheduled status check can retry if this abort request fails.
    }
  };

  const retry = async () => {
    if (stopped) return;
    try {
      const statuses = await unwrap<Record<string, unknown>>(
        client.session.status({}),
        `read child session ${sessionID} status after abort`,
      );
      if (stopped) return;
      const status = record(statuses[sessionID]);
      if (!status || status.type === "idle") {
        stop();
        return;
      }
      if (status.type === "busy" || status.type === "retry") await attempt();
    } catch {
      // Keep the next scheduled status check after a transient status error.
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

async function updateToolMetadata(ctx: ToolContext, input: { title?: string; metadata?: Record<string, unknown> }) {
  const result = ctx.metadata(input);
  if (isPromiseLike(result)) {
    await result;
    return;
  }
  if (!isEffectLike(result)) return;

  const runPromise = await effectRunPromise();
  await runPromise(result);
}

function isPromiseLike(value: unknown): value is PromiseLike<unknown> {
  return typeof record(value)?.then === "function";
}

function isEffectLike(value: unknown) {
  const root = record(value);
  return !!root && (typeof root.pipe === "function" || typeof root._op === "string");
}

async function effectRunPromise() {
  let mod: Record<string, unknown> | undefined;
  try {
    mod = record(await import("effect"));
  } catch (error) {
    throw new Error(`delegate failed to import effect for metadata update: ${errorMessage(error)}`, { cause: error });
  }
  const runPromise = record(mod?.Effect)?.runPromise;
  if (typeof runPromise !== "function") {
    throw new Error("delegate effect module is missing Effect.runPromise for metadata update");
  }
  return (effect: unknown) => Promise.resolve(Reflect.apply(runPromise, undefined, [effect]));
}
