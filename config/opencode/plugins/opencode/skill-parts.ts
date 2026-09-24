import { realpathSync } from "node:fs";
import path from "node:path";
import { z } from "zod";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Tool-part compaction shared by server and TUI plugins                                         │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

// ├─ Types ───────────────────────────────────────────────────────────────────────────────────────┤

/** OpenCode tool part whose other fields survive updates sent back to OpenCode. */
export type SkillToolPart = {
  id: string;
  sessionID: string;
  messageID: string;
  type?: string;
  tool?: string;
  state?: {
    status?: string;
    time?: {
      start?: number;
      end?: number;
      compacted?: number;
    };
    [key: string]: unknown;
  };
  [key: string]: unknown;
};

/** OpenCode part client; updates may return an `{ error }` envelope instead of throwing. */
export type PartClient = {
  part: {
    update(args: {
      sessionID: string;
      messageID: string;
      partID: string;
      directory?: string;
      workspace?: string;
      part?: SkillToolPart;
    }): Promise<unknown>;
  };
};

// ├─ Predicates ──────────────────────────────────────────────────────────────────────────────────┤

export function isSkillTool(tool: string) {
  return tool === "skill" || tool === "Skill";
}

export function isCompletedSkillPart(part: SkillToolPart) {
  return (
    part.type === "tool" &&
    typeof part.tool === "string" &&
    isSkillTool(part.tool) &&
    part.state?.status === "completed"
  );
}

export function isCompletedToolPart(part: SkillToolPart) {
  return part.type === "tool" && part.state?.status === "completed";
}

/** Checks for a compaction marker, including a marker set to zero. */
export function isCompactedPart(part: SkillToolPart) {
  return part.state?.time?.compacted !== undefined;
}

// ├─ Compaction time ─────────────────────────────────────────────────────────────────────────────┤

/** Returns a marked copy of an eligible part, or the original when no update needs persisting. */
export function withCompactedTime<T extends SkillToolPart>(part: T, time = Date.now()): T {
  const state = part.state;
  if (!state || state.status !== "completed" || !state.time || state.time.compacted !== undefined) return part;
  return {
    ...part,
    state: {
      ...state,
      time: {
        ...state.time,
        compacted: time,
      },
    },
  };
}

/** Marks completed skill parts as compacted in place in the outgoing message transform. */
export function stubSkillParts(parts: SkillToolPart[], time = Date.now()) {
  for (const part of parts) {
    if (!isCompletedSkillPart(part) || isCompactedPart(part)) continue;
    const state = part.state;
    if (!state?.time) continue;
    state.time.compacted = time;
  }
}

/** Returns a copy without the compaction marker, or the original when no marker exists. */
export function withoutCompactedTime<T extends SkillToolPart>(part: T): T {
  const state = part.state;
  if (!state?.time || state.time.compacted === undefined) return part;
  const { compacted: _compacted, ...time } = state.time;
  return {
    ...part,
    state: {
      ...state,
      time,
    },
  };
}

/** Returns a completed part with reloaded output and no compaction marker, or the original when ineligible. */
export function withReloadedOutput<T extends SkillToolPart>(part: T, output: string): T {
  const state = part.state;
  if (!state || state.status !== "completed" || !state.time) return part;
  const { compacted: _compacted, ...time } = state.time;
  return {
    ...part,
    state: {
      ...state,
      output,
      time,
    },
  };
}

/** Restores protected Markdown tool parts in place so their output survives compaction. */
export function uncompactProtectedParts(parts: SkillToolPart[], roots: ProtectRoots) {
  for (const part of parts) {
    if (!isCompletedToolPart(part) || !isCompactedPart(part)) continue;
    const filePath = toolMarkdownPath(part);
    if (!filePath || !isProtectedMarkdownPath(filePath, roots)) continue;
    const time = part.state?.time;
    if (!time) continue;
    delete time.compacted;
  }
}

// ├─ Persistence ─────────────────────────────────────────────────────────────────────────────────┤

