import { readFile } from "node:fs/promises";
import path from "node:path";
import { commandArgs, executable, separator, shellWords } from "./shell-words.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Commit message shape: one subject line, then one bullet per line that never wraps             │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

const width = 72;
const maxProblems = 8;
const opener = /(?<!<)<<(-?)\s*(['"]?)([\w.-]+)\2/gu;

/**
 * Returns a rejection for the first `git commit` in a shell command whose message breaks the commit skill's shape.
 * Messages come from `-m`, `-F <file>`, or `-F -` fed by a heredoc or `<` redirect; unknowable sources pass.
 */
export async function commitMessageRejection(command: string, cwd: string) {
  if (!command.includes("commit")) return undefined;
  for (const message of await commitMessages(command, cwd)) {
    const problems = lint(message);
    if (problems.length > 0) return rejection(problems);
  }
  return undefined;
}

async function commitMessages(command: string, cwd: string) {
  const { text, bodies } = heredocs(command);
  const words = shellWords(text);
  const messages: string[] = [];
  for (const [at, word] of words.entries()) {
    if (executable(word) !== "git") continue;
    const message = await commitMessage(words, at, bodies, cwd);
    if (message !== undefined) messages.push(message);
  }
  return messages;
}

/** Removes heredoc bodies from the command so shell words stay on opener lines. */
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

async function commitMessage(words: string[], at: number, bodies: string[], cwd: string) {
  const args = commandArgs(words, at);
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
  if (args[index] !== "commit") return undefined;

  const base = at + index + 2;
  const options = args.slice(index + 1);
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
  if (file !== "-") return read(path.resolve(dir, file));

  const own = options.findIndex((option) => option.startsWith("<<") && !option.startsWith("<<<"));
  if (own !== -1) return bodies[heredocsBefore(words, base + own)];
  const redirect = options.findIndex((option) => option.startsWith("<"));
  if (redirect !== -1) {
    const target = options[redirect] === "<" ? options[redirect + 1] : options[redirect].slice(1);
    return target ? read(path.resolve(cwd, target)) : undefined;
  }
  const segment = words.lastIndexOf(separator, at - 2) + 1;
  const piped = heredocsBefore(words, at);
  return at > 0 && words[at - 1] === separator && piped > heredocsBefore(words, segment) ? bodies[piped - 1] : undefined;
}

async function read(file: string) {
  try {
    return await readFile(file, "utf8");
  } catch {
    return undefined;
  }
}

// ├─ Shape ───────────────────────────────────────────────────────────────────────────────────────┤

/** Applies Git's default `whitespace` cleanup for `-m` and `-F` before checking lines. */
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
    const length = [...line].length;
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
  const chars = [...text];
  return JSON.stringify(chars.length > 32 ? `${chars.slice(0, 32).join("")}…` : text);
}

function rejection(problems: string[]) {
  const shown = problems.slice(0, maxProblems);
  if (problems.length > shown.length) shown.push(`${problems.length - shown.length} more problems`);
  return [
    "commit message rejected: commit bullets never wrap (commit skill, Body)",
    ...shown.map((problem) => `- ${problem}`),
    `Write the subject, a blank line, then one "- " bullet per line, each at most ${width} characters.`,
    'Shorten the bullet, move detail into "  - " bullets under it, or split the commit.',
  ].join("\n");
}
