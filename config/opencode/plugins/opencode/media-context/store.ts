import {
  closeSync,
  existsSync,
  mkdirSync,
  openSync,
  readFileSync,
  renameSync,
  statSync,
  unlinkSync,
  writeFileSync,
} from "node:fs";
import { join } from "node:path";
import { record } from "../../shared/record.ts";
import { isImageMime, isVideoMime, mediaKindForMime, runtimeDir, sha256, type MediaKind } from "./files";

const HANDLE_EXACT_PATTERN = /^@(?:[01]\d|2[0-3])_[0-5]\d_[0-5]\d(?:_(?:[2-9]|[1-9]\d+))?$/;
const ALIAS_EXACT_PATTERN = /^@[a-z0-9](?:[a-z0-9-]{0,46}[a-z0-9])?$/;
const NAME_EXACT_PATTERN = /^[a-z0-9](?:[a-z0-9-]{0,46}[a-z0-9])?$/;
const MAX_REGISTRY_BYTES = 256 * 1024;
export const MAX_REGISTRY_ENTRIES = 200;

/** A session media record stored in the local registry. */
export type MediaRegistryEntry = {
  handle: string;
  sessionID: string;
  messageID?: string;
  partID?: string;
  path: string;
  mime: string;
  kind: MediaKind;
  hash: string;
  source: string;
  name?: string;
  alias?: string;
  nameSource?: string;
  nameUpdatedAt?: number;
  createdAt: number;
  updatedAt: number;
};

type RegistryFile = {
  version: 1;
  entries: MediaRegistryEntry[];
};

export function readRegistry(sessionID: string): MediaRegistryEntry[] {
  return readWritableRegistry(sessionID) ?? [];
}

export function readWritableRegistry(sessionID: string): MediaRegistryEntry[] | undefined {
  try {
    const path = registryPath(sessionID);
    if (!canReadRegistryPath(path)) return undefined;

    if (!existsSync(path)) return [];
    const parsed = record(JSON.parse(readFileSync(path, "utf8")));
    const entries = parsed?.entries;
    if (!Array.isArray(entries) || entries.length > MAX_REGISTRY_ENTRIES) return undefined;
    return entries.map(normalizeEntry).filter(isDefined);
  } catch {
    return undefined;
  }
}

export function canReadRegistry(sessionID: string) {
  return canReadRegistryPath(registryPath(sessionID));
}

function canReadRegistryPath(path: string) {
  try {
    return !existsSync(path) || statSync(path).size <= MAX_REGISTRY_BYTES;
  } catch {
    return false;
  }
}

export function writeRegistry(sessionID: string, entries: MediaRegistryEntry[]) {
  const path = registryPath(sessionID);
  mkdirSync(runtimeDir(), { recursive: true, mode: 0o700 });
  const tmp = `${path}.${process.pid}.${Date.now()}.tmp`;
  writeFileSync(tmp, `${JSON.stringify({ version: 1, entries } satisfies RegistryFile, null, 2)}\n`, { mode: 0o600 });
  renameSync(tmp, path);
}

export function withRegistryLock<T>(sessionID: string, operation: () => T) {
  const path = registryPath(sessionID);
  mkdirSync(runtimeDir(), { recursive: true, mode: 0o700 });

  const lockPath = `${path}.lock`;
  let fd: number | undefined;
  try {
    fd = openSync(lockPath, "wx", 0o600);
  } catch (error) {
    if (!isFileExistsError(error)) throw error;
    removeStaleLock(lockPath);
    try {
      fd = openSync(lockPath, "wx", 0o600);
    } catch (retryError) {
      if (!isFileExistsError(retryError)) throw retryError;
      throw new Error(`media registry is busy for ${sessionID}`, { cause: retryError });
    }
  }

  try {
    return operation();
  } finally {
    closeSync(fd);
    try {
      unlinkSync(lockPath);
    } catch {}
  }
}

function removeStaleLock(path: string) {
  try {
    if (Date.now() - statSync(path).mtimeMs > 10_000) unlinkSync(path);
  } catch {}
}

function isFileExistsError(error: unknown) {
  return typeof error === "object" && error !== null && "code" in error && error.code === "EEXIST";
}

function registryPath(sessionID: string) {
  const clean = sessionID.replace(/[^a-zA-Z0-9_.-]+/g, "_").slice(0, 80) || "session";
  return join(runtimeDir(), `${clean}-${sha256(sessionID).slice(0, 12)}.json`);
}

function normalizeEntry(raw: unknown): MediaRegistryEntry | undefined {
  const value = record(raw);
  if (
    !value ||
    typeof value.handle !== "string" ||
    !HANDLE_EXACT_PATTERN.test(value.handle) ||
    typeof value.path !== "string"
  )
    return undefined;
  const kind = value.kind === "image" || value.kind === "video" ? value.kind : undefined;
  const mime = string(value.mime);
  return {
    handle: value.handle,
    sessionID: string(value.sessionID) || "",
    messageID: string(value.messageID),
    partID: string(value.partID),
    path: value.path,
    mime: normalizeMime(mime, kind),
    kind: normalizeKind(kind, mime),
    hash: string(value.hash) || sha256(`${value.path}:${value.handle}`),
    source: string(value.source) || "unknown source",
    name: normalizeStoredName(value.name),
    alias: normalizeStoredAlias(value.alias, value.name),
    nameSource: string(value.nameSource)?.slice(0, 80),
    nameUpdatedAt: number(value.nameUpdatedAt),
    createdAt: number(value.createdAt) || Date.now(),
    updatedAt: number(value.updatedAt) || Date.now(),
  };
}

export function string(value: unknown) {
  return typeof value === "string" ? value : undefined;
}

function number(value: unknown) {
  return typeof value === "number" ? value : undefined;
}

function isDefined<T>(value: T | undefined): value is T {
  return value !== undefined;
}

export function normalizeStoredName(value: unknown) {
  if (typeof value !== "string") return undefined;
  const clean = value
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 48)
    .replace(/-+$/g, "");
  return NAME_EXACT_PATTERN.test(clean) ? clean : undefined;
}

function normalizeStoredAlias(alias: unknown, name: unknown) {
  if (typeof alias === "string" && ALIAS_EXACT_PATTERN.test(alias)) return alias;
  const cleanName = normalizeStoredName(name);
  return cleanName ? `@${cleanName}` : undefined;
}

function normalizeKind(kind: MediaKind | undefined, mime: string | undefined): MediaKind {
  return kind ?? mediaKindForMime(mime) ?? "image";
}

function normalizeMime(mime: string | undefined, kind: MediaKind | undefined) {
  const normalizedKind = normalizeKind(kind, mime);
  if (normalizedKind === "image") return isImageMime(mime) ? (mime ?? "image/png") : "image/png";
  return isVideoMime(mime) ? (mime ?? "video/mp4") : "video/mp4";
}
