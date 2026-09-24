import { chmodSync, copyFileSync, existsSync } from "node:fs";
import { extname, join } from "node:path";
import { pathToFileURL } from "node:url";
import type { Part as V1Part } from "@opencode-ai/sdk";
import type { Part } from "@opencode-ai/sdk/v2";
import {
  cacheDir,
  extensionForMime,
  isExistingFile,
  localMediaPath,
  mediaHash,
  mediaKindForMime,
  mediaSourceLabel,
  sha256,
  type MediaFilePart,
} from "./files";
import {
  MAX_REGISTRY_ENTRIES,
  canReadRegistry,
  normalizeStoredName,
  readRegistry,
  readWritableRegistry,
  withRegistryLock,
  writeRegistry,
  type MediaRegistryEntry,
} from "./store";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Session media registry                                                                        │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

type FileSource = { type: "file"; path: string; text: { value: string; start: number; end: number } };

/** Media file part with the IDs and source text required for persistence. */
export type PersistedMediaFilePart = Omit<MediaFilePart, "id" | "sessionID" | "messageID" | "source"> & {
  id: string;
  sessionID: string;
  messageID: string;
  source: FileSource;
};

// ├─ Registration and lookup ─────────────────────────────────────────────────────────────────────┤

/** Extracts an image or video file part from an SDK part without reading its file. */
export function mediaPart(part: Part | V1Part): MediaFilePart | undefined {
  if (part.type !== "file") return undefined;
  const kind = mediaKindForMime(part.mime);
  if (!kind) return undefined;
  return {
    type: "file",
    mime: part.mime,
    url: part.url,
    kind,
    id: part.id,
    sessionID: part.sessionID,
    messageID: part.messageID,
    filename: part.filename,
    source: part.source,
  };
}

/** Registers local media under a timestamp handle, reusing matching rows and returning undefined when storage fails or is full. */
export function registerSessionMedia(sessionID: string, messageID: string | undefined, part: MediaFilePart) {
  try {
    const kind = part.kind ?? mediaKindForMime(part.mime);
    if (!kind) return undefined;

    const path = localMediaPath(part);
    if (!path || !canReadRegistry(sessionID)) return undefined;

    return withRegistryLock(sessionID, () => {
      if (!canReadRegistry(sessionID)) return undefined;

      const now = Date.now();
      const hash = mediaHash(part, path);
      const entries = readWritableRegistry(sessionID);
      if (!entries) return undefined;

      const existing = entries.find((entry) => sameMedia(entry, messageID, part, { hash, path }));
      if (existing) {
        if (!existing.name || existing.kind !== "image" || !isExistingFile(existing.path)) existing.path = path;
        existing.mime = part.mime || existing.mime;
        existing.kind = kind;
        existing.source = mediaSourceLabel(part);
        existing.updatedAt = now;
        writeRegistry(sessionID, entries);
        return existing;
      }

      if (entries.length >= MAX_REGISTRY_ENTRIES) return undefined;

      const entry: MediaRegistryEntry = {
        handle: nextMediaHandle(entries, now),
        sessionID,
        messageID,
        partID: part.id,
        path,
        mime: part.mime || (kind === "image" ? "image/png" : "video/mp4"),
        kind,
        hash,
        source: mediaSourceLabel(part),
        createdAt: now,
        updatedAt: now,
      };
      entries.push(entry);
      writeRegistry(sessionID, entries);
      return entry;
    });
  } catch {
    return undefined;
  }
}

/** Lists session media without locking, returning an empty list on read failure. */
export function listSessionMedia(sessionID: string) {
  try {
    return readRegistry(sessionID);
  } catch {
    return [];
  }
}

function sameMedia(
  entry: MediaRegistryEntry,
  messageID: string | undefined,
  part: MediaFilePart,
  target: { hash: string; path: string },
) {
  if (part.id && entry.partID === part.id && (!messageID || entry.messageID === messageID)) return true;
  if (entry.path === target.path) return true;
  return entry.hash === target.hash;
}

function nextMediaHandle(entries: MediaRegistryEntry[], timestamp: number) {
  const base = timestampHandleBase(timestamp);
  let max = entries.some((entry) => entry.handle === base) ? 1 : 0;

  for (const entry of entries) {
    const match = new RegExp(`^${base}_(\\d+)$`).exec(entry.handle);
    if (!match) continue;
    max = Math.max(max, Number(match[1]));
  }

  return max === 0 ? base : `${base}_${max + 1}`;
}

function timestampHandleBase(timestamp: number) {
  const date = new Date(timestamp);
  const hours = String(date.getHours()).padStart(2, "0");
  const minutes = String(date.getMinutes()).padStart(2, "0");
  const seconds = String(date.getSeconds()).padStart(2, "0");
  return `@${hours}_${minutes}_${seconds}`;
}

// ├─ Names and references ────────────────────────────────────────────────────────────────────────┤

export function mediaReference(entry: MediaRegistryEntry) {
  return entry.alias || entry.handle;
}

