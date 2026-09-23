import type { ToolContext } from "@opencode-ai/plugin";
import { record } from "../shared/record.ts";
import type { Execution, Rule } from "./permission.ts";
import { type Client, string, unwrap } from "./sdk.ts";

/** Arguments accepted by the delegate `task` tool. */
export type TaskArgs = {
  description: string;
  prompt: string;
  subagent_type: string;
  model?: string;
  effort?: string;
  lane?: string;
  compact?: boolean;
  unattended?: boolean;
};

/** OpenCode provider and model identifiers used to select a child model. */
export type ModelRef = {
  providerID: string;
  modelID: string;
};

export type AgentInfo = {
  name: string;
  permission?: unknown;
  model?: ModelRef;
  variant?: string;
};

export type PreparedTask = {
  args: TaskArgs;
  agent: AgentInfo;
  model: ModelRef;
  variant?: string;
  permission: Rule[];
  execution: Execution;
};

const KNOWN_EFFORTS = new Set(["default", "minimal", "low", "medium", "high", "xhigh"]);

export function parseModel(value: string): ModelRef {
  const clean = value.trim();
  const slash = clean.indexOf("/");
  if (slash <= 0 || slash === clean.length - 1) {
    throw new Error(`delegate model must be provider/model-id, got ${JSON.stringify(value)}`);
  }
  return { providerID: clean.slice(0, slash), modelID: clean.slice(slash + 1) };
}

export function taskArgs(value: unknown): TaskArgs {
  const root = record(value);
  if (!root) throw new Error("delegate task arguments must be an object");

  const args: TaskArgs = {
    description: requiredString(root, "description").trim(),
    prompt: requiredString(root, "prompt"),
    subagent_type: requiredString(root, "subagent_type").trim(),
  };
  const model = optionalString(root, "model");
  const effort = optionalString(root, "effort");
  const lane = optionalString(root, "lane");
  if (lane && /[\r\n]/u.test(lane)) throw new Error("delegate lane name must be one line");
  const compact = optionalBoolean(root, "compact");
  const unattended = optionalBoolean(root, "unattended");
  if (model !== undefined) args.model = model;
  if (effort !== undefined) args.effort = effort;
  if (lane !== undefined) args.lane = lane.trim();
  if (compact !== undefined) args.compact = compact;
  if (unattended !== undefined) args.unattended = unattended;
  return args;
}

export function applyDisplayArgs(input: unknown, args: TaskArgs, effort: string | undefined) {
  const base = stripEffortSuffix(args.description, effort);
  const description = effort ? `${base} · ${effort}` : base;
  args.description = description;
  if (effort) args.effort = effort;

  const root = record(input);
  if (!root) return;
  root.description = description;
  root.subagent_type = args.subagent_type;
  if (effort) root.effort = effort;
  if (args.unattended !== undefined) root.unattended = args.unattended;
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

export function parseEffort(args: TaskArgs) {
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

function optionalBoolean(root: Record<string, unknown>, name: "compact" | "unattended") {
  if (!Object.hasOwn(root, name) || root[name] === undefined) return undefined;
  if (typeof root[name] !== "boolean") throw new Error(`delegate task argument ${name} must be a boolean`);
  return root[name];
}

export async function readCurrentAssistantMessage(client: Client, ctx: ToolContext) {
  const message = await unwrap<Record<string, unknown>>(
    client.session.message({ path: { id: ctx.sessionID, messageID: ctx.messageID } }),
    `read parent message ${ctx.messageID}`,
  );
  const info = record(message.info);
  if (!info || info.role !== "assistant") {
    throw new Error("delegate cannot inherit model because the current message is not an assistant message");
  }

  const providerID = string(info.providerID) ?? string(record(info.model)?.providerID);
  const modelID = string(info.modelID) ?? string(record(info.model)?.modelID);
  if (!providerID || !modelID) throw new Error("delegate cannot inherit model because parent message lacks model IDs");

  const variant = string(info.variant) ?? string(record(info.model)?.variant);
  return { model: { providerID, modelID }, variant: variant === "default" ? undefined : variant };
}

export async function readAgent(client: Client, name: string): Promise<AgentInfo> {
  const agents = await unwrap<unknown[]>(client.app.agents({}), "list agents");
  const agent = agents.map(record).find((item) => item?.name === name);
  if (!agent) {
    const names = agents
      .map(record)
      .map((item) => string(item?.name))
      .filter(Boolean)
      .join(", ");
    throw new Error(
      `delegate task argument subagent_type must be a known agent, got ${JSON.stringify(name)}. Known agents: ${names || "none"}`,
    );
  }

  return {
    name,
    permission: agent.permission,
    model: modelRef(agent.model),
    variant: string(agent.variant),
  };
}

export async function validateVariant(client: Client, model: ModelRef, variant: string | undefined) {
  if (!variant) return;
  const modelInfo = await readProviderModel(client, model);
  const variants = record(modelInfo.variants) ?? {};
  const valid = Object.keys(variants);
  if (Object.hasOwn(variants, variant)) return;
  const suffix = valid.length ? valid.join(", ") : "none";
  throw new Error(
    `Unknown effort ${JSON.stringify(variant)} for ${model.providerID}/${model.modelID}. Valid efforts: ${suffix}`,
  );
}

async function readProviderModel(client: Client, model: ModelRef): Promise<Record<string, unknown>> {
  const response = await unwrap<Record<string, unknown>>(client.config.providers({}), "list providers");
  const providers = Array.isArray(response.providers) ? response.providers : [];
  const provider = providers.map(record).find((item) => item?.id === model.providerID);
  if (!provider) {
    const names = providers
      .map(record)
      .map((item) => string(item?.id))
      .filter(Boolean)
      .join(", ");
    throw new Error(`Unknown provider ${model.providerID}. Available providers: ${names}`);
  }

  const models = record(provider.models) ?? {};
  const direct = record(models[model.modelID]);
  if (direct) return direct;

  const byID = Object.values(models)
    .map(record)
    .find((item) => item?.id === model.modelID || record(item?.api)?.id === model.modelID);
  if (byID) return byID;

  const names = Object.keys(models).slice(0, 20).join(", ");
  throw new Error(`Unknown model ${model.providerID}/${model.modelID}. Known model keys include: ${names}`);
}

function modelRef(value: unknown): ModelRef | undefined {
  if (typeof value === "string" && value.trim()) return parseModel(value);
  const root = record(value);
  const providerID = string(root?.providerID);
  const modelID = string(root?.modelID);
  return providerID && modelID ? { providerID, modelID } : undefined;
}
