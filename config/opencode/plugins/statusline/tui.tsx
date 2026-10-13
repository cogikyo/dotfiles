import { Plugin } from "@opencode/plugin/tui";
import type { RGBA } from "@opentui/core";
import { createEffect, createMemo, createResource, createRoot, createSignal, For, on, onCleanup, Show } from "solid-js";
import { gitStatus, place, type GitStatus } from "./git.ts";
import { colors, pressure, tier } from "./palette.ts";
import { mirror } from "./toasts.tsx";
import { heat, motion, period, tail, tick, Working } from "./working.tsx";

type Context = Plugin.Context;
type Message = ReturnType<Context["data"]["session"]["message"]["list"]>[number];
type Assistant = Extract<Message, { type: "assistant" }>;
type Measured = Assistant & { tokens: NonNullable<Assistant["tokens"]> };

const poll = 2_000;
const reserve = 16_000;

const glyph = {
  context: "\u{328b}",
  filled: "\u{25c9}",
  empty: "\u{25cb}",
  branch: "\u{e725} ",
  worktree: "\u{ec7e} ",
  bullet: "\u{2022}",
  ahead: "\u{2bad}",
  behind: "\u{2baf}",
  modified: "\u{f417} ",
  staged: "\u{eafc}",
  deleted: "\u{f0683} ",
  untracked: "\u{e370} ",
  stashed: "\u{f0e27} ",
  conflicted: "\u{f47f} ",
  renamed: "\u{f0455} ",
};

const counts = [
  ["ahead", colors.green],
  ["behind", colors.brightRed],
  ["modified", colors.sky],
  ["staged", colors.yellow],
  ["deleted", colors.red],
  ["untracked", colors.yellow],
  ["stashed", colors.muted],
  ["conflicted", colors.pink],
  ["renamed", colors.magenta],
] as const;

const buffers = new Map<string, Promise<number | undefined>>();

export default Plugin.define({
  id: "statusline",
  setup(context) {
    return createRoot((dispose) => {
      context.ui.slot({
        replace: "prompt.footer.status",
        render: (input) => <Location context={context} sessionID={input.sessionID} />,
      });
      context.ui.slot({
        append: "prompt.footer",
        render: (input) => <Bar context={context} sessionID={input.sessionID} />,
      });
      context.ui.slot({
        append: "prompt.footer",
        render: (input) => (
          <Usage context={context} sessionID={input.sessionID} mode={input.mode} details={input.showDetails} />
        ),
      });
      context.ui.slot({
        append: "prompt.footer",
        render: (input) => <Working context={context} sessionID={input.sessionID} />,
      });
      const Mirror = mirror(context);
      context.ui.slot({
        append: "sidebar.footer",
        render: () => (
          <Mirror>
            <Footer context={context} />
          </Mirror>
        ),
      });
      return dispose;
    });
  },
});

function Location(props: { context: Context; sessionID?: string }) {
  const directory = createMemo(() => {
    const session = props.sessionID ? props.context.data.session.get(props.sessionID) : undefined;
    return (session?.location ?? props.context.location ?? props.context.data.location.default()).directory;
  });
  const [git, setGit] = createSignal<GitStatus>();
  createEffect(() => onCleanup(watch(directory(), setGit)));
  const [where] = createResource(directory, place);

  return (
    <box flexDirection="row" flexGrow={1} flexShrink={1} minWidth={0} marginLeft={1}>
      <text fg={colors.brightBlue} wrapMode="none" flexShrink={0}>
        <b>{props.context.ui.format.path(where()?.path ?? directory())}</b>
      </text>
      <Show when={git()?.branch ? git() : undefined}>
        {(status) => <Git status={status()} worktree={where()?.worktree ?? false} />}
      </Show>
    </box>
  );
}

function watch(dir: string, set: (status: GitStatus | undefined) => void) {
  let live = true;
  const read = async () => {
    const status = await gitStatus(dir);
    if (live) set(status);
  };
  void read();
  const timer = setInterval(() => void read(), poll);
  return () => {
    live = false;
    clearInterval(timer);
  };
}

