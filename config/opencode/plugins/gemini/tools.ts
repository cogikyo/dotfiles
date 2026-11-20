import type { Plugin } from "@opencode/plugin";
import { Schema } from "effect";
import type { Called, Content, Listed } from "./bridge.ts";
import type { Owner } from "./gate.ts";
import { gemini } from "./schema.ts";

export type Tool = Awaited<ReturnType<Plugin.Context["tool"]["list"]>>[number];

type Result = Awaited<ReturnType<Tool["execute"]>>;

type Rule = { readonly action: string; readonly resource: string; readonly effect: string };

const duplicates = ["shell", "read", "write", "edit", "patch"];
const interactive = [
  "question",
  "subagent",
  "terminal",
  "mail",
  "lanes",
  "lanes_compact",
  "lanes_close",
  "opencode_session_create",
  "drive_off",
  "compact",
];
const audited = ["x"];
const skipped = new Set([...duplicates, ...interactive, ...audited]);
const lines = 2_000;
const bytes = 50 * 1024;

export async function catalog(ctx: Plugin.Context, owner: Owner): Promise<Map<string, Tool>> {
  const { session, agent } = owner;
  if (!session || !agent) return new Map();
  const [tools, info, found] = await Promise.all([
    ctx.tool.list(),
    ctx.session.get({ sessionID: session }),
    ctx.agent.get({ agentID: agent, location: { directory: owner.directory } }),
  ]);
  const rules: readonly Rule[] = [...found.data.permissions, ...(info.permissions ?? [])];
  return new Map(
    tools
      .filter((tool) => !skipped.has(tool.id) && !disabled(tool.options?.permission ?? tool.id, rules))
      .map((tool) => [tool.id, tool]),
  );
}

export function listed(tool: Tool): Listed | undefined {
  const inputSchema = gemini(tool.input);
  return inputSchema && { name: tool.id, description: tool.description, inputSchema };
}

export async function decode(tool: Tool, input: Record<string, unknown>) {
  if (!Schema.isSchema(tool.input)) return { value: input };
  return Schema.decodeUnknownPromise(tool.input as Schema.Codec<unknown>)(input, { errors: "all" }).then(
    (value) => ({ value }),
    (error: unknown) => ({
      problem: `Invalid arguments for \`${tool.id}\`: ${error instanceof Error ? error.message : String(error)}`,
    }),
  );
}

export function called(result: Result): Called {
  const items =
    typeof result.content === "string"
      ? [{ type: "text" as const, text: result.content }]
      : (result.content ?? [{ type: "text" as const, text: text(result.output) }]);
  return { content: cap(items.map(content)) };
}

export function failed(reason: string): Called {
  return { content: [{ type: "text", text: reason }], isError: true };
}

function cap(items: Content[]): Content[] {
  const full = items.flatMap((item) => (item.type === "text" ? [item.text] : [])).join("\n");
  const total = full.split("\n").length;
  if (total <= lines && Buffer.byteLength(full) <= bytes) return items;
  const shown = Buffer.from(full.split("\n").slice(0, lines).join("\n"))
    .subarray(0, bytes)
    .toString()
    .replace(/\uFFFD$/, "");
  const count = shown.split("\n").length;
  const marker = `[showing lines 1-${count} of ${total}; output is capped at ${lines} lines or 50 KiB, so narrow the query with filters, a smaller limit, or an offset to see the rest]`;
  return [{ type: "text", text: `${shown}\n${marker}` }, ...items.filter((item) => item.type !== "text")];
}

function content(item: NonNullable<Exclude<Result["content"], string>>[number]): Content {
  if (item.type === "text") return item;
  const data = item.mime.startsWith("image/") ? /^data:[^,]*;base64,(.*)$/s.exec(item.uri)?.[1] : undefined;
  if (data) return { type: "image", data, mimeType: item.mime };
  return { type: "text", text: `${item.name ?? "file"} (${item.mime}): ${item.uri}` };
}

function text(output: unknown) {
  return typeof output === "string" ? output : (JSON.stringify(output) ?? "");
}

function disabled(action: string, rules: readonly Rule[]) {
  const rule = rules.findLast((item) => match(action, item.action));
  return rule?.resource === "*" && rule.effect === "deny";
}

function match(input: string, pattern: string) {
  const source = pattern
    .replace(/[.+^${}()|[\]\\]/g, "\\$&")
    .replace(/\*/g, ".*")
    .replace(/\?/g, ".");
  return new RegExp(`^${source}$`, "s").test(input);
}
