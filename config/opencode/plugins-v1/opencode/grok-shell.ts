import path from "node:path";

type Spec = { short: string; long: string[]; positional?: number };
type Option = { flag: string; value?: string };
type Step = number | "grok" | "stop";

const maxDepth = 8;
const assignment = /^[A-Za-z_]\w*=/u;
const redirect = /^\d*(?:[<>]|&>)/u;
const bareRedirect = /^\d*(?:>>?|<<?|&>>?|>&|<&)$/u;
const heredoc = /(<<-?\s*)(['"]?)(\w+)\2([^\n]*)\n([\s\S]*?)\n\s*\3[ \t]*(?=\n|$)/gu;
const keywords = new Set(["!", "{", "}", "if", "then", "else", "elif", "do", "while", "until"]);
const shells = new Set(["bash", "dash", "sh", "zsh"]);
const plain: Spec = { short: "", long: [] };

const specs: Record<string, Spec> = {
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

export function invokesGrok(source: string, depth = 0): boolean {
  if (depth > maxDepth) return false;
  const { segments, subs } = new Lexer(unheredoc(source)).run();
  return subs.some((sub) => invokesGrok(sub, depth + 1)) || segments.some((words) => runs(words, depth + 1));
}

function unheredoc(source: string) {
  return source.replace(heredoc, (...parts: string[]) => {
    const [, op, quote, tag, rest, body] = parts;
    return `${op}${tag}${rest} ${quote ? "" : `"${body.replaceAll('"', '\\"')}"`}`;
  });
}

function runs(words: string[], depth: number) {
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
    const step = launch(path.basename(word), words, index + 1, depth);
    if (step === "grok") return true;
    if (step === "stop") return false;
    index = step;
  }
  return false;
}

function launch(name: string, words: string[], start: number, depth: number): Step {
  if (name === "grok") return "grok";
  if (name === "eval") return invokesGrok(words.slice(start).join(" "), depth) ? "grok" : "stop";
  if (shells.has(name)) return shell(words, start, depth);
  if (name === "command") {
    const { next, found } = options(words, start, plain);
    return found.some((option) => option.flag === "v" || option.flag === "V") ? "stop" : next;
  }
  if (name === "env") return env(words, start, depth);
  const spec = specs[name];
  if (!spec) return "stop";
  return options(words, start, spec).next + (spec.positional ?? 0);
}

function shell(words: string[], start: number, depth: number): Step {
  const { next, found } = options(words, start, { short: "oO", long: [] });
  const script = found.some((option) => option.flag === "c") ? words[next] : undefined;
  return script !== undefined && invokesGrok(script, depth) ? "grok" : "stop";
}

function env(words: string[], start: number, depth: number): Step {
  const { next, found } = options(words, start, { short: "uCS", long: ["unset", "chdir", "split-string"] });
  const split = found.find((option) => option.flag === "S" || option.flag === "split-string");
  return split?.value !== undefined && invokesGrok(split.value, depth) ? "grok" : next;
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
    if (this.substitution()) return;
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
    this.take(this.source[this.at + 1] ?? "");
    this.at += 2;
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
