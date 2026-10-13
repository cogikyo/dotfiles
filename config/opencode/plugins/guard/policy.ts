import os from "node:os";
import { bareRedirect, type Command, launches, redirect } from "./shell.ts";

const home = os.homedir();
const scratch = `${home}/.local/state/opencode/scratch/`;
const homePrefix = /^(?:~|\$HOME|\$\{HOME\})\//u;
const sessionVar = /\$(?:OPENCODE_SESSION_ID\b|\{OPENCODE_SESSION_ID\})/gu;
const scratchHint = "write scratch files under ~/.local/state/opencode/scratch/$OPENCODE_SESSION_ID/ instead";

const fileMutators = new Set([
  "cp",
  "dd",
  "ed",
  "emacs",
  "ex",
  "install",
  "ln",
  "mkdir",
  "mv",
  "nano",
  "patch",
  "rmdir",
  "rsync",
  "tee",
  "touch",
  "trash",
  "trash-put",
  "truncate",
  "unlink",
  "vi",
  "vim",
  "wget",
]);

const gitMutators = new Set([
  "add",
  "am",
  "apply",
  "bisect",
  "checkout",
  "cherry-pick",
  "clean",
  "clone",
  "commit",
  "fetch",
  "gc",
  "init",
  "merge",
  "mv",
  "notes",
  "pull",
  "push",
  "rebase",
  "reset",
  "restore",
  "revert",
  "rm",
  "stash",
  "switch",
  "update-index",
  "update-ref",
]);

export function readOnly(agent: string | undefined) {
  if (!agent) return false;
  return (
    agent === "review" ||
    agent.startsWith("review/") ||
    agent === "scout" ||
    agent.startsWith("scout/") ||
    agent === "verify/source" ||
    agent === "verify/web"
  );
}

const credentials = [
  /(?:^|[\s/=@<>:])\.claude(?:-[\w-]+)?\/\.credentials\.json(?![\w.-])/u,
  /(?:^|[\s/=@<>:])opencode[\w-]*\/(?:opencode\/)?auth\.json(?![\w.-])/u,
  /(?:^|[\s/=@<>:])opencode\/service(?:-[\w.-]+)?\.json(?![\w.-])/u,
];
const database = /(?:^|[\s/=@<>:])opencode[\w-]*\/(?:opencode\/)?opencode\.db(?:-wal|-shm)?(?![\w.-])/u;

function unresolved(name: string) {
  return name !== "[" && name !== "[[" && /[$`*?[]|\{[^}]*,/u.test(name);
}

export function denial(resource: string, agent: string | undefined) {
  const commands = launches(resource);
  if (commands.some(([name]) => name === "rm")) return "rm is disabled; move files to trash with `trash -- <path>`";
  if (commands.some(([name]) => name === "grok")) return "grok is disabled in the shell; use the `x` tool for X search";
  if (commands.some(([name, ...args]) => (name === "opencode" || name === "opencode-v2") && args.includes("api"))) {
    return "agents must not call the OpenCode server API; `opencode api` is disabled in the shell; to create a session the user asked for, call `tools.opencode.session_create` through `execute`";
  }
  const dynamic = commands.find(([name]) => unresolved(name));
  if (dynamic) {
    return `the command name \`${dynamic[0]}\` is expanded at run time; write the command name literally so the guard can check it`;
  }
  const args = commands.flatMap(([, ...rest]) => rest);
  const secret = args.flatMap((arg) => credentials.map((file) => file.exec(arg)?.[0])).find(Boolean);
  if (secret) return `\`${secret.replace(/^[\s/=@<>:]/u, "")}\` is a credential file; agents must not read it`;
  if (args.some((arg) => database.test(arg))) {
    return "the OpenCode database holds provider credentials; agents must not open it from the shell; use the `sessions` tool for session history";
  }
  if (!readOnly(agent)) return undefined;
  const reason = mutation(resource, commands);
  return reason && `read-only agents cannot mutate; ${reason}; ${scratchHint}`;
}

