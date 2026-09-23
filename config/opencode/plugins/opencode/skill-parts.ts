import { realpathSync } from "node:fs";
import path from "node:path";

/** OpenCode tool-part fields used to mark, restore, and persist completed tool output. */
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

/** Subset of the OpenCode client used to update a session part. */
export type PartClient = {
  part?: {
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

/** Markdown files whose completed tool output must remain available after compaction. */
export type ProtectRoots = {
  configRoot: string;
  projectRoots: readonly string[];
  agentNames?: readonly string[];
};

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

export function isCompactedPart(part: SkillToolPart) {
  return part.state?.time?.compacted !== undefined;
}

/** Returns the original part unless it is completed, has timing data, and is not already compacted. */
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

/** Marks eligible parts in place because transformed message parts are returned by reference. */
export function stubSkillParts(parts: SkillToolPart[], time = Date.now()) {
  for (const part of parts) {
    if (!isCompletedSkillPart(part) || isCompactedPart(part)) continue;
    const state = part.state;
    if (!state?.time) continue;
    state.time.compacted = time;
  }
}

/** Throws when part.update is missing or returns an error envelope. */
export async function persistCompactedPart(client: PartClient, part: SkillToolPart) {
  const next = withCompactedTime(part);
  if (next === part) return;

  const parts = client.part;
  if (!parts?.update) throw new Error("part.update is unavailable");

  const result = await parts.update({
    sessionID: next.sessionID,
    messageID: next.messageID,
    partID: next.id,
    part: next,
  });
  const envelope = asObject(result);
  if (envelope && "error" in envelope && envelope.error !== undefined) {
    throw new Error("part.update failed");
  }
}

export async function persistCompactedToolParts(client: PartClient, parts: readonly SkillToolPart[]) {
  for (const part of parts) {
    if (!isCompletedToolPart(part) || isCompactedPart(part)) continue;
    await persistCompactedPart(client, part);
  }
}

export async function persistCompactedSkillParts(client: PartClient, parts: readonly SkillToolPart[]) {
  await persistCompactedToolParts(client, parts.filter(isCompletedSkillPart));
}

export async function persistCompactedPartsHttp(serverUrl: URL, directory: string, parts: readonly SkillToolPart[]) {
  const nextParts: SkillToolPart[] = [];
  for (const part of parts) {
    const next = withCompactedTime(part);
    if (next !== part) nextParts.push(next);
  }
  await persistUpdatedPartsHttp(serverUrl, directory, nextParts);
}

/** Reads the first Markdown path in filePath, path, filepath, or file input fields. */
export function toolMarkdownPath(part: SkillToolPart) {
  const input = part.state?.input;
  if (!input || typeof input !== "object") return undefined;
  const record = input as Record<string, unknown>;
  for (const key of ["filePath", "path", "filepath", "file"]) {
    const value = record[key];
    if (typeof value === "string" && isMarkdownPath(value)) return normalizeFilePath(value);
  }
  return undefined;
}

export function fileIdentity(filePath: string) {
  const normalizedPath = path.normalize(filePath);
  try {
    return realpathSync.native(normalizedPath);
  } catch {
    return normalizedPath;
  }
}

/** Protects the configured AGENTS.md files and named agent files under configRoot. */
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

/** Returns the original part unless it is completed and has timing data. */
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

/** Clears timestamps in place on protected completed tool parts in transformed messages. */
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

/** Throws when part.update is missing or returns an error envelope. */
export async function persistUpdatedPart(client: PartClient, part: SkillToolPart) {
  const parts = client.part;
  if (!parts?.update) throw new Error("part.update is unavailable");

  const result = await parts.update({
    sessionID: part.sessionID,
    messageID: part.messageID,
    partID: part.id,
    part,
  });
  const envelope = asObject(result);
  if (envelope && "error" in envelope && envelope.error !== undefined) {
    throw new Error("part.update failed");
  }
}

/** PATCHes each part at OpenCode's /session/{session}/message/{message}/part/{part} endpoint. */
export async function persistUpdatedPartsHttp(serverUrl: URL, directory: string, parts: readonly SkillToolPart[]) {
  for (const part of parts) {
    const url = new URL(`/session/${part.sessionID}/message/${part.messageID}/part/${part.id}`, serverUrl);
    if (directory) url.searchParams.set("directory", directory);
    const response = await fetch(url, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(part),
    });
    if (!response.ok) throw new Error(`part.update failed: ${response.status}`);
  }
}

function isMarkdownPath(value: string) {
  return /\.(md|mdx|markdown)$/i.test(value.split(/[?#]/, 1)[0]);
}

function normalizeFilePath(value: string) {
  return value.replace(/^file:\/\//, "").split(/[?#]/, 1)[0];
}

function asObject(value: unknown) {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : undefined;
}
