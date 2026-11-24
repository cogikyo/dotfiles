import path from "node:path";
import { expand, home, temp, within } from "./paths.ts";

export type Command = readonly [name: string, ...args: string[]];
export type Launch = { command: Command; dir: string | undefined; dirs?: ReadonlySet<string>; guess?: Guess };
type Guess = { wrapper: string; flag: string };
type Start = { at: number; guess?: Guess };

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

export type Spec = { short: string; long: string[] };
type Option = { flag: string; value?: string; at: number; long?: true };

export const maxDepth = 8;
export const opaque: Command = [""];
export const streamed: Command = [""];
export const redirection = ">";
export const unseen: Command = [""];
const assignment = /^[A-Za-z_]\w*=/u;
const redirect = /^(?:\d+|\{[A-Za-z_]\w*\})?(?:[<>]|&>)/u;
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
const outputs = /^(?:\d+|\{[A-Za-z_]\w*\})?(?:>&|&>>?|>>?\|?)/u;
const tracers = new Set(["strace", "ltrace"]);
const glued = /(?<=[^\s<>&|;()])(?<!(?:^|[\s<>&|;()])(?:\d+|\{[A-Za-z_]\w*\}))(?=&>|[<>])/gu;
const operator = /^(?:\d+|\{[A-Za-z_]\w*\})?(?:&>>?|>>?\|?|<<<|<<-?|<>|>&|<&|<)$/u;
const takes = /^--[^=]+$|^-[^-]*[A-Za-z]$/u;
const chdirs: Record<string, string[]> = {
  env: ["C", "chdir"],
  sudo: ["D", "chdir"],
  "systemd-run": ["working-directory"],
};

const wrappers: Record<string, number> = {
  builtin: 0,
  nohup: 0,
  setsid: 0,
  cage: 2,
  exec: 0,
  nice: 0,
  stdbuf: 0,
  time: 0,
  timeout: 1,
  sudo: 0,
  "systemd-run": 0,
  xargs: 0,
  env: 0,
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
  const args = argv(rest);
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
  if (!Object.hasOwn(wrappers, name)) return [];
  return scan(args, 0, reach(args, 0, wrappers[name]).end);
}

export function wrapped([name, ...args]: Command) {
  if (!Object.hasOwn(wrappers, name)) return [];
  return reach(args, 0, wrappers[name]).heads.map((at) => args[at]);
}

export function remote([name, ...args]: Command) {
  if (name !== "ssh") return [];
  return reach(args, 0, 1).heads.map((at) => args.slice(at).join(" "));
}

export function respaced(source: string, dir?: string, known: Record<string, string> = {}, handoffs = true) {
  const spaced = source.replace(glued, " ");
  return spaced === source ? [] : launches(spaced, dir, known, handoffs);
}

export function argv(words: readonly string[]) {
  const kept: string[] = [];
  for (let at = 0; at < words.length; at++) {
    if (operator.test(words[at])) at++;
    else if (!redirect.test(words[at])) kept.push(words[at]);
  }
  return kept;
}

function reach(words: readonly string[], start: number, positional = 0) {
  const heads = new Set<number>();
  const seen = new Set<number>();
  const queue = [start];
  let end = start;
  for (const at of queue) {
    if (seen.has(at) || at >= words.length) continue;
    seen.add(at);
    end = Math.max(end, at + 1);
    const word = words[at];
    if (!word.startsWith("-")) {
      heads.add(at + positional);
      continue;
    }
    if (word === "--") heads.add(at + 1 + positional);
    queue.push(at + 1);
    if (takes.test(word)) queue.push(at + 2);
  }
  return { heads: [...heads].filter((at) => at < words.length), end };
}

function starts(wrapper: string, words: readonly string[], start: number, heads: number[]): Start[] {
  if (heads.length < 2) return heads.map((at) => ({ at }));
  return heads.map((at) => ({ at, guess: { wrapper, flag: culprit(words, start, at) } }));
}

function culprit(words: readonly string[], start: number, head: number) {
  for (let at = head - 2; at >= start; at--)
    if (words[at].startsWith("-") && !words[at + 1].startsWith("-")) return words[at];
  return words[head - 1];
}

