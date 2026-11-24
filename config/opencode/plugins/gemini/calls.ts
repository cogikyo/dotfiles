import { existsSync, mkdirSync, statSync, writeFileSync } from "node:fs";
import { lstat, mkdir, readFile, rm } from "node:fs/promises";
import path from "node:path";
import { churned, scratch, within } from "../guard/paths.ts";
import { readOnly } from "../guard/policy.ts";
import { review } from "../guard/review.ts";
import { launches, literalPath, opaque, quote, redirection, streamed, unseen, type Launch } from "../guard/shell.ts";
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

export type Plan =
  | { refuse: string }
  | { tool?: string; input: Record<string, unknown>; given?: Record<string, unknown> };

export type Checked = { input: Record<string, unknown>; push?: string; overwrite?: Record<string, unknown> };

type Lane = Owner & { session: string };

type Place = { directory: string; root: string | undefined };

const tools: Record<string, (input: Record<string, unknown>, owner: Lane) => Plan> = {
  read_file: (input, owner) => file("read", input, owner.directory),
  write_file: (input, owner) =>
    readOnly(owner.agent) && climbs(input.Path)
      ? { refuse: `write_file cannot use \`..\` in Path; ${confinement(owner.session)}` }
      : file("write", input, owner.directory),
  edit_file: (input, owner) =>
    readOnly(owner.agent)
      ? {
          refuse: `read-only agents cannot edit files; write the whole changed file with \`write_file\`; ${confinement(owner.session)}`,
        }
      : file("edit", input, owner.directory),
  run_command: (input, owner) => shell(input, owner.directory),
  manage_task: task,
  search_web: (input) => ({ tool: "websearch", input: { query: input.query } }),
};

const actions = new Set(["list", "status", "kill"]);

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

export function plan(call: Call, owner: Lane): Plan {
  if (call.toolCallId.startsWith("interaction_"))
    return { refuse: "this OpenCode session cannot show agent questions; ask in your reply instead" };
  const reason = refusal(call.name);
  if (reason) return { refuse: reason };
  const input = object(call.rawInput) ? call.rawInput : {};
  if (bridged(call.name)) return { input };
  const steps = tools[call.name!](input, owner);
  return "refuse" in steps ? steps : { ...steps, given: input };
}

export function needs(tool: string | undefined, input: Record<string, unknown>, owner: Owner): Need[] {
  const place = { directory: owner.directory, root: project(owner.directory) };
  if (tool === "shell") return commands(String(input.command), String(input.workdir), place);
  if (tool === "websearch") return [{ action: "websearch", resources: [String(input.query)], save: ["*"] }];
  if (tool === "write" && readOnly(owner.agent)) {
    const resources = [`write_file ${String(input.path)}`];
    return [{ action: "shell", resources, save: resources }];
  }
  if (tool === "read" || tool === "write" || tool === "edit") return access(tool, String(input.path), place);
  return [];
}

export async function check(
  steps: Exclude<Plan, { refuse: string }>,
  owner: Lane,
): Promise<{ refuse: string } | Checked> {
  if (!steps.tool) return { input: steps.input };
  if (steps.tool === "write" && readOnly(owner.agent)) return jotted(steps.input, owner.session);
  const result = await review({
    tool: steps.tool,
    input: steps.input,
    agent: owner.agent,
    directory: owner.directory,
    session: owner.session,
  }).catch((error: unknown) => ({ problem: `the guard could not check this call: ${String(error)}` }));
  if ("problem" in result) return { refuse: result.problem };
  if (result.cage) {
    const { command, workdir } = result.cage;
    return {
      input: { command, workdir },
      overwrite: { CommandLine: commandLine(String(result.input?.command), workdir, owner.directory) },
    };
  }
  const input = { ...steps.input, ...result.input };
  const overwrite = result.input && steps.given && rewritten(steps.tool, steps.given, input, owner.directory);
  return { input, push: result.push, overwrite: overwrite || undefined };
}

