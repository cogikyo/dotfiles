import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import { open, stat } from "node:fs/promises";
import path from "node:path";

const id = "opencode-tool-guard";
const maxPatchBytes = 1024 * 1024;
const probeBytes = 8192;

type Session = Record<string, unknown>;
type PatchOperation = "Delete" | "Update";

type PatchTarget = {
  operation: PatchOperation;
  path: string;
};

const server: Plugin = async ({ client, directory, worktree }) => {
  const fallbackDirectory = worktree || directory;

  return {
    "tool.execute.before": async (input, output) => {
      if (input.tool === "bash") {
        const command = string(object(output.args)?.command);
        if (!command) return;

        if (invokesRm(command)) {
          throw new Error("rm is disabled; move files to trash with `trash -- <path>`");
        }
        if (invokesGoBuild(command)) {
          const session = await readSession(client, input.sessionID);
          if (sessionAgent(session) !== "verify/test") {
            throw new Error("direct `go build` is reserved for verify/test; use the repository-owned rebuild or update command when one exists");
          }
        }
        return;
      }

      if (input.tool !== "apply_patch") return;

      const patchText = string(object(output.args)?.patchText);
      if (!patchText) return;
      if (Buffer.byteLength(patchText) > maxPatchBytes) {
        throw new Error(`apply_patch input exceeds ${formatBytes(maxPatchBytes)}; split the text patch or use the owning generator`);
      }

      const targets = patchTargets(patchText);
      if (targets.length === 0) return;

      const session = await readSession(client, input.sessionID);
      const cwd = string(session.directory) || fallbackDirectory;
      for (const target of targets) await guardPatchTarget(cwd, target);
    },
  };
};

async function guardPatchTarget(cwd: string, target: PatchTarget) {
  const filePath = path.isAbsolute(target.path) ? path.normalize(target.path) : path.resolve(cwd, target.path);

  let info;
  try {
    info = await stat(filePath);
  } catch {
    return;
  }
  if (!info.isFile()) return;

  if (info.size > maxPatchBytes) {
    throw new Error(patchRejection(target, filePath, `is ${formatBytes(info.size)}, above the ${formatBytes(maxPatchBytes)} text-patch limit`));
  }
  if (await isBinary(filePath, info.size)) {
    throw new Error(patchRejection(target, filePath, "contains binary data"));
  }
}

async function isBinary(filePath: string, size: number) {
  if (size === 0) return false;

  const file = await open(filePath, "r");
  try {
    const buffer = Buffer.allocUnsafe(Math.min(size, probeBytes));
    const { bytesRead } = await file.read(buffer, 0, buffer.length, 0);
    const content = buffer.subarray(0, bytesRead);
    if (content.includes(0)) return true;

    try {
      new TextDecoder("utf-8", { fatal: true }).decode(content);
      return false;
    } catch {
      return true;
    }
  } finally {
    await file.close();
  }
}

function patchTargets(patchText: string): PatchTarget[] {
  const targets: PatchTarget[] = [];
  for (const match of patchText.matchAll(/^\*\*\* (Delete|Update) File: (.+)$/gmu)) {
    targets.push({ operation: match[1] as PatchOperation, path: match[2].trim() });
  }
  return targets;
}

function patchRejection(target: PatchTarget, filePath: string, reason: string) {
  const action = target.operation === "Delete"
    ? `move it to trash with \`trash -- ${JSON.stringify(filePath)}\``
    : "use a purpose-built binary or generated-file tool";
  return `apply_patch refused to ${target.operation.toLowerCase()} ${filePath}: file ${reason}; ${action}`;
}

function invokesGoBuild(command: string) {
  const words = shellWords(command);
  return words.some((word, index) => executable(word) === "go" && words[index + 1] === "build")
    || nestedShellCommands(words).some(invokesGoBuild);
}

function invokesRm(command: string) {
  return /(?:^|[\n;&|()])\s*(?:(?:sudo|command)\s+)*(?:\/usr\/bin\/)?rm(?:\s|$)/u.test(command)
    || nestedShellCommands(shellWords(command)).some(invokesRm);
}

function nestedShellCommands(words: string[]) {
  return words.filter((_, index) => {
    if (index < 2 || !/^-\w*c\w*$/u.test(words[index - 1])) return false;
    return ["bash", "dash", "sh", "zsh"].includes(executable(words[index - 2]));
  });
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
    if (char === "'" || char === "\"") {
      quote = char;
      continue;
    }
    if (/\s|[;&|()]/u.test(char)) {
      flush();
      continue;
    }
    word += char;
  }
  flush();
  return words;
}

function executable(word: string) {
  return path.basename(word);
}

async function readSession(client: Parameters<Plugin>[0]["client"], sessionID: string): Promise<Session> {
  const response = await client.session.get({ path: { id: sessionID } } as never);
  const envelope = object(response);
  if (envelope && "error" in envelope && envelope.error !== undefined) {
    throw new Error(`tool guard could not read session ${sessionID}`);
  }
  return object(envelope?.data) || envelope || {};
}

function sessionAgent(session: Session) {
  return string(session.agent) || string(object(session.agent)?.name);
}

function formatBytes(bytes: number) {
  return `${Math.ceil(bytes / 1024)} KiB`;
}

function object(value: unknown): Record<string, unknown> | undefined {
  return typeof value === "object" && value !== null ? value as Record<string, unknown> : undefined;
}

function string(value: unknown) {
  return typeof value === "string" && value !== "" ? value : undefined;
}

export default { id, server } satisfies PluginModule;
