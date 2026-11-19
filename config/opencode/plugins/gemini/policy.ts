import path from "node:path";
import { located, secrets, within } from "../guard/paths.ts";

export type Call = {
  toolCallId: string;
  kind?: string;
  title?: string;
  locations?: { path?: unknown }[];
  rawInput?: Record<string, unknown>;
};

export type Verdict =
  | { outcome: "allow" }
  | { outcome: "reject"; reason: string }
  | { outcome: "cancel"; reason: string };

const fields = new Set([
  "CommandLine",
  "Cwd",
  "WaitMsBeforeAsync",
  "toolSummary",
  "toolAction",
  "explanation",
  "Blocking",
  "BypassSandbox",
]);

const gitSubcommands = new Set(["log", "show", "diff", "blame"]);

const gitFlags = new Set([
  "-p",
  "-s",
  "-w",
  "-M",
  "-C",
  "-l",
  "-e",
  "--patch",
  "--no-patch",
  "--stat",
  "--shortstat",
  "--numstat",
  "--name-only",
  "--name-status",
  "--oneline",
  "--graph",
  "--decorate",
  "--all",
  "--reverse",
  "--follow",
  "--first-parent",
  "--merges",
  "--no-merges",
  "--full-history",
  "--abbrev-commit",
  "--summary",
  "--cached",
  "--staged",
  "--word-diff",
  "--ignore-all-space",
  "--no-color",
  "--porcelain",
  "--line-porcelain",
]);

const gitValues =
  /^(-\d+|-[nU]\d+|-L\S+|--(format|pretty)=(?!.*%G).*|--(max-count|skip|unified|since|until|after|before|author|committer|grep|date|diff-filter)=.*)$/;

const commands = new Map<string, (args: string[]) => boolean>([
  ["pwd", (args) => args.length === 0],
  ["ls", (args) => flags(args).every((arg) => /^-[1aAdFhl]+$/.test(arg))],
  [
    "git",
    ([sub, ...args]) =>
      sub !== undefined &&
      gitSubcommands.has(sub) &&
      flags(args).every((arg) => gitFlags.has(arg) || gitValues.test(arg)),
  ],
]);

export function decide(call: Call, cwd: string): Verdict {
  if (call.toolCallId.startsWith("interaction_"))
    return { outcome: "cancel", reason: "the agent asked a question, and no user is present to answer it" };
  if (call.kind === "think") return { outcome: "allow" };
  if (call.kind === "read" || call.kind === "search") return reach(call.kind, call.locations ?? []);
  if (call.kind !== "execute") return { outcome: "reject", reason: `${call.kind ?? "unknown"} tools are not allowed` };
  return shell(call.rawInput ?? {}, cwd);
}

function shell(input: Record<string, unknown>, cwd: string): Verdict {
  const extra = Object.keys(input).find((field) => !fields.has(field));
  if (extra) return { outcome: "reject", reason: `the command field ${extra} is not allowed` };
  if (input.BypassSandbox !== undefined && input.BypassSandbox !== false)
    return { outcome: "reject", reason: "commands must run inside the sandbox" };
  if (input.Cwd !== undefined && (typeof input.Cwd !== "string" || path.resolve(cwd, input.Cwd) !== path.resolve(cwd)))
    return { outcome: "reject", reason: "commands must run in the session's working folder" };
  const line = input.CommandLine;
  if (typeof line !== "string") return { outcome: "reject", reason: "the command line is missing" };
  const args = words(line.trim());
  if (!args) return { outcome: "reject", reason: "shell syntax beyond plain words and quotes is not allowed" };
  const [name, ...rest] = args;
  const check = name === undefined ? undefined : commands.get(name);
  if (!check) return { outcome: "reject", reason: `${name ?? "an empty command"} is not on the read-only list` };
  if (!check(rest)) return { outcome: "reject", reason: `these ${name} arguments are not allowed` };
  const end = rest.indexOf("--");
  const targets = rest.filter((arg, index) => (end !== -1 && index > end) || !arg.startsWith("-"));
  if (targets.length === 0) {
    if (name !== "ls") return { outcome: "allow" };
    targets.push(".");
  }
  return reach(
    "search",
    targets.map((arg) => ({ path: path.resolve(cwd, arg) })),
  );
}

function reach(kind: string, locations: { path?: unknown }[]): Verdict {
  if (locations.length === 0) return { outcome: "reject", reason: "the request names no path" };
  try {
    const hidden = secrets().flatMap((secret) => [secret, located(secret)]);
    for (const { path: target } of locations) {
      if (typeof target !== "string" || !path.isAbsolute(target))
        return { outcome: "reject", reason: `${JSON.stringify(target)} is not an absolute path` };
      const forms = [path.resolve(target), located(target)];
      if (forms.some((form) => hidden.some((secret) => within(form, secret))))
        return { outcome: "reject", reason: `${target} is hidden from read-only agents` };
      if (kind === "search" && forms.some((form) => hidden.some((secret) => within(secret, form))))
        return { outcome: "reject", reason: `${target} contains paths hidden from read-only agents` };
    }
  } catch (error) {
    return { outcome: "reject", reason: `the path could not be resolved: ${String(error)}` };
  }
  return { outcome: "allow" };
}

function words(line: string): string[] | undefined {
  if (/[^ -~]/.test(line)) return undefined;
  const token = /( +)|'([^']*)'|"([^"$`\\!]*)"|([\w./:=,+@%^~-]+)/y;
  const out: string[] = [];
  let word: string | undefined;
  while (token.lastIndex < line.length) {
    const match = token.exec(line);
    if (!match) return undefined;
    if (match[1]) {
      if (word !== undefined) out.push(word);
      word = undefined;
      continue;
    }
    const piece = match[2] ?? match[3] ?? match[4] ?? "";
    if (match[4]?.includes("~") && /(^|[=:])~/.test((word ?? "") + piece)) return undefined;
    word = (word ?? "") + piece;
  }
  if (word !== undefined) out.push(word);
  return out;
}

function flags(args: string[]) {
  const end = args.indexOf("--");
  return (end === -1 ? args : args.slice(0, end)).filter((arg) => arg.startsWith("-"));
}
