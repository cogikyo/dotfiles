import type { TuiPluginApi } from "@opencode-ai/plugin/tui";
import type { Message, Model, Provider } from "@opencode-ai/sdk/v2";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Session display                                                                               │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

// ├─ Limits ──────────────────────────────────────────────────────────────────────────────────────┤

export const COMPACTION_LIMIT = 225_000; // Hard token stop for delegate children.
export const COMPACTION_RESERVED = 25_000; // Default token budget reserved below the model input limit.

/** Token thresholds for delegate context warnings and the hard stop. */
export const CONTEXT_PRESSURE = {
  soft: 100_000,
  medium: 150_000,
  final: 200_000,
  hard: COMPACTION_LIMIT,
} as const;

// ├─ Readers ─────────────────────────────────────────────────────────────────────────────────────┤

export type SessionMeta = {
  agent: string;
  providerID: string;
  providerName: string;
  modelID: string;
  modelName: string;
  variant: string;
  cwd: string;
};

export type SessionUsage = {
  tokens: number;
  limit?: number;
  percent: number;
  colorPercent: number;
};

type AssistantLike = Extract<Message, { role: "assistant" }>;
type UserLike = Extract<Message, { role: "user" }>;

export function sessionMessages(api: TuiPluginApi, sessionID: string) {
  return api.state.session.messages(sessionID);
}

/** Uses the latest user or assistant message to identify the session's provider. */
export function sessionProviderID(api: TuiPluginApi, sessionID: string) {
  return providerIDFor(latestModelMessage(sessionMessages(api, sessionID)));
}

/** Resolves session labels and cwd from the latest message and TUI state, defaulting the agent to Build. */
export function sessionMeta(api: TuiPluginApi, sessionID: string): SessionMeta {
  const messages = sessionMessages(api, sessionID);
  const latest = latestModelMessage(messages);
  const providerID = sessionProviderID(api, sessionID);
  const modelID = modelIDFor(latest);
  const model = findModel(api.state.provider, providerID, modelID);

  return {
    agent: title(latest?.agent || "Build"),
    providerID,
    providerName: providerLabel(providerID),
    modelID,
    modelName: model?.name || modelLabel(modelID),
    variant: variantFor(latest),
    cwd: cwdFor(latest) || api.state.path.directory || api.state.path.worktree || "",
  };
}

/** Measures the latest assistant message with output tokens against the model context limit. */
export function sessionUsage(api: TuiPluginApi, sessionID: string): SessionUsage {
  const messages = sessionMessages(api, sessionID);
  const meta = sessionMeta(api, sessionID);
  const model = findModel(api.state.provider, meta.providerID, meta.modelID);
  const tokens = tokenTotal(latestAssistantMessage(messages));
  const limit = model?.limit.context;
  const percent = limit ? Math.min(100, (tokens / limit) * 100) : 0;

  return { tokens, limit, percent, colorPercent: percent };
}

/** Measures the latest assistant message with output tokens against the compaction limit. */
export function sessionContextUsage(api: TuiPluginApi, sessionID: string): SessionUsage {
  const messages = sessionMessages(api, sessionID);
  const meta = sessionMeta(api, sessionID);
  const model = findModel(api.state.provider, meta.providerID, meta.modelID);
  const tokens = contextTokenTotal(latestAssistantMessage(messages));
  const limit = contextCompactionLimit(model, compactionReserved(api));
  const percent = limit ? Math.min(100, (tokens / limit) * 100) : 0;

  return { tokens, limit, percent, colorPercent: percent };
}

// ├─ Threshold ───────────────────────────────────────────────────────────────────────────────────┤

/** Adds the reserved token budget to the delegate hard stop. */
export function compactionInputCap(reserved = COMPACTION_RESERVED) {
  return COMPACTION_LIMIT + reserved;
}

