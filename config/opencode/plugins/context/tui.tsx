import { Plugin } from "@opencode/plugin/tui";
import { RGBA } from "@opentui/core";
import { spawn } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import { homedir } from "node:os";
import path from "node:path";
import { Schema } from "effect";
import { createEffect, createMemo, createSignal, For, Show } from "solid-js";
import { configRoot, identity, isConfigAgents, isGlobal, isMarkdown, isSubagent, kindOf, label } from "./source.ts";

type Context = Plugin.Context;
type Session = NonNullable<ReturnType<Context["data"]["session"]["get"]>>;
type Message = ReturnType<Context["data"]["session"]["message"]["list"]>[number];
type Prompts = Record<string, string[]>;
type Load = { path: string; time: number };

type Item = {
  path: string;
  label: string;
  icon: string;
  color: RGBA;
  time: number;
  compacted: boolean;
};

const Config = Schema.Struct({
  agents: Schema.optional(Schema.Record(Schema.String, Schema.Struct({ system: Schema.optional(Schema.Unknown) }))),
});

const colors = {
  blue: RGBA.fromHex("#7492ef"),
  green: RGBA.fromHex("#95cb79"),
  yellow: RGBA.fromHex("#f5c069"),
  orange: RGBA.fromHex("#f8b486"),
  red: RGBA.fromHex("#f08898"),
  magenta: RGBA.fromHex("#b29ae8"),
  pink: RGBA.fromHex("#ea76c0"),
  cyan: RGBA.fromHex("#50dec8"),
  sky: RGBA.fromHex("#7cc5ef"),
  muted: RGBA.fromHex("#7b7fb0"),
} as const;

const glyph = {
  agents: "\u{f0bc9}",
  core: "\u{e272}",
  collab: "\u{f1218}",
  routing: "\u{f0781}",
  subagent: "\u{e36e}",
  folder: "\u{ea83}",
  library: "\u{eb46}",
  skill: "\u{f0e7}",
  skillProject: "\u{f03d7}",
  command: "\u{f0633}",
  commandProject: "\u{f085b}",
  partial: "\u{f0219}",
  markdown: "\u{f0354}",
  compacted: "\u{ef2d}",
  readme: "\u{ef0d}",
  spec: "\u{f1345}",
};

export default Plugin.define({
  id: "context",
  setup(context) {
    const prompts = agentPrompts();
    context.ui.slot({
      append: "sidebar.content",
      render: (input) => <Sidebar context={context} sessionID={input.sessionID} prompts={prompts} />,
    });
  },
});

function Sidebar(props: { context: Context; sessionID: string; prompts: Prompts }) {
  const [open, setOpen] = createSignal(true);
  const session = () => props.context.data.session.get(props.sessionID);
  createEffect(() => {
    const location = session()?.location;
    if (location && !props.context.data.location.skill.list(location))
      void props.context.data.location.skill.sync(location);
  });
  const list = createMemo(() => {
    const current = session();
    return current ? items(props.context, current, props.prompts) : [];
  });

  return (
    <Show when={list().length > 0}>
      <box flexDirection="column">
        <box flexDirection="row" onMouseDown={() => setOpen((value) => !value)}>
          <text fg={props.context.theme.text.base} wrapMode="none">
            {open() ? "▼ " : "▶ "}
          </text>
          <text fg={props.context.theme.text.base} wrapMode="none">
            <b>Markdown Context</b>
          </text>
          <text fg={props.context.theme.text.muted} wrapMode="none">{` ${list().length} read`}</text>
        </box>
        <Show when={open()}>
          <For each={list()}>
            {(entry) => (
              <box flexDirection="row" onMouseDown={() => edit(props.context, entry.path)}>
                <text fg={entry.color} wrapMode="none" flexShrink={0}>{`${entry.icon} `}</text>
                <text fg={props.context.theme.text.muted} wrapMode="none" flexShrink={1}>
                  {entry.label}
                </text>
              </box>
            )}
          </For>
        </Show>
      </box>
    </Show>
  );
}

function items(context: Context, session: Session, prompts: Prompts) {
  const roots = [
    ...new Set([session.location.directory, context.data.project.get(session.projectID)?.canonical]),
  ].filter((root): root is string => !!root);
  const pinned = new Map<string, Item>();
  for (const file of pinnedFiles(context, session, prompts)) {
    const key = identity(file);
    if (!pinned.has(key) && existsSync(file)) pinned.set(key, item(roots, file, 0, false));
  }

  const messages = context.data.session.message.list(session.id);
  const boundary = messages.findLastIndex((message) => message.type === "compaction" && message.status === "completed");
  const skills = context.data.location.skill.list(session.location);
  const skill = (id: string) => skills?.find((entry) => entry.id === id)?.path;
  const reads = new Map<string, Item>();
  messages.forEach((message, index) => {
    for (const load of loads(message, session.location.directory, skill)) {
      const key = identity(load.path);
      if (pinned.has(key)) continue;
      const compacted = index < boundary;
      const existing = reads.get(key);
      if (!existing) {
        reads.set(key, item(roots, load.path, load.time, compacted));
        continue;
      }
      if (load.time >= existing.time)
        reads.set(key, item(roots, load.path, load.time, existing.compacted && compacted));
      else existing.compacted &&= compacted;
    }
  });

  return [...pinned.values(), ...[...reads.values()].toSorted((left, right) => right.time - left.time)];
}

