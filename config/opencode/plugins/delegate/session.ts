import type { PluginInput, ToolContext } from "@opencode-ai/plugin";

export type TaskArgs = {
  description: string;
  prompt: string;
  subagent_type: string;
  model?: string;
  effort?: string;
  task_id?: string;
};

export type ModelRef = {
  providerID: string;
  modelID: string;
};

type Client = PluginInput["client"];

type Rule = {
  permission: string;
  pattern: string;
  action: "allow" | "ask" | "deny";
};

type AgentInfo = {
  name: string;
  permission?: unknown;
  model?: ModelRef;
  variant?: string;
};

type PreparedTask = {
  args: TaskArgs;
  agent: AgentInfo;
  model: ModelRef;
  variant?: string;
  permission: Rule[];
  driveParent: boolean;
};

const CONTENT_FILTER_ADVICE = "child unrecoverable; re-brief a fresh child (reword the brief first, switch provider as last resort); never resume this session";
const INTERRUPTED_ADVICE = "completion unknown; reconcile durable state before re-running because the child may have edited files";
const KNOWN_EFFORTS = new Set(["default", "minimal", "low", "medium", "high", "xhigh"]);
const STATUS_POLL_MS = 300;
const STARTUP_TIMEOUT_MS = 120_000;

export async function prepareTask(client: Client, ctx: ToolContext, input: unknown): Promise<PreparedTask> {
  const args = taskArgs(input);
  const effort = parseEffort(args);
  applyDisplayArgs(input, args, effort);
  const agent = await readAgent(client, args.subagent_type);

  await askTaskPermission(ctx, { ...args, effort });

  const parentMessage = await readCurrentAssistantMessage(client, ctx);
  const model = args.model ? parseModel(args.model) : (agent.model ?? parentMessage.model);
  const variant = effort ?? (args.model ? undefined : agent.model ? agent.variant : parentMessage.variant);

  await validateVariant(client, model, variant);

  const childPermission = await deriveChildPermission(client, ctx.sessionID, agent);

  return {
    args,
    agent,
    model,
    variant,
    permission: childPermission.rules,
    driveParent: childPermission.driveParent,
  };
}

export async function runChildTask(input: {
  client: Client;
  ctx: ToolContext;
  args: TaskArgs;
  prepared: PreparedTask;
  notes: string[];
}) {
  if (input.args.task_id && input.prepared.driveParent) {
    throw new Error(
      "delegate task_id resume is disabled from Drive; re-brief a fresh child so Drive can apply its current deny-only AFK envelope",
    );
  }

  const child = input.args.task_id
    ? await readExistingChild(input.client, input.args.task_id, input.ctx.abort)
    : await createChild(input.client, input.ctx, input.args, input.prepared);

  const metadata = { sessionId: child.id };
  const notes = [...input.notes];

  try {
    await updateToolMetadata(input.ctx, { metadata });
  } catch (error) {
    notes.push(`delegate metadata update failed: ${errorMessage(error)}`);
  }

  const childAbort = createChildAbort(input.client, child.id);
  const abort = childAbort.start;

  input.ctx.abort.addEventListener("abort", abort);
  try {
    if (input.ctx.abort.aborted) throw new Error("delegate task aborted before child prompt");
    let completion: Awaited<ReturnType<typeof waitForChild>>;
    try {
      const initialMessages = await readChildMessages(input.client, child.id, input.ctx.abort);
      const initialMessageIDs = new Set(initialMessages.map(messageID).filter((id): id is string => !!id));
      await unwrap(
        input.client.session.promptAsync({
          path: { id: child.id },
          body: {
            model: input.prepared.model,
            ...(input.prepared.variant ? { variant: input.prepared.variant } : {}),
            agent: input.prepared.agent.name,
            parts: [{ type: "text", text: input.args.prompt }],
          },
          signal: input.ctx.abort,
        } as never),
        `prompt child session ${child.id}`,
      );
      completion = await waitForChild(input.client, child.id, initialMessageIDs, input.ctx.abort, childAbort.start);
    } catch (error) {
      if (isContentFilterBlock(error)) return blockedResult(input.args, metadata, child.id);
      throw error;
    }

    if (completion.interruption) {
      return interruptedResult(input.args, metadata, child.id, notes, completion.interruption);
    }
    const response = completion.assistant;
    if (!response) return interruptedResult(input.args, metadata, child.id, notes);
    const info = object(response.info);
    if (info?.error) {
      if (isContentFilterBlock(info.error)) return blockedResult(input.args, metadata, child.id);
      throw new Error(`delegate child failed: ${errorMessage(info.error)}`);
    }

    const text = withNotes(lastTextPart(response), notes);
    return {
      title: input.args.description,
      metadata,
      output: renderOutput({ sessionID: child.id, state: "completed", text }),
    };
  } finally {
    input.ctx.abort.removeEventListener("abort", abort);
    childAbort.stop();
  }
}

