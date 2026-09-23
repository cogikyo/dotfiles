/** @jsxImportSource @opentui/solid */
import type { ToolPart } from "@opencode-ai/sdk/v2";
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from "@opencode-ai/plugin/tui";
import { existsSync, readFileSync, realpathSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { For, Show, createSignal, onCleanup } from "solid-js";
import { colors } from "../shared/colors.ts";
import { icons } from "../shared/icons.ts";
import { openInNvim } from "../shared/open-nvim.ts";
import {
  isProtectedMarkdownPath,
  persistCompactedToolParts,
  persistUpdatedPart,
  withReloadedOutput,
  type ProtectRoots,
  type SkillToolPart,
} from "./skill-parts.ts";
import { SidebarSection } from "../shared/sidebar-section.tsx";

const id = "opencode-markdown-context";
const MAX_LABEL_LENGTH = 36;

type MarkdownSourceKind = "readme" | "agents" | "agent" | "skill" | "command" | "partial" | "spec" | "markdown";

const configRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");

type PartRef = {
  messageID: string;
  partID: string;
};

type MarkdownContextItem = {
  key: string;
  path: string;
  label: string;
  kind: MarkdownSourceKind;
  compacted: boolean;
  pinned: boolean;
  time: number;
  refs: PartRef[];
};

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ SIDEBAR AND EVENTS                                                                            │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

function MarkdownContext(props: { api: TuiPluginApi; sessionID: string }) {
  const [revision, setRevision] = createSignal(0);
  const refresh = () => setRevision((value) => value + 1);

  const disposers = [
    props.api.event.on("message.updated", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("message.removed", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("message.part.updated", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("message.part.removed", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("session.compacted", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
  ];
  onCleanup(() => {
    for (const dispose of disposers) dispose();
  });

  const items = () => {
    revision();
    return markdownContextItems(props.api, props.sessionID);
  };

  return (
    <Show when={items().length > 0}>
      <SidebarSection api={props.api} title="Markdown Context" detail={`${items().length} read`}>
        <For each={items()}>
          {(item) => (
            <box flexDirection="row" gap={0}>
              <Show when={canUnload(props.api, props.sessionID, item)}>
                <text
                  fg={props.api.theme.current.textMuted}
                  wrapMode="none"
                  onMouseDown={() => void unloadItem(props.api, props.sessionID, item)}
                >
                  {`${icons.error} `}
                </text>
              </Show>
              <Show when={canReload(item)}>
                <text
                  fg={props.api.theme.current.textMuted}
                  wrapMode="none"
                  onMouseDown={() => void reloadItem(props.api, props.sessionID, item)}
                >
                  {`${icons.restore} `}
                </text>
              </Show>
              <box
                flexDirection="row"
                gap={0}
                onMouseDown={() => openInNvim(props.api, item.path, "Markdown open failed")}
              >
                <text fg={sourceColor(props.api, item)} wrapMode="none">
                  {sourceIcon(props.api, item)}
                </text>
                <text fg={props.api.theme.current.textMuted} wrapMode="none">
                  {item.label}
                </text>
              </box>
            </box>
          )}
        </For>
      </SidebarSection>
    </Show>
  );
}

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ CONTEXT ITEMS                                                                                 │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

function markdownContextItems(api: TuiPluginApi, sessionID: string) {
  const pinned = pinnedContextItems(api, sessionID);
  const reads = new Map<string, MarkdownContextItem>();
  const messages = api.state.session.messages(sessionID);

  for (const message of messages) {
    for (const part of api.state.part(message.id)) {
      const item = skillToolItem(api, part) ?? markdownReadItem(api, part);
      if (!item) continue;

      const pin = pinned.find((entry) => entry.key === item.key);
      if (pin) {
        pin.refs.push(...item.refs);
        pin.compacted = pin.compacted || item.compacted;
        continue;
      }

      const existing = reads.get(item.key);
      if (existing) {
        existing.refs.push(...item.refs);
        if (item.time >= existing.time) {
          existing.time = item.time;
          existing.path = item.path;
          existing.label = item.label;
        }
        existing.compacted = existing.compacted && item.compacted;
        continue;
      }
      reads.set(item.key, item);
    }
  }

  return [...pinned, ...Array.from(reads.values()).toSorted((left, right) => right.time - left.time)];
}

function pinnedContextItems(api: TuiPluginApi, sessionID: string) {
  const items: MarkdownContextItem[] = [];
  const seen = new Set<string>();

  const push = (filePath: string) => {
    if (!existsSync(filePath)) return;
    const item = markdownFileItem(api, filePath, 0, false, [], true);
    if (seen.has(item.key)) return;
    seen.add(item.key);
    items.push(item);
  };

  push(path.join(configRoot, "AGENTS.md"));
  for (const root of projectRoots(api)) {
    push(path.join(root, "AGENTS.md"));
  }

  const agent = currentAgent(api, sessionID);
  if (agent) push(path.join(configRoot, "agents", `${agent}.md`));

  return items;
}

function currentAgent(api: TuiPluginApi, sessionID: string) {
  const messages = api.state.session.messages(sessionID);
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if ("agent" in message && typeof message.agent === "string" && message.agent) return message.agent;
  }
  return undefined;
}

function markdownReadItem(
  api: TuiPluginApi,
  part: ReturnType<TuiPluginApi["state"]["part"]>[number],
): MarkdownContextItem | undefined {
  if (part.type !== "tool" || !isReadTool(part.tool)) return undefined;
  if (part.state.status !== "completed") return undefined;

  const filePath = markdownPathFromInput(part.state.input);
  if (!filePath) return undefined;
  return markdownFileItem(api, filePath, part.state.time.end, part.state.time.compacted !== undefined, [
    { messageID: part.messageID, partID: part.id },
  ]);
}

function skillToolItem(
  api: TuiPluginApi,
  part: ReturnType<TuiPluginApi["state"]["part"]>[number],
): MarkdownContextItem | undefined {
  if (part.type !== "tool" || !isSkillTool(part.tool)) return undefined;
  if (part.state.status !== "completed") return undefined;

  const filePath = skillPathFromTool(part);
  if (!filePath) return undefined;
  return markdownFileItem(api, filePath, part.state.time.end, part.state.time.compacted !== undefined, [
    { messageID: part.messageID, partID: part.id },
  ]);
}

function skillPathFromTool(tool: ToolPart) {
  if (tool.state.status !== "completed") return undefined;
  const dir = tool.state.metadata.dir;
  if (typeof dir === "string" && dir) {
    const filePath = path.join(dir, "SKILL.md");
    if (existsSync(filePath)) return filePath;
  }

  const name = tool.state.input.name;
  if (typeof name !== "string" || !name) return undefined;
  const filePath = path.join(configRoot, "skills", name, "SKILL.md");
  return existsSync(filePath) ? filePath : undefined;
}

function markdownFileItem(
  api: TuiPluginApi,
  filePath: string,
  time: number,
  compacted: boolean,
  refs: PartRef[] = [],
  pinned = false,
): MarkdownContextItem {
  const kind = markdownSourceKind(filePath);
  return {
    key: markdownIdentity(filePath),
    path: filePath,
    label: displayPath(api, filePath, kind),
    kind,
    compacted,
    pinned,
    time,
    refs,
  };
}

function protectRoots(api: TuiPluginApi, sessionID: string): ProtectRoots {
  const agent = currentAgent(api, sessionID);
  return {
    configRoot,
    projectRoots: projectRoots(api),
    agentNames: ["collab", agent].filter((name): name is string => Boolean(name)),
  };
}

function canUnload(api: TuiPluginApi, sessionID: string, item: MarkdownContextItem) {
  if (item.pinned || item.compacted || item.refs.length === 0) return false;
  return !isProtectedMarkdownPath(item.path, protectRoots(api, sessionID));
}

function canReload(item: MarkdownContextItem) {
  return item.compacted && item.refs.length > 0;
}

async function unloadItem(api: TuiPluginApi, sessionID: string, item: MarkdownContextItem) {
  const ids = new Set(item.refs.map((ref) => `${ref.messageID}:${ref.partID}`));
  const parts: SkillToolPart[] = [];
  const messages = api.state.session.messages(sessionID);

  for (const message of messages) {
    for (const part of api.state.part(message.id)) {
      if (part.type !== "tool" || part.state.status !== "completed" || part.state.time.compacted !== undefined)
        continue;
      if (!ids.has(`${part.messageID}:${part.id}`)) continue;
      parts.push(part);
    }
  }

  try {
    await persistCompactedToolParts(api.client, parts);
  } catch (error) {
    api.ui.toast({
      variant: "warning",
      title: "Context unload failed",
      message: error instanceof Error ? error.message : item.label,
    });
  }
}

async function reloadItem(api: TuiPluginApi, sessionID: string, item: MarkdownContextItem) {
  let output: string;
  try {
    output = numberedRead(readFileSync(item.path, "utf8"));
  } catch (error) {
    api.ui.toast({
      variant: "warning",
      title: "Context reload failed",
      message: error instanceof Error ? error.message : item.label,
    });
    return;
  }

  const ids = new Set(item.refs.map((ref) => `${ref.messageID}:${ref.partID}`));
  const parts: SkillToolPart[] = [];
  const messages = api.state.session.messages(sessionID);

  for (const message of messages) {
    for (const part of api.state.part(message.id)) {
      if (part.type !== "tool" || part.state.status !== "completed") continue;
      if (!ids.has(`${part.messageID}:${part.id}`)) continue;
      parts.push(withReloadedOutput(part as SkillToolPart, output));
    }
  }

  try {
    for (const part of parts) await persistUpdatedPart(api.client, part);
  } catch (error) {
    api.ui.toast({
      variant: "warning",
      title: "Context reload failed",
      message: error instanceof Error ? error.message : item.label,
    });
  }
}

function numberedRead(content: string) {
  const lines = content.split("\n");
  const width = Math.max(6, String(lines.length).length);
  return lines.map((line, index) => `${String(index + 1).padStart(width)}| ${line}`).join("\n");
}

function markdownPathFromInput(input: Record<string, unknown>) {
  for (const key of ["filePath", "path", "filepath", "file"]) {
    const value = input[key];
    if (typeof value === "string" && isMarkdownPath(value)) return normalizeFilePath(value);
  }

  for (const value of Object.values(input)) {
    if (typeof value === "string" && isMarkdownPath(value)) return normalizeFilePath(value);
  }

  return undefined;
}

function isReadTool(tool: string) {
  return tool === "read" || tool === "Read" || tool === "file.read" || tool === "file_read";
}

function isSkillTool(tool: string) {
  return tool === "skill" || tool === "Skill";
}

function isMarkdownPath(value: string) {
  return /\.(md|mdx|markdown)$/i.test(value.split(/[?#]/, 1)[0]);
}

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ PATH CLASSIFICATION                                                                           │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

function markdownSourceKind(filePath: string): MarkdownSourceKind {
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

function agentSegments(filePath: string) {
  const parts = path.normalize(filePath).split(/[\\/]/u).filter(Boolean);
  const index = parts.findIndex((part) => part === "agents" || part === "agent");
  if (index === -1) return undefined;
  const rest = parts.slice(index + 1);
  if (rest.length === 0 || !isMarkdownPath(rest.at(-1) ?? "")) return undefined;
  return rest;
}

function agentLabel(filePath: string) {
  const rest = agentSegments(filePath);
  if (!rest) return stripMarkdownExtension(path.basename(filePath));
  return stripMarkdownExtension(rest.join("/")).split("/").map(titleSegment).join("/");
}

function isSubagent(filePath: string) {
  return (agentSegments(filePath)?.length ?? 0) > 1;
}

function commandSegments(filePath: string) {
  const parts = path.normalize(filePath).split(/[\\/]/u).filter(Boolean);
  const index = parts.findIndex((part) => part === "commands" || part === "command");
  if (index === -1) return undefined;
  const rest = parts.slice(index + 1);
  if (rest.length === 0 || !isMarkdownPath(rest.at(-1) ?? "")) return undefined;
  return rest;
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

function commandLabel(api: TuiPluginApi, filePath: string) {
  const name = commandName(filePath);
  if (isGlobalOpencodePath(filePath)) return name;
  return `${commandProjectOwner(api, filePath)}/${name}`;
}

function titleSegment(value: string) {
  return value ? value.charAt(0).toUpperCase() + value.slice(1) : value;
}

function isGlobalOpencodePath(filePath: string) {
  const file = markdownIdentity(filePath);
  const root = markdownIdentity(configRoot);
  return file === root || file.startsWith(root + path.sep);
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

function normalizeFilePath(value: string) {
  const clean = value.replace(/^file:\/\//, "").split(/[?#]/, 1)[0];
  if (clean.startsWith("~/")) return path.join(process.env.HOME || "~", clean.slice(2));
  return clean;
}

function markdownIdentity(filePath: string) {
  const normalizedPath = path.normalize(filePath);
  try {
    return realpathSync.native(normalizedPath);
  } catch {
    return normalizedPath;
  }
}

function displayPath(api: TuiPluginApi, filePath: string, kind: MarkdownSourceKind) {
  return compactPath(contextLabel(api, filePath, kind));
}

function relativeInside(candidate: string) {
  return candidate !== ".." && !candidate.startsWith(`..${path.sep}`) && !path.isAbsolute(candidate);
}

function relativePath(api: TuiPluginApi, filePath: string) {
  const directory = api.state.path.directory ? path.normalize(api.state.path.directory) : "";
  const worktree = api.state.path.worktree ? path.normalize(api.state.path.worktree) : "";
  const normalized = path.normalize(filePath);
  const base = directory || worktree;
  const resolved = path.isAbsolute(normalized)
    ? normalized
    : base
      ? path.normalize(path.join(base, normalized))
      : normalized;

  let relative: string | undefined;
  let matchLength = -1;
  for (const root of [directory, worktree]) {
    if (!root || isFilesystemRoot(root)) continue;
    const candidate = path.relative(root, resolved);
    if (!relativeInside(candidate)) continue;
    if (root.length < matchLength) continue;
    matchLength = root.length;
    relative = candidate;
  }

  if (relative === undefined) {
    const home = process.env.HOME ? path.normalize(process.env.HOME) : "";
    if (home && !isFilesystemRoot(home)) {
      const candidate = path.relative(home, resolved);
      if (relativeInside(candidate)) relative = candidate;
    }
    relative ??= resolved;
  }

  const parts = relative.split(path.sep).filter(Boolean);
  const worktreeIndex = parts.lastIndexOf(".worktrees");
  if (worktreeIndex !== -1 && parts.length > worktreeIndex + 1) parts.splice(worktreeIndex, 2);
  const leaked = [worktree, directory].filter(Boolean).map((root) => path.basename(root));
  if (parts[0] && leaked.includes(parts[0])) parts.shift();
  return parts.join(path.sep) || relative;
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

function specLabel(filePath: string) {
  const parts = path.normalize(filePath).split(/[\\/]/u).filter(Boolean);
  const specIndex = parts.lastIndexOf(".spec");
  const owner = parts[specIndex - 1];
  const nestedPath = parts.slice(specIndex + 1);

  return stripMarkdownExtension([owner, ...nestedPath].filter(Boolean).join(path.sep));
}

function isFilesystemRoot(value: string) {
  const normalized = path.normalize(value);
  return normalized === path.parse(normalized).root;
}

function projectRoots(api: TuiPluginApi) {
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

function truncateMiddle(value: string, maxLength: number) {
  if (maxLength <= 0) return "";
  if (value.length <= maxLength) return value;
  if (maxLength <= 3) return ".".repeat(maxLength);

  const headLength = Math.ceil((maxLength - 3) / 2);
  const tailLength = Math.floor((maxLength - 3) / 2);
  return `${value.slice(0, headLength)}...${value.slice(value.length - tailLength)}`;
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

function sourceColor(api: TuiPluginApi, item: MarkdownContextItem) {
  const c = colors(api.theme.current);
  if (item.compacted) return c.red;

  switch (item.kind) {
    case "readme":
      return c.green;
    case "agents":
      return isConfigAgents(item.path) ? c.cyan : c.blue;
    case "agent":
      return isSubagent(item.path) ? c.magenta : c.blue;
    case "skill":
      return isGlobalOpencodePath(item.path) ? c.orange : c.pink;
    case "command":
      return isGlobalOpencodePath(item.path) ? c.sky : c.cyan;
    case "partial":
      return c.yellow;
    case "spec":
      return c.cyan;
    case "markdown":
      return c.muted;
  }
}

function isConfigAgents(filePath: string) {
  return path.basename(filePath).toLowerCase() === "agents.md" && isGlobalOpencodePath(filePath);
}

function isRootAgents(api: TuiPluginApi, filePath: string) {
  const dir = path.normalize(path.dirname(filePath));
  return projectRoots(api).some((root) => path.normalize(root) === dir);
}

function sourceIcon(api: TuiPluginApi, item: MarkdownContextItem) {
  if (item.compacted) return `${icons.compacted} `;

  switch (item.kind) {
    case "readme":
      return `${icons.readme} `;
    case "agents":
      if (isConfigAgents(item.path)) return `${icons.agentsCore} `;
      return `${isRootAgents(api, item.path) ? icons.folderLibrary : icons.folder} `;
    case "agent":
      return `${isSubagent(item.path) ? icons.subagent : icons.agents} `;
    case "skill":
      return `${isGlobalOpencodePath(item.path) ? icons.skill : icons.skillProject} `;
    case "command":
      return `${isGlobalOpencodePath(item.path) ? icons.command : icons.commandProject} `;
    case "partial":
      return `${icons.partial} `;
    case "spec":
      return `${icons.spec} `;
    case "markdown":
      return `${icons.markdown} `;
  }
}

const tui: TuiPlugin = async (api) => {
  api.slots.register({
    order: 120,
    slots: {
      sidebar_content(_ctx, props: { session_id: string }) {
        return <MarkdownContext api={api} sessionID={props.session_id} />;
      },
    },
  });
};

const plugin: TuiPluginModule & { id: string } = {
  id,
  tui,
};

/** Adds the TUI sidebar_content slot for loaded Markdown context.
 * Tracks message and part updates/removals and session.compacted.
 */
export default plugin;
