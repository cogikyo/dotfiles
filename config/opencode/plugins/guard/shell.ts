import path from "node:path";
import { expand, home, temp, within } from "./paths.ts";

export type Command = readonly [name: string, ...args: string[]];
export type Launch = { command: Command; dir: string | undefined; dirs?: ReadonlySet<string> };

type Context = {
  depth: number;
  resolve: boolean;
  known: Record<string, string>;
  handoffs: boolean;
  paths?: { base: string; dirs: Set<string> };
};
type Scope = Context & {
  dir: string | undefined;
  bodies: string[];
  dynamic: ReadonlySet<number>;
  temporary: ReadonlyMap<number, string>;
  literal?: boolean;
};

type Spec = { short: string; long: string[]; positional?: number };
type Option = { flag: string; value?: string; at: number };

export const maxDepth = 8;
export const opaque: Command = [""];
export const streamed: Command = [""];
export const redirection = ">";
export const unseen: Command = [""];
const assignment = /^[A-Za-z_]\w*=/u;
const redirect = /^\d*(?:[<>]|&>)/u;
const bareRedirect = /^\d*(?:>>?|<<?|&>>?|>&|<&)$/u;
const heredoc =
  /(<<-?\s*)(['"]?)((?<=')[^'\n]+|(?<=")[^"\n]+|(?<!['"])[^\s;&|()<>'"\\]+)\2([^\n]*)\n([\s\S]*?)\n\s*\3[ \t]*(?=\n|$)/gu;
const marker = /^0?<<-?\uE000(\d+)$/u;
const inert = new Set(["cd", "chdir", "echo", "true", "false", "test", "[", "pwd", ":", "printf"]);
const builtins = new Set(
  [
    "alias autoload bg bind bindkey break builtin bye caller case cd chdir command compgen complete compopt continue coproc",
    "declare dirs disable disown echotc echoti emulate enable eval exec exit export false fc fg float for foreach function",
    "functions getln getopts hash help history integer jobs kill let limit local log logout mapfile noglob popd print printf",
    "pushd pushln pwd read readarray readonly rehash repeat return sched select set setopt shift shopt source suspend test",
    "times trap true ttyctl type typeset ulimit umask unalias unfunction unhash unlimit unset unsetopt vared wait whence",
    "where which zcompile zformat zle zmodload zparseopts zregexparse zstyle zsystem [[ . : [",
  ]
    .join(" ")
    .split(" "),
);
const specials =
  /^(?:PWD|OLDPWD|HOME|PATH|IFS|CDPATH|REPLY|OPTARG|OPTIND|RANDOM|SECONDS|LINENO|MATCH|MBEGIN|MEND|_|(?:BASH|ZSH)_\w*)$/u;
