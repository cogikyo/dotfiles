import { readdirSync, readFileSync, realpathSync, statSync } from "node:fs";
import path from "node:path";
import { profile as gemini } from "../gemini/agy.ts";
import { brain, expand, firefox, home, located, temp, tilde, within } from "./paths.ts";
import {
  argv,
  type Command,
  flags,
  type Launch,
  launches,
  literalLaunches,
  literalPath,
  maxDepth,
  named,
  opaque,
  options,
  redirection,
  remote,
  respaced,
  type Spec,
  streamed,
  unseen,
  wrapped,
} from "./shell.ts";

const unchecked = `the command nests substitutions, arithmetic, or inner shells more than ${maxDepth} levels deep, or contains arithmetic the guard cannot parse, so the guard cannot check every command; flatten it or split it into separate calls`;

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

export const owners = new Set(["collab", "build/git"]);
const credentials = [
  /(?:^|[\s/=@<>:])\.claude(?:-[\w-]+)?\/\.credentials\.json(?![\w.-])/u,
  /(?:^|[\s/=@<>:])opencode[\w-]*\/(?:opencode\/)?auth\.json(?![\w.-])/u,
  /(?:^|[\s/=@<>:])opencode\/service(?:-[\w.-]+)?\.json(?![\w.-])/u,
];
const database = /(?:^|[\s/=@<>:])opencode[\w-]*\/(?:opencode\/)?opencode\.db(?:-wal|-shm)?(?![\w.-])/u;
const credentialFiles = [...credentials, database].map((file) => new RegExp(`${file.source}$`, file.flags));
const credentialName = /^(?:service.*\.json|auth\.json|opencode\.db.*|\.credentials\.json)$/u;
const databaseReason =
  "the OpenCode database holds provider credentials; agents must not open it from the shell; use the `sessions` tool for session history";
