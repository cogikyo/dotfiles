import type { ToolContext } from "@opencode-ai/plugin";
import { type Agent, agents, type Client, message, providers, type Rule } from "../shared/opencode.ts";
import type { Execution } from "./permission.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Task arguments                                                                                │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

/** Inputs to the delegate `task` tool before defaults and inheritance are resolved. */
export type TaskArgs = {
  description: string;
  prompt: string;
  subagent_type: string;
  /** `provider/model-id`; omitted means use the agent pin or parent model. */
  model?: string;
  /** Omitted means inherit effort unless the model is explicit. */
  effort?: string;
  /** Named reusable child; omitted means one-shot. */
  lane?: string;
  /** Summarizes an idle lane before its next prompt. */
  compact?: boolean;
  /** Defaults to true; an unattended parent cannot have an attended child. */
  unattended?: boolean;
};

export type ModelRef = {
  providerID: string;
  modelID: string;
};

/** Resolved agent, model, permissions, and execution mode for one task call. */
export type PreparedTask = {
  args: TaskArgs;
  agent: Agent;
  model: ModelRef;
  variant?: string;
  permission: Rule[];
  execution: Execution;
};

// ├─ Normalization ───────────────────────────────────────────────────────────────────────────────┤

/** Trims display and selector fields and rejects blank values or multiline lane names. */
export function taskArgs(input: TaskArgs): TaskArgs {
  const { description, prompt, subagent_type, model, effort, lane } = input;
  for (const [name, value] of Object.entries({ description, prompt, subagent_type })) {
    if (!value.trim()) throw new Error(`delegate task argument ${name} must not be empty`);
  }
  for (const [name, value] of Object.entries({ model, effort, lane })) {
    if (value !== undefined && !value.trim()) {
      throw new Error(`delegate task argument ${name} must not be empty when provided`);
    }
  }
  if (lane && /[\r\n]/u.test(lane)) throw new Error("delegate lane name must be one line");
  return {
    ...input,
    description: description.trim(),
    subagent_type: subagent_type.trim(),
    effort: effort?.trim(),
    lane: lane?.trim(),
  };
}

/** Keeps the tool input and prepared display label in sync without stacking effort suffixes. */
export function applyDisplayArgs(input: TaskArgs, args: TaskArgs) {
  const base = stripEffortSuffix(args.description, args.effort);
  args.description = args.effort ? `${base} · ${args.effort}` : base;
  input.description = args.description;
  input.subagent_type = args.subagent_type;
  if (args.effort) input.effort = args.effort;
}

const KNOWN_EFFORTS = new Set(["default", "minimal", "low", "medium", "high", "xhigh"]);

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

// ├─ Model and agent resolution ──────────────────────────────────────────────────────────────────┤

/** Parses `provider/model-id`, allowing slashes inside the model ID. */
export function parseModel(value: string): ModelRef {
  const clean = value.trim();
  const slash = clean.indexOf("/");
  if (slash <= 0 || slash === clean.length - 1) {
    throw new Error(`delegate model must be provider/model-id, got ${JSON.stringify(value)}`);
  }
  return { providerID: clean.slice(0, slash), modelID: clean.slice(slash + 1) };
}

/** Reads the parent assistant's model and effort for inheritance, omitting the `default` variant. */
export async function readCurrentAssistantMessage(client: Client, ctx: ToolContext) {
  const { info } = await message(client, ctx.sessionID, ctx.messageID, {
    label: `delegate read parent message ${ctx.messageID}`,
  });
  if (info.role !== "assistant") {
    throw new Error("delegate cannot inherit model because the current message is not an assistant message");
  }
  const { providerID, modelID, variant } = info;
  if (!providerID || !modelID) throw new Error("delegate cannot inherit model because parent message lacks model IDs");
  return { model: { providerID, modelID }, variant: variant === "default" ? undefined : variant };
}

/** Resolves an agent by name and lists available names when it is unknown. */
export async function readAgent(client: Client, name: string): Promise<Agent> {
  const known = await agents(client, { label: "delegate list agents" });
  const agent = known.find((item) => item.name === name);
  if (agent) return agent;
  const names = known.map((item) => item.name).join(", ");
  throw new Error(
    `delegate task argument subagent_type must be a known agent, got ${JSON.stringify(name)}. Known agents: ${names || "none"}`,
  );
}

/** Checks a requested effort against the target model's variants; an omitted effort needs no check. */
export async function validateVariant(client: Client, model: ModelRef, variant: string | undefined) {
  if (!variant) return;
  const variants = (await readProviderModel(client, model)).variants ?? {};
  if (Object.hasOwn(variants, variant)) return;
  const valid = Object.keys(variants);
  const suffix = valid.length ? valid.join(", ") : "none";
  throw new Error(
    `Unknown effort ${JSON.stringify(variant)} for ${model.providerID}/${model.modelID}. Valid efforts: ${suffix}`,
  );
}

async function readProviderModel(client: Client, model: ModelRef) {
  const known = await providers(client, { label: "delegate list providers" });
  const provider = known.find((item) => item.id === model.providerID);
  if (!provider) {
    const names = known.map((item) => item.id).join(", ");
    throw new Error(`Unknown provider ${model.providerID}. Available providers: ${names}`);
  }

  const models = provider.models;
  const match = Object.hasOwn(models, model.modelID)
    ? models[model.modelID]
    : Object.values(models).find((item) => item.id === model.modelID || item.api.id === model.modelID);
  if (match) return match;

  const names = Object.keys(models).slice(0, 20).join(", ");
  throw new Error(`Unknown model ${model.providerID}/${model.modelID}. Known model keys include: ${names}`);
}