const simple = /\$(?:\{[A-Za-z_]\w*\}|[A-Za-z_]\w*(?![\w[:({])|[\d?$#!@*-]|['"]|\((?!\()|(?=[\s;&|)}]|$))/uy;
const definition = /\bfunction\s|[\w.-]\s*\(\s*\)/u;
const leading = /^\s*cd[ \t]+('[^'\n]*'|"[^"\n]*"|[^\s;&|()<>'"]+)[ \t]*(?:&&|;|\n)\s*/u;
const keywords = new Set(["!", "{", "}", "if", "then", "else", "elif", "do", "while", "until"]);
const runners = new Set(["-exec", "-execdir", "-ok", "-okdir"]);
const strings = new Set(["parallel", "su", "runuser", "script", "tmux", "screen", "fc", "sched"]);
const switches = "abcdefghiklmnprstuvxBCEHPT";
const toggles = new Set([
  "norc",
  "noprofile",
  "login",
  "posix",
  "restricted",
  "verbose",
  "noediting",
  "no-rcs",
  "no-globalrcs",
]);
const valued = new Set(["rcfile", "init-file", "emulate"]);
const openers = new Set(["{", "if", "while", "until", "for", "select", "case"]);
const closers = new Set(["}", "fi", "done", "esac"]);
const reference = /\$(?:\{([A-Za-z_]\w*)\}|([A-Za-z_]\w*)(?![\w[:({]))/uy;
const literal = /^(?!~)[^$`]*$/u;
const navigators = new Set(["cd", "chdir", "pushd", "popd"]);
const resets = new Set([...navigators, "eval", "source", "."]);
const breaks = new Set([";", "\n", "&", "||", "(", ")"]);
const shells = new Set(["bash", "dash", "ksh", "sh", "zsh"]);
const devices = /^(?:-|\/(?:dev|proc)\/.*)$/u;
const outputs = /^\d*(?:>&|&>>?|>>?\|?)/u;
const plain: Spec = { short: "", long: [] };
const env: Spec = { short: "uCS", long: ["unset", "chdir", "split-string"] };
const ssh: Spec = { short: "BbcDEeFIiJLlmOoPpQRSWw", long: [] };
const chdirs: Record<string, string[]> = {
  env: ["C", "chdir"],
  sudo: ["D", "chdir"],
  "systemd-run": ["working-directory"],
};

const wrappers: Record<string, Spec> = {
  builtin: plain,
  nohup: plain,
  setsid: plain,
  cage: { short: "", long: ["hide"], positional: 2 },
  exec: { short: "a", long: [] },
  nice: { short: "n", long: ["adjustment"] },
  stdbuf: { short: "ioe", long: ["input", "output", "error"] },
  time: { short: "fo", long: ["format", "output"] },
  timeout: { short: "sk", long: ["signal", "kill-after"], positional: 1 },
  sudo: {
    short: "ugCDhprtUR",
    long: ["user", "group", "close-from", "chdir", "host", "prompt", "role", "type", "other-user", "chroot"],
  },
  "systemd-run": {
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
  },
  xargs: {
    short: "adEILnPs",
    long: ["arg-file", "delimiter", "max-args", "max-procs", "max-chars", "process-slot-var"],
  },
};

export function launches(source: string, dir?: string, known: Record<string, string> = {}, handoffs = true): Launch[] {
  const blind = walk(source, dir, { depth: 0, resolve: false, known, handoffs });
  if (!source.includes("$") || !steady(source, blind)) return blind;
  return walk(source, dir, { depth: 0, resolve: true, known, handoffs });
}

export function literalLaunches(source: string, dir: string, handoffs: boolean) {
  return walk(source, dir, {
    depth: 0,
    resolve: false,
    known: {},
    handoffs,
    paths: { base: dir, dirs: new Set([dir]) },
  });
}

export function literalPath(word: string, dir: string | undefined) {
  if (!word || /[$`*?[{]/u.test(word) || (word.startsWith("~") && word !== "~" && !word.startsWith("~/"))) {
    return undefined;
  }
  const full = expand(word);
  return path.isAbsolute(full) ? path.normalize(full) : dir === undefined ? undefined : path.resolve(dir, full);
}

function literalDestination(words: readonly string[], base: string | undefined) {
  const [name, ...rest] = words;
  if (name !== "cd" && name !== "chdir" && name !== "pushd") return undefined;
  const args = operands(rest, 0);
  if (args[0] === "--") args.shift();
  if (args.length > 1 || args[0] === "-") return undefined;
  const target = args[0] ?? (name === "cd" || name === "chdir" ? home : undefined);
  return target === undefined ? undefined : literalPath(target, base);
}

function steady(source: string, launched: Launch[]) {
  source = source.replace(/\$\(\([\d\s()+*/%<>&|^~!?:.-]*\)\)/gu, "0");
  if (source.includes("((") || definition.test(source)) return false;
  for (let at = source.indexOf("$"); at >= 0; at = source.indexOf("$", at + 1)) {
    simple.lastIndex = at;
    if (!simple.test(source)) return false;
  }
  return launched.every(({ command: [name, ...args] }) => {
    if (/[$`]/u.test(name)) return false;
    if (name === "printf") return !args.some((arg) => arg.startsWith("-v"));
    return inert.has(name) || !builtins.has(name);
  });
}

export function enter(source: string, workdir: string) {
  // Do not fold a chain: skipping an intermediate failed cd could let later commands run.
  const match = leading.exec(source);
  const target = match && unquote(match[1]);
  const rest = match ? source.slice(match[0].length) : "";
  if (!target || target.split("/").includes("..") || !rest.trim() || /^[&|;]/u.test(rest)) {
    return { source, dir: workdir };
  }
  return { source: rest, dir: path.resolve(workdir, target) };
}

function unquote(word: string) {
  const quoted = /^(['"])(.*)\1$/su.exec(word);
  if (quoted?.[1] === '"' && /[$`\\]/u.test(quoted[2])) return undefined;
  const text = quoted ? quoted[2] : word;
  if (text === "" || /^[-+#]/u.test(text)) return undefined;
  if (quoted) return text;
  if (/[$`\\*?[{]/u.test(text)) return undefined;
  if (text === "~" || text.startsWith("~/")) return home + text.slice(1);
  return text.startsWith("~") ? undefined : text;
}

function walk(source: string, dir: string | undefined, context: Context): Launch[] {
  if (context.depth > maxDepth) return [{ command: opaque, dir }];
  const bodies: string[] = [];
  const unquoted: string[] = [];
  const prepared = unheredoc(source, bodies, unquoted);
  const lexer = new Lexer(prepared, context).run();
  const inner = { ...context, depth: context.depth + 1 };
  const launched = [
    ...lexer.redirects.flatMap(({ segment, word }): Launch[] => {
      const target = lexer.segments[segment][word + 1];
      return target === undefined ? [] : [{ command: [redirection, target], dir, dirs: context.paths?.dirs }];
    }),
    ...expansions(lexer, inner),
    ...unquoted.flatMap((body) => walk(`: "${body.replaceAll('"', '\\"')}"`, undefined, inner)),
  ];
  let here = dir;
  let after = dir;
  lexer.segments.forEach((words, index) => {
    const before = lexer.ops[index - 1] ?? "";
    const op = lexer.ops[index] ?? "";
    const step = peel(words, {
      ...inner,
      dir: here,
      bodies,
      dynamic: lexer.dynamic[index],
      temporary: lexer.temporary[index],
    });
    launched.push(...step.launched);
    if (context.paths) {
      here = step.dir;
      return;
    }
    if (step.moved) {
      const clean = navigators.has(words[0]) && lexer.levels[index] === 0 && op === "&&" && !/\|/u.test(before);
      here = clean ? destination(words, here) : undefined;
      after = undefined;
    }
    if (breaks.has(op)) here = after;
  });
  return launched;
}

function expansions({ subs, sums, hidden }: Lexer, context: Context): Launch[] {
  if (hidden) return [{ command: opaque, dir: undefined }];
  return [
    ...subs.flatMap((sub) => walk(sub, undefined, context)),
    ...sums.flatMap((sum) =>
      context.depth < maxDepth
        ? expansions(new Lexer(sum, context, true).run(), { ...context, depth: context.depth + 1 })
        : [{ command: opaque, dir: undefined }],
    ),
  ];
}

function destination([name, ...args]: string[], base: string | undefined) {
  if (name === "popd") return undefined;
  if (args.length === 0) return name === "cd" || name === "chdir" ? home : undefined;
  const [to] = args;
  if (args.length > 1 || to === "" || /^[~+-]/u.test(to) || /[$`*?[{]/u.test(to)) return undefined;
  if (to.split("/").includes("..")) return undefined;
  if (path.isAbsolute(to)) return path.normalize(to);
  return base === undefined ? undefined : path.resolve(base, to);
}

export function flags([name, ...args]: Command) {
  const spec = name === "env" ? env : wrappers[name];
  return spec ? options(args, 0, spec).found : [];
}

export function wrapped([name, ...args]: Command) {
  const spec = wrappers[name];
  return spec ? args[options(args, 0, spec).next + (spec.positional ?? 0)] : undefined;
}

export function remote([name, ...args]: Command) {
  if (name !== "ssh") return undefined;
  const { next } = options(args, 0, ssh);
  return args.slice(next + 1).join(" ") || undefined;
}

function unheredoc(source: string, bodies: string[], unquoted: string[]) {
  return source.replace(heredoc, (...parts: string[]) => {
    const [, op, quote, , rest, body] = parts;
    bodies.push(body);
    if (!quote) unquoted.push(body);
    return `${op.trimEnd()}\uE000${bodies.length - 1}${rest}`;
  });
}

function peel(words: string[], scope: Scope, start = 0) {
  const launched: Launch[] = [];
  let moved = false;
  let dir = scope.dir;
  let index = start;
  while (index < words.length) {
    const word = words[index];
    if (assignment.test(word) || keywords.has(word)) {
      index++;
      continue;
    }
    if (redirect.test(word)) {
      index += bareRedirect.test(word) ? 2 : 1;
      continue;
    }
    const name = path.basename(word);
    const command: Command = [
      name,
      ...words.slice(index + 1).map((arg, at) => (name === "rm" ? (scope.temporary.get(index + at + 1) ?? arg) : arg)),
    ];
    launched.push({ command, dir, ...(scope.paths ? { dirs: scope.paths.dirs } : {}) });
    if (scope.paths) {
      const next = literalDestination(command, dir);
      const base = literalDestination(command, scope.paths.base);
      if (next !== undefined) scope.paths.dirs.add(next);
      if (base !== undefined) scope.paths.dirs.add(base);
      if (next !== undefined) dir = next;
    }
    moved ||= resets.has(name) || (name === "emulate" && emulation(words, index + 1).script);
    if (flags(command).some(({ flag }) => chdirs[name]?.includes(flag))) dir = undefined;
    const next = unwrap(name, words, index + 1, { ...scope, dir }, launched);
    if (next === undefined) {
      launched.push(...nested(name, words, index + 1, { ...scope, dir }));
      break;
    }
    index = next;
  }
  if (!scope.paths) launched.push(...writes(words.slice(start), dir));
  return { launched, moved, dir };
}

function writes(words: string[], dir: string | undefined): Launch[] {
  return words.flatMap((word, at) => {
    const op = outputs.exec(word)?.[0];
    if (op === undefined) return [];
    const target = word.slice(op.length) || words[at + 1];
    if (target === undefined || (op.endsWith(">&") && /^(?:\d+|-)$/u.test(target))) return [];
    return [{ command: [redirection, target], dir }];
  });
}

function nested(name: string, words: string[], start: number, scope: Scope): Launch[] {
  const args = words.slice(start);
  if (["watch", "flock"].includes(name)) {
    if (!scope.handoffs) return [{ command: unseen, dir: scope.dir }];
    return handoff(name, words, start, scope);
  }
  if (strings.has(name) || evaluates(name, args)) return [{ command: unseen, dir: scope.dir }];
  if (shells.has(name) || name === "eval" || name === "emulate" || name === "source" || name === ".") return [];
  if (name !== "find") {
    if (!invokes(args)) return [];
    if (!scope.handoffs) return [{ command: unseen, dir: scope.dir }];
    return runner(name, words, start, scope);
  }
  return args.flatMap((arg, at) => {
    if (!runners.has(arg)) return [];
    const end = args.findIndex((word, after) => after > at && (word === ";" || word === "+"));
    const inner = args[at + 1];
    const dir = arg === "-exec" || arg === "-ok" ? scope.dir : undefined;
    if (!shells.has(path.basename(inner ?? "")))
      return invokes(args.slice(at + 1, end < 0 ? undefined : end)) ? [{ command: unseen, dir }] : [];
    const launched: Launch[] = [];
    unwrap(
      path.basename(inner),
      words.slice(0, end < 0 ? undefined : start + end),
      start + at + 2,
      { ...scope, dir },
      launched,
    );
    return launched;
  });
}

function runner(name: string, words: string[], start: number, scope: Scope): Launch[] {
  const blind = [{ command: unseen, dir: scope.dir }];
  if (name !== "unshare" && (name !== "uv" || words[start] !== "run")) return blind;
  const spec =
    name === "unshare"
      ? {
          short: "RwSGl",
          long: [
            "root",
            "wd",
            "setuid",
            "setgid",
            "map-user",
            "map-group",
            "map-users",
            "map-groups",
            "owner",
            "setgroups",
            "monotonic",
            "boottime",
            "load-interp",
            "propagation",
          ],
        }
      : {
          short: "pw",
          long: [
            "with",
            "with-editable",
            "with-requirements",
            "project",
            "directory",
            "python",
            "package",
            "env-file",
            "index",
            "index-url",
            "extra-index-url",
            "config-file",
          ],
        };
  const { next, found } = options(words, name === "uv" ? start + 1 : start, spec);
  if (name === "unshare" && found.some(({ flag }) => flag === "R" || flag === "root")) return blind;
  if (name === "uv" && found.some(({ flag }) => ["m", "module", "s", "script", "gui-script"].includes(flag)))
    return blind;
  if (!shells.has(path.basename(words[next] ?? ""))) return blind;
  const moved = found.some(({ flag }) => flag === "wd" || flag === "directory" || (name === "unshare" && flag === "w"));
  return peel(words, { ...scope, dir: moved ? undefined : scope.dir, literal: true }, next).launched;
}

function handoff(name: string, words: string[], start: number, scope: Scope): Launch[] {
  const blind = [{ command: unseen, dir: scope.dir }];
  if (name === "watch") {
    const { next, found } = options(words, start, { short: "n", long: ["interval"] });
    if (found.some(({ flag }) => flag === "x" || flag === "exec"))
      return peel(words, { ...scope, literal: true }, next).launched;
    if (next >= words.length || [...scope.dynamic].some((index) => index >= next)) return blind;
    return walk(words.slice(next).join(" "), scope.dir, scope);
  }
  return flock(words, start, scope);
}

function flock(words: string[], start: number, scope: Scope): Launch[] {
  const blind = [{ command: unseen, dir: scope.dir }];
  const spec = {
    short: "wEc",
    long: ["wait", "timeout", "conflict-exit-code", "command"],
  };
  const front = options(words, start, spec);
  const trailing = options(words, front.next + 1, spec);
  const end = words.indexOf("--", start);
  if ([...scope.dynamic].some((index) => index >= start && (end < 0 || index < end))) return blind;
  const script = [...front.found, ...trailing.found].find(({ flag }) => flag === "c" || flag === "command");
  if (script) {
    if (script.value === undefined || scope.dynamic.has(script.at)) return blind;
    return walk(script.value, scope.dir, scope);
  }
  const command = trailing.next;
  return command >= words.length ? [] : peel(words, { ...scope, literal: true }, command).launched;
}

function evaluates(name: string, args: string[]) {
  if (name === "trap") return args[0] !== undefined && args[0] !== "-" && args[0] !== "" && !/^-[lp]+$/u.test(args[0]);
  if (name === "alias") return args.some((arg) => arg.includes("="));
  return name === "zle" && args.includes("-N");
}

function interpreter(words: string[], start: number) {
  const state = { next: start, script: false, stdin: false, noexec: false, unknown: false };
  for (; state.next < words.length; state.next++) {
    const word = words[state.next];
    if (word === "--" || word === "-") {
      state.next++;
      break;
    }
    if (!/^[-+]./u.test(word)) break;
    const on = word.startsWith("-");
    if (word.startsWith("--")) {
      const [flag, value] = word.slice(2).split(/=(.*)/su);
      if (valued.has(flag) && value === undefined) state.next++;
      else if (!toggles.has(flag) && !valued.has(flag)) state.unknown = true;
      continue;
    }
    for (let at = 0; at < word.length - 1; at++) {
      const letter = word[at + 1];
      if (letter === "o" || letter === "O") {
        const option = word.slice(at + 2) || words[++state.next];
        if (option === "noexec") state.noexec = on;
        break;
      }
      if (!switches.includes(letter) || (letter === "c" && !on)) state.unknown = true;
      if (letter === "n") state.noexec = on;
      if (letter === "c") state.script = true;
      if (letter === "s") state.stdin = true;
    }
  }
  return state;
}

function invokes(args: string[]) {
  return args.some((arg, at) => shells.has(path.basename(arg)) && /^-[A-Za-z]*c/u.test(args[at + 1] ?? ""));
}

function emulation(words: string[], start: number) {
  const front = interpreter(words, start);
  return front.script ? front : interpreter(words, front.next + 1);
}

function unwrap(name: string, words: string[], start: number, scope: Scope, launched: Launch[]) {
  if (name === "eval") {
    launched.push(...walk(words.slice(start).join(" "), scope.dir, scope));
    return undefined;
  }
  if (name === "emulate") {
    if ([...scope.dynamic].some((at) => at >= start)) {
      launched.push({ command: unseen, dir: scope.dir });
      return undefined;
    }
    const { next, script } = emulation(words, start);
    if (!script) return undefined;
    const body = words[next];
    launched.push(...(body === undefined ? [{ command: unseen, dir: scope.dir }] : walk(body, scope.dir, scope)));
    return undefined;
  }
  if (shells.has(name)) {
    const { next, script, stdin: reads, noexec, unknown } = interpreter(words, start);
    if (unknown) {
      launched.push({ command: unseen, dir: scope.dir });
      return undefined;
    }
    if (noexec) return undefined;
    if (script) {
      const body = words[next];
      launched.push(
        ...(body === undefined || (scope.literal && scope.dynamic.has(next))
          ? [{ command: unseen, dir: scope.dir }]
          : walk(body, scope.dir, scope)),
      );
      return undefined;
    }
    const file = operands(words, next)[0];
    if (file === undefined || devices.test(file) || reads) {
      stdin(words, start, scope, launched);
    }
    return undefined;
  }
  if (name === "source" || name === ".") {
    if (devices.test(operands(words, start)[0] ?? "")) stdin(words, start, scope, launched);
    return undefined;
  }
  if (name === "command") {
    const { next, found } = options(words, start, plain);
    return found.some((option) => option.flag === "v" || option.flag === "V") ? undefined : next;
  }
  if (name === "env") {
    const { next, found } = options(words, start, env);
    const split = found.find((option) => option.flag === "S" || option.flag === "split-string");
    if (split?.value !== undefined) launched.push(...walk(split.value, scope.dir, scope));
    return next;
  }
  const spec = wrappers[name];
  if (!spec) return undefined;
  return options(words, start, spec).next + (spec.positional ?? 0);
}

function operands(words: string[], start: number) {
  const found: string[] = [];
  for (let at = start; at < words.length; at++) {
    if (marker.test(words[at]) || words[at].startsWith("<<<")) {
      if (words[at] === "<<<") at++;
      continue;
    }
    if (redirect.test(words[at])) {
      if (bareRedirect.test(words[at])) at++;
      continue;
    }
    found.push(words[at]);
  }
  return found;
}

function stdin(words: string[], start: number, scope: Scope, launched: Launch[]) {
  for (let at = start; at < words.length; at++) {
    const word = words[at];
    const body = marker.exec(word)?.[1];
    const script =
      body !== undefined
        ? scope.bodies[Number(body)]
        : word === "<<<" || word === "0<<<"
          ? words[at + 1]
          : /^0?<<</u.test(word)
            ? word.replace(/^0?<<</u, "")
            : undefined;
    if (script !== undefined) {
      launched.push(...walk(script, scope.dir, scope));
      return;
    }
  }
  launched.push({ command: streamed, dir: scope.dir });
}

function options(words: string[], start: number, spec: Spec) {
  const found: Option[] = [];
  let index = start;
  while (index < words.length && words[index].startsWith("-")) {
    const position = index;
    const word = words[index++];
    if (word === "--") break;
    if (word.startsWith("--")) {
      const [flag, ...rest] = word.slice(2).split("=");
      const value = rest.length ? rest.join("=") : spec.long.includes(flag) ? words[index++] : undefined;
      found.push({ flag, value, at: index > position + 1 ? index - 1 : position });
      continue;
    }
    for (let at = 1; at < word.length; at++) {
      const flag = word[at];
      if (!spec.short.includes(flag)) {
        found.push({ flag, at: position });
        continue;
      }
      const value = word.slice(at + 1) || words[index++];
      found.push({ flag, value, at: index > position + 1 ? index - 1 : position });
      break;
    }
  }
  return { next: index, found };
}

class Lexer {
  readonly segments: string[][] = [[]];
  readonly redirects: { segment: number; word: number }[] = [];
  readonly dynamic: Set<number>[] = [new Set()];
  readonly temporary: Map<number, string>[] = [new Map()];
  readonly subs: string[] = [];
  readonly sums: string[] = [];
  hidden = false;
  private bound = 0;
  private word = "";
  private open = false;
  private at = 0;
  readonly ops: string[] = [];
  readonly levels: number[] = [0];
  private readonly vars: Map<string, string>;
  private readonly resolve: boolean;
  private readonly paths: boolean;
  private joined = false;
  private blocks = 0;
  private parens = 0;
  private uncertain = false;
  private referenced: string | undefined;
  private created: string | undefined;
  private readonly creations: Map<number, string>[] = [new Map()];
  private readonly dirs = new Map<string, string>();

  constructor(
    private readonly source: string,
    { resolve, known, paths }: Context,
    private readonly math = false,
  ) {
    this.resolve = resolve;
    this.paths = paths !== undefined;
    this.vars = new Map(Object.entries(known));
  }

  run() {
    while (this.at < this.source.length) {
      if (!this.math) this.step();
      else if (this.source[this.at] === "\\") this.escape();
      else if (!this.substitution()) this.at++;
    }
    this.flush();
    return this;
  }

  literal(name: string) {
    return (
      this.segments.length === 1 &&
      this.segments[0][0] === name &&
      !this.subs.length &&
      !this.sums.length &&
      !this.hidden &&
      !this.dynamic[0].size
    );
  }

  private split(char: string) {
    this.flush();
    const pair = (char === "&" || char === "|") && this.source[this.at] === char;
    if (pair) this.at++;
    this.bind(!this.joined && char !== "(" && (pair || char === "\n" || char === ";"));
    this.ops.push(pair ? char + char : char);
    if (char === "(") this.parens++;
    if (char === ")") this.parens = Math.max(0, this.parens - 1);
    this.levels.push(this.blocks + this.parens);
    this.segments.push([]);
    this.dynamic.push(new Set());
    this.temporary.push(new Map());
    this.creations.push(new Map());
    this.joined = char !== "\n" && char !== ";" && !(char === "&" && !pair);
  }

  private bind(definite: boolean) {
    const words = this.segments[this.segments.length - 1];
    const [first = ""] = words;
    if (openers.has(first)) this.blocks++;
    if (closers.has(first)) this.blocks = Math.max(0, this.blocks - 1);
    const pure = words.length > 0 && words.every((word) => assignment.test(word));
    for (const word of words) {
      const name = /^([A-Za-z_]\w*)\+?=/u.exec(word)?.[1] ?? word;
      this.vars.delete(name);
      this.dirs.delete(name);
    }
    if (!pure || !definite || this.blocks > 0 || this.parens > 0) return;
    for (const [index, word] of words.entries()) {
      const at = word.indexOf("=");
      const name = word.slice(0, at);
      this.vars.delete(name);
      this.dirs.delete(name);
      const value = word.slice(at + 1);
      if (literal.test(value)) this.vars.set(name, value);
      const created = this.creations[this.segments.length - 1].get(index);
      if (created !== undefined) this.dirs.set(name, created);
    }
  }

  private variable(quoted: boolean) {
    if (this.source[this.at] !== "$") return false;
    reference.lastIndex = this.at;
    const match = reference.exec(this.source);
    const name = match?.[1] ?? match?.[2];
    if (name === undefined || !match) return false;
    const value = !this.resolve || specials.test(name) ? undefined : this.vars.get(name);
    const whole = this.word === "";
    this.take(value ?? match[0], value === undefined);
    if (quoted && whole && value === undefined) this.referenced = this.dirs.get(name);
    this.at = reference.lastIndex;
    return true;
  }

  private process() {
    if (this.open || !/[<>=]/u.test(this.source[this.at]) || this.source[this.at + 1] !== "(") return false;
    const end = closing(this.source, this.at + 2);
    this.subs.push(this.source.slice(this.at + 2, end));
    this.take("/dev/fd/63");
    this.at = end + 1;
    return true;
  }

  private step() {
    const char = this.source[this.at];
    if (char === "\\") return this.escape();
    if (char === "'") {
      const end = until(this.source.indexOf("'", this.at + 1), this.source);
      this.take(this.source.slice(this.at + 1, end));
      this.at = end + 1;
      return;
    }
    if (char === '"') return this.quoted();
    if (this.source.startsWith("$'", this.at)) return this.ansi();
    if (this.source.startsWith('$"', this.at)) {
      this.at++;
      return this.quoted();
    }
    if (this.substitution()) return;
    if (this.comment()) return;
    if (this.arithmetic(this.at)) return this.split(";");
    if (this.paths && (char === "<" || char === ">") && this.redirect()) return;
    this.at++;
    if (char === "&" && (/[<>]/u.test(this.source[this.at - 2] ?? "") || this.source[this.at] === ">")) {
      return this.take(char);
    }
    if (/[\n;&|()]/u.test(char)) return this.split(char);
    if (/\s/u.test(char)) return this.flush();
    this.take(char, char === "$");
  }

  private escape() {
    const next = this.source[this.at + 1] ?? "";
    this.at += 2;
    if (next !== "\n") this.take(next);
  }

  private quoted() {
    this.open = true;
    this.at++;
    while (this.at < this.source.length && this.source[this.at] !== '"') {
      if (this.source[this.at] === "\\") this.escape();
      else if (!this.substitution(true)) {
        const char = this.source[this.at++];
        this.take(char, char === "$");
      }
    }
    this.at++;
  }

  private ansi() {
    let end = this.at + 2;
    while (end < this.source.length && this.source[end] !== "'") end += this.source[end] === "\\" ? 2 : 1;
    this.take(decode(this.source.slice(this.at + 2, end)));
    this.at = end + 1;
  }

  private substitution(quoted = false) {
    if (this.variable(quoted) || this.process()) return true;
    const start = this.at;
    let end: number;
    if (this.source[start] === "$" && this.arithmetic(start + 1)) {
      this.take("$()", true);
      return true;
    }
    if (this.source.startsWith("$(", start)) {
      end = closing(this.source, start + 2);
      this.subs.push(this.source.slice(start + 2, end));
    } else if (this.source[start] === "`") {
      end = tick(this.source, start + 1);
      this.subs.push(this.source.slice(start + 1, end));
    } else {
      return false;
    }
    const created =
      this.resolve && /^[A-Za-z_]\w*=$/u.test(this.word)
        ? temporary(this.source.slice(start + (this.source[start] === "`" ? 1 : 2), end))
        : undefined;
    this.take("$()", true);
    this.created = created;
    this.at = end + 1;
    return true;
  }

  private arithmetic(open: number) {
    if (open < this.bound || !this.source.startsWith("((", open)) return false;
    const end = closing(this.source, open + 2) + 1;
    const body = this.source.slice(open + 2, end - 1);
    if (this.source[end] !== ")") {
      this.bound = end;
      this.hidden ||= /['#\\]/u.test(this.source.slice(open, end));
      return false;
    }
    this.sums.push(body);
    this.at = end + 1;
    return true;
  }

  private comment() {
    if (this.open || this.source[this.at] !== "#") return false;
    this.at = until(this.source.indexOf("\n", this.at), this.source);
    return true;
  }

  private redirect() {
    const op = /^[<>]+[&|]?/u.exec(this.source.slice(this.at))?.[0];
    if (op === undefined) return false;
    if (!op.startsWith("<<")) {
      if (!/^\d*$/u.test(this.word) && this.word !== "&") this.flush();
      const segment = this.segments.length - 1;
      this.redirects.push({ segment, word: this.segments[segment].length });
    }
    this.take(op);
    this.at += op.length;
    if (!op.startsWith("<<")) this.flush();
    return true;
  }

  private take(text: string, dynamic = false) {
    this.referenced = undefined;
    this.created = undefined;
    this.uncertain ||= dynamic;
    this.word += text;
    this.open = true;
  }

  private flush() {
    if (this.open) {
      const index = this.segments.length - 1;
      const at = this.segments[index].length;
      if (this.uncertain) this.dynamic[index].add(at);
      if (this.referenced !== undefined) this.temporary[index].set(at, this.referenced);
      if (this.created !== undefined) this.creations[index].set(at, this.created);
      this.segments[index].push(this.word);
    }
    this.word = "";
    this.open = false;
    this.uncertain = false;
    this.referenced = undefined;
    this.created = undefined;
  }
}

function temporary(source: string) {
  const lexer = new Lexer(source, { depth: 0, resolve: false, known: {}, handoffs: false }).run();
  if (!lexer.literal("mktemp")) return undefined;
  const [, ...args] = lexer.segments[0];
  const { next, found } = options(args, 0, { short: "p", long: ["suffix"] });
  if (args.length - next > 1 || !found.some(({ flag }) => flag === "d" || flag === "directory")) return undefined;
  if (
    found.some(
      ({ flag, value }) =>
        !["d", "directory", "q", "quiet", "p", "tmpdir", "suffix"].includes(flag) ||
        (["p", "tmpdir", "suffix"].includes(flag) && value === undefined) ||
        (flag === "suffix" && value?.includes("/")),
    )
  )
    return undefined;
  const base = found.findLast(({ flag }) => flag === "p" || flag === "tmpdir")?.value;
  const suffix = found.findLast(({ flag }) => flag === "suffix")?.value ?? "";
  if (/[\s$`*?[{]|\.\./u.test(args.join(""))) return undefined;
  const template = args[next] ?? "tmp.XXXXXXXXXX";
  if (!path.isAbsolute(base ?? "/") || !/X{3}[^/]*$/u.test(template)) return undefined;
  const target = base === undefined ? template : path.resolve(base, template);
  if (!within(path.dirname(target), temp)) return undefined;
  return target + suffix;
}

const escapes: Record<string, string> = {
  a: "\x07",
  b: "\b",
  e: "\x1b",
  E: "\x1b",
  f: "\f",
  n: "\n",
  r: "\r",
  t: "\t",
  v: "\v",
};
const ansiEscape = /\\(?:([0-7]{1,3})|x([\da-fA-F]{1,2})|u([\da-fA-F]{1,4})|U([\da-fA-F]{1,8})|c(.)|([\s\S]))/gu;

function decode(body: string) {
  return body.replace(ansiEscape, (...parts: string[]) => {
    const [, octal, hex, short, long, control, other] = parts;
    if (octal) return String.fromCharCode(Number.parseInt(octal, 8) & 0xff);
    if (hex) return String.fromCharCode(Number.parseInt(hex, 16));
    const point = short ?? long;
    if (point) return String.fromCodePoint(Math.min(Number.parseInt(point, 16), 0x10ffff));
    if (control) return String.fromCharCode(control.charCodeAt(0) & 0x1f);
    if (other in escapes) return escapes[other];
    return `\\'"?`.includes(other) ? other : `\\${other}`;
  });
}

function closing(source: string, start: number) {
  let depth = 1;
  for (let at = start; at < source.length; at++) {
    const char = source[at];
    if (char === "\\") at++;
    else if (char === "'") at = until(source.indexOf("'", at + 1), source);
    else if (char === '"') at = quoteEnd(source, at + 1);
    else if (char === "`") at = tick(source, at + 1);
    else if (char === "(") depth++;
    else if (char === ")" && --depth === 0) return at;
  }
  return source.length;
}

function quoteEnd(source: string, start: number) {
  for (let at = start; at < source.length; at++) {
    if (source[at] === "\\") at++;
    else if (source[at] === '"') return at;
  }
  return source.length;
}

function tick(source: string, start: number) {
  for (let at = start; at < source.length; at++) {
    if (source[at] === "\\") at++;
    else if (source[at] === "`") return at;
  }
  return source.length;
}

function until(index: number, source: string) {
  return index < 0 ? source.length : index;
}
