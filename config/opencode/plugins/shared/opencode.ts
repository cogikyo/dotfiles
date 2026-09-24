import type { PluginInput } from "@opencode-ai/plugin";
import type { SessionPromptData } from "@opencode-ai/sdk";
import type * as v2 from "@opencode-ai/sdk/v2";
import { z } from "zod";
import { errorMessage } from "./error.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Server client views                                                                           │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

// Server-only readers use the v1 client result shape and validate its wire data against v2 views.
// TUI plugins use a v2 client and must not value-import this module.

// ├─ Transport ───────────────────────────────────────────────────────────────────────────────────┤

/** Server plugin client; the readers below expect its v1 result shape. */
export type Client = PluginInput["client"];

/** Labels client errors and optionally passes through an abort signal. */
export type Call = { label: string; signal?: AbortSignal };

// ├─ Views ───────────────────────────────────────────────────────────────────────────────────────┤

// Schemas validate the fields these v2 views retain and discard the rest.

const Action = z.enum(["allow", "deny", "ask"]);

export type Rule = v2.PermissionRule;
const Rule: z.ZodType<Rule> = z.object({ permission: z.string(), pattern: z.string(), action: Action });

export type Permission = v2.PermissionActionConfig | Record<string, v2.PermissionRuleConfig>;
const Permission: z.ZodType<Permission> = z.union([
  Action,
  z.record(z.string(), z.union([Action, z.record(z.string(), Action)])),
]);

export type Session = Pick<
  v2.Session,
  "id" | "title" | "agent" | "parentID" | "directory" | "metadata" | "permission"
> & { time: Pick<v2.Session["time"], "created" | "updated"> };
const Session: z.ZodType<Session> = z.object({
  id: z.string(),
  title: z.string(),
  agent: z.string().optional(),
  parentID: z.string().optional(),
  directory: z.string(),
  metadata: z.record(z.string(), z.unknown()).optional(),
  permission: z.array(Rule).optional(),
  time: z.object({ created: z.number(), updated: z.number() }),
});

type Retry = Extract<v2.SessionStatus, { type: "retry" }>;

/** Session status narrowed to idle, busy, or retry without retry actions. */
export type Status = Exclude<v2.SessionStatus, Retry> | Pick<Retry, "type" | "attempt" | "message" | "next">;
const Status: z.ZodType<Status> = z.discriminatedUnion("type", [
  z.object({ type: z.literal("idle") }),
  z.object({ type: z.literal("busy") }),
  z.object({ type: z.literal("retry"), attempt: z.number(), message: z.string(), next: z.number() }),
]);

export type User = Pick<v2.UserMessage, "id" | "role" | "model">;
const User = z.object({
  id: z.string(),
  role: z.literal("user"),
  model: z.object({ providerID: z.string(), modelID: z.string(), variant: z.string().optional() }),
});

type Tokens = v2.AssistantMessage["tokens"];

/** Assistant view whose wire error is a plain object, not necessarily an Error instance. */
export type Assistant = Pick<v2.AssistantMessage, "id" | "role" | "providerID" | "modelID" | "variant" | "finish"> & {
  error?: unknown;
  tokens: Pick<Tokens, "total" | "input" | "output" | "cache">;
};
const Assistant = z.object({
  id: z.string(),
  role: z.literal("assistant"),
  providerID: z.string(),
  modelID: z.string(),
  variant: z.string().optional(),
  finish: z.string().optional(),
  error: z.unknown(),
  tokens: z.object({
    total: z.number().optional(),
    input: z.number(),
    output: z.number(),
    cache: z.object({ read: z.number(), write: z.number() }),
  }),
});

/** Message parts retained by the server readers. */
export type Part = Pick<v2.TextPart, "type" | "text"> | Pick<v2.CompactionPart, "type" | "auto">;
const Part = z.discriminatedUnion("type", [
  z.object({ type: z.literal("text"), text: z.string() }),
  z.object({ type: z.literal("compaction"), auto: z.boolean() }),
]);
const kinds = new Set<string>(Part.options.map((option) => option.shape.type.value));
const Parts: z.ZodType<Part[]> = z
  .array(z.looseObject({ type: z.string() }))
  .transform((items) => items.filter((item) => kinds.has(item.type)))
  .pipe(z.array(Part));

