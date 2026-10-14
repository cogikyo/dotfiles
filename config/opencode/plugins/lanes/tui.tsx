import { MouseButton, type MouseEvent } from "@opentui/core";
import { Plugin } from "@opencode/plugin/tui";
import { createEffect, createMemo, createSignal, For, Show } from "solid-js";
import { hover, Section, spinner } from "../context/section.tsx";
import { colors, pressure, roles } from "../statusline/palette.ts";
import { api } from "./rpc.ts";

type Context = Plugin.Context;
type Session = ReturnType<Context["data"]["session"]["list"]>[number];
type Message = ReturnType<Context["data"]["session"]["message"]["list"]>[number];
type Assistant = Extract<Message, { type: "assistant" }>;
type Measured = Assistant & { tokens: NonNullable<Assistant["tokens"]> };
type Status = "running" | "idle" | "error" | "interrupted";
type Action = "steer" | "interrupt" | "continue" | "open";
type Row = {
  session: Session;
  name: string;
  status: Status;
  agent: string;
  model: string;
  tokens: number;
  spent: string;
};

const stop = 300_000;

const glyph = {
  idle: "\u{f03e6}",
  error: "\u{f015c}",
};

const icons: Partial<Record<string, string>> = {
  build: "\u{f1887} ",
  review: "\u{f0208} ",
  verify: "\u{f0565} ",
  scout: "\u{f018b} ",
};

const scopes: Partial<Record<string, string>> = {
  general: "\u{265e}",
  owner: "\u{f085a}",
  patch: "\u{f0daf}",
  scribe: "\u{f0d13}",
  git: "\u{f062c}",
  debug: "\u{f00e4}",
  architect: "\u{f04aa}",
  simplify: "\u{f0190}",
  critic: "\u{f029b}",
  copy: "\u{f0284}",
  entropy: "\u{f0f38}",
  design: "\u{f03d8}",
  security: "\u{f033e}",
  profile: "\u{f04c5}",
  context: "\u{f034d}",
  library: "\u{f0331}",
  dirty: "\u{f0dc8}",
  session: "\u{f02da}",
  web: "\u{f059f}",
  source: "\u{f0169}",
  test: "\u{f0093}",
  browser: "\u{f0cfd}",
};

const families: [RegExp, string][] = [
  [/^opus/, "opus"],
  [/^sonnet/, "sonnet"],
  [/sol/, "sol"],
  [/luna/, "luna"],
  [/astra/, "astra"],
  [/fable/, "fable"],
  [/grok/, "grok"],
];

const laneOf = (session: Session) => (typeof session.metadata?.lane === "string" ? session.metadata.lane : undefined);

function status(context: Context, session: Session): Status {
  if (context.data.session.status(session.id) === "running") return "running";
  if (session.outcome === "failed") return "error";
  return session.outcome === "interrupted" ? "interrupted" : "idle";
}

function lanes(context: Context, rootID: string) {
  return context.data.session
    .list()
    .filter((session) => session.parentID === rootID && session.metadata?.closed === undefined)
    .filter((session) => laneOf(session) !== undefined || status(context, session) !== "idle");
}

function row(context: Context, session: Session): Row {
  const last = latest(context.data.session.message.list(session.id));
  const usage = last?.tokens;
  const tokens = usage ? usage.input + usage.output + usage.reasoning + usage.cache.read + usage.cache.write : 0;
  return {
    session,
    name: laneOf(session) ?? session.title ?? session.id,
    status: status(context, session),
    agent: label(session.agent ?? "unknown"),
    model: family(last?.model.id ?? session.model?.id ?? "unknown"),
    tokens,
    spent: usage ? count(tokens) : "",
  };
}

function latest(messages: readonly Message[]) {
  const compaction = messages.findLastIndex(
    (message) => message.type === "compaction" && message.status === "completed",
  );
  return messages.findLast(
    (message, at): message is Measured =>
      message.type === "assistant" && message.tokens !== undefined && at > compaction,
  );
}