function Git(props: { status: GitStatus; worktree: boolean }) {
  const icon = () => (props.worktree ? glyph.worktree : glyph.branch);
  return (
    <text wrapMode="none" truncate flexShrink={1}>
      <span style={{ fg: branchColor(props.status) }}>{` ${icon()}${props.status.branch}`}</span>
      <For each={counts}>
        {([name, fg]) => (
          <Show when={props.status[name] > 0}>
            <span style={{ fg }}>{` ${glyph[name]}${props.status[name]}`}</span>
          </Show>
        )}
      </For>
    </text>
  );
}

function Footer(props: { context: Context }) {
  const theme = () => props.context.theme.text;

  return (
    <text fg={theme().muted} wrapMode="none">
      <span style={{ fg: colors.green }}>{glyph.bullet}</span>
      <span style={{ fg: theme().base }}>
        <b>{" OpenCode"}</b>
      </span>
      {` ${props.context.app.version}`}
    </text>
  );
}

function branchColor(status: GitStatus): RGBA {
  if (status.behind > 0 || status.conflicted > 0) return colors.brightRed;
  if (status.modified > 0) return colors.sky;
  if (status.staged > 0) return colors.yellow;
  if (status.deleted > 0) return colors.red;
  if (status.untracked > 0) return colors.yellow;
  if (status.ahead > 0) return colors.green;
  if (status.renamed > 0) return colors.magenta;
  return colors.blue;
}

function Bar(props: { context: Context; sessionID?: string }) {
  const directory = () =>
    props.sessionID ? props.context.data.session.get(props.sessionID)?.location.directory : undefined;
  const [buffer] = createResource(directory, (dir) => configured(props.context, dir));
  const percent = createMemo(() => {
    if (!props.sessionID || buffer.state !== "ready") return;
    return usage(props.context, props.sessionID, buffer());
  });
  const level = createMemo(() => {
    const value = percent();
    return value === undefined ? undefined : tier(value);
  });
  const running = () =>
    props.sessionID !== undefined && props.context.data.session.status(props.sessionID) === "running";
  const [now, setNow] = createSignal(0);
  let lit = 0;

  createEffect(() => {
    if (!running()) return setNow(0);
    const timer = setInterval(() => setNow(motion() ? performance.now() : 0), tick);
    onCleanup(() => clearInterval(timer));
  });
  createEffect(
    on(level, (next, previous) => {
      if (next !== undefined && previous !== undefined && next > previous) lit = performance.now();
    }),
  );
  const dots = createMemo(() => {
    const value = percent();
    return value === undefined ? [] : gauge(value, now(), lit);
  });

  return (
    <Show when={percent()}>
      <box position="absolute" top={-2} right={2}>
        <text wrapMode="none">
          <For each={dots().map((_, index) => index)}>
            {(index) => <span style={{ fg: dots()[index]?.fg }}>{dots()[index]?.char}</span>}
          </For>
        </text>
      </box>
    </Show>
  );
}

function gauge(value: number, now: number, lit: number) {
  const level = tier(value);
  const color = pressure(value);
  const chars = [
    glyph.context,
    ...Array<string>(level + 1).fill(glyph.filled),
    ...Array<string>(8 - level).fill(glyph.empty),
  ];
  const reach = level + 3;
  const head = Math.floor(((now % period) / period) * (reach + tail));
  return chars.map((char, index) => {
    const step = head - index;
    const wave = now && index < reach && step >= 0 && step <= tail ? 1 - step / (tail + 1) : 0;
    const flash = now && index === level + 1 ? Math.max(0, 1 - (now - lit) / 600) : 0;
    return { char, fg: heat(color, Math.max(wave, flash)) };
  });
}

function configured(context: Context, directory: string) {
  const cached = buffers.get(directory);
  if (cached) return cached;
  const pending = context.client.config.get({ location: { directory } }).then(
    (entries) =>
      entries.reduce<number | undefined>(
        (value, entry) => (entry.type === "document" ? (entry.info.compaction?.buffer ?? value) : value),
        undefined,
      ),
    (error: unknown) => {
      buffers.delete(directory);
      context.ui.toast.show({
        message: `Statusline: read compaction config: ${error instanceof Error ? error.message : String(error)}`,
        variant: "error",
      });
      throw error;
    },
  );
  buffers.set(directory, pending);
  return pending;
}

