type Token = { kind: "open" | "close" | "string" | "word"; start: number; end: number; text: string };

export type Report = { dropped: Record<string, number> };

const supported = new Set([
  "eq?",
  "not-eq?",
  "any-eq?",
  "any-not-eq?",
  "match?",
  "not-match?",
  "any-match?",
  "any-not-match?",
  "any-of?",
  "not-any-of?",
  "is?",
  "is-not?",
  "set!",
]);

const lua = new Map([
  ["lua-match?", "match?"],
  ["not-lua-match?", "not-match?"],
  ["any-lua-match?", "any-match?"],
]);

export function convert(source: string): { text: string; report: Report } {
  const tokens = tokenize(source);
  const report: Report = { dropped: {} };
  const edits: { start: number; end: number; text: string }[] = [];
  for (const pattern of patterns(tokens)) {
    const replaced: typeof edits = [];
    let drop: string | undefined;
    for (const predicate of predicates(tokens, pattern)) {
      const [name, ...args] = predicate.args;
      const operator = name.text.slice(1);
      if (operator === "set!" && args[0]?.text.startsWith("@")) {
        replaced.push({ start: predicate.start, end: predicate.end, text: "" });
        continue;
      }
      if (supported.has(operator)) continue;
      const rewritten = rewrite(operator, args);
      if (!rewritten) {
        // OpenTUI ignores unknown predicates, so an unconverted constraint must drop the whole pattern.
        drop = operator;
        break;
      }
      replaced.push({ start: predicate.start, end: predicate.end, text: rewritten });
    }
    if (drop) {
      report.dropped[drop] = (report.dropped[drop] ?? 0) + 1;
      edits.push({ start: tokens[pattern.first].start, end: tokens[pattern.last].end, text: "" });
      continue;
    }
    edits.push(...replaced);
  }
  let text = source;
  for (const edit of edits.toSorted((a, b) => b.start - a.start)) {
    text = text.slice(0, edit.start) + edit.text + text.slice(edit.end);
  }
  return { text, report };
}

function rewrite(operator: string, args: Token[]): string | undefined {
  const [capture, ...values] = args;
  if (!capture?.text.startsWith("@") || values.some((value) => value.kind !== "string")) return undefined;
  const strings = values.map((value) => decode(value.text));
  if (operator === "contains?") {
    return `(#match? ${capture.text} ${encode(strings.map((value) => value.replace(/[.*+?^${}()|[\]\\/]/g, "\\$&")).join("|"))})`;
  }
  const target = lua.get(operator);
  if (!target || strings.length !== 1) return undefined;
  const regex = luaPattern(strings[0]);
  return regex === undefined ? undefined : `(#${target} ${capture.text} ${encode(regex)})`;
}

const classes = new Map([
  ["a", "A-Za-z"],
  ["c", "\\x00-\\x1f\\x7f"],
  ["d", "0-9"],
  ["g", "!-~"],
  ["l", "a-z"],
  ["p", "!-\\/:-@\\[-`{-~"],
  ["s", " \\t\\n\\r\\f\\v"],
  ["u", "A-Z"],
  ["w", "A-Za-z0-9"],
  ["x", "0-9A-Fa-f"],
]);

function luaPattern(pattern: string): string | undefined {
  let out = "";
  for (let i = 0; i < pattern.length; i++) {
    const char = pattern[i];
    if (char === "^" && i === 0) out += "^";
    else if (char === "$" && i === pattern.length - 1) out += "$";
    else if (char === "(" && pattern[i + 1] !== ")") out += "(?:";
    else if (char === ")") out += ")"; // Lua treats a quantifier after ")" as literal.
    else {
      const item = luaItem(pattern, i);
      if (!item) return undefined;
      const quantifier = pattern[item.end + 1];
      out += item.text;
      i = item.end;
      if (!quantifier || !"*+?-".includes(quantifier)) continue;
      out += quantifier === "-" ? "*?" : quantifier;
      i++;
    }
  }
  return out;
}