function mutation(resource: string, commands: Command[]) {
  const source = resource.replace(heredoc, (body) => body.slice(0, body.indexOf("\n")));
  const literalRoot = /["'\\]~|~["'\\]|['\\]\$\{?H|\$\{?(?:H(?:OM?)?)?["'\\]/u.test(source);
  const scratched = (target: string) => inScratch(target) && !(literalRoot && homePrefix.test(target));
  if (!redirects(source).every(scratched)) return "shell output redirection is disabled";
  const fed = commands.some(([name]) => name === "xargs");
  const safe = (targets: string[]) => !fed && targets.length > 0 && targets.every(scratched);
  for (const [name, ...args] of commands) {
    if (fileMutators.has(name) && !safe(writes(name, args)))
      return `filesystem mutation command \`${name}\` is disabled`;
    if (name === "gio" && args[0] === "trash") return "filesystem mutation command `gio trash` is disabled";
    if (name === "curl") {
      const targets = curlTargets(args);
      if (targets.length > 0 && !safe(targets)) return "file-writing `curl` options are disabled";
    }
    if ((name === "sed" || name === "perl") && args.some(inPlace)) return "in-place text editing is disabled";
    if (name === "git" && gitMutation(args)) return "Git mutation commands are disabled";
  }
  return undefined;
}

function inScratch(target: string) {
  const resolved = target.replace(homePrefix, `${home}/`).replace(sessionVar, "session");
  if (!resolved.startsWith(scratch) || resolved.length === scratch.length) return false;
  return !/[$`*?[{]/u.test(resolved) && !resolved.split("/").includes("..");
}

function writes(name: string, args: string[]) {
  const { paths, redirected } = operands(args);
  if (name === "cp") return paths.length > 1 && plainCopy(args) ? [...redirected, ...paths.slice(-1)] : [];
  return ["mkdir", "tee", "touch", "trash"].includes(name) ? [...paths, ...redirected] : [];
}

function operands(args: string[]) {
  const paths: string[] = [];
  const redirected: string[] = [];
  let flags = true;
  for (let index = 0; index < args.length; index++) {
    const arg = args[index];
    if (redirect.test(arg)) {
      const target = bareRedirect.test(arg) ? (args[++index] ?? "") : arg.replace(/^\d*[<>&|]+/u, "");
      const duplicate = /^\d*[<>]&/u.test(arg) && /^(?:\d+|-)$/u.test(target);
      if (!duplicate && target !== "/dev/null") redirected.push(target);
    } else if (flags && arg === "--") flags = false;
    else if (!flags || !arg.startsWith("-")) paths.push(arg);
  }
  return { paths, redirected };
}

const copyLong: [RegExp, string][] = [
  [/^--(?:l|sy|t)/u, "l"],
  [/^--rec/u, "r"],
  [/^--ar/u, "a"],
  [/^--no-d/u, "P"],
  [/^--der/u, "L"],
];

function plainCopy(args: string[]) {
  let recursive = false;
  let deref = "";
  for (const arg of args) {
    if (arg === "--") break;
    if (!arg.startsWith("-")) continue;
    const flags = arg.startsWith("--") ? (copyLong.find(([long]) => long.test(arg))?.[1] ?? "") : arg.slice(1);
    for (const flag of flags) {
      if ("lst".includes(flag)) return false;
      recursive ||= "rRa".includes(flag);
      // GNU cp applies the last symlink option, so later -a, -P, -d, or -H override -L.
      if ("aPdH".includes(flag)) deref = "P";
      if (flag === "L") deref = "L";
    }
  }
  return deref === "L" || (!deref && !recursive);
}

function curlTargets(args: string[]) {
  const targets: string[] = [];
  let remote = false;
  let dir = false;
  for (let index = 0; index < args.length; index++) {
    const arg = args[index];
    const long = /^--(output|output-dir|remote-name|remote-name-all)(?:=(.*))?$/u.exec(arg);
    if (long?.[1].startsWith("remote")) remote = true;
    else if (long) {
      targets.push(long[2] ?? args[++index] ?? "");
      dir ||= long[1] === "output-dir";
    } else if (/^-[^-]/u.test(arg)) {
      remote ||= /^-[^o]*O/u.test(arg);
      const output = /^-[^o]*o(.*)$/u.exec(arg);
      if (output) targets.push(output[1] || (args[++index] ?? ""));
    }
  }
  if (remote && !dir) targets.push(".");
  return targets;
}

function inPlace(arg: string) {
  return (
    arg === "-i" ||
    arg.startsWith("-i.") ||
    arg === "-pi" ||
    arg.startsWith("-pi.") ||
    arg === "--in-place" ||
    arg.startsWith("--in-place=")
  );
}

const harmlessRedirect = /^(?:\d?>>?|&>>?)\s*\/dev\/null\b|^\d?>&\d/u;
const heredoc = /<<-?\s*(['"]?)(\w+)\1[^\n]*\n[\s\S]*?\n\s*\2[ \t]*(?=\n|$)/gu;

const redirectOp = /^(?:&>>?|\d?>[>|&]?)[ \t]*/u;

function redirects(command: string) {
  const targets: string[] = [];
  let quote = "";
  let escaped = false;
  for (let index = 0; index < command.length; index++) {
    const char = command[index];
    if (escaped) {
      escaped = false;
      continue;
    }
    if (char === "\\" && quote !== "'") {
      escaped = true;
      continue;
    }
    if (quote) {
      if (char === quote) quote = "";
      continue;
    }
    if (char === "'" || char === '"') {
      quote = char;
      continue;
    }
    if (char !== ">") continue;
    const start = /[\d&]/u.test(command[index - 1] ?? "") ? index - 1 : index;
    const rest = command.slice(start);
    const harmless = rest.match(harmlessRedirect);
    if (harmless) {
      index = start + harmless[0].length - 1;
      continue;
    }
    const target = word(command, start + (redirectOp.exec(rest)?.[0].length ?? rest.length));
    targets.push(target.text);
    index = target.end - 1;
  }
  return targets;
}

function word(source: string, start: number) {
  let text = "";
  let quote = "";
  let index = start;
  for (; index < source.length; index++) {
    const char = source[index];
    if (char === "\\" && quote !== "'") text += source[++index] ?? "";
    else if (quote) {
      if (char === quote) quote = "";
      else text += char;
    } else if (char === "'" || char === '"') quote = char;
    else if (/[\s;&|()<>]/u.test(char)) break;
    else text += char;
  }
  return { text, end: index };
}

function gitMutation(args: string[]) {
  const command = subcommand(args);
  if (!command) return false;
  if (command.name === "stash") return !["list", "show"].includes(command.args[0] ?? "");
  if (gitMutators.has(command.name)) return true;
  if (command.name === "branch") return branchMutation(command.args);
  if (command.name === "config") return configMutation(command.args);
  if (command.name === "tag") return tagMutation(command.args);
  return command.name === "worktree" && command.args.length > 0 && command.args[0] !== "list";
}

function subcommand(args: string[]) {
  for (let index = 0; index < args.length; index++) {
    const arg = args[index];
    if (["-C", "-c", "--git-dir", "--work-tree", "--namespace", "--config-env"].includes(arg)) {
      index++;
      continue;
    }
    if (arg.startsWith("-")) continue;
    return { name: arg, args: args.slice(index + 1) };
  }
  return undefined;
}

function branchMutation(args: string[]) {
  const mutating = new Set([
    "-c",
    "-C",
    "-d",
    "-D",
    "-f",
    "-m",
    "-M",
    "--copy",
    "--delete",
    "--edit-description",
    "--force",
    "--move",
    "--set-upstream-to",
    "--unset-upstream",
  ]);
  if (args.some((arg) => mutating.has(arg))) return true;
  const values = new Set([
    "--contains",
    "--format",
    "--merged",
    "--no-contains",
    "--no-merged",
    "--points-at",
    "--sort",
  ]);
  let listing = false;
  for (let index = 0; index < args.length; index++) {
    const arg = args[index];
    if (["-a", "--all", "-l", "--list", "-r", "--remotes", "--show-current"].includes(arg)) {
      listing = true;
      continue;
    }
    if (values.has(arg)) {
      listing = true;
      index++;
      continue;
    }
    if (arg.startsWith("-")) continue;
    if (!listing) return true;
  }
  return false;
}

function configMutation(args: string[]) {
  const mutating = new Set([
    "--add",
    "--edit",
    "--remove-section",
    "--rename-section",
    "--replace-all",
    "--unset",
    "--unset-all",
  ]);
  if (args.some((arg) => mutating.has(arg))) return true;
  const reading = new Set(["--get", "--get-all", "--get-regexp", "--get-urlmatch", "-l", "--list"]);
  if (args.some((arg) => reading.has(arg))) return false;
  return args.filter((arg) => !arg.startsWith("-")).length > 1;
}

function tagMutation(args: string[]) {
  if (args.length === 0) return false;
  const listing = new Set(["-l", "--list", "--contains", "--no-contains", "--merged", "--no-merged", "--points-at"]);
  return !args.some(
    (arg) => listing.has(arg) || ["--column", "--format", "--sort"].some((option) => arg.startsWith(option)),
  );
}