function family(model: string) {
  const bare = model.replace(/^.*\//, "").replace(/^claude-/, "");
  return families.find(([pattern]) => pattern.test(bare))?.[1] ?? bare;
}

function label(agent: string) {
  const [role, ...rest] = agent.split("/");
  const icon = icons[role];
  if (!icon || !rest.length) return agent;
  const scope = rest.join("/");
  return `${icon}${scopes[scope] ?? scope}`;
}

function count(tokens: number) {
  if (tokens >= 999_500) return `${(tokens / 1_000_000).toFixed(1)}M`;
  return tokens < 1_000 ? String(tokens) : `${Math.round(tokens / 1_000)}K`;
}

async function close(context: Context, rootID: string, names: string[]) {
  const directory = context.data.session.get(rootID)?.location.directory;
  try {
    const outcomes = await context.client
      .rpc(api)
      .close({ sessionID: rootID, lanes: names }, directory ? { location: { directory } } : undefined);
    const skipped = outcomes.filter((outcome) => outcome.skipped !== undefined);
    const closed = outcomes.length - skipped.length;
    context.ui.toast.show({
      message: [
        `Closed ${closed} lane${closed === 1 ? "" : "s"}`,
        ...skipped.map((outcome) => `Skipped ${outcome.lane}: ${outcome.skipped}`),
      ].join("\n"),
      variant: skipped.length ? "warning" : "info",
    });
  } catch (error) {
    context.ui.toast.show({
      title: "Failed to close lanes",
      message: error instanceof Error ? error.message : JSON.stringify(error),
      variant: "error",
    });
  }
}

async function act(context: Context, session: Session) {
  const name = laneOf(session) ?? session.title ?? session.id;
  const action = await context.ui.dialog.select<Action>({
    title: `Lane ${name}`,
    options: [
      { title: "Steer", value: "steer", description: "Send a message into the running turn" },
      { title: "Interrupt", value: "interrupt", description: "Stop the current turn" },
      { title: "Continue", value: "continue", description: 'Send "Continue."; the parent is not notified' },
      { title: "Open", value: "open", description: "Switch to the child session" },
    ],
  });
  if (action === undefined) return;
  if (action === "open") return context.ui.router.navigate({ type: "session", sessionID: session.id });
  if (action === "interrupt") return void (await context.client.session.interrupt({ sessionID: session.id }));
  const text =
    action === "continue"
      ? "Continue."
      : await context.ui.dialog.prompt({ title: `Steer ${name}`, placeholder: "Message" });
  if (!text?.trim()) return;
  await context.client.session.prompt({ sessionID: session.id, text, delivery: "steer" });
  context.ui.toast.show({ message: action === "continue" ? `Continued ${name}` : `Steered ${name}`, variant: "info" });
}

function Sidebar(props: { context: Context; sessionID: string }) {
  const root = createMemo(() => props.context.data.session.get(props.sessionID)?.parentID === undefined);
  const rows = createMemo(() =>
    (root() ? lanes(props.context, props.sessionID) : [])
      .map((session) => row(props.context, session))
      .toSorted((left, right) => left.name.localeCompare(right.name)),
  );
  const sessions = createMemo(() => rows().map((entry) => entry.session), [], {
    equals: (left, right) => left.length === right.length && left.every((session, at) => session === right[at]),
  });
  const width = createMemo(() => ({
    model: Math.max(0, ...rows().map((entry) => entry.model.length)),
    spent: Math.max(0, ...rows().map((entry) => entry.spent.length)),
  }));
  const frame = spinner(() => rows().some((entry) => entry.status === "running"));
  createEffect(() => {
    for (const entry of rows()) void props.context.data.session.message.sync(entry.session.id);
  });

  return (
    <Show when={rows().length}>
      <Section context={props.context} title="Lanes" detail={String(rows().length)}>
        <For each={sessions()}>
          {(session) => <Lane context={props.context} session={session} width={width()} frame={frame()} />}
        </For>
      </Section>
    </Show>
  );
}

function Lane(props: { context: Context; session: Session; width: { model: number; spent: number }; frame: string }) {
  const [hovered, setHovered] = createSignal(false);
  const [aimed, setAimed] = createSignal(false);
  const entry = createMemo(() => row(props.context, props.session));
  const theme = () => props.context.theme.text;
  const named = () => laneOf(entry().session) !== undefined;
  const closable = () => named() && entry().status !== "running";
  const icon = () => {
    if (aimed() && closable()) return glyph.error;
    if (entry().status === "running") return props.frame;
    return entry().status === "error" ? glyph.error : glyph.idle;
  };
  const tone = () => {
    if (aimed() && closable()) return theme().muted;
    if (entry().status === "running") return colors.green;
    if (entry().status === "error") return colors.red;
    return entry().status === "interrupted" ? colors.yellow : theme().muted;
  };
  const ink = () => {
    if (hovered()) return hover;
    return named() ? theme().base : theme().muted;
  };
  const [room, setRoom] = createSignal<number>();
  const title = () => {
    const value = entry().name;
    const max = room();
    return max === undefined || value.length <= max ? value : `${value.slice(0, Math.max(0, max - 1))}…`;
  };
  const role = () => roles[entry().session.agent?.split("/")[0] ?? ""] ?? theme().muted;
  const spent = () => (props.width.spent ? ` ${entry().spent.padStart(props.width.spent)}` : "");
  const click = (event: MouseEvent) => {
    if (event.button === MouseButton.RIGHT) return void act(props.context, entry().session);
    props.context.ui.router.navigate({ type: "session", sessionID: entry().session.id });
  };

  return (
    <box
      flexDirection="row"
      onMouseUp={click}
      onMouseOver={() => setHovered(true)}
      onMouseOut={() => setHovered(false)}
    >
      <text
        fg={tone()}
        wrapMode="none"
        flexShrink={0}
        onMouseOver={() => setAimed(true)}
        onMouseOut={() => setAimed(false)}
        onMouseUp={(event) => {
          if (!closable() || event.button === MouseButton.RIGHT) return;
          event.stopPropagation();
          const { parentID } = entry().session;
          const lane = laneOf(entry().session);
          if (parentID && lane) void close(props.context, parentID, [lane]);
        }}
      >{`${icon()} `}</text>
      <box
        flexGrow={1}
        flexShrink={1}
        minWidth={0}
        onSizeChange={function () {
          setRoom(this.width);
        }}
      >
        <text fg={ink()} wrapMode="none">
          {title()}
        </text>
      </box>
      <text fg={theme().muted} wrapMode="none" flexShrink={0}>
        {" ["}
        <span style={{ fg: role() }}>{entry().agent}</span>
        {` ${entry().model.padStart(props.width.model)}]`}
      </text>
      <text fg={pressure((entry().tokens / stop) * 100)} wrapMode="none" flexShrink={0}>
        {spent()}
      </text>
    </box>
  );
}

export default Plugin.define({
  id: "lanes",
  setup(context) {
    context.ui.slot({
      append: "sidebar.content",
      render: (props) => <Sidebar context={context} sessionID={props.sessionID} />,
    });

    context.keymap.layer(() => ({
      mode: "global",
      commands: [
        {
          id: "lanes.pick",
          title: "Steer, interrupt, or continue a lane",
          group: "Session",
          palette: true,
          slash: { name: "lanes" },
          async run() {
            const route = context.ui.router.current();
            if (route.type !== "session") return;
            const rootID = context.data.session.root(route.sessionID);
            const session = await context.ui.dialog.select<Session>({
              title: "Lanes",
              options: lanes(context, rootID).map((child) => ({
                title: laneOf(child) ?? child.title ?? child.id,
                value: child,
                description: `${status(context, child)} · ${child.agent ?? ""}`,
              })),
            });
            if (session) await act(context, session);
          },
        },
        {
          id: "lanes.close",
          title: "Close idle lanes",
          description:
            "Close idle named lanes so they cannot resume; the next call with a closed name starts a fresh child",
          group: "Session",
          palette: true,
          slash: { name: "lanes-close" },
          async run() {
            const route = context.ui.router.current();
            if (route.type !== "session") return;
            const rootID = context.data.session.root(route.sessionID);
            const idle = lanes(context, rootID).flatMap((child) => {
              const lane = laneOf(child);
              return lane !== undefined && status(context, child) !== "running" ? [lane] : [];
            });
            await close(context, rootID, idle);
          },
        },
      ],
    }));

    return context.data.on("session.metadata.updated", (event) => {
      const { sessionID } = event.data;
      if (!context.data.session.get(sessionID)) return;
      context.data.session.invalidate(sessionID);
      void context.data.session.sync(sessionID);
    });
  },
});