function pinnedFiles(context: Context, session: Session, prompts: Prompts) {
  const start = session.location.directory;
  const home = homedir();
  const stop = inside(home, start) ? home : (context.data.project.get(session.projectID)?.canonical ?? start);
  const files = [path.join(configRoot, "AGENTS.md")];
  for (let dir = start; ; dir = path.dirname(dir)) {
    files.push(path.join(dir, "AGENTS.md"));
    if (dir === stop || dir === path.dirname(dir)) break;
  }
  if (session.agent)
    files.push(...(prompts[session.agent] ?? [path.join(configRoot, "agents", `${session.agent}.md`)]));
  return files;
}

function loads(message: Message, directory: string, skill: (id: string) => string | undefined): Load[] {
  if (message.type === "skill") {
    const file = skill(message.skill);
    return file ? [{ path: file, time: message.time.created }] : [];
  }
  if (message.type === "synthetic") {
    const instruction = message.metadata?.instruction;
    const paths = instruction && typeof instruction === "object" && "paths" in instruction ? instruction.paths : [];
    return Array.isArray(paths)
      ? paths.filter((file) => typeof file === "string").map((file) => ({ path: file, time: message.time.created }))
      : [];
  }
  if (message.type !== "assistant") return [];
  return message.content.flatMap((part): Load[] => {
    if (part.type !== "tool" || part.state.status !== "completed") return [];
    const time = part.time.completed ?? message.time.created;
    const input = part.state.input;
    if (part.name === "read" && typeof input.path === "string" && isMarkdown(input.path))
      return [{ path: path.resolve(directory, expand(input.path)), time }];
    if (part.name !== "skill" || typeof input.id !== "string") return [];
    const dir = part.state.metadata?.directory;
    const file = skill(input.id) ?? (typeof dir === "string" ? path.join(dir, "SKILL.md") : undefined);
    return file ? [{ path: file, time }] : [];
  });
}

function item(roots: readonly string[], file: string, time: number, compacted: boolean): Item {
  const kind = kindOf(file);
  return {
    path: file,
    label: label(roots, file, kind),
    time,
    compacted,
    ...(compacted ? { icon: glyph.compacted, color: colors.red } : appearance(roots, file, kind)),
  };
}

function appearance(roots: readonly string[], file: string, kind: ReturnType<typeof kindOf>) {
  switch (kind) {
    case "readme":
      return { icon: glyph.readme, color: colors.green };
    case "agents":
      if (isConfigAgents(file)) return { icon: coreIcon(file), color: colors.cyan };
      return { icon: isRoot(roots, file) ? glyph.library : glyph.folder, color: colors.blue };
    case "agent":
      return isSubagent(file)
        ? { icon: glyph.subagent, color: colors.magenta }
        : { icon: glyph.agents, color: colors.blue };
    case "skill":
      return isGlobal(file)
        ? { icon: glyph.skill, color: colors.orange }
        : { icon: glyph.skillProject, color: colors.pink };
    case "command":
      return isGlobal(file)
        ? { icon: glyph.command, color: colors.sky }
        : { icon: glyph.commandProject, color: colors.cyan };
    case "partial":
      return { icon: glyph.partial, color: colors.yellow };
    case "spec":
      return { icon: glyph.spec, color: colors.cyan };
    case "markdown":
      return { icon: glyph.markdown, color: colors.muted };
  }
}

function coreIcon(file: string) {
  const name = path.basename(file);
  if (name === "COLLAB.md") return glyph.collab;
  if (name === "ROUTING.md") return glyph.routing;
  return glyph.core;
}

function isRoot(roots: readonly string[], file: string) {
  const dir = path.normalize(path.dirname(file));
  return roots.some((root) => path.normalize(root) === dir);
}

function inside(root: string, file: string) {
  const relative = path.relative(root, file);
  return relative === "" || (!relative.startsWith("..") && !path.isAbsolute(relative));
}

function expand(value: string) {
  const clean = value.replace(/^file:\/\//, "").split(/[?#]/, 1)[0];
  return clean.startsWith("~/") ? path.join(homedir(), clean.slice(2)) : clean;
}

function agentPrompts(): Prompts {
  const config = Schema.decodeUnknownSync(Config)(
    JSON.parse(readFileSync(path.join(configRoot, "opencode.json"), "utf8")),
  );
  return Object.fromEntries(
    Object.entries(config.agents ?? {}).flatMap(([name, agent]) => {
      if (typeof agent.system !== "string") return [];
      const files = [...agent.system.matchAll(/\{file:([^}]+)\}/g)].map(([, file]) =>
        path.resolve(configRoot, file.startsWith("~/") ? path.join(homedir(), file.slice(2)) : file),
      );
      return files.length > 0 ? [[name, files]] : [];
    }),
  );
}

function edit(context: Context, file: string) {
  const child = spawn("hyprd", ["edit", file], { detached: true, stdio: ["ignore", "ignore", "pipe"] });
  let stderr = "";
  child.stderr.on("data", (chunk) => {
    stderr += String(chunk);
  });
  const fail = (message: string) =>
    context.ui.toast.show({ title: "Markdown open failed", message, variant: "warning" });
  child.once("error", (error) => fail(error.message || file));
  child.once("close", (code) => {
    if (code !== 0) fail(stderr.trim() || `hyprd exited ${code ?? "without a status"}: ${file}`);
  });
  child.unref();
}