/** Persists an eligible part's compaction marker; update failures throw. */
export async function persistCompactedPart(client: PartClient, part: SkillToolPart) {
  const next = withCompactedTime(part);
  if (next === part) return;
  await persistUpdatedPart(client, next);
}

/** Persists compaction markers for eligible completed tool parts; update failures throw. */
export async function persistCompactedToolParts(client: PartClient, parts: readonly SkillToolPart[]) {
  const pending = parts.filter((part) => isCompletedToolPart(part) && !isCompactedPart(part));
  await Promise.all(pending.map((part) => persistCompactedPart(client, part)));
}

/** PATCHes eligible parts with compaction markers; a failed request throws. */
export async function persistCompactedPartsHttp(serverUrl: URL, directory: string, parts: readonly SkillToolPart[]) {
  const nextParts: SkillToolPart[] = [];
  for (const part of parts) {
    const next = withCompactedTime(part);
    if (next !== part) nextParts.push(next);
  }
  await persistUpdatedPartsHttp(serverUrl, directory, nextParts);
}

const Envelope = z.object({ error: z.unknown() }).catch({ error: undefined });

/** Writes a part through OpenCode and throws when the update fails or returns an error. */
export async function persistUpdatedPart(client: PartClient, part: SkillToolPart) {
  const result = await client.part.update({
    sessionID: part.sessionID,
    messageID: part.messageID,
    partID: part.id,
    part,
  });
  if (Envelope.parse(result).error !== undefined) throw new Error("part.update failed");
}

/** PATCHes each part to its OpenCode session endpoint and throws on a failed response. */
export async function persistUpdatedPartsHttp(serverUrl: URL, directory: string, parts: readonly SkillToolPart[]) {
  await Promise.all(
    parts.map(async (part) => {
      const url = new URL(`/session/${part.sessionID}/message/${part.messageID}/part/${part.id}`, serverUrl);
      if (directory) url.searchParams.set("directory", directory);
      const response = await fetch(url, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(part),
      });
      if (!response.ok) throw new Error(`part.update failed: ${response.status}`);
    }),
  );
}

// ├─ Protected paths ─────────────────────────────────────────────────────────────────────────────┤

/** Roots for Markdown that must survive compaction; `collab` is protected even when omitted from `agentNames`. */
export type ProtectRoots = {
  configRoot: string;
  projectRoots: readonly string[];
  agentNames?: readonly string[];
};

const text = z.string().optional().catch(undefined);
const Input = z.object({ filePath: text, path: text, filepath: text, file: text }).catch({});

/** Finds the first Markdown path in a tool's file input fields, without expanding `~`. */
export function toolMarkdownPath(part: SkillToolPart) {
  const input = Input.parse(part.state?.input);
  const value = [input.filePath, input.path, input.filepath, input.file].find(
    (candidate) => candidate !== undefined && isMarkdownPath(candidate),
  );
  return value === undefined ? undefined : normalizeFilePath(value);
}

function isMarkdownPath(value: string) {
  return /\.(md|mdx|markdown)$/i.test(value.split(/[?#]/, 1)[0]);
}

function normalizeFilePath(value: string) {
  return value.replace(/^file:\/\//, "").split(/[?#]/, 1)[0];
}

/** Resolves an existing file's identity, falling back to its normalized path. */
export function fileIdentity(filePath: string) {
  const normalizedPath = path.normalize(filePath);
  try {
    return realpathSync.native(normalizedPath);
  } catch {
    return normalizedPath;
  }
}

/** Protects root `AGENTS.md` files and selected config agent files, including symlinked paths. */
export function isProtectedMarkdownPath(filePath: string, roots: ProtectRoots) {
  const id = fileIdentity(filePath);
  if (id === fileIdentity(path.join(roots.configRoot, "AGENTS.md"))) return true;
  for (const root of roots.projectRoots) {
    if (!root) continue;
    if (id === fileIdentity(path.join(root, "AGENTS.md"))) return true;
  }
  const names = new Set(["collab", ...(roots.agentNames ?? [])]);
  for (const name of names) {
    if (!name) continue;
    if (id === fileIdentity(path.join(roots.configRoot, "agents", `${name}.md`))) return true;
  }
  return false;
}
