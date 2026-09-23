/** @jsxImportSource @opentui/solid */
import type { ToolPart } from "@opencode-ai/sdk/v2";
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from "@opencode-ai/plugin/tui";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";
import { For, Show, createRenderEffect, createSignal, onCleanup, untrack } from "solid-js";
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
import { ActionIcon, type IconAction } from "../shared/action-icon.tsx";
import { SidebarSection } from "../shared/sidebar-section.tsx";
import {
  configRoot,
  displayPath,
  isConfigAgents,
  isGlobalOpencodePath,
  isMarkdownPath,
  isSubagent,
  markdownIdentity,
  markdownSourceKind,
  projectRoots,
  type MarkdownSourceKind,
} from "./markdown-source.ts";

const id = "opencode-markdown-context";

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

  createRenderEffect(() => {
    const disposers = [
      props.api.event.on("message.updated", (event) => {
        if (event.properties.sessionID === untrack(() => props.sessionID)) refresh();
      }),
      props.api.event.on("message.removed", (event) => {
        if (event.properties.sessionID === untrack(() => props.sessionID)) refresh();
      }),
      props.api.event.on("message.part.updated", (event) => {
        if (event.properties.sessionID === untrack(() => props.sessionID)) refresh();
      }),
      props.api.event.on("message.part.removed", (event) => {
        if (event.properties.sessionID === untrack(() => props.sessionID)) refresh();
      }),
      props.api.event.on("session.compacted", (event) => {
        if (event.properties.sessionID === untrack(() => props.sessionID)) refresh();
      }),
    ];
    onCleanup(() => {
      for (const dispose of disposers) dispose();
    });
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
            <box
              flexDirection="row"
              gap={0}
              onMouseDown={() => openInNvim(props.api, item.path, "Markdown open failed")}
            >
              <ActionIcon
                api={props.api}
                icon={sourceIcon(props.api, item)}
                fg={sourceColor(props.api, item)}
                action={rowAction(props.api, props.sessionID, item)}
              />
              <text fg={props.api.theme.current.textMuted} wrapMode="none" flexShrink={1}>
                {item.label}
              </text>
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
    const item = markdownFileItem(api, filePath, { time: 0, compacted: false, refs: [], pinned: true });
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
  return markdownFileItem(api, filePath, {
    time: part.state.time.end,
    compacted: part.state.time.compacted !== undefined,
    refs: [{ messageID: part.messageID, partID: part.id }],
  });
}

function skillToolItem(
  api: TuiPluginApi,
  part: ReturnType<TuiPluginApi["state"]["part"]>[number],
): MarkdownContextItem | undefined {
  if (part.type !== "tool" || !isSkillTool(part.tool)) return undefined;
  if (part.state.status !== "completed") return undefined;

  const filePath = skillPathFromTool(part);
  if (!filePath) return undefined;
  return markdownFileItem(api, filePath, {
    time: part.state.time.end,
    compacted: part.state.time.compacted !== undefined,
    refs: [{ messageID: part.messageID, partID: part.id }],
  });
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
  options: {
    time: number;
    compacted: boolean;
    refs: PartRef[];
    pinned?: boolean;
  },
): MarkdownContextItem {
  const kind = markdownSourceKind(filePath);
  return {
    key: markdownIdentity(filePath),
    path: filePath,
    label: displayPath(api, filePath, kind),
    kind,
    compacted: options.compacted,
    pinned: options.pinned ?? false,
    time: options.time,
    refs: options.refs,
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

function rowAction(api: TuiPluginApi, sessionID: string, item: MarkdownContextItem): IconAction | undefined {
  if (canUnload(api, sessionID, item)) return { icon: icons.error, run: () => void unloadItem(api, sessionID, item) };
  if (canReload(item)) return { icon: icons.restore, run: () => void reloadItem(api, sessionID, item) };
  return undefined;
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
    await Promise.all(parts.map((part) => persistUpdatedPart(api.client, part)));
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

function normalizeFilePath(value: string) {
  const clean = value.replace(/^file:\/\//, "").split(/[?#]/, 1)[0];
  if (clean.startsWith("~/")) return path.join(process.env.HOME || "~", clean.slice(2));
  return clean;
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

function isRootAgents(api: TuiPluginApi, filePath: string) {
  const dir = path.normalize(path.dirname(filePath));
  return projectRoots(api).some((root) => path.normalize(root) === dir);
}

function sourceIcon(api: TuiPluginApi, item: MarkdownContextItem) {
  if (item.compacted) return icons.compacted;

  switch (item.kind) {
    case "readme":
      return icons.readme;
    case "agents":
      if (isConfigAgents(item.path)) return icons.agentsCore;
      return isRootAgents(api, item.path) ? icons.folderLibrary : icons.folder;
    case "agent":
      return isSubagent(item.path) ? icons.subagent : icons.agents;
    case "skill":
      return isGlobalOpencodePath(item.path) ? icons.skill : icons.skillProject;
    case "command":
      return isGlobalOpencodePath(item.path) ? icons.command : icons.commandProject;
    case "partial":
      return icons.partial;
    case "spec":
      return icons.spec;
    case "markdown":
      return icons.markdown;
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