const profile = /[\s/=@<>:'"(,`]\.(?:config\/)?mozilla(?:\/firefox(?![\w.-])|\/?(?![^\s'"),;`]))/u;
const profileReason =
  "Firefox profile folders (`~/.config/mozilla/firefox`, `~/.mozilla/firefox`) hold the user's cookies, saved logins, and site storage; agents must not read, copy, or archive them; browser checks go through `verify/browser`";
const geminiPath = /(?:\.local\/state|XDG_STATE_HOME)\}?\/opencode\/gemini(?![\w.-])([^\s'"`;|&<>)]*)/gu;
const brainTail = `/${path.relative(gemini, brain)}`;
const geminiReason = `agy's profile \`${tilde(gemini)}\` holds its bridge token, MCP config, hooks, and bridged results; agents must not read or write it`;

function geminiArg(arg: string) {
  return [...arg.matchAll(geminiPath)].some(
    ([, tail = ""]) => !within(tail, brainTail) || /(?:^|\/)\.\.(?:\/|$)/u.test(tail),
  );
}

export function credential(file: string) {
  const target = path.resolve(file);
  if (firefox.some((root) => within(target, root) || target === path.dirname(root))) return profileReason;
  if (within(target, gemini) && !within(target, brain)) return geminiReason;
  if (!credentialFiles.some((pattern) => pattern.test(file))) return undefined;
  return database.test(file) ? databaseReason : `\`${file}\` is a credential file; agents must not read it`;
}

function pathCredentials(launched: Launch[]) {
  const found = new Set<string>();
  const check = (file: string) => {
    let reason = credential(file);
    if (!reason && credentialName.test(path.basename(file))) {
      try {
        reason = credential(realpathSync(file));
      } catch {
        reason = undefined;
      }
    }
    if (reason) found.add(reason);
  };
  for (const {
    command: [name, ...words],
    dirs,
  } of launched) {
    if (name === "cage" || dirs === undefined) continue;
    for (const dir of dirs) check(dir);
    for (const word of words) {
      const file = literalPath(word, "/");
      if (file === undefined) continue;
      const full = expand(word);
      if (path.isAbsolute(full)) check(file);
      else for (const base of dirs) check(path.resolve(base, full));
    }
  }
  return found;
}

const relocations =
  /\bGIT_(?:DIR|WORK_TREE|COMMON_DIR|INDEX_FILE|OBJECT_DIRECTORY|ALTERNATE_OBJECT_DIRECTORIES|CONFIG\w*)\b/u;
const globals = new Set([
  "-C",
  "-c",
  "--git-dir",
  "--work-tree",
  "--namespace",
  "--config-env",
  "--super-prefix",
  "--attr-source",
]);
const runners = new Set(["-exec", "-execdir", "-ok", "-okdir"]);
const rewrites = ["--force", "--force-with-lease", "--force-if-includes", "--delete", "--mirror", "--prune"];
const pushes = new Set(["push", "send-pack"]);
type Program = { long: string[]; short?: string };
const programs: Record<string, Program> = {
  "ls-remote": { long: ["--upload-pack", "--exec"] },
  fetch: { long: ["--upload-pack"] },
  "fetch-pack": { long: ["--upload-pack", "--exec"] },
  pull: { long: ["--upload-pack"] },
  clone: { long: ["--upload-pack"], short: "u" },
  push: { long: ["--receive-pack", "--exec"] },
  "send-pack": { long: ["--receive-pack", "--exec"] },
  archive: { long: ["--exec"] },
  rebase: { long: ["--exec"], short: "x" },
  grep: { long: ["--open-files-in-pager"], short: "O" },
};
const tools = new Set(["docker", "podman", "nerdctl", "kubectl", "npm", "pnpm", "yarn", "bun", "cargo", "brew"]);
const verbs = new Set(["exec", "run", "x", "dlx"]);
const systemd: Spec = {
  short: "HMCupE",
  long: [
    "host",
    "machine",
    "capsule",
    "unit",
    "property",
    "description",
    "slice",
    "expand-environment",
    "service-type",
    "uid",
    "gid",
    "nice",
    "working-directory",
    "root-directory",
    "setenv",
    "output",
    "json",
    "job-mode",
    "background",
    "path-property",
    "socket-property",
    "on-active",
    "on-boot",
    "on-startup",
    "on-unit-active",
    "on-unit-inactive",
    "on-calendar",
    "timer-property",
  ],
};
const copiers = new Set(["tee", "cp", "mv", "ln", "install"]);
const nonliteral = /[$`*?[{]|^~/u;
const expansion = /[$`]/u;
const entries = 20_000;
const attended = path.join(home, "LeadPier");
const dangerous =
  /^(?:core\.(?:fsmonitor|hookspath|sshcommand|editor|pager|askpass|gitproxy|worktree)|.+\.command|alias\..+|uploadpack\..+|credential\..*|diff\.external|diff\..+\.textconv|filter\..+|gpg\..+|sequence\.editor|protocol\..+|include(?:if)?\..+|pager\..+|merge\..+\.driver|remote\..+\.(?:uploadpack|receivepack|proxy))$/iu;
const harmless =
  /^(?:(?:user|author|committer)\.\w+|(?:commit|tag)\.gpgsign|init\.defaultbranch|color\..+|advice\..+)$/iu;
const gitPath = /(?:^|[/<>=])\.git(?:\/|$)/u;

type Read = true | ((args: string[]) => boolean);

const parts = (args: string[], valued: string[] = []) => {
  const switches: string[] = [];
  const operands: string[] = [];
  for (let index = 0; index < args.length; index++) {
    const arg = args[index];
    if (arg === "--") {
      operands.push(...args.slice(index + 1));
      break;
    }
    if (arg.startsWith("--")) {
      const [flag] = arg.split("=");
      switches.push(flag);
      if (!arg.includes("=") && valued.includes(flag)) index++;
      continue;
    }
    if (!arg.startsWith("-") || arg === "-") {
      operands.push(arg);
      continue;
    }
    for (let at = 1; at < arg.length; at++) {
      const flag = `-${arg[at]}`;
      switches.push(flag);
      if (!valued.includes(flag)) continue;
      if (at === arg.length - 1) index++;
      break;
    }
  }
  return { switches, operands };
};

const abbreviates = (flag: string, names: string[]) =>
  flag.startsWith("--") && flag.length > 2 && names.some((name) => name.startsWith(flag));

const listing = (writes: string[], lists: string[], valued: string[]) => (args: string[]) => {
  const { switches, operands } = parts(args, valued);
  if (switches.some((flag) => writes.includes(flag) || abbreviates(flag, writes))) return false;
  return operands.length === 0 || switches.some((flag) => lists.includes(flag));
};

const first =
  (allowed: (string | undefined)[], valued: string[] = []) =>
  (args: string[]) =>
    allowed.includes(parts(args, valued).operands[0]);

const fetchValued = [
  "--depth",
  "--deepen",
  "--shallow-since",
  "--shallow-exclude",
  "--refmap",
  "-o",
  "--server-option",
  "-j",
  "--jobs",
  "--negotiation-tip",
  "--filter",
  "--upload-pack",
  "--submodule-prefix",
  "--recurse-submodules-default",
  "--origin",
  "--branch",
  "-b",
  "--reference",
  "--config",
  "-c",
  "--template",
  "--separate-git-dir",
];

const guarded = (args: string[], names: string[], shorts: string) =>
  !parts(args, fetchValued).switches.some(
    (flag) => abbreviates(flag, names) || (/^-\w$/u.test(flag) && shorts.includes(flag[1])),
  );

function inquiry(args: string[]) {
  const { switches, operands } = parts(args, ["-f", "--file", "--blob", "--type", "--default", "--comment"]);
  const writes = new Set([
    "--unset",
    "--unset-all",
    "--add",
    "--replace-all",
    "--rename-section",
    "--remove-section",
    "-e",
    "--edit",
  ]);
  if (switches.some((flag) => writes.has(flag) || abbreviates(flag, [...writes]))) return false;
  if (switches.some((flag) => flag.startsWith("--get") || flag === "--list" || flag === "-l")) return true;
  if (operands[0] === "get" || operands[0] === "list") return true;
  return operands.length === 1 && !["set", "unset", "rename-section", "remove-section", "edit"].includes(operands[0]);
}

const filters = ["--contains", "--no-contains", "--points-at", "--merged", "--no-merged", "--sort", "--format"];

const reads: Record<string, Read> = {
  status: true,
  log: true,
  show: true,
  diff: true,
  "diff-tree": true,
  "diff-files": true,
  "diff-index": true,
  "ls-files": true,
  "ls-tree": true,
  "ls-remote": true,
  "rev-parse": true,
  "rev-list": true,
  "merge-base": true,
  "merge-tree": true,
  describe: true,
  blame: true,
  annotate: true,
  grep: true,
  "cat-file": true,
  "for-each-ref": true,
  "show-ref": true,
  "show-branch": true,
  shortlog: true,
  whatchanged: true,
  "range-diff": true,
  "name-rev": true,
  "check-ignore": true,
  "check-attr": true,
  cherry: true,
  "count-objects": true,
  "verify-commit": true,
  "verify-tag": true,
  "format-patch": true,
  archive: true,
  clone: (args) =>
    guarded(args, ["--template", "--config", "--recurse-submodules", "--recursive", "--separate-git-dir"], "c"),
  version: true,
  help: true,
  var: true,
  fetch: (args) => {
    const risky = ["--refmap", "--recurse-submodules", "--update-head-ok", "--prune", "--prune-tags", "--force"];
    if (!guarded(args, risky, "uPpf")) return false;
    return parts(args, fetchValued)
      .operands.slice(1)
      .every((spec) => !spec.includes(":") && !spec.startsWith("+"));
  },
  tag: listing(
    [
      "-d",
      "--delete",
      "-a",
      "--annotate",
      "-s",
      "--sign",
      "-u",
      "--local-user",
      "-f",
      "--force",
      "-m",
      "--message",
      "-F",
      "--file",
      "-e",
      "--edit",
      "--create-reflog",
    ],
    ["-l", "--list", "-n", "-v", "--verify", "--contains", "--no-contains", "--points-at", "--merged", "--no-merged"],
    [...filters, "-u", "-m", "-F", "--local-user", "--message", "--file", "--cleanup"],
  ),
  branch: listing(
    [
      "-d",
      "-D",
      "--delete",
      "-m",
      "-M",
      "--move",
      "-c",
      "-C",
      "--copy",
      "-u",
      "--set-upstream-to",
      "--unset-upstream",
      "--edit-description",
      "-f",
      "--force",
      "-t",
      "--track",
      "--no-track",
      "--create-reflog",
      "--recurse-submodules",
    ],
    [
      "-l",
      "--list",
      "-a",
      "--all",
      "-r",
      "--remotes",
      "-v",
      "--verbose",
      "--contains",
      "--no-contains",
      "--merged",
      "--no-merged",
      "--points-at",
      "--show-current",
    ],
    [...filters, "-u", "--set-upstream-to"],
  ),
  config: inquiry,
  apply: (args) => {
    const { switches } = parts(args);
    return (
      !switches.includes("--apply") &&
      switches.some((flag) => ["--check", "--stat", "--numstat", "--summary"].includes(flag))
    );
  },
  "symbolic-ref": (args) => {
    const { switches, operands } = parts(args, ["-m"]);
    return operands.length === 1 && !switches.some((flag) => flag === "-d" || abbreviates(flag, ["--delete"]));
  },
  "hash-object": (args) => !parts(args, ["-t", "--path"]).switches.includes("-w"),
  stash: first(["list", "show"]),
  remote: first([undefined, "show", "get-url"]),
  worktree: first(["list"]),
  reflog: (args) => !["expire", "delete", "drop", "write"].includes(parts(args).operands[0]),
  submodule: first([undefined, "status", "summary"]),
  notes: first([undefined, "list", "show"], ["--ref"]),
};

function unresolved(name: string) {
  return name !== "[" && name !== "[[" && /[$`*?[]|\{[^}]*,/u.test(name);
}

function targets(args: string[]) {
  const end = args.indexOf("--");
  const flagged = end < 0 ? args : args.slice(0, end);
  const rest = end < 0 ? [] : args.slice(end + 1);
  return [...flagged.filter((arg) => arg === "-" || !arg.startsWith("-")), ...rest];
}

function removable(operand: string, cwd: string | undefined) {
  if (/[$`]/u.test(operand) || /^[~=]/u.test(operand) || operand.endsWith("/") || operand.includes("..")) return false;
  if (path.isAbsolute(operand) && operand.split("/").includes(".")) return false;
  if (!path.isAbsolute(operand) && cwd === undefined) return false;
  const target = path.resolve(cwd ?? "/", operand);
  const dir = path.dirname(target);
  if (/[*?[{]/u.test(dir)) return false;
  try {
    const real = located(dir);
    return within(real, temp) && !(real === temp && /[*?[{]/u.test(path.basename(target)));
  } catch {
    return false;
  }
}

function deletion({ command, dir }: Launch) {
  const [name, ...args] = command;
  if (name === "rm") {
    const files = targets(args);
    return files.length > 0 && files.every((file) => removable(file, dir));
  }
  if (name === "xargs") return !wrapped(command).some((word) => path.basename(word) === "rm");
  return name !== "find" || sweep(args, dir);
}

function segments(args: string[], dir: string | undefined) {
  return args.flatMap((arg, at) => {
    if (!runners.has(arg)) return [];
    const end = args.findIndex((word, after) => after > at && (word === ";" || word === "+"));
    const base = arg === "-exec" || arg === "-ok" ? dir : undefined;
    return [{ words: args.slice(at + 1, end < 0 ? undefined : end), base }];
  });
}

function confined({ words, base }: { words: string[]; base: string | undefined }): boolean {
  return words.every((word, at) => {
    const name = path.basename(word);
    if (name === "find") return sweep(words.slice(at + 1), base);
    return name !== "rm" || targets(words.slice(at + 1)).every((file) => file === "{}" || removable(file, base));
  });
}

function sweep(args: string[], dir: string | undefined): boolean {
  const execs = segments(args, dir);
  if (!execs.every(confined)) return false;
  const removes = execs.some(({ words }) => words.some((word) => path.basename(word) === "rm"));
  if (!removes && !args.includes("-delete")) return true;
  let at = 0;
  while (at < args.length && /^-[HLPO]/u.test(args[at])) at++;
  if (args[at] === "-D") at += 2;
  const roots: string[] = [];
  while (at < args.length && !/^[-(!,)]/u.test(args[at])) roots.push(args[at++]);
  return (roots.length > 0 ? roots : ["."]).every((root) => removable(root, dir));
}

function collected([, ...args]: Command) {
  return options(args, 0, systemd).found.some(
    ({ flag, value }) =>
      flag === "G" ||
      flag === "collect" ||
      ((flag === "p" || flag === "property") && value?.startsWith("CollectMode=")),
  );
}

function units(command: Command) {
  return flags(command).flatMap((found) =>
    named(found, ["u", "unit"]) && found.value !== undefined ? [found.value] : [],
  );
}

function text(file: string) {
  try {
    return readFileSync(file, "utf8");
  } catch {
    return undefined;
  }
}

function contained(gitdir: string) {
  if (!within(gitdir, temp)) return false;
  const common = text(path.join(gitdir, "commondir"))?.trim();
  const base = common ? located(path.resolve(gitdir, common)) : gitdir;
  if (!within(base, temp)) return false;
  const budget = { left: entries };
  if (![...new Set([gitdir, base])].every((dir) => unlinked(dir, budget))) return false;
  const objects = path.join(base, "objects");
  const alternates = text(path.join(objects, "info/alternates")) ?? "";
  for (const line of alternates.split("\n").map((entry) => entry.trim())) {
    if (line && !line.startsWith("#") && !within(located(path.resolve(objects, line)), temp)) return false;
  }
  const config = `${text(path.join(base, "config")) ?? ""}\n${text(path.join(gitdir, "config.worktree")) ?? ""}`;
  return !/^\s*(?:worktree\s*=|\[\s*include)/imu.test(config);
}

function unlinked(dir: string, budget: { left: number }): boolean {
  let children;
  try {
    children = readdirSync(dir, { withFileTypes: true });
  } catch {
    return false;
  }
  return children.every((child) => {
    if (--budget.left < 0 || child.isSymbolicLink()) return false;
    return !child.isDirectory() || unlinked(path.join(dir, child.name), budget);
  });
}

function scratch(dir: string) {
  const real = located(dir);
  if (!within(real, temp)) return false;
  for (let at = real; within(at, temp); at = path.dirname(at)) {
    const link = path.join(at, ".git");
    const info = statSync(link, { throwIfNoEntry: false });
    if (info?.isDirectory()) return contained(located(link));
    if (info) {
      const pointer = /^gitdir: (.+)$/mu.exec(text(located(link)) ?? "")?.[1].trim();
      return pointer !== undefined && contained(located(path.resolve(at, pointer)));
    }
    if (
      statSync(path.join(at, "HEAD"), { throwIfNoEntry: false }) &&
      statSync(path.join(at, "objects"), { throwIfNoEntry: false })
    ) {
      return contained(at);
    }
  }
  return true;
}

type Invocation = { args: string[]; cwd: string | undefined; local: boolean };

function locate(args: string[], cwd: string | undefined) {
  let dir = cwd;
  const dirs: (string | undefined)[] = [];
  const written: string[] = [];
  const configs: string[] = [];
  let at = 0;
  for (; at < args.length && args[at].startsWith("-"); at++) {
    const arg = args[at];
    const [flag, attached] = /^-[Cc]./u.test(arg) ? [arg.slice(0, 2), arg.slice(2)] : arg.split(/=(.*)/su);
    if (!globals.has(flag)) continue;
    const value = attached ?? args[++at];
    if (value === undefined) break;
    if (flag === "-c" || flag === "--config-env") configs.push(value.split("=")[0]);
    if (flag === "-C" || flag === "--git-dir" || flag === "--work-tree") written.push(value);
    const target = resolved(value, dir);
    if (flag === "-C") dir = target;
    if (flag === "--git-dir" || flag === "--work-tree") dirs.push(target);
  }
  return { at, dir, places: [dir, ...dirs], written, configs };
}

function resolved(place: string | undefined, dir: string | undefined) {
  if (place === undefined || expansion.test(place)) return undefined;
  const full = expand(place);
  return path.isAbsolute(full) ? full : dir && path.resolve(dir, full);
}

function created(sub: string, rest: string[], dir: string | undefined) {
  if (sub !== "clone" && sub !== "init") return [];
  const { operands } = parts(rest, fetchValued);
  const target = sub === "clone" ? (operands[1] ?? path.basename(operands[0] ?? "", ".git")) : operands[0];
  const separate = rest.flatMap((arg, at) => {
    const [flag, value] = arg.split(/=(.*)/su);
    return abbreviates(flag, ["--separate-git-dir"]) ? [value ?? rest[at + 1]] : [];
  });
  return [...(target === undefined ? [] : [target]), ...separate].map((place) => resolved(place, dir));
}

function git({ args, cwd, local }: Invocation) {
  const { at, dir, places, configs } = locate(args, cwd);
  const sub = args[at];
  if (sub === undefined) return undefined;
  const rest = args.slice(at + 1);
  const all = [...places, ...created(sub, rest, dir)];
  const settings =
    sub === "config" &&
    parts(rest).switches.some((flag) => ["--global", "--system", "--file", "-f", "--blob"].includes(flag));
  const trusted =
    local &&
    !settings &&
    configs.every((key) => harmless.test(key)) &&
    all.every((place) => place !== undefined && scratch(place));
  if (trusted) return undefined;
  const read: Read | undefined = Object.hasOwn(reads, sub) ? reads[sub] : undefined;
  if (read === true || read?.(rest)) return undefined;
  const label = read ? [sub, ...rest.slice(0, 1)].join(" ") : sub;
  return `children may not run \`git ${label}\` in a real repo; return the step to Collab`;
}

function tampers({ command: [name, ...args] }: Launch) {
  if (name === redirection) return nonliteral.test(args[0]) || gitPath.test(args[0]);
  if (copiers.has(name) && args.some((arg) => nonliteral.test(arg))) return true;
  if (name !== "git") return args.some((arg) => gitPath.test(arg));
  const at = locate(args, undefined).at;
  if (args[at] !== "config") return false;
  const rest = args.slice(at + 1);
  if (inquiry(rest)) return false;
  const { operands } = parts(rest, ["-f", "--file", "--blob", "--type", "--default", "--comment"]);
  const key = operands[0] === "set" || operands[0] === "unset" ? operands[1] : operands[0];
  return !harmless.test(key ?? "");
}

function option(args: string[], { long, short = "" }: Program) {
  for (const arg of args) {
    if (arg.startsWith("--")) {
      const [flag] = arg.split("=");
      if (abbreviates(flag, long)) return flag;
      continue;
    }
    if (!arg.startsWith("-")) continue;
    for (const char of arg.slice(1)) if (short.includes(char)) return `-${char}`;
  }
  return undefined;
}

function program({ args }: { args: string[] }) {
  const { at } = locate(args, undefined);
  if (args.slice(0, at).some((arg) => arg.startsWith("--exec-path=")))
    return "`git --exec-path=` makes Git run its helper programs from a chosen directory; agents may not pass it";
  const sub = args[at];
  if (sub === undefined || !Object.hasOwn(programs, sub)) return undefined;
  const flag = option(args.slice(at + 1), programs[sub]);
  return flag && `\`git ${sub}\` with \`${flag}\` makes Git run a chosen program; agents may not pass it`;
}

function dangers({ args }: { args: string[] }) {
  return args.flatMap((arg, at) => {
    const [flag, attached] = arg.split(/=(.*)/su);
    const short = /^-[A-Za-z0-9]*?c(.*)$/su.exec(arg)?.[1];
    const value = abbreviates(flag, ["--config", "--config-env"])
      ? (attached ?? args[at + 1])
      : short === undefined
        ? undefined
        : short || args[at + 1];
    const key = value?.split("=")[0];
    return key && dangerous.test(key)
      ? [`\`git -c ${key}\` sets a Git option that can run commands; agents may not pass it`]
      : [];
  });
}

function rewrite(sub: string, args: string[]) {
  for (const arg of args) {
    const flag = arg.startsWith("--") ? arg.split("=")[0] : /^-[^-]*?([fd])/u.exec(arg)?.[1];
    if (flag && (!flag.startsWith("--") || abbreviates(flag, rewrites)))
      return `\`git ${sub}\` with \`${flag.startsWith("--") ? flag : `-${flag}`}\` rewrites or deletes remote history`;
  }
  const spec = args.find((arg) => arg.startsWith("+") || arg.startsWith(":"));
  return spec && `\`git ${sub}\` with the refspec \`${spec}\` rewrites or deletes remote history`;
}

function push({ args, cwd, local, head, hidden }: Reading) {
  const { at, places, configs } = locate(args, cwd);
  const sub = args[at];
  if (sub === undefined || !pushes.has(sub)) return undefined;
  const forced = rewrite(sub, args.slice(at + 1));
  if (forced || !head) return forced;
  if (hidden)
    return `\`git ${sub}\` sits behind a wrapper, redirect, or helper path the guard does not parse; run it as plain \`git ${sub}\``;
  if (configs.length > 0) return `\`git ${sub}\` with \`-c\` or \`--config-env\` can change what it pushes`;
  if (!local || places.some((place) => place === undefined)) {
    return `\`git ${sub}\` runs in a repo the guard cannot determine`;
  }
  const place = places.find((dir) => dir !== undefined && within(located(dir), located(attended)));
  if (place) return `\`git ${sub}\` from ${place} needs approval for repos under ${attended}`;
  return undefined;
}

function expanded({ args, head }: Reading) {
  const { at, written } = locate(args, undefined);
  const sub = args[at];
  if (!head || sub === undefined || !pushes.has(sub)) return [];
  return [...written, ...args.slice(at + 1)].filter((word) => expansion.test(word));
}

type Reading = Invocation & { head: boolean; hidden: boolean };

function readings(launched: Launch[], local: boolean) {
  const calls: Reading[] = [];
  const removals: Launch[] = [];
  for (const { command, dir } of launched) {
    const scripts = remote(command);
    if (scripts.length > 0) {
      for (const script of scripts) calls.push(...readings(launches(script, undefined, {}, false), false).calls);
      continue;
    }
    const [name, ...args] = command;
    if (name === "find") {
      for (const {
        words: [head, ...tail],
        base,
      } of segments(args, dir))
        if (head !== undefined)
          calls.push(...readings([{ command: [path.basename(head), ...tail], dir: base }], local).calls);
      continue;
    }
    const words = argv(command);
    const subs = new Set<number>();
    words.forEach((word, at) => {
      const base = path.basename(word);
      const helper = /^git-([\w-]+)$/u.exec(base)?.[1];
      if (base === "git" || helper) {
        const rest = helper ? [helper, ...words.slice(at + 1)] : words.slice(at + 1);
        const sub = locate(rest, undefined).at;
        if (!helper) subs.add(at + 1 + sub);
        const seen = at === 0 && name === "git" ? args[locate(args, undefined).at] : undefined;
        calls.push({ args: rest, cwd: dir, local, head: at === 0, hidden: at === 0 && rest[sub] !== seen });
      } else if (base === "find") {
        removals.push({ command: ["find", ...words.slice(at + 1)], dir });
      } else if (base === "rm" && !subs.has(at) && !swept(words, at) && !exempt(words, at))
        removals.push({ command: ["rm", ...words.slice(at + 1)], dir });
    });
  }
  return { calls, removals };
}

function swept(words: string[], at: number) {
  for (let index = at - 1; index >= 0 && words[index] !== ";" && words[index] !== "+"; index--)
    if (runners.has(words[index])) return words.slice(0, index).some((word) => path.basename(word) === "find");
  return false;
}

function exempt(words: string[], at: number) {
  return (
    tools.has(path.basename(words[0])) &&
    words.slice(1, at).every((word) => /^[a-z][a-z-]*$/u.test(word) && !verbs.has(word))
  );
}

export function inspect(
  source: string,
  agent: string | undefined,
  workdir: string,
  known: Record<string, string> = {},
) {
  const caged = readOnly(agent);
  const owner = owners.has(agent ?? "");
  const launched = launches(source, workdir, known, !owner);
  const commands = launched.map(({ command }) => command);
  const { calls, removals } = readings(
    [...launched, ...respaced(source, workdir, known, !owner)],
    !relocations.test(source) && !launched.some(tampers),
  );
  const found = new Set<string>();
  if (commands.some((command) => command[0] === "systemd-run" && !collected(command))) {
    found.add(
      "systemd-run must pass `--collect` (`-G`), since a failed unit otherwise stays loaded and hyprd health reports it; when a test needs the failed unit to stay, pass `-p CollectMode=inactive` instead",
    );
  }
  if (
    commands.some(
      (command) => command[0] === "systemd-run" && units(command).some((unit) => unit.startsWith("cogikyo-")),
    )
  ) {
    found.add(
      "agent shells may not start `cogikyo-*` systemd units, since those names carry voice-loop rights on cogikyo's bus; Cullyn starts them from his key or terminal instead",
    );
  }
  if (commands.some(([name]) => name === "grok"))
    found.add("grok is disabled in the shell; use the `x` tool for X search");
  if (commands.some(([name, ...args]) => (name === "opencode" || name === "opencode-v2") && args.includes("api"))) {
    found.add(
      "agents must not call the OpenCode server API; `opencode api` is disabled in the shell; to create a session the user asked for, call `tools.opencode.session_create` through `execute`",
    );
  }
  // cage's --hide operands name denied files, not attempted reads.
  const args = commands.filter(([name]) => name !== "cage").flatMap(([, ...rest]) => rest);
  for (const secret of args.flatMap((arg) => credentials.map((file) => file.exec(arg)?.[0])).filter(Boolean)) {
    found.add(`\`${secret?.replace(/^[\s/=@<>:]/u, "")}\` is a credential file; agents must not read it`);
  }
  if (args.some((arg) => database.test(arg))) found.add(databaseReason);
  if (args.some((arg) => profile.test(arg))) found.add(profileReason);
  if (args.some(geminiArg)) found.add(geminiReason);
  for (const problem of calls.map(program)) if (problem) found.add(problem);
  if (!caged) {
    if (commands.includes(opaque)) found.add(unchecked);
    const blind = commands.includes(unseen) || /\bTRAP[A-Z]+\s*\(\s*\)|\bfunction\s+TRAP[A-Z]/u.test(source);
    if (blind && !owner) {
      found.add(
        "the command hands another program or a deferred builtin a script whose body or execution context the guard cannot recover literally, or passes shell options the guard does not parse, so the guard cannot check its git or `rm` calls; pass a literal script body or run the commands directly",
      );
    }
    if (commands.includes(streamed)) {
      found.add(
        "a shell here reads its script from stdin, a pipe, or a device the guard cannot see; pass the script with `-c` or a heredoc instead",
      );
    }
    // Cage already confines read-only deletion to scratch tmp/ and cache/.
    if (![...launched, ...removals].every(deletion)) {
      found.add(
        `\`rm\`, \`find -delete\`, and \`find -exec rm\` take only paths inside ${temp}/, written literally or through a variable assigned earlier in the command, absolute or relative to a known directory (\`workdir\`, or \`cd <dir> &&\`), without \`..\`, a trailing slash, or a top-level glob, and \`xargs rm\` is not allowed (\`$OPENCODE_SCRATCH\` is unset outside the read-only cage); move anything else to trash with \`trash -- <path>\``,
      );
    }
    const dynamic = launched.filter(({ command: [name] }) => unresolved(name));
    const names = [...new Set(dynamic.filter(({ guess }) => !guess).map(({ command: [name] }) => `\`${name}\``))];
    if (names.length > 0) {
      found.add(
        `the command name ${names.join(", ")} is expanded at run time; write the command name literally so the guard can check it`,
      );
    }
    for (const { command, guess } of dynamic) {
      if (!guess) continue;
      found.add(
        `\`${command[0]}\` may be the command name, expanded at run time, because the guard cannot tell whether \`${guess.flag}\` in \`${guess.wrapper} ${guess.flag}\` takes a value; end the wrapper options with \`--\` before the command, write a value as \`--option=value\`, or drop \`${guess.flag}\`, so the guard can find the command`,
      );
    }
    for (const reason of pathCredentials(literalLaunches(source, workdir, !owner))) found.add(reason);
    for (const problem of calls.flatMap(dangers)) found.add(problem);
    if (!owner)
      for (const problem of calls.map((call) => (call.head ? git(call) : push(call)))) if (problem) found.add(problem);
    const expansions = [...new Set(calls.flatMap(expanded))];
    if (expansions.length > 0) {
      found.add(
        `the \`git push\` repo path or argument ${expansions.map((word) => `\`${word}\``).join(", ")} is expanded at run time; write it literally, or through a variable assigned earlier in the command, so the guard can check the push`,
      );
    }
  }
  return {
    problems: [...found],
    push: calls.map(push).find(Boolean),
    commits: calls
      .filter(({ head, args: words }) => head && words[locate(words, undefined).at] === "commit")
      .map(({ cwd }) => cwd),
  };
}
