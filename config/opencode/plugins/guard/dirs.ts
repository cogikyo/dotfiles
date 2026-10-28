import { existsSync, readFileSync } from "node:fs";
import path from "node:path";
import { home, tilde } from "./paths.ts";

const defaults: Record<string, string> = {
  DESKTOP: "Desktop",
  DOCUMENTS: "Documents",
  DOWNLOAD: "Downloads",
  MUSIC: "Music",
  PICTURES: "Pictures",
  PUBLICSHARE: "Public",
  TEMPLATES: "Templates",
  VIDEOS: "Videos",
};
const entry = /^XDG_([A-Z]+)_DIR="([^"]*)"/gmu;
const exempt: Record<string, readonly string[] | "all"> = {
  edit: ["oldString"],
  grep: ["pattern"],
  sessions: "all",
};
const prefix = ["~", "\\$HOME", "\\$\\{HOME\\}", RegExp.escape(home)].join("|");

type Dir = { name: string; target: string };

function config() {
  const base = process.env.XDG_CONFIG_HOME;
  return path.join(base && path.isAbsolute(base) ? base : path.join(home, ".config"), "user-dirs.dirs");
}

function load(file: string): Dir[] {
  let source: string;
  try {
    source = readFileSync(file, "utf8");
  } catch {
    // A config read failure must not block every tool call.
    return [];
  }
  return [...source.matchAll(entry)].flatMap(([, name, value]) => {
    const target = value.replace(/^\$HOME(?=\/|$)/u, home);
    return path.isAbsolute(target) ? [{ name, target: path.resolve(target) }] : [];
  });
}

function guide(dirs: Dir[]) {
  const groups = new Map<string, string[]>();
  for (const { name, target } of dirs) {
    if (target === home) continue;
    const label = (defaults[name] ?? name).toLowerCase();
    groups.set(target, [...(groups.get(target) ?? []), label]);
  }
  return [...groups].map(([target, labels]) => `${labels.join("/")} ${tilde(target)}`).join(", ");
}

function strings(value: unknown, skip: readonly string[] = []): string[] {
  if (typeof value === "string") return [value];
  if (Array.isArray(value)) return value.flatMap((item) => strings(item));
  if (typeof value !== "object" || value === null) return [];
  return Object.entries(value).flatMap(([key, item]) => (skip.includes(key) ? [] : strings(item)));
}

function changed(line: string) {
  if (line.trim().startsWith("***")) return true;
  const marker = line.trimEnd();
  if (marker === "@@" || marker.startsWith("@@ ")) return false;
  return !line.startsWith("-") && !line.startsWith(" ");
}

// Keep stale references searchable and removable.
function texts(tool: string, input: unknown) {
  const skip = exempt[tool] ?? [];
  if (skip === "all") return [];
  const found = strings(input, skip);
  if (tool !== "patch") return found;
  return found.map((text) => text.split("\n").filter(changed).join("\n"));
}

export function relocated(tool: string, input: unknown) {
  const file = config();
  const dirs = load(file);
  const guarded = dirs.flatMap(({ name, target }) => {
    const dir = defaults[name];
    if (!dir) return [];
    const standard = path.join(home, dir);
    // Existing defaults stay usable for cleanup moves or deliberate user choices.
    return target === standard || existsSync(standard) ? [] : [dir];
  });
  if (guarded.length === 0) return undefined;
  const pattern = new RegExp(`(?<![\\w./-])(?:${prefix})/(?:${guarded.join("|")})(?![\\w-]|\\.\\w)`, "u");
  for (const text of texts(tool, input)) {
    const match = pattern.exec(text);
    if (!match) continue;
    return `\`${match[0]}\` is not a user dir on this machine; look up user dirs in ${tilde(file)} before choosing a location: ${guide(dirs)}.`;
  }
  return undefined;
}
