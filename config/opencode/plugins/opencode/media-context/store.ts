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
import { z } from "zod";
import { isImageMime, isVideoMime, mediaKindForMime, runtimeDir, sha256, type MediaKind } from "./files";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ On-disk registry                                                                              │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

// ├─ Entry schema ────────────────────────────────────────────────────────────────────────────────┤

/** Maximum session rows accepted for registration and file validation. */
export const MAX_REGISTRY_ENTRIES = 200;

const MAX_REGISTRY_BYTES = 256 * 1024;

/** Session media row with a timestamp handle and an optional named alias pointing to its local file. */
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

const HANDLE_EXACT_PATTERN = /^@(?:[01]\d|2[0-3])_[0-5]\d_[0-5]\d(?:_(?:[2-9]|[1-9]\d+))?$/;
const ALIAS_EXACT_PATTERN = /^@[a-z0-9](?:[a-z0-9-]{0,46}[a-z0-9])?$/;
const NAME_EXACT_PATTERN = /^[a-z0-9](?:[a-z0-9-]{0,46}[a-z0-9])?$/;

const text = z.string().optional().catch(undefined);
const count = z.number().optional().catch(undefined);

const Entry = z
  .object({
    handle: z.string().regex(HANDLE_EXACT_PATTERN),
    path: z.string(),
    kind: z.enum(["image", "video"]).optional().catch(undefined),
    mime: text,
    sessionID: text,
    messageID: text,
    partID: text,
    hash: text,
    source: text,
    name: text,
    alias: text,
    nameSource: text,
    nameUpdatedAt: count,
    createdAt: count,
    updatedAt: count,
  })
  .transform((value): MediaRegistryEntry => ({
    handle: value.handle,
    sessionID: value.sessionID || "",
    messageID: value.messageID,
    partID: value.partID,
    path: value.path,
    mime: normalizeMime(value.mime, value.kind),
    kind: normalizeKind(value.kind, value.mime),
    hash: value.hash || sha256(`${value.path}:${value.handle}`),
    source: value.source || "unknown source",
    name: normalizeStoredName(value.name),
    alias: normalizeStoredAlias(value.alias, value.name),
    nameSource: value.nameSource?.slice(0, 80),
    nameUpdatedAt: value.nameUpdatedAt,
    createdAt: value.createdAt || Date.now(),
    updatedAt: value.updatedAt || Date.now(),
  }));

// Bad rows are dropped, but exceeding the row cap invalidates the whole file.
const Registry = z
  .object({
    entries: z
      .array(z.unknown())
      .max(MAX_REGISTRY_ENTRIES)
      .transform((entries) =>
        entries.flatMap((entry) => {
          const parsed = Entry.safeParse(entry);
          return parsed.success ? [parsed.data] : [];
        }),
      ),
  })
  .transform((registry) => registry.entries);

/** Turns a name into a lowercase slug of at most 48 characters, or undefined when none can be formed. */
export function normalizeStoredName(value: string | undefined) {
  if (value === undefined) return undefined;
  const clean = value
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 48)
    .replace(/-+$/g, "");
  return NAME_EXACT_PATTERN.test(clean) ? clean : undefined;
}

function normalizeStoredAlias(alias: string | undefined, name: string | undefined) {
  if (alias !== undefined && ALIAS_EXACT_PATTERN.test(alias)) return alias;
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

// ├─ Reads ───────────────────────────────────────────────────────────────────────────────────────┤

/** Reads session rows without locking, returning an empty list when the registry cannot be read. */
export function readRegistry(sessionID: string): MediaRegistryEntry[] {
  return readWritableRegistry(sessionID) ?? [];
}

/** Reads rows without locking; undefined means the existing registry is invalid and must not be overwritten. */
export function readWritableRegistry(sessionID: string): MediaRegistryEntry[] | undefined {
  try {
    const path = registryPath(sessionID);
    if (!canReadRegistryPath(path)) return undefined;

    if (!existsSync(path)) return [];
    const parsed = Registry.safeParse(JSON.parse(readFileSync(path, "utf8")));
    return parsed.success ? parsed.data : undefined;
  } catch {
    return undefined;
  }
}

/** Checks whether the registry file is missing or within the 256 KiB read cap. */
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

// ├─ Writes and lock ─────────────────────────────────────────────────────────────────────────────┤

/** Replaces the session registry atomically without locking or validating its rows; filesystem errors throw. */
export function writeRegistry(sessionID: string, entries: MediaRegistryEntry[]) {
  const path = registryPath(sessionID);
  mkdirSync(runtimeDir(), { recursive: true, mode: 0o700 });
  const tmp = `${path}.${process.pid}.${Date.now()}.tmp`;
  writeFileSync(tmp, `${JSON.stringify({ version: 1, entries } satisfies RegistryFile, null, 2)}\n`, { mode: 0o600 });
  renameSync(tmp, path);
}

/** Runs a synchronous registry operation under a per-session lock, releasing it on return or error. */
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