function Usage(props: { context: Context; sessionID?: string; mode: "normal" | "shell"; details: boolean }) {
  const [hovered, setHovered] = createSignal(false);
  const theme = () => props.context.theme.text;
  const live = createMemo(() => {
    const id = props.sessionID;
    if (!id) return "";
    const data = props.context.data;
    const agents = data.session.family(id).filter((child) => child !== id && data.session.status(child) === "running");
    const shells = data.shell.list(props.context.location).filter((shell) => shell.metadata.sessionID === id);
    return [plural(agents.length, "subagent"), plural(shells.length, "shell")].filter(Boolean).join(" · ");
  });
  const size = createMemo(() => {
    const current = props.sessionID ? load(props.context, props.sessionID) : undefined;
    if (!current) return "";
    const limit = current.model?.limit.context;
    const value = count(current.tokens);
    return limit ? `${value} (${Math.round((current.tokens / limit) * 100)}%)` : value;
  });

  return (
    <Show
      when={props.mode === "normal"}
      fallback={
        <text fg={theme().base} wrapMode="none" flexShrink={0}>
          esc <span style={{ fg: theme().muted }}>exit shell mode</span>
        </text>
      }
    >
      <box flexDirection="row" flexShrink={0}>
        <Show when={live()}>
          <text
            fg={hovered() ? theme().base : theme().muted}
            wrapMode="none"
            onMouseOver={() => setHovered(true)}
            onMouseOut={() => setHovered(false)}
            onMouseUp={() => props.context.keymap.dispatch("session.child.first")}
          >
            {`${props.context.keymap.shortcuts("session.child.first")[0] ?? ""} ${live()}`.trim()}
          </text>
        </Show>
        <text fg={theme().muted} wrapMode="none">
          <Show when={live() && props.details && size()}>{" · "}</Show>
          <Show when={props.details}>{size()}</Show>
        </text>
      </box>
    </Show>
  );
}

function plural(value: number, noun: string) {
  return value ? `${value} ${noun}${value === 1 ? "" : "s"}` : "";
}

function count(tokens: number) {
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(1)}M`;
  return tokens >= 1_000 ? `${(tokens / 1_000).toFixed(1)}K` : String(tokens);
}

function load(context: Context, sessionID: string) {
  const session = context.data.session.get(sessionID);
  if (!session) return;
  const last = measured(context.data.session.message.list(sessionID), session.revert?.messageID);
  if (!last) return;
  const tokens =
    last.tokens.input + last.tokens.output + last.tokens.reasoning + last.tokens.cache.read + last.tokens.cache.write;
  if (tokens <= 0) return;
  const model = context.data.location.model
    .list(session.location)
    ?.find((entry) => entry.providerID === last.model.providerID && entry.id === last.model.id);
  return { tokens, model };
}

function usage(context: Context, sessionID: string, buffer: number | undefined) {
  const current = load(context, sessionID);
  if (!current?.model) return;
  const window = current.model.limit.input || current.model.limit.context;
  if (window <= 0) return;
  const threshold = window - (buffer ?? Math.max(Math.floor(window * 0.1), window >= 2 * reserve ? reserve : 0));
  if (threshold <= 0) return;
  return (current.tokens / threshold) * 100;
}

function measured(messages: readonly Message[], boundary?: string) {
  const index = boundary ? messages.findIndex((message) => message.id === boundary) : -1;
  if (boundary && index === -1) return;
  const end = index === -1 ? messages.length : index;
  const compaction = messages.findLastIndex(
    (message, at) => message.type === "compaction" && message.status === "completed" && at < end,
  );
  return messages.findLast(
    (message, at): message is Measured =>
      message.type === "assistant" && message.tokens !== undefined && at > compaction && at < end,
  );
}
