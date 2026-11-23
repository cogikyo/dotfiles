import { statSync } from "node:fs";
import { realpath } from "node:fs/promises";
import path from "node:path";
import { commitRejection } from "./commit.ts";
import { relocated } from "./dirs.ts";
import { confine, expand, home, scratch, screen, secrets, within } from "./paths.ts";
import { credential, inspect, readOnly } from "./policy.ts";
import { enter, quote } from "./shell.ts";

const cage = path.join(home, ".local/bin/cage");
const allowed = [
  "shell",
  "read",
  "grep",
  "glob",
  "webfetch",
  "websearch",
  "skill",
  "opencode_sessions",
  "x",
  "diagram",
  "workflow",
  "subagent",
  "execute",
];
const files: Record<string, { key: string; recursive: boolean; required: boolean }> = {
  read: { key: "path", recursive: false, required: true },
  grep: { key: "path", recursive: true, required: false },
  glob: { key: "path", recursive: true, required: false },
  diagram: { key: "file", recursive: false, required: true },
};

export type Call = { tool: string; input: unknown; agent: string | undefined; directory: string; session: string };

export type Review =
  | { problem: string }
  | {
      input?: Record<string, unknown>;
      push?: string;
      cage?: { command: string; workdir: string; scratch: string };
    };

export function text(input: unknown, key: string) {
  if (typeof input !== "object" || input === null) return undefined;
  const value = (input as Record<string, unknown>)[key];
  return typeof value === "string" && value !== "" ? value : undefined;
}

function caged(command: string, dir: string) {
  const hides = secrets().flatMap((secret) => ["--hide", quote(secret)]);
  return [quote(cage), ...hides, quote(dir), "--", "bash", "-c", quote(command)].join(" ");
}

function given(call: Call, key: string) {
  const value = text(call.input, key);
  return call.tool === "glob" && (value === "undefined" || value === "null") ? undefined : value;
}

function resolve(directory: string, target: string | undefined) {
  const expanded = expand(target ?? ".");
  return path.isAbsolute(expanded) ? path.normalize(expanded) : path.resolve(directory, expanded);
}

async function confined(call: Call): Promise<Review> {
  const file = files[call.tool];
  if (typeof call.input !== "object" || call.input === null) return {};
  const target = given(call, file.key);
  if (target === undefined && file.required) return {};
  try {
    return { input: { [file.key]: await confine(target ?? ".", call.directory, file.recursive) } };
  } catch (error) {
    const code = (error as NodeJS.ErrnoException).code;
    const missing = code === "ENOENT" || code === "ENOTDIR";
    return {
      problem: missing
        ? `${call.tool} cannot use ${target ?? call.directory}: no such file or directory`
        : `${call.tool} cannot use ${target ?? call.directory}: ${(error as Error).message}; read-only agents can read it only through the sandboxed shell, if at all`,
    };
  }
}

export async function review(call: Call): Promise<Review> {
  const stray = relocated(call.tool, call.input);
  if (stray) return { problem: stray };
  const restricted = readOnly(call.agent);
  if (restricted && !allowed.includes(call.tool)) {
    return {
      problem: `read-only agents cannot call \`${call.tool}\`; they may call only ${allowed.map((name) => `\`${name}\``).join(", ")}`,
    };
  }
  if (restricted && call.tool in files) return confined(call);
  if (!restricted && (call.tool === "read" || call.tool === "grep")) {
    const target = text(call.input, "path");
    if (target) {
      const lexical = resolve(call.directory, target);
      const real = await realpath(lexical).catch(() => undefined);
      const reason = credential(lexical) ?? (real === undefined ? undefined : credential(real));
      if (reason) return { problem: `${call.tool} cannot use ${target}: ${reason}` };
    }
  }
  if (call.tool === "grep" || call.tool === "glob") {
    const root = path.resolve(resolve(call.directory, given(call, "path")));
    const real = await realpath(root).catch(() => root);
    if ([root, real].some((form) => within(home, form)))
      return { problem: `${call.tool} from ${root} would search the home directory; use a narrower path` };
    if (screen()(root)) return { problem: `${call.tool} cannot search ${root}: it is hidden from agents` };
    return {};
  }
  if (call.tool !== "shell") return {};
  const requested = text(call.input, "command");
  if (!requested) return {};
  const base = resolve(call.directory, text(call.input, "workdir"));
  const { source: command, dir: cwd } = enter(requested, base);
  const input: Record<string, unknown> = {};
  if (command !== requested) {
    if (!statSync(cwd, { throwIfNoEntry: false })?.isDirectory()) return { problem: `cd: ${cwd} is not a directory` };
    Object.assign(input, { command, workdir: cwd });
  }
  const dir = path.join(scratch, call.session);
  const known: Record<string, string> = restricted ? { OPENCODE_SCRATCH: dir } : {};
  const { problems, commits, push } = inspect(command, call.agent, cwd, known);
  if (!restricted && commits.includes(undefined)) {
    problems.push("the guard can't tell which repo this commit runs in; set `workdir` to the repo");
  } else if (!restricted) {
    for (const repo of new Set(commits)) {
      const rejection = await commitRejection(command, repo ?? cwd);
      if (!rejection) continue;
      problems.push(rejection);
      break;
    }
  }
  if (problems.length === 1) return { problem: problems[0] };
  if (problems.length > 1) {
    const list = problems.map((problem) => `- ${problem.replaceAll("\n", "\n  ")}`).join("\n");
    return { problem: `the guard rejected this command for ${problems.length} reasons:\n${list}` };
  }
  const changed = Object.keys(input).length > 0 ? input : undefined;
  if (!restricted) return { input: changed, push };
  return {
    input: { ...input, command: caged(command, dir) },
    cage: { command, workdir: cwd, scratch: dir },
  };
}