/** Names an image by copying it into the cache and storing an alias, leaving its original file and handle unchanged. */
export function updateImageName(sessionID: string, handle: string, name: string, source: string) {
  const cleanName = normalizeStoredName(name);
  if (!cleanName) return undefined;

  try {
    return withRegistryLock(sessionID, () => {
      const entries = readWritableRegistry(sessionID);
      if (!entries) return undefined;

      const entry = entries.find((candidate) => candidate.handle === handle);
      if (!entry || entry.kind !== "image" || !isExistingFile(entry.path)) return undefined;

      const alias = uniqueAlias(entries, entry, cleanName);
      const finalName = alias.slice(1);
      const nextPath = uniqueNamedCachePath(finalName, extensionForEntry(entry));
      copyFileSync(entry.path, nextPath);
      chmodSync(nextPath, 0o600);

      const now = Date.now();
      entry.path = nextPath;
      entry.name = finalName;
      entry.alias = alias;
      entry.nameSource = source || "model";
      entry.nameUpdatedAt = now;
      entry.updatedAt = now;
      writeRegistry(sessionID, entries);
      return entry;
    });
  } catch {
    return undefined;
  }
}

/** Resolves standalone handles or aliases in text to registry rows whose files still exist. */
export function resolveMediaReferences(sessionID: string, text: string) {
  const requested = requestedMediaReferences(text);
  if (requested.size === 0) return [];

  const entries = readRegistry(sessionID);
  const resolved: MediaRegistryEntry[] = [];
  for (const entry of entries) {
    if (!entryReferences(entry).some((reference) => requested.has(reference))) continue;
    if (!isExistingFile(entry.path)) continue;
    resolved.push(entry);
  }
  return resolved;
}

/** Builds a file part for a registry row without checking the file; its source text is a reference, not a message span. */
export function mediaFilePartForEntry(
  entry: MediaRegistryEntry,
  sessionID: string,
  messageID: string,
  index: number,
): PersistedMediaFilePart {
  const reference = mediaReference(entry);
  return {
    id: filePartID(messageID, index),
    sessionID,
    messageID,
    type: "file",
    mime: entry.mime,
    kind: entry.kind,
    filename: `${reference.slice(1)}${extensionForMime(entry.mime, entry.kind)}`,
    url: pathToFileURL(entry.path).href,
    source: {
      type: "file",
      path: entry.path,
      text: { value: reference, start: 0, end: reference.length },
    },
  };
}

const HANDLE_PATTERN =
  /(?:^|[^A-Za-z0-9_.\\/-])(@(?:[01]\d|2[0-3])_[0-5]\d_[0-5]\d(?:_(?:[2-9]|[1-9]\d+))?)(?![A-Za-z0-9_\\/-]|\.[A-Za-z0-9])/g;
const ALIAS_PATTERN =
  /(?:^|[^A-Za-z0-9_.\\/-])(@[a-z0-9](?:[a-z0-9-]{0,46}[a-z0-9])?)(?![A-Za-z0-9_\\/-]|\.[A-Za-z0-9])/g;

function requestedMediaReferences(text: string) {
  const requested = new Set<string>();
  HANDLE_PATTERN.lastIndex = 0;
  ALIAS_PATTERN.lastIndex = 0;

  for (const match of text.matchAll(HANDLE_PATTERN)) {
    if (match[1]) requested.add(match[1]);
  }
  for (const match of text.matchAll(ALIAS_PATTERN)) {
    if (match[1]) requested.add(match[1]);
  }

  return requested;
}

function entryReferences(entry: MediaRegistryEntry) {
  return entry.alias ? [entry.handle, entry.alias] : [entry.handle];
}

// ├─ Aliases and file names ──────────────────────────────────────────────────────────────────────┤

// The time-based fallback after 200 attempts may collide with an existing alias.
function uniqueAlias(entries: MediaRegistryEntry[], current: MediaRegistryEntry, name: string) {
  const used = new Set(entries.filter((entry) => entry.handle !== current.handle).flatMap(entryReferences));
  for (let index = 1; index <= 200; index++) {
    const candidateName = index === 1 ? name : withNumericSuffix(name, index);
    const alias = `@${candidateName}`;
    if (!used.has(alias)) return alias;
  }
  return `@${withNumericSuffix(name, Date.now() % 100_000)}`;
}

function withNumericSuffix(name: string, index: number) {
  const suffix = `-${index}`;
  return `${name.slice(0, Math.max(1, 48 - suffix.length)).replace(/-+$/g, "")}${suffix}`;
}

function uniqueNamedCachePath(name: string, ext: string) {
  for (let index = 1; index <= 200; index++) {
    const suffix = index === 1 ? "" : `-${index}`;
    const candidate = join(cacheDir(), `${name}${suffix}${ext}`);
    if (!existsSync(candidate)) return candidate;
  }
  return join(cacheDir(), `${name}-${Date.now().toString(36)}${ext}`);
}

function extensionForEntry(entry: MediaRegistryEntry) {
  return extname(entry.path) || extensionForMime(entry.mime, entry.kind) || ".img";
}

function filePartID(messageID: string, index: number) {
  const cleanMessageID = messageID
    .replace(/[^a-zA-Z0-9_-]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .slice(0, 64);
  const suffix = sha256(`${messageID}:${index}:${Date.now()}`).slice(0, 12);
  return `prt_${cleanMessageID || "media_ref"}_${index}_${suffix}`;
}
