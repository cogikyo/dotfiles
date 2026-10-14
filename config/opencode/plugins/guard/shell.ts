import path from "node:path";

export type Command = readonly [name: string, ...args: string[]];

type Spec = { short: string; long: string[]; positional?: number };
type Option = { flag: string; value?: string };

const maxDepth = 8;
const assignment = /^[A-Za-z_]\w*=/u;
export const redirect = /^\d*(?:[<>]|&>)/u;
export const bareRedirect = /^\d*(?:>>?|<<?|&>>?|>&|<&)$/u;
const heredoc =
  /(<<-?\s*)(['"]?)((?<=')[^'\n]+|(?<=")[^"\n]+|(?<!['"])[^\s;&|()<>'"\\]+)\2([^\n]*)\n([\s\S]*?)\n\s*\3[ \t]*(?=\n|$)/gu;
const keywords = new Set(["!", "{", "}", "if", "then", "else", "elif", "do", "while", "until"]);
const shells = new Set(["bash", "dash", "sh", "zsh"]);
const plain: Spec = { short: "", long: [] };

const wrappers: Record<string, Spec> = {
  builtin: plain,
  nohup: plain,
  setsid: plain,
  exec: { short: "a", long: [] },
  nice: { short: "n", long: ["adjustment"] },
  stdbuf: { short: "ioe", long: ["input", "output", "error"] },
  time: { short: "fo", long: ["format", "output"] },
  timeout: { short: "sk", long: ["signal", "kill-after"], positional: 1 },
  sudo: {
    short: "ugCDhprtUR",
    long: ["user", "group", "close-from", "chdir", "host", "prompt", "role", "type", "other-user", "chroot"],
  },
  xargs: {
    short: "adEILnPs",
    long: ["arg-file", "delimiter", "max-args", "max-procs", "max-chars", "process-slot-var"],
  },
};

export function launches(source: string, depth = 0): Command[] {
  if (depth > maxDepth) return [];
  const { segments, subs } = new Lexer(unheredoc(source)).run();
  return [...subs.flatMap((sub) => launches(sub, depth + 1)), ...segments.flatMap((words) => peel(words, depth + 1))];
}

function unheredoc(source: string) {
  return source.replace(heredoc, (...parts: string[]) => {
    const [, op, quote, tag, rest, body] = parts;
    return `${op}${quote}${tag}${quote}${rest} ${quote ? "" : `"${body.replaceAll('"', '\\"')}"`}`;
  });
}

function peel(words: string[], depth: number) {
  const commands: Command[] = [];
  let index = 0;
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
    commands.push([name, ...words.slice(index + 1)]);
    const next = unwrap(name, words, index + 1, depth, commands);
    if (next === undefined) break;
    index = next;
  }
  return commands;
}

function unwrap(name: string, words: string[], start: number, depth: number, commands: Command[]) {
  if (name === "eval") {
    commands.push(...launches(words.slice(start).join(" "), depth));
    return undefined;
  }
  if (shells.has(name)) {
    const { next, found } = options(words, start, { short: "oO", long: [] });
    const script = found.some((option) => option.flag === "c") ? words[next] : undefined;
    if (script !== undefined) commands.push(...launches(script, depth));
    return undefined;
  }
  if (name === "command") {
    const { next, found } = options(words, start, plain);
    return found.some((option) => option.flag === "v" || option.flag === "V") ? undefined : next;
  }
  if (name === "env") {
    const { next, found } = options(words, start, { short: "uCS", long: ["unset", "chdir", "split-string"] });
    const split = found.find((option) => option.flag === "S" || option.flag === "split-string");
    if (split?.value !== undefined) commands.push(...launches(split.value, depth));
    return next;
  }
  const spec = wrappers[name];
  if (!spec) return undefined;
  return options(words, start, spec).next + (spec.positional ?? 0);
}

function options(words: string[], start: number, spec: Spec) {
  const found: Option[] = [];
  let index = start;
  while (index < words.length && words[index].startsWith("-")) {
    const word = words[index++];
    if (word === "--") break;
    if (word.startsWith("--")) {
      const [flag, ...rest] = word.slice(2).split("=");
      const value = rest.length ? rest.join("=") : spec.long.includes(flag) ? words[index++] : undefined;
      found.push({ flag, value });
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
  readonly subs: string[] = [];
  private word = "";
  private open = false;
  private at = 0;

  constructor(private readonly source: string) {}

  run() {
    while (this.at < this.source.length) this.step();
    this.flush();
    return this;
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
    this.at++;
    if (char === "&" && /[<>]/u.test(this.source[this.at - 2] ?? "")) return this.take(char);
    if (char === "&" && this.source[this.at] === ">") return this.take(char);
    if (/[\n;&|()]/u.test(char)) {
      this.flush();
      this.segments.push([]);
      return;
    }
    if (/\s/u.test(char)) return this.flush();
    this.take(char);
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
      else if (!this.substitution()) this.take(this.source[this.at++]);
    }
    this.at++;
  }

  private ansi() {
    let end = this.at + 2;
    while (end < this.source.length && this.source[end] !== "'") end += this.source[end] === "\\" ? 2 : 1;
    this.take(decode(this.source.slice(this.at + 2, end)));
    this.at = end + 1;
  }

  private substitution() {
    const start = this.at;
    let end: number;
    if (this.source.startsWith("$(", start)) {
      end = closing(this.source, start + 2);
      this.subs.push(this.source.slice(start + 2, end));
    } else if (this.source[start] === "`") {
      end = tick(this.source, start + 1);
      this.subs.push(this.source.slice(start + 1, end));
    } else {
      return false;
    }
    this.take("$()");
    this.at = end + 1;
    return true;
  }

  private comment() {
    if (this.open || this.source[this.at] !== "#") return false;
    this.at = until(this.source.indexOf("\n", this.at), this.source);
    return true;
  }

  private take(text: string) {
    this.word += text;
    this.open = true;
  }

  private flush() {
    if (this.open) this.segments[this.segments.length - 1].push(this.word);
    this.word = "";
    this.open = false;
  }
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