export function mark(id: string, overwrite: Record<string, unknown>) {
  mkdirSync(cages, { recursive: true });
  writeFileSync(path.join(cages, id), JSON.stringify(overwrite));
}

export async function wrapped(id: string, overwrite: Record<string, unknown>) {
  if (!/^[\w-]+$/.test(id)) return "missing";
  const marker = path.join(cages, id);
  const marked = await readFile(marker, "utf8").catch(() => undefined);
  await rm(marker, { force: true });
  if (marked === undefined) return "missing";
  return marked === JSON.stringify(overwrite) ? "applied" : "differs";
}

function rewritten(
  tool: string,
  given: Record<string, unknown>,
  input: Record<string, unknown>,
  directory: string,
): Record<string, unknown> | undefined {
  if (tool === "shell") {
    const line = commandLine(String(input.command), String(input.workdir), directory);
    return line === given.CommandLine ? undefined : { CommandLine: line };
  }
  if (tool === "read" && input.path !== given.Path) return { Path: input.path };
  return undefined;
}

function commandLine(command: string, workdir: string, directory: string) {
  return workdir === directory ? command : `cd -- ${quote(workdir)} && ${command}`;
}

function confinement(session: string) {
  return `read-only agents write only regular files directly in ${path.join(scratch, session)}, not in its subdirectories, through symlinks, or onto hard links`;
}

function climbs(given: unknown) {
  return typeof given === "string" && given.split(/[\\/]+/u).includes("..");
}

// Lexical after rejecting "..": cage blocks symlinks and renames in scratch root, so only the final entry needs lstat.
async function jotted(input: Record<string, unknown>, session: string): Promise<{ refuse: string } | Checked> {
  const dir = path.join(scratch, session);
  const target = String(input.path);
  const refuse = (why: string) => ({ refuse: `write_file cannot use ${target}: ${why}; ${confinement(session)}` });
  if (churned.has(path.basename(target))) return refuse(`the sandbox keeps ${path.basename(target)}/ for itself`);
  try {
    if (path.dirname(target) !== dir) return refuse("it is not directly in the scratch directory");
    await mkdir(dir, { recursive: true, mode: 0o700 });
    const stat = await lstat(target).catch((error: NodeJS.ErrnoException) => {
      if (error.code === "ENOENT") return undefined;
      throw error;
    });
    if (stat?.isSymbolicLink()) return refuse("it is a symlink");
    if (stat && !stat.isFile()) return refuse("it is not a regular file");
    if (stat && stat.nlink > 1) return refuse("it has other hard links");
  } catch (error) {
    return refuse(error instanceof Error ? error.message : String(error));
  }
  return { input: { path: target, content: input.content } };
}

function task(input: Record<string, unknown>): Plan {
  if (actions.has(String(input.Action))) return { input };
  return {
    refuse: `\`manage_task\` supports only ${[...actions].map((action) => `\`${action}\``).join(", ")} in this OpenCode session`,
  };
}

function file(tool: "read" | "write" | "edit", input: Record<string, unknown>, directory: string): Plan {
  if (typeof input.Path !== "string" || !input.Path) return { refuse: "the request names no Path" };
  const absolute = path.resolve(directory, input.Path);
  if (tool === "read") return { tool, input: { path: absolute } };
  if (tool === "write") return { tool, input: { path: absolute, content: input.Content } };
  return { tool, input: { path: absolute, oldString: input.Target, newString: input.Replacement } };
}

function shell(input: Record<string, unknown>, directory: string): Plan {
  const command = input.CommandLine;
  if (typeof command !== "string" || !command.trim()) return { refuse: "the command line is missing" };
  return { tool: "shell", input: { command, workdir: directory } };
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
  const ambiguous = runs.some((run) => run.guess !== undefined);
  const shown = runs
    .filter((run) => run.command !== opaque && run.command !== streamed && run.command !== unseen)
    .filter((run) => run.command[0] !== redirection)
    .map((run) => run.command.join(" ").trim())
    .filter(Boolean);
  const resources = [...new Set(hidden || ambiguous || shown.length === 0 ? [...shown, line] : shown)];
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