/** Session message with only text and compaction parts. */
export type Message = { info: User | Assistant; parts: Part[] };
const Message: z.ZodType<Message> = z.object({ info: z.discriminatedUnion("role", [User, Assistant]), parts: Parts });

export type Reply = { info: Assistant; parts: Part[] };
const Reply: z.ZodType<Reply> = z.object({ info: Assistant, parts: Parts });

export type Agent = Pick<v2.Agent, "name" | "permission" | "model" | "variant">;
const Agent: z.ZodType<Agent> = z.object({
  name: z.string(),
  permission: z.array(Rule),
  model: z.object({ providerID: z.string(), modelID: z.string() }).optional(),
  variant: z.string().optional(),
});

export type Model = Pick<v2.Model, "id" | "variants"> & { api: Pick<v2.Model["api"], "id"> };
const Model: z.ZodType<Model> = z.object({
  id: z.string(),
  api: z.object({ id: z.string() }),
  variants: z.record(z.string(), z.record(z.string(), z.unknown())).optional(),
});

export type Provider = Pick<v2.Provider, "id"> & { models: Record<string, Model> };
const Providers: z.ZodType<Provider[]> = z
  .object({ providers: z.array(z.object({ id: z.string(), models: z.record(z.string(), Model) })) })
  .transform((response) => response.providers);

export type Config = {
  agent?: Record<string, { permission?: Permission }>;
  experimental?: Pick<NonNullable<v2.Config["experimental"]>, "primary_tools">;
};
const Config: z.ZodType<Config> = z.object({
  agent: z.record(z.string(), z.object({ permission: Permission.optional() })).optional(),
  experimental: z.object({ primary_tools: z.array(z.string()).optional() }).optional(),
});

// ├─ Readers ─────────────────────────────────────────────────────────────────────────────────────┤

type Result<T> = { data: T; error: undefined } | { data: undefined; error: unknown };

/** Unwraps a v1 result, throwing a labeled error for failure or missing data. */
export async function unwrap<T>(promise: Promise<Result<T>>, label: string): Promise<T> {
  const response = await promise;
  if (response.error !== undefined) throw new Error(`${label} failed: ${errorMessage(response.error)}`);
  if (response.data === undefined) throw new Error(`${label} returned no data`);
  return response.data;
}

export function session(client: Client, id: string, { label, signal }: Call) {
  return read(client.session.get({ path: { id }, signal }), Session, label);
}

export function children(client: Client, id: string, { label, signal }: Call) {
  return read(client.session.children({ path: { id }, signal }), z.array(Session), label);
}

/** Creates a session using v2 body fields missing from the v1 generated types. */
export function create(client: Client, body: NonNullable<v2.SessionCreateData["body"]>, { label, signal }: Call) {
  return read(client.session.create({ body, signal }), Session, label);
}

export function statuses(client: Client, { label, signal }: Call) {
  return read(client.session.status({ signal }), z.record(z.string(), Status), label);
}

/** Reads session messages with only text and compaction parts. */
export function messages(client: Client, id: string, { label, signal }: Call) {
  return read(client.session.messages({ path: { id }, signal }), z.array(Message), label);
}

export function message(client: Client, id: string, messageID: string, { label, signal }: Call) {
  return read(client.session.message({ path: { id, messageID }, signal }), Message, label);
}

export function prompt(client: Client, id: string, body: NonNullable<SessionPromptData["body"]>, call: Call) {
  return read(client.session.prompt({ path: { id }, body, signal: call.signal }), Reply, call.label);
}

export function agents(client: Client, { label, signal }: Call) {
  return read(client.app.agents({ signal }), z.array(Agent), label);
}

/** Lists provider models without the response's default-provider map. */
export function providers(client: Client, { label, signal }: Call) {
  return read(client.config.providers({ signal }), Providers, label);
}

export function config(client: Client, { label, signal }: Call) {
  return read(client.config.get({ signal }), Config, label);
}

// ├─ Parse ───────────────────────────────────────────────────────────────────────────────────────┤

/** Validates a v1 response against its v2 view, preserving the Zod error as the cause. */
async function read<T>(promise: Promise<Result<unknown>>, schema: z.ZodType<T>, label: string): Promise<T> {
  const result = schema.safeParse(await unwrap(promise, label));
  if (result.success) return result.data;
  throw new Error(`${label} returned unexpected data: ${z.prettifyError(result.error)}`, { cause: result.error });
}
