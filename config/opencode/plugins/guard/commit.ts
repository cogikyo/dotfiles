import { readFile, stat } from "node:fs/promises";
import path from "node:path";

const width = 72;
const maxProblems = 8;
const separator = "\u0000";
const opener = /(?<!<)<<(-?)\s*(['"]?)([\w.-]+)\2/gu;

export async function commitRejection(command: string, cwd: string) {
  if (!command.includes("commit")) return undefined;
  for (const { message, dir } of await messages(command, cwd)) {
    const shape = lint(message);
    const scope = await scopeProblem(message, dir);
    if (shape.length > 0 || scope) return rejection(shape, scope);
  }
  return undefined;
}

async function messages(command: string, cwd: string) {
  const { text, bodies } = heredocs(command);
  const words = shellWords(text);
  const found: { message: string; dir: string }[] = [];
  for (const [at, word] of words.entries()) {
    if (path.basename(word) !== "git") continue;
    const git = gitOptions(commandArgs(words, at), cwd);
    if (git === undefined) continue;
    const message = await commitMessage(words, at, git, bodies);
    if (message !== undefined) found.push({ message, dir: git.dir });
  }
  return found;
}

function shellWords(command: string) {
  const words: string[] = [];
  let word = "";
  let quote = "";
  let escaped = false;
  const flush = () => {
    if (word) words.push(word);
    word = "";
  };
  for (const char of command) {
    if (escaped) {
      word += char;
      escaped = false;
      continue;
    }
    if (char === "\\" && quote !== "'") {
      escaped = true;
      continue;
    }
    if (quote) {
      if (char === quote) quote = "";
      else word += char;
      continue;
    }
    if (char === "'" || char === '"') {
      quote = char;
      continue;
    }
    if (/[\n;&|()]/u.test(char)) {
      flush();
      if (words.at(-1) !== separator) words.push(separator);
      continue;
    }
    if (/\s/u.test(char)) {
      flush();
      continue;
    }
    word += char;
  }
  flush();
  return words;
}

function commandArgs(words: string[], index: number) {
  const end = words.indexOf(separator, index + 1);
  return words.slice(index + 1, end === -1 ? undefined : end);
}

function heredocs(command: string) {
  const kept: string[] = [];
  const bodies: string[] = [];
  const lines = command.split("\n");
  for (let index = 0; index < lines.length; index++) {
    const line = lines[index];
    kept.push(line);
    for (const match of line.matchAll(opener)) {
      const body: string[] = [];
      while (++index < lines.length) {
        const candidate = match[1] ? lines[index].replace(/^\t+/u, "") : lines[index];
        if (candidate === match[3]) break;
        body.push(lines[index]);
      }
      bodies.push(body.join("\n"));
    }
  }
  return { text: kept.join("\n"), bodies };
}

function heredocsBefore(words: string[], end: number) {
  return words.slice(0, end).reduce((count, word) => count + [...word.matchAll(opener)].length, 0);
}

function gitOptions(args: string[], cwd: string) {
  let dir = cwd;
  let index = 0;
  for (; index < args.length; index++) {
    const arg = args[index];
    if (arg === "-C") {
      dir = path.resolve(dir, args[++index] ?? "");
      continue;
    }
    if (["-c", "--git-dir", "--work-tree", "--namespace", "--config-env"].includes(arg)) {
      index++;
      continue;
    }
    if (!arg.startsWith("-")) break;
  }
  return args[index] === "commit" ? { cwd, dir, index } : undefined;
}

async function commitMessage(
  words: string[],
  at: number,
  git: { cwd: string; dir: string; index: number },
  bodies: string[],
) {
  const base = at + git.index + 2;
  const options = commandArgs(words, at).slice(git.index + 1);
  const paragraphs: string[] = [];
  let file: string | undefined;
  for (let i = 0; i < options.length; i++) {
    const option = options[i];
    const long = /^--(message|file)(?:=(.*))?$/su.exec(option);
    const short = /^-([aeinopqsvz]*)([mF])(.*)$/su.exec(option);
    if (!long && !short) continue;
    const kind = long ? long[1] : short?.[2] === "m" ? "message" : "file";
    const attached = long ? long[2] : short?.[3] || undefined;
    const position = attached === undefined ? ++i : i;
    const value = attached ?? options[i];
    if (kind === "file") {
      file = value;
      continue;
    }
    if (value === undefined) continue;
    if (/^\$\(cat\s+<</u.test(value)) {
      const body = bodies[heredocsBefore(words, base + position)];
      if (body === undefined) return undefined;
      paragraphs.push(body);
      continue;
    }
    if (value.includes("$") || value.includes("`")) return undefined;
    paragraphs.push(value);
  }
  if (paragraphs.length > 0) return paragraphs.join("\n\n");
  if (file === undefined) return undefined;
  if (file !== "-") return read(path.resolve(git.dir, file));

  const own = options.findIndex((option) => option.startsWith("<<") && !option.startsWith("<<<"));
  if (own !== -1) return bodies[heredocsBefore(words, base + own)];
  const redirect = options.findIndex((option) => option.startsWith("<"));
  if (redirect !== -1) {
    const target = options[redirect] === "<" ? options[redirect + 1] : options[redirect].slice(1);
    return target ? read(path.resolve(git.cwd, target)) : undefined;
  }
  const segment = words.lastIndexOf(separator, at - 2) + 1;
  const piped = heredocsBefore(words, at);
  return at > 0 && words[at - 1] === separator && piped > heredocsBefore(words, segment)
    ? bodies[piped - 1]
    : undefined;
}

async function read(file: string) {
  try {
    return await readFile(file, "utf8");
  } catch {
    return undefined;
  }
}

function normalize(message: string) {
  const lines: string[] = [];
  for (const line of message.split("\n").map((line) => line.trimEnd())) {
    if (line === "" && (lines.length === 0 || lines.at(-1) === "")) continue;
    lines.push(line);
  }
  while (lines.at(-1) === "") lines.pop();
  return lines;
}

function lint(message: string) {
  const lines = normalize(message);
  const problems: string[] = [];
  for (const [index, line] of lines.entries()) {
    const length = Array.from(line).length;
    if (length <= width) continue;
    const name = index === 0 ? "the subject" : `line ${index + 1} ${quote(line)}`;
    problems.push(`${name} is ${length} characters; the limit is ${width}`);
  }
  if (lines.length > 1 && lines[1] !== "") {
    problems.push(`line 2 ${quote(lines[1])} must be blank; the subject is one line`);
  }
  let depth = -1;
  for (let index = 2; index < lines.length; index++) {
    const line = lines[index];
    const number = index + 1;
    if (line === "") {
      problems.push(`line ${number} is blank; the body is one bullet list`);
      depth = -1;
      continue;
    }
    const bullet = /^( *)- \S/u.exec(line);
    if (!bullet) {
      problems.push(
        depth >= 0
          ? `line ${number} ${quote(line)} wraps the bullet on line ${number - 1}`
          : `line ${number} ${quote(line)} is not a bullet`,
      );
      continue;
    }
    const indent = bullet[1].length;
    if (indent % 2 !== 0 || indent / 2 > depth + 1) {
      problems.push(`line ${number} ${quote(line)} must nest by two spaces under the bullet above`);
      continue;
    }
    depth = indent / 2;
  }
  return problems;
}

function quote(line: string) {
  const text = line.trim();
  const chars = Array.from(text);
  return JSON.stringify(chars.length > 32 ? `${chars.slice(0, 32).join("")}…` : text);
}

async function scopeProblem(message: string, dir: string) {
  const scope = /^[\w-]+\(([^()]+)\)!?:/u.exec(normalize(message)[0] ?? "")?.[1];
  if (scope === undefined) return undefined;
  const repo = await repository(dir);
  if (repo === undefined || scope.split("/")[0].toLowerCase() !== repo.toLowerCase()) {
    return undefined;
  }
  return `the scope ${quote(scope)} starts with the repository name ${quote(repo)}; scope by the area inside the repository instead`;
}

async function repository(dir: string) {
  for (let at = dir; ; at = path.dirname(at)) {
    const git = path.join(at, ".git");
    const info = await stat(git).catch(() => undefined);
    if (info?.isDirectory()) return path.basename(at);
    if (info?.isFile()) {
      const gitdir = /^gitdir: (.+)$/mu.exec((await read(git)) ?? "")?.[1].trim();
      if (!gitdir) return undefined;
      const [main, worktree] = path.resolve(at, gitdir).split("/.git/worktrees/");
      return worktree === undefined ? undefined : path.basename(main);
    }
    if (path.dirname(at) === at) return undefined;
  }
}

function rejection(shape: string[], scope: string | undefined) {
  const lines: string[] = [];
  if (scope) {
    lines.push("commit message rejected: scopes never name the repository (commit skill, Subject)", `- ${scope}`);
  }
  if (shape.length === 0) return lines.join("\n");
  const shown = shape.slice(0, maxProblems);
  if (shape.length > shown.length) shown.push(`${shape.length - shown.length} more problems`);
  lines.push(
    "commit message rejected: commit bullets never wrap (commit skill, Body)",
    ...shown.map((problem) => `- ${problem}`),
    `Write the subject, a blank line, then one "- " bullet per line, each at most ${width} characters.`,
    'Shorten the bullet, move detail into "  - " bullets under it, or split the commit.',
  );
  return lines.join("\n");
}
