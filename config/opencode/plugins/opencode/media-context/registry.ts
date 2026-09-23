import { createHash } from "node:crypto";
import {
  chmodSync,
  closeSync,
  copyFileSync,
  existsSync,
  lstatSync,
  mkdirSync,
  openSync,
  readFileSync,
  realpathSync,
  renameSync,
  statSync,
  unlinkSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { extname, join, normalize } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { record } from "../../shared/record.ts";

const HANDLE_PATTERN =
  /(?:^|[^A-Za-z0-9_.\\/-])(@(?:[01]\d|2[0-3])_[0-5]\d_[0-5]\d(?:_(?:[2-9]|[1-9]\d+))?)(?![A-Za-z0-9_\\/-]|\.[A-Za-z0-9])/g;
const HANDLE_EXACT_PATTERN = /^@(?:[01]\d|2[0-3])_[0-5]\d_[0-5]\d(?:_(?:[2-9]|[1-9]\d+))?$/;
const ALIAS_PATTERN =
  /(?:^|[^A-Za-z0-9_.\\/-])(@[a-z0-9](?:[a-z0-9-]{0,46}[a-z0-9])?)(?![A-Za-z0-9_\\/-]|\.[A-Za-z0-9])/g;
const ALIAS_EXACT_PATTERN = /^@[a-z0-9](?:[a-z0-9-]{0,46}[a-z0-9])?$/;
const NAME_EXACT_PATTERN = /^[a-z0-9](?:[a-z0-9-]{0,46}[a-z0-9])?$/;
const MAX_REGISTRY_BYTES = 256 * 1024;
const MAX_REGISTRY_ENTRIES = 200;
const MAX_DATA_IMAGE_BYTES = 2 * 1024 * 1024;
const MAX_VIDEO_SCAN_CHARS = 20_000;
const MAX_VIDEO_CANDIDATES = 20;
const MAX_VIDEO_CANDIDATE_LENGTH = 1_024;
const VIDEO_MIME_BY_EXTENSION = new Map([
  [".mp4", "video/mp4"],
  [".mov", "video/quicktime"],
  [".mkv", "video/x-matroska"],
  [".webm", "video/webm"],
  [".avi", "video/x-msvideo"],
  [".m4v", "video/x-m4v"],
]);
type MediaKind = "image" | "video";
type FileSource = { type: "file"; path: string; text: { value: string; start: number; end: number } };

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Session media registry                                                                        │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

/** A provider or persisted message part that refers to local image or video media. */
export type MediaFilePart = {
  id?: string;
  sessionID?: string;
  messageID?: string;
  type: "file";
  mime: string;
  kind?: MediaKind;
  filename?: string;
  url: string;
  source?: { type: string; path?: string; text?: { value: string; start: number; end: number } };
};

/** A media part with the message identity and source fields required for persistence. */
export type PersistedMediaFilePart = Omit<MediaFilePart, "id" | "sessionID" | "messageID" | "source"> & {
  id: string;
  sessionID: string;
  messageID: string;
  source: FileSource;
};

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

// ├─ Registration and lookup ─────────────────────────────────────────────────────────────────────┤
/** Narrows an unknown message part to supported image or video media. */
export function mediaPart(part: unknown): MediaFilePart | undefined {
  const candidate = record(part);
  const mime = string(candidate?.mime);
  const url = string(candidate?.url);
  const kind = mediaKindForMime(mime);
  if (candidate?.type !== "file" || !mime || url === undefined || !kind) return undefined;
  return {
    type: "file",
    mime,
    url,
    kind: candidate.kind === "image" || candidate.kind === "video" ? candidate.kind : kind,
    id: string(candidate.id),
    sessionID: string(candidate.sessionID),
    messageID: string(candidate.messageID),
    filename: string(candidate.filename),
    source: mediaSource(candidate.source),
  };
}

function mediaSource(value: unknown): MediaFilePart["source"] {
  const source = record(value);
  if (!source || typeof source.type !== "string") return undefined;
  const text = record(source.text);
  return {
    type: source.type,
    path: string(source.path),
    text:
      text && typeof text.value === "string" && typeof text.start === "number" && typeof text.end === "number"
        ? { value: text.value, start: text.start, end: text.end }
        : undefined,
  };
}

/** Adds or updates local media in the session registry. */
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

/** Reads registered media for a session, returning an empty list on read failure. */
export function listSessionMedia(sessionID: string) {
  try {
    return readRegistry(sessionID);
  } catch {
    return [];
  }
}

// ├─ Names and references ────────────────────────────────────────────────────────────────────────┤
/** Returns a named alias when available, otherwise the generated handle. */
export function mediaReference(entry: MediaRegistryEntry) {
  return entry.alias || entry.handle;
}

/** Saves a normalized image name, unique alias, and named cache copy. */
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

/** Resolves handles and aliases in text to existing local media files. */
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

/** Builds the persisted file part used to add a registered image to provider context. */
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

// ├─ Local media paths ───────────────────────────────────────────────────────────────────────────┤
/** Resolves supported media parts to a local file path. */
export function localMediaPath(part: MediaFilePart) {
  const kind = part.kind ?? mediaKindForMime(part.mime);
  if (!kind) return undefined;

  const path = sourcePath(part);
  if (path) return kind === "video" ? allowedExistingVideoFile(path) : path;

  const fileURLPath = filePathFromURL(part.url);
  if (fileURLPath && isExistingFile(fileURLPath))
    return kind === "video" ? allowedExistingVideoFile(fileURLPath) : fileURLPath;

  return kind === "image" ? materializeDataImage(part) : undefined;
}

/** Returns a video path only when it is an existing file under an allowed root. */
export function allowedExistingVideoFile(path: string) {
  try {
    if (!isUnderAllowedVideoRoot(path) || !isExistingFile(path)) return undefined;
    const real = realpathSync(path);
    return isUnderAllowedVideoRoot(real) ? path : undefined;
  } catch {
    return undefined;
  }
}

/** Returns the roots from which video paths may be resolved. */
export function allowedVideoRoots() {
  const roots = ["/home/cullyn/", "/tmp/"];
  const runtime = process.env.XDG_RUNTIME_DIR;
  if (runtime?.startsWith("/")) roots.push(withTrailingSlash(normalize(runtime)));
  return roots;
}

/** Extracts supported local video paths from a bounded text scan. */
export function videoPathParts(text: string): MediaFilePart[] {
  const candidates = videoPathCandidates(text.slice(0, MAX_VIDEO_SCAN_CHARS));
  const parts: MediaFilePart[] = [];

  for (const candidate of candidates) {
    const mime = videoMime(candidate.path);
    if (!mime || !allowedExistingVideoFile(candidate.path)) continue;

    parts.push({
      type: "file",
      kind: "video",
      mime,
      url: pathToFileURL(candidate.path).href,
      source: {
        type: "file",
        path: candidate.path,
        text: { value: candidate.path, start: candidate.start, end: candidate.end },
      },
    });
  }

  return parts;
}

type VideoPathCandidate = { path: string; start: number; end: number };

function videoPathCandidates(text: string): VideoPathCandidate[] {
  const candidates: VideoPathCandidate[] = [];
  const seen = new Set<string>();

  const add = (path: string, start: number, end: number) => {
    if (candidates.length >= MAX_VIDEO_CANDIDATES) return;
    const candidate = path.trim();
    if (candidate.length === 0 || candidate.length > MAX_VIDEO_CANDIDATE_LENGTH || seen.has(candidate)) return;
    if (!candidate.startsWith("/")) return;
    if (hasGlob(candidate) || !videoMime(candidate)) return;
    seen.add(candidate);
    candidates.push({ path: candidate, start, end });
  };

  for (const match of text.matchAll(/(["'`])([^"'`\n]{1,1024})\1/g)) {
    const value = match[2];
    if (value === undefined || match.index === undefined) continue;
    add(value, match.index + 1, match.index + 1 + value.length);
  }

  const roots = allowedVideoRoots();
  for (let index = 0; index < text.length && candidates.length < MAX_VIDEO_CANDIDATES; index++) {
    const root = roots.find((value) => text.startsWith(value, index));
    if (!root) continue;

    const segment =
      text.slice(index, Math.min(text.length, index + MAX_VIDEO_CANDIDATE_LENGTH)).split(/[\n"'`]/, 1)[0] ?? "";
    for (const match of segment.matchAll(/\.(?:mp4|mov|mkv|webm|avi|m4v)(?=$|[^A-Za-z0-9])/gi)) {
      add(segment.slice(0, (match.index ?? 0) + match[0].length), index, index + (match.index ?? 0) + match[0].length);
      break;
    }

    index += Math.max(0, root.length - 1);
  }

  return candidates;
}

/** Reports whether a path names an existing regular file. */
export function isExistingFile(value: string) {
  try {
    return existsSync(value) && lstatSync(value).isFile();
  } catch {
    return false;
  }
}

function videoMime(path: string) {
  return VIDEO_MIME_BY_EXTENSION.get(extname(path).toLowerCase());
}

function hasGlob(path: string) {
  return /[*?[\]]/.test(path);
}

function isUnderAllowedVideoRoot(path: string) {
  if (!path.startsWith("/")) return false;
  const normalized = normalize(path);
  return allowedVideoRoots().some((root) => normalized.startsWith(root));
}

function withTrailingSlash(value: string) {
  return value.endsWith("/") ? value : `${value}/`;
}

// ├─ Registry storage ────────────────────────────────────────────────────────────────────────────┤
function readRegistry(sessionID: string): MediaRegistryEntry[] {
  return readWritableRegistry(sessionID) ?? [];
}

function readWritableRegistry(sessionID: string): MediaRegistryEntry[] | undefined {
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

function canReadRegistry(sessionID: string) {
  return canReadRegistryPath(registryPath(sessionID));
}

function canReadRegistryPath(path: string) {
  try {
    return !existsSync(path) || statSync(path).size <= MAX_REGISTRY_BYTES;
  } catch {
    return false;
  }
}

function writeRegistry(sessionID: string, entries: MediaRegistryEntry[]) {
  const path = registryPath(sessionID);
  mkdirSync(runtimeDir(), { recursive: true, mode: 0o700 });
  const tmp = `${path}.${process.pid}.${Date.now()}.tmp`;
  writeFileSync(tmp, `${JSON.stringify({ version: 1, entries } satisfies RegistryFile, null, 2)}\n`, { mode: 0o600 });
  renameSync(tmp, path);
}

function withRegistryLock<T>(sessionID: string, operation: () => T) {
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

// ├─ Registry formats ────────────────────────────────────────────────────────────────────────────┤
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

function string(value: unknown) {
  return typeof value === "string" ? value : undefined;
}

function number(value: unknown) {
  return typeof value === "number" ? value : undefined;
}

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
function normalizeStoredName(value: unknown) {
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

function isDefined<T>(value: T | undefined): value is T {
  return value !== undefined;
}

function filePartID(messageID: string, index: number) {
  const cleanMessageID = messageID
    .replace(/[^a-zA-Z0-9_-]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .slice(0, 64);
  const suffix = sha256(`${messageID}:${index}:${Date.now()}`).slice(0, 12);
  return `prt_${cleanMessageID || "media_ref"}_${index}_${suffix}`;
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

function mediaHash(part: MediaFilePart, path: string) {
  const urlHashInput = isDataURL(part.url) ? "data-image" : part.url;
  try {
    const stat = lstatSync(path);
    const real = stat.isFile() ? realpathSync(path) : path;
    return sha256(`${real}:${stat.size}:${stat.mtimeMs}:${urlHashInput}`);
  } catch {
    return sha256(`${path}:${urlHashInput}`);
  }
}

function materializeDataImage(part: MediaFilePart) {
  const parsed = parseDataImage(part.url);
  if (!parsed) return undefined;

  const ext = extensionForMime(parsed.mime, "image") || extensionForMime(part.mime, "image") || ".img";
  const hash = sha256(`${parsed.mime}:${parsed.payload}`);
  const path = join(cacheDir(), `data-image-${hash.slice(0, 32)}${ext}`);
  if (isExistingFile(path)) return path;

  const buffer = Buffer.from(parsed.payload, "base64");
  if (buffer.byteLength > MAX_DATA_IMAGE_BYTES) return undefined;
  writeFileSync(path, buffer, { mode: 0o600 });
  return path;
}

function parseDataImage(value: string) {
  if (value.slice(0, "data:image/".length).toLowerCase() !== "data:image/") return undefined;

  const comma = value.indexOf(",");
  if (comma < 0 || comma > 256) return undefined;

  const metadata = value.slice("data:".length, comma).toLowerCase();
  if (!metadata.endsWith(";base64")) return undefined;

  const payloadStart = comma + 1;
  const payloadLength = value.length - payloadStart;
  const maxBase64Length = Math.ceil(MAX_DATA_IMAGE_BYTES / 3) * 4 + 4;
  if (payloadLength === 0 || payloadLength > maxBase64Length) return undefined;

  const padding = value.endsWith("==") ? 2 : value.endsWith("=") ? 1 : 0;
  const decodedBytes = Math.floor((payloadLength * 3) / 4) - padding;
  if (decodedBytes > MAX_DATA_IMAGE_BYTES) return undefined;

  const mime = metadata.slice(0, -";base64".length);
  return isImageMime(mime) ? { mime, payload: value.slice(payloadStart) } : undefined;
}

function cacheDir() {
  const dir = join(runtimeDir(), "cache");
  mkdirSync(dir, { recursive: true, mode: 0o700 });
  return dir;
}

function runtimeDir() {
  const base = process.env.XDG_RUNTIME_DIR || join(tmpdir(), `opencode-${process.getuid?.() ?? "user"}`);
  return join(base, "opencode", "media-context");
}

function registryPath(sessionID: string) {
  const clean = sessionID.replace(/[^a-zA-Z0-9_.-]+/g, "_").slice(0, 80) || "session";
  return join(runtimeDir(), `${clean}-${sha256(sessionID).slice(0, 12)}.json`);
}

// ├─ Media formats ───────────────────────────────────────────────────────────────────────────────┤
function mediaSourceLabel(part: MediaFilePart) {
  if (sourcePath(part)) return part.source?.type === "clipboard" ? "clipboard" : "local source";
  if (filePathFromURL(part.url)) return "file URL";
  if (parseDataImage(part.url)) return "clipboard data image";
  return "unsupported media source";
}

function sourcePath(part: MediaFilePart) {
  const path = part.source?.path;
  return path && isExistingFile(path) ? path : undefined;
}

function filePathFromURL(value: string) {
  if (!value.startsWith("file:")) return undefined;
  try {
    return fileURLToPath(value);
  } catch {
    return undefined;
  }
}

function isDataURL(value: string) {
  return value.slice(0, "data:".length).toLowerCase() === "data:";
}

function normalizeKind(kind: MediaKind | undefined, mime: string | undefined): MediaKind {
  return kind ?? mediaKindForMime(mime) ?? "image";
}

function normalizeMime(mime: string | undefined, kind: MediaKind | undefined) {
  const normalizedKind = normalizeKind(kind, mime);
  if (normalizedKind === "image") return isImageMime(mime) ? (mime ?? "image/png") : "image/png";
  return isVideoMime(mime) ? (mime ?? "video/mp4") : "video/mp4";
}

function mediaKindForMime(mime: string | undefined): MediaKind | undefined {
  if (isImageMime(mime)) return "image";
  if (isVideoMime(mime)) return "video";
  return undefined;
}

function isImageMime(mime: string | undefined) {
  return Boolean(mime?.startsWith("image/"));
}

function isVideoMime(mime: string | undefined) {
  return Boolean(mime?.startsWith("video/"));
}

function extensionForMime(mime: string, kind: MediaKind) {
  if (mime === "image/jpeg") return ".jpg";
  if (mime === "image/png") return ".png";
  if (mime === "image/gif") return ".gif";
  if (mime === "image/webp") return ".webp";
  if (mime === "video/mp4") return ".mp4";
  if (mime === "video/quicktime") return ".mov";
  if (mime === "video/x-matroska") return ".mkv";
  if (mime === "video/webm") return ".webm";
  if (mime === "video/x-msvideo") return ".avi";
  if (mime === "video/x-m4v") return ".m4v";
  return kind === "video" ? ".mp4" : "";
}

function sha256(value: string | Buffer) {
  return createHash("sha256").update(value).digest("hex");
}
