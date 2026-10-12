import { createHash } from "node:crypto";
import { existsSync, lstatSync, mkdirSync, realpathSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { extname, join, normalize } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Local media files                                                                             │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

// ├─ Path resolution ─────────────────────────────────────────────────────────────────────────────┤

export type MediaKind = "image" | "video";

/** Image or video file part with a local source path or file/data URL. */
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

/** Resolves local image or allowed video files, caching data images when needed; cache writes can throw. */
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

/** Accepts regular video files whose path and realpath remain under an allowed root. */
export function allowedExistingVideoFile(path: string) {
  try {
    if (!isUnderAllowedVideoRoot(path) || !isExistingFile(path)) return undefined;
    const real = realpathSync(path);
    return isUnderAllowedVideoRoot(real) ? path : undefined;
  } catch {
    return undefined;
  }
}

/** Lists allowed video roots: `/home/cullyn/`, `/tmp/`, and an absolute `$XDG_RUNTIME_DIR`. */
export function allowedVideoRoots() {
  const roots = ["/home/cullyn/", "/tmp/"];
  const runtime = process.env.XDG_RUNTIME_DIR;
  if (runtime?.startsWith("/")) roots.push(withTrailingSlash(normalize(runtime)));
  return roots;
}

/** Checks for a regular file without following symlinks. */
export function isExistingFile(value: string) {
  try {
    return existsSync(value) && lstatSync(value).isFile();
  } catch {
    return false;
  }
}

// ├─ Video text scan ─────────────────────────────────────────────────────────────────────────────┤

/** Finds allowed existing video paths in user text and returns file parts without session or message IDs. */
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

// ├─ Identity and cache ──────────────────────────────────────────────────────────────────────────┤

/** Hashes a media file's identity and URL, excluding data-URL payloads from the digest input. */
export function mediaHash(part: MediaFilePart, path: string) {
  const urlHashInput = isDataURL(part.url) ? "data-image" : part.url;
  try {
    const stat = lstatSync(path);
    const real = stat.isFile() ? realpathSync(path) : path;
    return sha256(`${real}:${stat.size}:${stat.mtimeMs}:${urlHashInput}`);
  } catch {
    return sha256(`${path}:${urlHashInput}`);
  }
}

/** Creates and returns the private media cache directory; filesystem errors throw. */
export function cacheDir() {
  const dir = join(runtimeDir(), "cache");
  mkdirSync(dir, { recursive: true, mode: 0o700 });
  return dir;
}

/** Locates the media runtime directory under `$XDG_RUNTIME_DIR` or the system temp directory. */
export function runtimeDir() {
  const base = process.env.XDG_RUNTIME_DIR || join(tmpdir(), `opencode-${process.getuid?.() ?? "user"}`);
  return join(base, "opencode", "media-context");
}

/** Labels a media part's source for its registry row. */
export function mediaSourceLabel(part: MediaFilePart) {
  if (sourcePath(part)) return part.source?.type === "clipboard" ? "clipboard" : "local source";
  if (filePathFromURL(part.url)) return "file URL";
  if (parseDataImage(part.url)) return "clipboard data image";
  return "unsupported media source";
}

// ├─ MIME classification ─────────────────────────────────────────────────────────────────────────┤

export function mediaKindForMime(mime: string | undefined): MediaKind | undefined {
  if (isImageMime(mime)) return "image";
  if (isVideoMime(mime)) return "video";
  return undefined;
}

export function isImageMime(mime: string | undefined) {
  return Boolean(mime?.startsWith("image/"));
}

export function isVideoMime(mime: string | undefined) {
  return Boolean(mime?.startsWith("video/"));
}

/** Maps media MIME types to extensions, defaulting unknown videos to `.mp4`. */
export function extensionForMime(mime: string, kind: MediaKind) {
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

export function sha256(value: string | Buffer) {
  return createHash("sha256").update(value).digest("hex");
}

// ├─ Resolution helpers ──────────────────────────────────────────────────────────────────────────┤

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

function isUnderAllowedVideoRoot(path: string) {
  if (!path.startsWith("/")) return false;
  const normalized = normalize(path);
  return allowedVideoRoots().some((root) => normalized.startsWith(root));
}

function withTrailingSlash(value: string) {
  return value.endsWith("/") ? value : `${value}/`;
}

function hasGlob(path: string) {
  return /[*?[\]]/.test(path);
}

// ├─ Video candidates ────────────────────────────────────────────────────────────────────────────┤

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

function videoMime(path: string) {
  return VIDEO_MIME_BY_EXTENSION.get(extname(path).toLowerCase());
}

// ├─ Data images ─────────────────────────────────────────────────────────────────────────────────┤

const MAX_DATA_IMAGE_BYTES = 2 * 1024 * 1024;

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
