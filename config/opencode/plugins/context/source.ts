import { realpathSync } from "node:fs";
import path from "node:path";

export const configRoot = path.resolve(import.meta.dirname, "..", "..");

const rootFiles = ["AGENTS.md", "COLLAB.md", "ROUTING.md"];
const maxLabel = 36;

export type Kind = "readme" | "agents" | "agent" | "principle" | "skill" | "command" | "partial" | "markdown";

export function isMarkdown(value: string) {
  return /\.(md|mdx|markdown)$/i.test(value.split(/[?#]/, 1)[0]);
}

export function kindOf(filePath: string): Kind {
  const normalized = path.normalize(filePath);
  const leaf = path.basename(normalized).toLowerCase();
  if (leaf === "readme.md") return "readme";
  if (leaf === "agents.md" || isRootFile(normalized)) return "agents";
  if (leaf === "skill.md") return isPrinciple(normalized) ? "principle" : "skill";
  if (segmentsAfter(normalized, "agents", "agent")) return "agent";
  if (segmentsAfter(normalized, "commands", "command")) return "command";
  if (/^[A-Z][A-Z0-9_-]*\.md$/.test(path.basename(normalized))) return "partial";
  return "markdown";
}

const identities = new Map<string, string>();

export function identity(filePath: string) {
  const normalized = path.normalize(filePath);
  const cached = identities.get(normalized);
  if (cached) return cached;
  let resolved = normalized;
  try {
    resolved = realpathSync.native(normalized);
  } catch {
    return normalized;
  }
  identities.set(normalized, resolved);
  return resolved;
}

export function isGlobal(filePath: string) {
  const file = identity(filePath);
  const root = identity(configRoot);
  return file === root || file.startsWith(root + path.sep);
}

export function isBuiltin(filePath: string) {
  return path.dirname(path.normalize(filePath)) === "/builtin";
}

export function isConfigAgents(filePath: string) {
  if (isRootFile(filePath)) return true;
  return path.basename(filePath).toLowerCase() === "agents.md" && isGlobal(filePath);
}

export function isSubagent(filePath: string) {
  return (segmentsAfter(filePath, "agents", "agent")?.length ?? 0) > 1;
}

export function label(roots: readonly string[], filePath: string, kind: Kind) {
  return compact(rawLabel(roots, filePath, kind));
}

function rawLabel(roots: readonly string[], filePath: string, kind: Kind) {
  if (kind === "agent") return agentLabel(filePath);
  if (kind === "principle") return path.basename(path.dirname(filePath));
  if (kind === "skill") return ownedLabel(roots, filePath, skillName(filePath), "skills", "skill");
  if (kind === "command") return ownedLabel(roots, filePath, commandName(filePath), "commands", "command");
  const relative = relativePath(roots, filePath);
  if (kind === "readme" || kind === "agents") {
    if (kind === "agents" && isConfigAgents(filePath)) return configLabel(filePath);
    const dir = path.dirname(relative);
    return dir === "." ? rootName(roots, filePath) : dir;
  }
  return stripExtension(relative);
}

function isRootFile(filePath: string) {
  const id = identity(filePath);
  return rootFiles.some((file) => id === identity(path.join(configRoot, file)));
}

function isPrinciple(filePath: string) {
  const parts = filePath.split(path.sep);
  return parts.at(-3) === "principles" && parts.at(-4) === "skills";
}

function segmentsAfter(filePath: string, ...names: string[]) {
  const parts = path.normalize(filePath).split(path.sep).filter(Boolean);
  const index = parts.findIndex((part) => names.includes(part));
  if (index === -1) return undefined;
  const rest = parts.slice(index + 1);
  if (rest.length === 0 || !isMarkdown(rest.at(-1) ?? "")) return undefined;
  return rest;
}

function title(value: string) {
  return value ? value.charAt(0).toUpperCase() + value.slice(1) : value;
}

function stripExtension(value: string) {
  return value.replace(/\.(md|mdx|markdown)$/i, "");
}

function configLabel(filePath: string) {
  const leaf = path.basename(filePath);
  if (leaf.toLowerCase() === "agents.md") return "OpenCode";
  return title(stripExtension(leaf).toLowerCase());
}

function agentLabel(filePath: string) {
  const rest = segmentsAfter(filePath, "agents", "agent");
  if (!rest) return stripExtension(path.basename(filePath));
  return stripExtension(rest.join("/")).split("/").map(title).join("/");
}

function skillName(filePath: string) {
  const parent = path.basename(path.dirname(filePath));
  if (!parent || parent === "." || parent === "skills" || parent === "skill") return "Skill";
  return parent.split("-").map(title).join("-");
}

function commandName(filePath: string) {
  const rest = segmentsAfter(filePath, "commands", "command");
  if (!rest) return "Command";
  return stripExtension(rest.join("/"))
    .split("/")
    .map((segment) => segment.split("-").map(title).join("-"))
    .join("/");
}

function ownedLabel(roots: readonly string[], filePath: string, name: string, ...dirs: string[]) {
  if (isGlobal(filePath)) return name;
  const parts = path.normalize(filePath).split(path.sep).filter(Boolean);
  let index = parts.findLastIndex((part) => dirs.includes(part)) - 1;
  if (parts[index] === ".opencode" || parts[index] === "opencode") index -= 1;
  const owner = index >= 0 ? parts[index].replace(/^\./, "") : "";
  return `${owner || rootName(roots, filePath)}/${name}`;
}

function rootName(roots: readonly string[], filePath: string) {
  const root = roots[0] || path.dirname(filePath);
  return path.basename(root) || path.basename(path.dirname(filePath)) || path.basename(filePath);
}

function relativePath(roots: readonly string[], filePath: string) {
  const normalized = path.normalize(filePath);
  const resolved = path.isAbsolute(normalized) || !roots[0] ? normalized : path.join(roots[0], normalized);
  const home = process.env.HOME ? path.normalize(process.env.HOME) : "";
  const relative = within(roots, resolved) ?? within([home], resolved) ?? resolved;
  const parts = relative.split(path.sep).filter(Boolean);
  const worktree = parts.lastIndexOf(".worktrees");
  if (worktree !== -1 && parts.length > worktree + 1) parts.splice(worktree, 2);
  if (parts[0] && roots.some((root) => path.basename(root) === parts[0])) parts.shift();
  return parts.join(path.sep) || relative;
}

function within(roots: readonly string[], resolved: string) {
  let best: string | undefined;
  let length = -1;
  for (const root of roots) {
    if (!root || root === path.parse(root).root) continue;
    const candidate = path.relative(root, resolved);
    if (candidate === ".." || candidate.startsWith(`..${path.sep}`) || path.isAbsolute(candidate)) continue;
    if (root.length < length) continue;
    length = root.length;
    best = candidate;
  }
  return best;
}

function compact(value: string) {
  const parts = value.split(path.sep).filter(Boolean);
  if (parts.length <= 2) return value.length <= maxLabel ? value : `${value.slice(0, maxLabel - 3)}...`;
  const leaf = parts.at(-1) ?? value;
  const parent = parts.at(-2) ?? "";
  if (leaf.length >= maxLabel) return shortenFile(leaf);
  const joined = `${parent}/${leaf}`;
  if (joined.length <= maxLabel) return joined.length + 4 <= maxLabel ? `.../${joined}` : joined;
  const marked = maxLabel - leaf.length - 5;
  if (marked > 0) return `.../${middle(parent, marked)}/${leaf}`;
  const bare = maxLabel - leaf.length - 1;
  return bare > 0 ? `${middle(parent, bare)}/${leaf}` : leaf;
}

function shortenFile(value: string) {
  if (value.length <= maxLabel) return value;
  const ext = path.extname(value);
  if (ext.length > 1 && maxLabel > ext.length) return `${value.slice(0, Math.min(3, maxLabel - ext.length))}${ext}`;
  return middle(value, maxLabel);
}

function middle(value: string, max: number) {
  if (value.length <= max) return value;
  if (max <= 3) return ".".repeat(max);
  return `${value.slice(0, Math.ceil((max - 3) / 2))}...${value.slice(value.length - Math.floor((max - 3) / 2))}`;
}