function scan(words: readonly string[], start: number, end = words.length): Option[] {
  return words.slice(start, end).flatMap((word, offset): Option[] => {
    const at = start + offset;
    if (!/^-[^-]|^--./u.test(word)) return [];
    if (word.startsWith("--")) {
      const [flag, ...rest] = word.slice(2).split("=");
      return [
        rest.length
          ? { flag, value: rest.join("="), at, long: true }
          : { flag, value: words[at + 1], at: at + 1, long: true },
      ];
    }
    const found: Option[] = [];
    for (let letter = 1; letter < word.length; letter++) {
      const attached = word.slice(letter + 1);
      found.push({
        flag: word[letter],
        ...(attached ? { value: attached, at } : { value: words[at + 1], at: at + 1 }),
      });
    }
    return found;
  });
}

export function named({ flag, long }: Option, names: readonly string[]) {
  return long ? flag !== "" && names.some((name) => name.length > 1 && name.startsWith(flag)) : names.includes(flag);
}

function unheredoc(source: string, bodies: string[], unquoted: string[]) {
  return source.replace(heredoc, (...parts: string[]) => {
    const [, op, quote, , rest, body] = parts;
    bodies.push(body);
    if (!quote) unquoted.push(body);
    return `${op.trimEnd()}\uE000${bodies.length - 1}${rest}`;
  });
}

function peel(words: string[], scope: Scope, from: Start[] = [{ at: 0 }]) {
  const launched: Launch[] = [];
  const leaves: { index: number; dir: string | undefined }[] = [];
  const queue = from.map(({ at, guess }) => ({ index: skip(words, at), dir: scope.dir, guess }));
  const covered = new Set(queue.map(({ index }) => index));
  const seen = new Set(queue.map(({ index }) => `${index}:${scope.dir}`));
  let moved = false;
  let dir = scope.dir;
  for (const head of queue) {
    const { index, guess } = head;
    dir = head.dir;
    if (index >= words.length) continue;
    const name = path.basename(words[index]);
    const command: Command = [
      name,
      ...words.slice(index + 1).map((arg, at) => (name === "rm" ? (scope.temporary.get(index + at + 1) ?? arg) : arg)),
    ];
    launched.push({ command, dir, ...(scope.paths ? { dirs: scope.paths.dirs } : {}), ...(guess ? { guess } : {}) });
    if (scope.paths) {
      const next = literalDestination(command, dir);
      const base = literalDestination(command, scope.paths.base);
      if (next !== undefined) scope.paths.dirs.add(next);
      if (base !== undefined) scope.paths.dirs.add(base);
      if (next !== undefined) dir = next;
    }
    moved ||= resets.has(name) || (name === "emulate" && emulation(words, index + 1).script);
    if (flags(command).some((option) => named(option, chdirs[name] ?? []))) dir = undefined;
    const next = unwrap(words, index, { ...scope, dir }, launched);
    if (next === undefined) {
      leaves.push({ index, dir });
      continue;
    }
    for (const start of next) {
      const at = skip(words, start.at);
      if (seen.has(`${at}:${dir}`)) continue;
      seen.add(`${at}:${dir}`);
      covered.add(at);
      queue.push({ index: at, dir, guess: start.guess });
    }
  }
  for (const leaf of leaves) launched.push(...nested(words, leaf.index, { ...scope, dir: leaf.dir }, covered));
  if (!scope.paths) launched.push(...writes(words.slice(Math.min(...from.map(({ at }) => at))), dir));
  return { launched, moved, dir };
}

