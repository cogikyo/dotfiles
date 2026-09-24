import type { TuiPluginApi } from "@opencode-ai/plugin/tui";
import { realpathSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Markdown path labels for the TUI context sidebar                                              │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

const MAX_LABEL_LENGTH = 36;

export type MarkdownSourceKind = "readme" | "agents" | "agent" | "skill" | "command" | "partial" | "spec" | "markdown";

export const configRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");

// ├─ Kinds ───────────────────────────────────────────────────────────────────────────────────────┤

/** Matches Markdown extensions regardless of case, query, or hash. */
export function isMarkdownPath(value: string) {
  return /\.(md|mdx|markdown)$/i.test(value.split(/[?#]/, 1)[0]);
}

/** Classifies a Markdown path for sidebar display, giving `.spec` paths priority. */
export function markdownSourceKind(filePath: string): MarkdownSourceKind {
  const normalizedPath = path.normalize(filePath);
  const leaf = path.basename(normalizedPath).toLowerCase();

  if (normalizedPath.split(/[\\/]/u).includes(".spec")) return "spec";
  if (leaf === "readme.md") return "readme";
  if (leaf === "agents.md") return "agents";
  if (leaf === "skill.md") return "skill";
  if (agentSegments(normalizedPath)) return "agent";
  if (commandSegments(normalizedPath)) return "command";
  if (/^[A-Z][A-Z0-9_-]*\.md$/.test(path.basename(normalizedPath))) return "partial";
  return "markdown";
}

export function isSubagent(filePath: string) {
  return (agentSegments(filePath)?.length ?? 0) > 1;
}

function agentSegments(filePath: string) {
  const parts = path.normalize(filePath).split(/[\\/]/u).filter(Boolean);
  const index = parts.findIndex((part) => part === "agents" || part === "agent");
  if (index === -1) return undefined;
  const rest = parts.slice(index + 1);
  if (rest.length === 0 || !isMarkdownPath(rest.at(-1) ?? "")) return undefined;
  return rest;
}

function commandSegments(filePath: string) {
  const parts = path.normalize(filePath).split(/[\\/]/u).filter(Boolean);
  const index = parts.findIndex((part) => part === "commands" || part === "command");
  if (index === -1) return undefined;
  const rest = parts.slice(index + 1);
  if (rest.length === 0 || !isMarkdownPath(rest.at(-1) ?? "")) return undefined;
  return rest;
}

// ├─ Labels ──────────────────────────────────────────────────────────────────────────────────────┤

/** Formats a Markdown path as a sidebar label of at most 36 characters. */
export function displayPath(api: TuiPluginApi, filePath: string, kind: MarkdownSourceKind) {
  return compactPath(contextLabel(api, filePath, kind));
}

function contextLabel(api: TuiPluginApi, filePath: string, kind: MarkdownSourceKind) {
  const label = relativePath(api, filePath);

  if (kind === "spec") return specLabel(filePath);
  if (kind === "agent") return agentLabel(filePath);
  if (kind === "skill") return skillLabel(api, filePath);
  if (kind === "command") return commandLabel(api, filePath);

  if (kind === "readme" || kind === "agents") {
    if (kind === "agents" && isConfigAgents(filePath)) return "OpenCode";
    const dir = path.dirname(label);
    return dir === "." ? contextRootName(api, filePath) : dir;
  }

  return stripMarkdownExtension(label);
}

function agentLabel(filePath: string) {
  const rest = agentSegments(filePath);
  if (!rest) return stripMarkdownExtension(path.basename(filePath));
  return stripMarkdownExtension(rest.join("/")).split("/").map(titleSegment).join("/");
}

function commandLabel(api: TuiPluginApi, filePath: string) {
  const name = commandName(filePath);
  if (isGlobalOpencodePath(filePath)) return name;
  return `${commandProjectOwner(api, filePath)}/${name}`;
}

function titleSegment(value: string) {
  return value ? value.charAt(0).toUpperCase() + value.slice(1) : value;
}

function commandName(filePath: string) {
  const rest = commandSegments(filePath);
  if (!rest) return "Command";
  return stripMarkdownExtension(rest.join("/"))
    .split("/")
    .map((segment) => segment.split("-").map(titleSegment).join("-"))
    .join("/");
}

function commandProjectOwner(api: TuiPluginApi, filePath: string) {
  const parts = path.normalize(filePath).split(/[\\/]/u).filter(Boolean);
  const commandsIndex = parts.reduce(
    (found, part, index) => (part === "commands" || part === "command" ? index : found),
    -1,
  );
  if (commandsIndex > 0) {
    let ownerIndex = commandsIndex - 1;
    if (parts[ownerIndex] === ".opencode" || parts[ownerIndex] === "opencode") ownerIndex -= 1;
    const owner = parts[ownerIndex];
    if (owner) return owner.replace(/^\./, "");
  }
  return contextRootName(api, filePath);
}

function skillName(filePath: string) {
  const parent = path.basename(path.dirname(filePath));
  if (!parent || parent === "." || parent === "skills" || parent === "skill") return "Skill";
  return parent.split("-").map(titleSegment).join("-");
}

function skillProjectOwner(api: TuiPluginApi, filePath: string) {
  const parts = path.normalize(filePath).split(/[\\/]/u).filter(Boolean);
  const skillsIndex = parts.reduce((found, part, index) => (part === "skills" || part === "skill" ? index : found), -1);
  if (skillsIndex > 0) {
    let ownerIndex = skillsIndex - 1;
    if (parts[ownerIndex] === ".opencode" || parts[ownerIndex] === "opencode") ownerIndex -= 1;
    const owner = parts[ownerIndex];
    if (owner) return owner.replace(/^\./, "");
  }
  return contextRootName(api, filePath);
}

function skillLabel(api: TuiPluginApi, filePath: string) {
  const name = skillName(filePath);
  if (isGlobalOpencodePath(filePath)) return name;
  return `${skillProjectOwner(api, filePath)}/${name}`;
}

function specLabel(filePath: string) {
  const parts = path.normalize(filePath).split(/[\\/]/u).filter(Boolean);
  const specIndex = parts.lastIndexOf(".spec");
  const owner = parts[specIndex - 1];
  const nestedPath = parts.slice(specIndex + 1);

  return stripMarkdownExtension([owner, ...nestedPath].filter(Boolean).join(path.sep));
}

function primaryProjectRoot(api: TuiPluginApi) {
  return projectRoots(api)[0] || "";
}

function contextRootName(api: TuiPluginApi, filePath: string) {
  const root = primaryProjectRoot(api) || path.dirname(filePath);
  return path.basename(root) || path.basename(path.dirname(filePath)) || path.basename(filePath);
}

function stripMarkdownExtension(label: string) {
  return label.replace(/\.(md|mdx|markdown)$/i, "");
}

// ├─ Paths ───────────────────────────────────────────────────────────────────────────────────────┤

/** Resolves an existing file's identity, falling back to its normalized path. */
export function markdownIdentity(filePath: string) {
  const normalizedPath = path.normalize(filePath);
  try {
    return realpathSync.native(normalizedPath);
  } catch {
    return normalizedPath;
  }
}

export function isGlobalOpencodePath(filePath: string) {
  const file = markdownIdentity(filePath);
  const root = markdownIdentity(configRoot);
  return file === root || file.startsWith(root + path.sep);
}

/** Returns distinct usable project roots, with the session directory first. */
export function projectRoots(api: TuiPluginApi) {
  const roots: string[] = [];
  const seen = new Set<string>();
  for (const value of [api.state.path.directory, api.state.path.worktree]) {
    if (!value || isFilesystemRoot(value)) continue;
    const normalized = path.normalize(value);
    if (seen.has(normalized)) continue;
    seen.add(normalized);
    roots.push(normalized);
  }
  return roots;
}

/** Shortens a long label around `...` while preserving both ends. */
export function truncateMiddle(value: string, maxLength: number) {
  if (maxLength <= 0) return "";
  if (value.length <= maxLength) return value;
  if (maxLength <= 3) return ".".repeat(maxLength);

  const headLength = Math.ceil((maxLength - 3) / 2);
  const tailLength = Math.floor((maxLength - 3) / 2);
  return `${value.slice(0, headLength)}...${value.slice(value.length - tailLength)}`;
}

export function isConfigAgents(filePath: string) {
  return path.basename(filePath).toLowerCase() === "agents.md" && isGlobalOpencodePath(filePath);
}

function relativeInside(candidate: string) {
  return candidate !== ".." && !candidate.startsWith(`..${path.sep}`) && !path.isAbsolute(candidate);
}

function relativePath(api: TuiPluginApi, filePath: string) {
  const directory = api.state.path.directory ? path.normalize(api.state.path.directory) : "";
  const worktree = api.state.path.worktree ? path.normalize(api.state.path.worktree) : "";
  const normalized = path.normalize(filePath);
  const base = directory || worktree;
  const resolved = path.isAbsolute(normalized) || !base ? normalized : path.normalize(path.join(base, normalized));
  const home = process.env.HOME ? path.normalize(process.env.HOME) : "";
  const relative = rootRelative([directory, worktree], resolved) ?? rootRelative([home], resolved) ?? resolved;

  const parts = relative.split(path.sep).filter(Boolean);
  const worktreeIndex = parts.lastIndexOf(".worktrees");
  if (worktreeIndex !== -1 && parts.length > worktreeIndex + 1) parts.splice(worktreeIndex, 2);
  const leaked = [worktree, directory].filter(Boolean).map((root) => path.basename(root));
  if (parts[0] && leaked.includes(parts[0])) parts.shift();
  return parts.join(path.sep) || relative;
}

function rootRelative(roots: string[], resolved: string) {
  let relative: string | undefined;
  let matchLength = -1;
  for (const root of roots) {
    if (!root || isFilesystemRoot(root)) continue;
    const candidate = path.relative(root, resolved);
    if (!relativeInside(candidate)) continue;
    if (root.length < matchLength) continue;
    matchLength = root.length;
    relative = candidate;
  }
  return relative;
}

function isFilesystemRoot(value: string) {
  const normalized = path.normalize(value);
  return normalized === path.parse(normalized).root;
}

function compactPath(label: string) {
  const parts = label.split(path.sep).filter(Boolean);
  if (parts.length <= 2) return label.length <= MAX_LABEL_LENGTH ? label : truncateLabel(label);

  const leaf = parts.at(-1) ?? label;
  const parent = parts.at(-2) ?? "";
  if (leaf.length >= MAX_LABEL_LENGTH) return truncateFileName(leaf, MAX_LABEL_LENGTH);

  const joined = `${parent}/${leaf}`;
  if (joined.length <= MAX_LABEL_LENGTH) {
    const marked = `.../${joined}`;
    return marked.length <= MAX_LABEL_LENGTH ? marked : joined;
  }

  const reserved = leaf.length + 1;
  const markedBudget = MAX_LABEL_LENGTH - reserved - 4;
  if (markedBudget > 0) return `.../${truncateMiddle(parent, markedBudget)}/${leaf}`;
  const parentBudget = MAX_LABEL_LENGTH - reserved;
  if (parentBudget > 0) return `${truncateMiddle(parent, parentBudget)}/${leaf}`;
  return leaf;
}

function truncateLabel(label: string) {
  if (label.length <= MAX_LABEL_LENGTH) return label;
  return `${label.slice(0, Math.max(0, MAX_LABEL_LENGTH - 3))}...`;
}

function truncateFileName(value: string, maxLength: number) {
  if (value.length <= maxLength) return value;

  const ext = path.extname(value);
  if (ext.length > 1 && maxLength > ext.length) {
    const stemLength = Math.min(3, maxLength - ext.length);
    return `${value.slice(0, stemLength)}${ext}`;
  }

  return truncateMiddle(value, maxLength);
}
