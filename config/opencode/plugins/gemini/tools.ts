import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
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
const bytes = 8 * 1024;

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

export async function called(result: Result, file: string): Promise<Called> {
  const items =
    typeof result.content === "string"
      ? [{ type: "text" as const, text: result.content }]
      : (result.content ?? [{ type: "text" as const, text: text(result.output) }]);
  return { content: await spill(items.map(content), file) };
}

export function failed(reason: string): Called {
  return { content: [{ type: "text", text: reason }], isError: true };
}

async function spill(items: Content[], file: string): Promise<Content[]> {
  const full = items.flatMap((item) => (item.type === "text" ? [item.text] : [])).join("\n");
  const total = Buffer.byteLength(full);
  if (total <= bytes) return items;
  const cut = Buffer.from(full)
    .subarray(0, bytes)
    .toString()
    .replace(/\uFFFD$/, "");
  const end = cut.lastIndexOf("\n");
  const head = end > 0 ? cut.slice(0, end) : cut;
  const shown = `showing the first ${Buffer.byteLength(head)} of ${total} bytes`;
  const marker = await mkdir(path.dirname(file), { recursive: true })
    .then(() => writeFile(file, full))
    .then(
      () => `[${shown}; the full output is in ${file}, so read the rest from there]`,
      (error: unknown) => `[${shown}; saving the full output to ${file} failed: ${String(error)}]`,
    );
  return [{ type: "text", text: `${head}\n${marker}` }, ...items.filter((item) => item.type !== "text")];
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