/** Finds the pre-compaction input threshold from the model limits, if available. */
export function contextCompactionLimit(model: Pick<Model, "limit"> | undefined, reserved: number) {
  if (!model || !model.limit.context) return undefined;
  const context = model.limit.context;
  const input = model.limit.input;
  if (typeof input === "number" && input > 0) return Math.max(0, input - reserved);
  const output = Math.min(model.limit.output, 32_000) || 32_000;
  return Math.max(0, context - output);
}

// ├─ Formatting ──────────────────────────────────────────────────────────────────────────────────┤

/** Abbreviates paths inside HOME with `~`. */
export function shortDir(dir: string) {
  if (!dir) return "";
  const home = process.env.HOME;
  if (home && dir === home) return "~";
  if (home && dir.startsWith(home + "/")) return "~/" + dir.slice(home.length + 1);
  return dir;
}

/** Formats token counts with K or M suffixes. */
export function formatTokens(tokens: number) {
  if (tokens >= 1_000_000) return `${trim(tokens / 1_000_000)}M`;
  if (tokens >= 1_000) return `${trim(tokens / 1_000)}K`;
  return String(tokens);
}

// ├─ Lookup ──────────────────────────────────────────────────────────────────────────────────────┤

function compactionReserved(api: TuiPluginApi) {
  const reserved = api.state.config.compaction?.reserved;
  return typeof reserved === "number" && reserved >= 0 ? reserved : COMPACTION_RESERVED;
}

function latestModelMessage(messages: ReadonlyArray<Message>): (AssistantLike | UserLike) | undefined {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message.role === "assistant" || message.role === "user") return message;
  }
  return undefined;
}

function latestAssistantMessage(messages: ReadonlyArray<Message>): AssistantLike | undefined {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message.role === "assistant" && message.tokens.output > 0) return message;
  }
  return undefined;
}

function providerIDFor(message?: AssistantLike | UserLike) {
  if (!message) return "";
  return message.role === "assistant" ? message.providerID : message.model.providerID;
}

function modelIDFor(message?: AssistantLike | UserLike) {
  if (!message) return "";
  return message.role === "assistant" ? message.modelID : message.model.modelID;
}

function variantFor(message?: AssistantLike | UserLike) {
  if (!message) return "";
  return message.role === "assistant" ? message.variant || "" : message.model.variant || "";
}

function cwdFor(message?: AssistantLike | UserLike) {
  if (!message || message.role !== "assistant") return "";
  return message.path.cwd;
}

function findModel(providers: ReadonlyArray<Provider>, providerID: string, modelID: string): Model | undefined {
  if (!providerID || !modelID) return undefined;
  return providers.find((provider) => provider.id === providerID)?.models[modelID];
}

function tokenTotal(message?: AssistantLike) {
  if (!message) return 0;
  const tokens = message.tokens;
  return tokens.input + tokens.output + tokens.reasoning + tokens.cache.read + tokens.cache.write;
}

export function contextTokenTotal(message?: {
  tokens: Pick<AssistantLike["tokens"], "total" | "input" | "output" | "cache">;
}) {
  if (!message) return 0;
  const tokens = message.tokens;
  return tokens.total || tokens.input + tokens.output + tokens.cache.read + tokens.cache.write;
}

function providerLabel(providerID: string) {
  if (providerID === "openai") return "OpenAI";
  if (providerID === "anthropic") return "Claude";
  return title(providerID);
}

function modelLabel(modelID: string) {
  return modelID
    .replace(/^claude-/, "Claude ")
    .replace(/^gpt-/, "GPT-")
    .split(/[-_]/)
    .filter(Boolean)
    .map((part) => (/^\d/.test(part) || part.toUpperCase() === part ? part : title(part)))
    .join(" ");
}

function title(value: string) {
  if (!value) return "";
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function trim(value: number) {
  return value >= 100 ? value.toFixed(0) : value >= 10 ? value.toFixed(1) : value.toFixed(2);
}