function skip(words: readonly string[], index: number) {
  while (index < words.length) {
    const word = words[index];
    if (redirect.test(word)) index += operator.test(word) ? 2 : 1;
    else if (assignment.test(word) || keywords.has(word)) index++;
    else break;
  }
  return index;
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

function nested(words: string[], index: number, scope: Scope, covered = new Set<number>()): Launch[] {
  const name = path.basename(words[index]);
  const start = index + 1;
  const args = words.slice(start);
  if (name === "watch" || name === "flock") {
    if (!scope.handoffs) return [{ command: unseen, dir: scope.dir }];
    return name === "watch" ? watch(words, start, scope) : flock(words, start, scope);
  }
  if (strings.has(name) || evaluates(name, args)) return [{ command: unseen, dir: scope.dir }];
  if (name !== "find") {
    const shell = invokes(words, start, words.length, covered);
    const known = name === "unshare" || (name === "uv" && words[start] === "run");
    if (!scope.handoffs || !known) return shell ? [{ command: unseen, dir: scope.dir }] : [];
    return runner(words, index, scope, shell);
  }
  return args.flatMap((arg, at) => {
    if (!runners.has(arg)) return [];
    const end = args.findIndex((word, after) => after > at && (word === ";" || word === "+"));
    const inner = args[at + 1];
    const dir = arg === "-exec" || arg === "-ok" ? scope.dir : undefined;
    if (!shells.has(path.basename(inner ?? "")))
      return invokes(words, start + at + 1, end < 0 ? words.length : start + end) ? [{ command: unseen, dir }] : [];
    const { next, script } = interpreter(words, start + at + 2);
    if (script && args[end] === ";" && words[next]?.includes("{}")) return [{ command: unseen, dir }];
    const launched: Launch[] = [];
    unwrap(words.slice(0, end < 0 ? undefined : start + end), start + at + 1, { ...scope, dir }, launched);
    return launched;
  });
}

function runner(words: string[], index: number, scope: Scope, shell: boolean): Launch[] {
  const uv = path.basename(words[index]) === "uv";
  const from = index + (uv ? 2 : 1);
  const { heads, end } = reach(words, from);
  const found = scan(words, from, end);
  const has = (names: string[]) => found.some((option) => named(option, names));
  if (uv ? has(["m", "module", "s", "script", "gui-script"]) : has(["R", "root"]))
    return shell || !uv ? [{ command: unseen, dir: scope.dir }] : [];
  const moved = has(uv ? ["directory"] : ["w", "wd"]);
  const kept = starts(uv ? "uv run" : "unshare", words, from, heads);
  return peel(words, { ...scope, dir: moved ? undefined : scope.dir, literal: true }, kept).launched;
}

function watch(words: string[], start: number, scope: Scope): Launch[] {
  const blind = [{ command: unseen, dir: scope.dir }];
  const { heads, end } = reach(words, start);
  if (heads.length === 0) return blind;
  const execs = words
    .slice(start, end)
    .flatMap((word, offset) =>
      scan([word], 0).some((option) => named(option, ["x", "exec"])) ? [start + offset] : [],
    );
  const launched: Launch[] = [];
  const kept = starts("watch", words, start, heads);
  for (const head of kept) {
    if (execs.some((at) => at < head.at)) launched.push(...peel(words, { ...scope, literal: true }, [head]).launched);
    if ([...scope.dynamic].some((index) => index >= head.at)) {
      launched.push(...blind);
      continue;
    }
    const body = walk(words.slice(head.at).join(" "), scope.dir, scope);
    const name = path.basename(words[head.at]);
    for (const launch of body) if (head.guess && launch.command[0] === name) launch.guess = head.guess;
    launched.push(...body);
  }
  return launched;
}

function flock(words: string[], start: number, scope: Scope): Launch[] {
  const blind = [{ command: unseen, dir: scope.dir }];
  const stop = words.indexOf("--", start);
  if ([...scope.dynamic].some((index) => index >= start && (stop < 0 || index < stop))) return blind;
  const locks = reach(words, start, 1);
  const chains = locks.heads.map((lock) => reach(words, lock));
  const end = Math.max(locks.end, ...chains.map((chain) => chain.end));
  const scripts = scan(words, start, end).filter((option) => named(option, ["c", "command"]));
  if (scripts.some(({ value, at }) => value === undefined || scope.dynamic.has(at))) return blind;
  const heads = [...new Set(chains.flatMap((chain) => chain.heads))];
  return [
    ...scripts.flatMap(({ value = "" }) => walk(value, scope.dir, scope)),
    ...(heads.length ? peel(words, { ...scope, literal: true }, starts("flock", words, start, heads)).launched : []),
  ];
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

function invokes(words: string[], start: number, end: number, covered = new Set<number>()) {
  for (let at = start; at < end && !covered.has(at); at++)
    if (shells.has(path.basename(words[at])) && /^-[A-Za-z]*c/u.test(words[at + 1] ?? "")) return true;
  return false;
}

function emulation(words: string[], start: number) {
  const front = interpreter(words, start);
  return front.script ? front : interpreter(words, front.next + 1);
}

function unwrap(words: string[], index: number, scope: Scope, launched: Launch[]): Start[] | undefined {
  const name = path.basename(words[index]);
  const start = index + 1;
  if (name === "eval") {
    launched.push(...walk(words.slice(start).join(" "), scope.dir, scope));
    return [];
  }
  if (name === "emulate") {
    if ([...scope.dynamic].some((at) => at >= start)) {
      launched.push({ command: unseen, dir: scope.dir });
      return [];
    }
    const { next, script } = emulation(words, start);
    if (!script) return [];
    const body = words[next];
    launched.push(...(body === undefined ? [{ command: unseen, dir: scope.dir }] : walk(body, scope.dir, scope)));
    return [];
  }
  if (shells.has(name)) {
    const { next, script, stdin: reads, noexec, unknown } = interpreter(words, start);
    if (unknown) {
      launched.push({ command: unseen, dir: scope.dir });
      return [];
    }
    if (noexec) return [];
    if (script) {
      const body = words[next];
      launched.push(
        ...(body === undefined || (scope.literal && scope.dynamic.has(next))
          ? [{ command: unseen, dir: scope.dir }]
          : walk(body, scope.dir, scope)),
      );
      return [];
    }
    const file = argv(words.slice(next))[0];
    if (file === undefined || devices.test(file) || reads) {
      stdin(words, start, scope, launched);
    }
    return [];
  }
  if (name === "source" || name === ".") {
    if (devices.test(argv(words.slice(start))[0] ?? "")) stdin(words, start, scope, launched);
    return [];
  }
  if (name === "command") {
    const { next, found } = options(words, start, { short: "", long: [] });
    return found.some((option) => option.flag === "v" || option.flag === "V") ? undefined : [{ at: next }];
  }
  if (tracers.has(name)) {
    launched.push(
      ...values(scan(words, start), ["o", "output"], /^[|!]/u).flatMap((pipe) => walk(pipe.slice(1), scope.dir, scope)),
    );
    return undefined;
  }
  if (!Object.hasOwn(wrappers, name)) return undefined;
  const { heads, end } = reach(words, start, wrappers[name]);
  const found = scan(words, start, end);
  if (name === "env")
    launched.push(...values(found, ["S", "split-string"]).flatMap((split) => walk(split, scope.dir, scope)));
  if (name === "xargs" && substitutes(words, start, found)) launched.push({ command: unseen, dir: scope.dir });
  return starts(name, words, start, heads);
}

function substitutes(words: string[], start: number, found: Option[]) {
  const swaps = found
    .filter((option) => named(option, ["I", "i", "J", "replace"]))
    .flatMap(({ value }) => (value ? [value, "{}"] : ["{}"]));
  const shell = words.findIndex((word, at) => at >= start && shells.has(path.basename(word)));
  return shell >= 0 && words.slice(shell + 1).some((word) => swaps.some((swap) => word.includes(swap)));
}

function values(found: Option[], names: string[], shape = /^/u) {
  return found.flatMap((option) =>
    named(option, names) && option.value !== undefined && shape.test(option.value) ? [option.value] : [],
  );
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

export function options(words: readonly string[], start: number, spec: Spec) {
  const found: { flag: string; value?: string }[] = [];
  let index = start;
  while (index < words.length && words[index].startsWith("-")) {
    const word = words[index++];
    if (word === "--") break;
    if (word.startsWith("--")) {
      const [flag, ...rest] = word.slice(2).split("=");
      found.push({ flag, value: rest.length ? rest.join("=") : spec.long.includes(flag) ? words[index++] : undefined });
      continue;
    }
    for (let at = 1; at < word.length; at++) {
      const flag = word[at];
      if (!spec.short.includes(flag)) {
        found.push({ flag });
        continue;
      }
      found.push({ flag, value: word.slice(at + 1) || words[index++] });
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

export function quote(word: string) {
  return `'${word.replaceAll("'", `'\\''`)}'`;
}
