import { existsSync, statSync } from "node:fs";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { within } from "../guard/paths.ts";
import { review } from "../guard/review.ts";
import { launches, literalPath, opaque, redirection, streamed, unseen, type Launch } from "../guard/shell.ts";
import { cages } from "./agy.ts";
import type { Owner } from "./gate.ts";

export type Call = {
  toolCallId: string;
  name?: string;
  kind?: string;
  title?: string;
  rawInput?: unknown;
};

export type Need = { action: string; resources: string[]; save: string[] };

export type Plan = { refuse: string } | { tool?: string; input: Record<string, unknown> };

type Place = { directory: string; root: string | undefined };

const tools: Record<string, (input: Record<string, unknown>, directory: string) => Plan> = {
  read_file: (input, directory) => file("read", input, directory),
  write_file: (input, directory) => file("write", input, directory),
  edit_file: (input, directory) => file("edit", input, directory),
  run_command: shell,
  command_status: (input) => ({ input }),
};

const prefix = "mcp_opencode_";

export function bridged(name: unknown) {
  return typeof name === "string" && name.startsWith(prefix) ? name.slice(prefix.length) : undefined;
}

export function refusal(name: unknown) {
  if (typeof name === "string" && (name in tools || bridged(name))) return undefined;
  return `\`${String(name)}\` is not available in this OpenCode session; use ${Object.keys(tools)
    .map((tool) => `\`${tool}\``)
    .join(", ")} or a \`${prefix}*\` tool`;
}

export function plan(call: Call, directory: string): Plan {
  if (call.toolCallId.startsWith("interaction_"))
    return { refuse: "this OpenCode session cannot show agent questions; ask in your reply instead" };
  const reason = refusal(call.name);
  if (reason) return { refuse: reason };
  const input = object(call.rawInput) ? call.rawInput : {};
  if (bridged(call.name)) return { input };
  return tools[call.name!](input, directory);
}

export function needs(tool: string | undefined, input: Record<string, unknown>, directory: string): Need[] {
  const place = { directory, root: project(directory) };
  if (tool === "shell") return commands(String(input.command), String(input.workdir), place);
  if (tool === "read" || tool === "write" || tool === "edit") return access(tool, String(input.path), place);
  return [];
}

export function sandbox(line: string, workdir: string) {
  return { CommandLine: line, Cwd: workdir, BypassSandbox: true };
}

export async function check(
  steps: Exclude<Plan, { refuse: string }>,
  owner: Owner & { session: string },
): Promise<{ refuse: string } | { input: Record<string, unknown>; push?: string; cage?: ReturnType<typeof sandbox> }> {
  if (!steps.tool) return { input: steps.input };
  const result = await review({
    tool: steps.tool,
    input: steps.input,
    agent: owner.agent,
    directory: owner.directory,
    session: owner.session,
  }).catch((error: unknown) => ({ problem: `the guard could not check this call: ${String(error)}` }));
  if ("problem" in result) return { refuse: result.problem };
  if (!result.cage) return { input: { ...steps.input, ...result.input }, push: result.push };
  const { command, workdir } = result.cage;
  return { input: { command, workdir }, cage: sandbox(String(result.input?.command), workdir) };
}

export async function mark(id: string, cage: Record<string, unknown>) {
  await mkdir(cages, { recursive: true });
  await writeFile(path.join(cages, id), JSON.stringify(cage));
}

export async function wrapped(id: string, cage: Record<string, unknown>) {
  if (!/^[\w-]+$/.test(id)) return false;
  const marker = path.join(cages, id);
  const marked = await readFile(marker, "utf8").catch(() => undefined);
  await rm(marker, { force: true });
  return marked === JSON.stringify(cage);
}

function file(tool: "read" | "write" | "edit", input: Record<string, unknown>, directory: string): Plan {
  if (typeof input.Path !== "string" || !input.Path) return { refuse: "the request names no Path" };
  const absolute = path.resolve(directory, input.Path);
  if (tool === "read") return { tool, input: { path: absolute } };
  if (tool === "write") return { tool, input: { path: absolute, content: input.Content } };
  return { tool, input: { path: absolute, oldString: input.Target, newString: input.Replacement } };
}

function shell(input: Record<string, unknown>, directory: string): Plan {
  const line = input.CommandLine;
  if (typeof line !== "string" || !line.trim()) return { refuse: "the command line is missing" };
  if (input.BypassSandbox !== undefined && input.BypassSandbox !== false)
    return { refuse: "commands must run inside agy's sandbox" };
  const workdir = typeof input.Cwd === "string" && input.Cwd ? path.resolve(directory, input.Cwd) : directory;
  return { tool: "shell", input: { command: line, workdir } };
}

function access(tool: "read" | "write" | "edit", absolute: string, place: Place): Need[] {
  const inside = internal(absolute, place);
  const list: Need[] = inside ? [] : [external(absolute, tool === "read" ? undefined : "file")];
  const resource = inside ? path.relative(place.directory, absolute) || "." : absolute;
  list.push({ action: tool === "read" ? "read" : "edit", resources: [resource], save: ["*"] });
  return list;
}

function commands(line: string, workdir: string, place: Place): Need[] {
  const runs = launches(line, workdir);
  const hidden = runs.some((run) => run.command === opaque || run.command === streamed || run.command === unseen);
  const shown = runs
    .filter((run) => run.command !== opaque && run.command !== streamed && run.command !== unseen)
    .filter((run) => run.command[0] !== redirection)
    .map((run) => run.command.join(" ").trim())
    .filter(Boolean);
  const resources = [...new Set(hidden || shown.length === 0 ? [...shown, line] : shown)];
  const dirs = new Set([workdir, ...runs.flatMap((run) => [run.dir, destination(run)].filter(defined))]);
  const list = [...dirs].filter((dir) => !internal(dir, place)).map((dir) => external(dir, "directory"));
  list.push({ action: "shell", resources, save: resources });
  return list;
}

function destination({ command, dir }: Launch) {
  const [name, ...args] = command;
  if (name !== "cd" && name !== "pushd") return undefined;
  const operands = args.filter((arg) => arg !== "--" && !/^-[A-Za-z@]+$/u.test(arg));
  if (operands.length > 1 || operands.some((arg) => /^[-+]/u.test(arg))) return undefined;
  const target = operands[0] ?? (name === "cd" ? "~" : undefined);
  return target === undefined ? undefined : literalPath(target, dir);
}

function defined<T>(value: T | undefined): value is T {
  return value !== undefined;
}

function internal(absolute: string, place: Place) {
  if (within(absolute, place.directory)) return true;
  return place.root !== undefined && place.root !== path.parse(place.root).root && within(absolute, place.root);
}

function external(absolute: string, kind: "file" | "directory" | undefined): Need {
  const type = kind ?? (statSync(absolute, { throwIfNoEntry: false })?.isDirectory() ? "directory" : "file");
  const directory = type === "directory" ? absolute : path.dirname(absolute);
  return {
    action: "external_directory",
    resources: [path.join(directory, "*")],
    save: [path.join(project(directory) ?? directory, "*")],
  };
}

function project(directory: string) {
  for (let current = directory; ; current = path.dirname(current)) {
    if ([".git", ".hg"].some((marker) => existsSync(path.join(current, marker)))) return current;
    if (path.dirname(current) === current) return undefined;
  }
}

function object(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