async function waitForChild(
  client: Client,
  sessionID: string,
  initialMessageIDs: Set<string>,
  signal: AbortSignal,
  abortChild: () => void,
) {
  let active = false;
  const startup = new AbortController();
  const startupTimer = setTimeout(
    () => startup.abort(new Error("delegate child startup timed out")),
    STARTUP_TIMEOUT_MS,
  );
  const waitSignal = AbortSignal.any([signal, startup.signal]);

  try {
    while (true) {
      await abortableDelay(STATUS_POLL_MS, waitSignal);
      const statuses = await unwrap<Record<string, unknown>>(
        client.session.status({ signal: waitSignal } as never),
        `read child session ${sessionID} status`,
      );
      const status = object(statuses[sessionID]);
      if (status?.type === "busy" || status?.type === "retry") {
        active = true;
        clearTimeout(startupTimer);
        continue;
      }
      if (status && status.type !== "idle") continue;

      const messages = await readChildMessages(client, sessionID, waitSignal);
      const turnMessages = messages.filter((message) => {
        const id = messageID(message);
        return !!id && !initialMessageIDs.has(id);
      });
      if (!active && turnMessages.length) {
        active = true;
        clearTimeout(startupTimer);
        continue;
      }
      if (!active) continue;
      return { assistant: lastAssistantMessage(turnMessages) };
    }
  } catch (error) {
    if (startup.signal.aborted && !signal.aborted) {
      abortChild();
      return { interruption: "child showed no activity within 120 seconds" };
    }
    throw error;
  } finally {
    clearTimeout(startupTimer);
  }
}

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
        client.session.abort({ path: { id: sessionID } } as never),
        `abort child session ${sessionID}`,
      );
      if (aborted) stop();
    } catch {
      // A later status-correlated attempt can still confirm and abort the runner.
    }
  };

  const retry = async () => {
    if (stopped) return;
    try {
      const statuses = await unwrap<Record<string, unknown>>(
        client.session.status({} as never),
        `read child session ${sessionID} status after abort`,
      );
      if (stopped) return;
      const status = object(statuses[sessionID]);
      if (!status || status.type === "idle") {
        stop();
        return;
      }
      if (status.type === "busy" || status.type === "retry") await attempt();
    } catch {
      // The next scheduled status check remains an independent chance to confirm liveness.
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

async function readChildMessages(client: Client, sessionID: string, signal: AbortSignal) {
  return unwrap<unknown[]>(
    client.session.messages({ path: { id: sessionID }, signal } as never),
    `read child session ${sessionID} messages`,
  );
}

function lastAssistantMessage(messages: unknown[]) {
  for (let index = messages.length - 1; index >= 0; index--) {
    const message = object(messages[index]);
    if (object(message?.info)?.role === "assistant") return message;
  }
  return undefined;
}

function messageID(message: unknown) {
  return string(object(object(message)?.info)?.id);
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

async function updateToolMetadata(
  ctx: ToolContext,
  input: { title?: string; metadata?: Record<string, unknown> },
) {
  const result = (ctx.metadata as (input: { title?: string; metadata?: Record<string, unknown> }) => unknown)(input);
  if (isPromiseLike(result)) {
    await result;
    return;
  }
  if (!isEffectLike(result)) return;

  const runPromise = await effectRunPromise();
  await runPromise(result);
}

function parseModel(value: string): ModelRef {
  const clean = value.trim();
  const slash = clean.indexOf("/");
  if (slash <= 0 || slash === clean.length - 1) {
    throw new Error(`delegate model must be provider/model-id, got ${JSON.stringify(value)}`);
  }
  return { providerID: clean.slice(0, slash), modelID: clean.slice(slash + 1) };
}

function taskArgs(value: unknown): TaskArgs {
  const root = object(value);
  if (!root) throw new Error("delegate task arguments must be an object");

  const args: TaskArgs = {
    description: requiredString(root, "description").trim(),
    prompt: requiredString(root, "prompt"),
    subagent_type: requiredString(root, "subagent_type").trim(),
  };
  const model = optionalString(root, "model");
  const effort = optionalString(root, "effort");
  const taskID = optionalString(root, "task_id");
  if (model !== undefined) args.model = model;
  if (effort !== undefined) args.effort = effort;
  if (taskID !== undefined) args.task_id = taskID;
  return args;
}

function applyDisplayArgs(input: unknown, args: TaskArgs, effort: string | undefined) {
  const base = stripEffortSuffix(args.description, effort);
  const description = effort ? `${base} · ${effort}` : base;
  args.description = description;
  if (effort) args.effort = effort;

  const root = object(input);
  if (!root) return;
  root.description = description;
  root.subagent_type = args.subagent_type;
  if (effort) root.effort = effort;
}

function stripEffortSuffix(description: string, effort: string | undefined) {
  const efforts = new Set(effort ? [...KNOWN_EFFORTS, effort] : KNOWN_EFFORTS);
  let clean = description;

  while (true) {
    const match = clean.match(/^(.*) · ([^·\n]+)$/u);
    if (!match) return clean;
    if (!efforts.has(match[2].trim())) return clean;
    clean = match[1].trimEnd();
  }
}

function parseEffort(args: TaskArgs) {
  if (args.effort === undefined) return undefined;
  const clean = args.effort?.trim();
  if (!clean) throw new Error("delegate effort must not be empty when provided");
  return clean;
}

function requiredString(root: Record<string, unknown>, name: keyof TaskArgs) {
  if (!Object.hasOwn(root, name) || root[name] === undefined) {
    throw new Error(`delegate task missing required argument: ${name}`);
  }
  if (typeof root[name] !== "string") throw new Error(`delegate task argument ${name} must be a string`);
  if (!root[name].trim()) throw new Error(`delegate task argument ${name} must not be empty`);
  return root[name];
}

function optionalString(root: Record<string, unknown>, name: keyof TaskArgs) {
  if (!Object.hasOwn(root, name) || root[name] === undefined) return undefined;
  if (typeof root[name] !== "string") throw new Error(`delegate task argument ${name} must be a string`);
  if (!root[name].trim()) throw new Error(`delegate task argument ${name} must not be empty when provided`);
  return root[name];
}

async function askTaskPermission(ctx: ToolContext, args: TaskArgs) {
  await (ctx.ask({
    permission: "task",
    patterns: [args.subagent_type],
    always: ["*"],
    metadata: {
      description: args.description,
      subagent_type: args.subagent_type,
      model: args.model?.trim(),
      effort: args.effort,
    },
  }) as unknown as Promise<void>);
}

async function readCurrentAssistantMessage(client: Client, ctx: ToolContext) {
  const message = await unwrap<Record<string, unknown>>(
    client.session.message({ path: { id: ctx.sessionID, messageID: ctx.messageID } } as never),
    `read parent message ${ctx.messageID}`,
  );
  const info = object(message.info);
  if (!info || info.role !== "assistant") {
    throw new Error("delegate cannot inherit model because the current message is not an assistant message");
  }

  const providerID = string(info.providerID) ?? string(object(info.model)?.providerID);
  const modelID = string(info.modelID) ?? string(object(info.model)?.modelID);
  if (!providerID || !modelID) throw new Error("delegate cannot inherit model because parent message lacks model IDs");

  const variant = string(info.variant) ?? string(object(info.model)?.variant);
  return { model: { providerID, modelID }, variant: variant === "default" ? undefined : variant };
}

async function readAgent(client: Client, name: string): Promise<AgentInfo> {
  const agents = await unwrap<unknown[]>(client.app.agents({} as never), "list agents");
  const agent = agents.map(object).find((item) => item?.name === name);
  if (!agent) {
    const names = agents.map(object).map((item) => string(item?.name)).filter(Boolean).join(", ");
    throw new Error(`delegate task argument subagent_type must be a known agent, got ${JSON.stringify(name)}. Known agents: ${names || "none"}`);
  }

  return {
    name,
    permission: agent.permission,
    model: modelRef(agent.model),
    variant: string(agent.variant),
  };
}

async function validateVariant(client: Client, model: ModelRef, variant: string | undefined) {
  if (!variant) return;
  const modelInfo = await readProviderModel(client, model);
  const variants = object(modelInfo.variants) ?? {};
  const valid = Object.keys(variants);
  if (Object.hasOwn(variants, variant)) return;
  const suffix = valid.length ? valid.join(", ") : "none";
  throw new Error(`Unknown effort ${JSON.stringify(variant)} for ${model.providerID}/${model.modelID}. Valid efforts: ${suffix}`);
}

async function readProviderModel(client: Client, model: ModelRef): Promise<Record<string, unknown>> {
  const response = await unwrap<Record<string, unknown>>(client.config.providers({} as never), "list providers");
  const providers = Array.isArray(response.providers) ? response.providers : [];
  const provider = providers.map(object).find((item) => item?.id === model.providerID);
  if (!provider) {
    const names = providers.map(object).map((item) => string(item?.id)).filter(Boolean).join(", ");
    throw new Error(`Unknown provider ${model.providerID}. Available providers: ${names}`);
  }

  const models = object(provider.models) ?? {};
  const direct = object(models[model.modelID]);
  if (direct) return direct;

  const byID = Object.values(models).map(object).find((item) => item?.id === model.modelID || object(item?.api)?.id === model.modelID);
  if (byID) return byID;

  const names = Object.keys(models).slice(0, 20).join(", ");
  throw new Error(`Unknown model ${model.providerID}/${model.modelID}. Known model keys include: ${names}`);
}

async function deriveChildPermission(
  client: Client,
  parentSessionID: string,
  agent: AgentInfo,
): Promise<{ rules: Rule[]; driveParent: boolean }> {
  const [parent, config] = await Promise.all([
    unwrap<Record<string, unknown>>(client.session.get({ path: { id: parentSessionID } } as never), `read parent session ${parentSessionID}`),
    unwrap<Record<string, unknown>>(client.config.get({} as never), "read config"),
  ]);

  const parentRules = normalizeRules(parent.permission);
  const inherited = parentRules.filter(
    (rule) => rule.permission === "external_directory" || rule.action === "deny",
  );
  const agentRules = normalizeRules(agent.permission);
  const defaultRules = defaultAgentRules(agent.name, agentRules);
  const driveParent = isDriveSession(parent);
  const driveDenies = driveParent
    ? askRulesAsDenies([...normalizeRules(config.permission), ...parentRules])
    : [];
  const childDenies: Rule[] = [
    ...(hasPermissionRule(agentRules, "todowrite") ? [] : [deny("todowrite")]),
    ...(hasPermissionRule(agentRules, "task") ? [] : [deny("task")]),
    ...primaryTools(config).map(deny),
  ];

  return {
    rules: dedupeRules([...defaultRules, ...agentRules, ...childDenies, ...driveDenies, ...inherited]),
    driveParent,
  };
}

async function readExistingChild(client: Client, sessionID: string, signal: AbortSignal) {
  const session = await unwrap<Record<string, unknown>>(
    client.session.get({ path: { id: sessionID } } as never),
    `read child session ${sessionID}`,
  );
  const id = string(session.id);
  if (!id) throw new Error(`delegate child session ${sessionID} did not return an id`);

  const statuses = await unwrap<Record<string, unknown>>(
    client.session.status({ signal } as never),
    `read child session ${sessionID} status before resume`,
  );
  const status = object(statuses[id]);
  if (status && status.type !== "idle") {
    throw new Error(`delegate cannot resume busy child session ${id}; task_id resumes require an idle child`);
  }
  return { id };
}

async function createChild(client: Client, ctx: ToolContext, args: TaskArgs, prepared: PreparedTask) {
  const session = await unwrap<Record<string, unknown>>(
    client.session.create({
      body: {
        parentID: ctx.sessionID,
        title: `${args.description} (@${prepared.agent.name} subagent)`,
        agent: prepared.agent.name,
        permission: prepared.permission,
      },
    } as never),
    `create child session for ${prepared.agent.name}`,
  );
  const id = string(session.id);
  if (!id) throw new Error("delegate child session create response did not include an id");
  return { id };
}

async function unwrap<T>(promise: Promise<unknown>, label: string): Promise<T> {
  const response = await promise;
  const envelope = object(response);
  if (envelope && "error" in envelope && envelope.error !== undefined) {
    throw new Error(`delegate ${label} failed: ${errorMessage(envelope.error)}`);
  }
  if (envelope && "data" in envelope) return envelope.data as T;
  return response as T;
}

function normalizeRules(value: unknown): Rule[] {
  if (Array.isArray(value)) return value.flatMap(parseRule);
  const root = object(value);
  if (!root) return [];

  return Object.entries(root).flatMap(([permission, entry]) => {
    if (isAction(entry)) return [{ permission, pattern: "*", action: entry }];
    const patterns = object(entry);
    if (!patterns) return [];
    return Object.entries(patterns).flatMap(([pattern, action]) => (isAction(action) ? [{ permission, pattern, action }] : []));
  });
}

function parseRule(value: unknown): Rule[] {
  const root = object(value);
  const permission = string(root?.permission);
  const pattern = string(root?.pattern);
  const action = root?.action;
  if (!permission || !pattern || !isAction(action)) return [];
  return [{ permission, pattern, action }];
}

function hasPermissionRule(rules: Rule[], permission: string) {
  return rules.some((rule) => rule.permission === permission);
}

function defaultAgentRules(agentName: string, explicitRules: Rule[]) {
  if (!agentName.startsWith("review/")) return [];

  const rules: Rule[] = [];
  for (const permission of ["read", "glob", "grep", "list", "webfetch", "websearch", "lsp"]) {
    if (!hasPermissionRule(explicitRules, permission)) {
      rules.push(allow(permission));
      if (permission === "grep") rules.push({ permission: "grep", pattern: "/", action: "deny" });
    }
  }
  for (const permission of ["edit", "bash", "task", "todowrite", "question"]) {
    if (!hasPermissionRule(explicitRules, permission)) rules.push(deny(permission));
  }
  return rules;
}

function isDriveSession(session: Record<string, unknown>) {
  const agent = session.agent;
  return string(agent) === "drive" || string(object(agent)?.name) === "drive";
}

function askRulesAsDenies(rules: Rule[]) {
  return rules
    .filter((rule) => rule.action === "ask")
    .map((rule): Rule => ({ ...rule, action: "deny" }));
}

function primaryTools(config: Record<string, unknown>) {
  const experimental = object(config.experimental);
  return Array.isArray(experimental?.primary_tools) ? experimental.primary_tools.filter((item): item is string => typeof item === "string") : [];
}

function deny(permission: string): Rule {
  return { permission, pattern: "*", action: "deny" };
}

function allow(permission: string): Rule {
  return { permission, pattern: "*", action: "allow" };
}

function dedupeRules(rules: Rule[]) {
  const seen = new Set<string>();
  return rules.filter((rule) => {
    const key = `${rule.permission}\0${rule.pattern}\0${rule.action}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function modelRef(value: unknown): ModelRef | undefined {
  if (typeof value === "string" && value.trim()) return parseModel(value);
  const root = object(value);
  const providerID = string(root?.providerID);
  const modelID = string(root?.modelID);
  return providerID && modelID ? { providerID, modelID } : undefined;
}

function lastTextPart(value: unknown) {
  const parts = object(value)?.parts;
  if (!Array.isArray(parts)) return "";
  for (let index = parts.length - 1; index >= 0; index--) {
    const part = object(parts[index]);
    if (part?.type === "text" && typeof part.text === "string") return part.text;
  }
  return "";
}

function withNotes(text: string, notes: string[]) {
  if (!notes.length) return text;
  return [`[${notes.join("; ")}]`, text].filter(Boolean).join("\n\n");
}

function blockedResult(args: TaskArgs, metadata: Record<string, unknown>, sessionID: string) {
  const text = [`blocked: content_filter`, `child_session_id: ${sessionID}`, `advice: ${CONTENT_FILTER_ADVICE}`].join("\n");
  return {
    title: args.description,
    metadata,
    output: renderOutput({ sessionID, state: "error", text }),
  };
}

function interruptedResult(
  args: TaskArgs,
  metadata: Record<string, unknown>,
  sessionID: string,
  notes: string[],
  reason = "child became idle without assistant output",
) {
  const text = withNotes(
    [`interrupted: ${reason}`, `child_session_id: ${sessionID}`, `advice: ${INTERRUPTED_ADVICE}`].join("\n"),
    notes,
  );
  return {
    title: args.description,
    metadata,
    output: renderOutput({ sessionID, state: "error", text }),
  };
}

function renderOutput(input: { sessionID: string; state: "completed" | "error"; text: string }) {
  const tag = input.state === "error" ? "task_error" : "task_result";
  return [`<task id="${input.sessionID}" state="${input.state}">`, `<${tag}>`, input.text, `</${tag}>`, "</task>"].join("\n");
}

function isContentFilterBlock(error: unknown) {
  const root = object(error);
  const name = string(root?.name) ?? (error instanceof Error ? error.name : undefined);
  if (isContentFilterText(name)) return true;

  const data = object(root?.data);
  const message = string(root?.message) ?? string(data?.message) ?? (error instanceof Error || typeof error === "string" ? String(error) : undefined);
  return isContentFilterText(message);
}

function isContentFilterText(value: string | undefined) {
  if (!value) return false;
  const compact = value.toLowerCase().replace(/[^a-z]/gu, "");
  return compact.includes("contentfilter") || compact.includes("refusal");
}

function object(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;
}

function isPromiseLike(value: unknown): value is PromiseLike<unknown> {
  return typeof object(value)?.then === "function";
}

function isEffectLike(value: unknown) {
  const root = object(value);
  return !!root && (typeof root.pipe === "function" || typeof root._op === "string");
}

async function effectRunPromise() {
  let mod: Record<string, unknown> | undefined;
  try {
    const dynamicImport = new Function("specifier", "return import(specifier)") as (specifier: string) => Promise<unknown>;
    mod = object(await dynamicImport("effect"));
  } catch (error) {
    throw new Error(`delegate failed to import effect for metadata update: ${errorMessage(error)}`);
  }
  const runPromise = object(mod?.Effect)?.runPromise;
  if (typeof runPromise !== "function") {
    throw new Error("delegate effect module is missing Effect.runPromise for metadata update");
  }
  return (effect: unknown) => Promise.resolve((runPromise as (effect: unknown) => unknown)(effect));
}

function string(value: unknown) {
  return typeof value === "string" && value ? value : undefined;
}

function isAction(value: unknown): value is Rule["action"] {
  return value === "allow" || value === "ask" || value === "deny";
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  try {
    return JSON.stringify(error);
  } catch {
    return String(error);
  }
}