function luaItem(pattern: string, i: number): { text: string; end: number } | undefined {
  const char = pattern[i];
  if (char === "(") return undefined;
  if (char === ".") return { text: "[\\s\\S]", end: i };
  if (char === "[") {
    const set = luaSet(pattern, i);
    return set;
  }
  if (char !== "%") return { text: escape(char), end: i };
  const next = pattern[i + 1];
  if (next === undefined || /[0-9bf]/.test(next)) return undefined;
  const body = classes.get(next.toLowerCase());
  if (body) return { text: next === next.toLowerCase() ? `[${body}]` : `[^${body}]`, end: i + 1 };
  if (/[A-Za-z]/.test(next)) return undefined;
  return { text: escape(next), end: i + 1 };
}

function luaSet(pattern: string, open: number): { text: string; end: number } | undefined {
  let i = open + 1;
  let out = "[";
  if (pattern[i] === "^") {
    out += "^";
    i++;
  }
  for (const first = i; i < pattern.length; i++) {
    const char = pattern[i];
    if (char === "]" && i !== first) return { text: `${out}]`, end: i };
    if (char !== "%") {
      out += char === "\\" || char === "[" || char === "]" ? `\\${char}` : char;
      continue;
    }
    const next = pattern[++i];
    if (next === undefined) return undefined;
    const body = classes.get(next);
    if (!body && /[A-Za-z0-9]/.test(next)) return undefined;
    out += body ?? `\\${next}`;
  }
  return undefined;
}

function escape(char: string): string {
  return /[\\^$.*+?()[\]{}|/-]/.test(char) ? `\\${char}` : char;
}

function decode(literal: string): string {
  const escapes = new Map([
    ["n", "\n"],
    ["t", "\t"],
    ["r", "\r"],
    ["0", "\0"],
  ]);
  return literal.slice(1, -1).replace(/\\(.)/g, (_, char: string) => escapes.get(char) ?? char);
}

function encode(value: string): string {
  return `"${value.replace(/\\/g, "\\\\").replace(/"/g, '\\"').replace(/\n/g, "\\n").replace(/\t/g, "\\t").replace(/\r/g, "\\r")}"`;
}

const brackets = new Map<string, Token["kind"]>([
  ["(", "open"],
  ["[", "open"],
  [")", "close"],
  ["]", "close"],
]);

function tokenize(source: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;
  while (i < source.length) {
    const char = source[i];
    const bracket = brackets.get(char);
    const start = i;
    if (/\s/.test(char)) {
      i++;
      continue;
    }
    if (char === ";") {
      while (i < source.length && source[i] !== "\n") i++;
      continue;
    }
    if (bracket) {
      i++;
      tokens.push({ kind: bracket, start, end: i, text: char });
      continue;
    }
    if (char === '"') {
      i++;
      while (i < source.length && source[i] !== '"') i += source[i] === "\\" ? 2 : 1;
      i++;
      tokens.push({ kind: "string", start, end: i, text: source.slice(start, i) });
      continue;
    }
    while (i < source.length && !/[\s()[\]";]/.test(source[i])) i++;
    tokens.push({ kind: "word", start, end: i, text: source.slice(start, i) });
  }
  return tokens;
}

function patterns(tokens: Token[]): { first: number; last: number }[] {
  const found: { first: number; last: number }[] = [];
  let depth = 0;
  for (const [index, token] of tokens.entries()) {
    const attached = token.kind === "word" && (token.text.startsWith("@") || ["*", "+", "?"].includes(token.text));
    if (depth === 0 && !attached && token.kind !== "close") found.push({ first: index, last: index });
    if (token.kind === "open") depth++;
    if (token.kind === "close") depth--;
    const current = found.at(-1);
    if (current) current.last = index;
  }
  return found;
}

function predicates(tokens: Token[], pattern: { first: number; last: number }) {
  const found: { start: number; end: number; args: Token[] }[] = [];
  for (let i = pattern.first; i <= pattern.last; i++) {
    if (tokens[i].kind !== "open" || !tokens[i + 1]?.text.startsWith("#")) continue;
    let end = i + 1;
    while (tokens[end].kind !== "close") end++;
    found.push({ start: tokens[i].start, end: tokens[end].end, args: tokens.slice(i + 1, end) });
  }
  return found;
}
