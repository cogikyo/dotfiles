import { type Command, launches } from "./shell.ts";

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
  return reason && `read-only agents cannot mutate; ${reason}`;
}

function mutation(resource: string, commands: Command[]) {
  if (redirects(resource)) return "shell output redirection is disabled";
  for (const [name, ...args] of commands) {
    if (fileMutators.has(name)) return `filesystem mutation command \`${name}\` is disabled`;
    if (name === "gio" && args[0] === "trash") return "filesystem mutation command `gio trash` is disabled";
    if (name === "curl" && args.some((arg) => /^(?:-o|-O|--output|--output-dir|--remote-name)(?:=|$)/u.test(arg))) {
      return "file-writing `curl` options are disabled";
    }
    if ((name === "sed" || name === "perl") && args.some(inPlace)) return "in-place text editing is disabled";
    if (name === "git" && gitMutation(args)) return "Git mutation commands are disabled";
  }
  return undefined;
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

function redirects(source: string) {
  const command = source.replace(heredoc, (body) => body.slice(0, body.indexOf("\n")));
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
    const harmless = command.slice(start).match(harmlessRedirect);
    if (!harmless) return true;
    index = start + harmless[0].length - 1;
  }
  return false;
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
