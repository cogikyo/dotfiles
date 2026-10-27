import type { RGBA } from "@opentui/core";
import { Plugin } from "@opencode/plugin/tui";
import { createEffect, createMemo, createSignal, For, Show } from "solid-js";
import { hover, Section, spinner } from "../context/section.tsx";
import { attention, timeAgo } from "../sessions/rows.tsx";
import { tag } from "../sessions/tags.ts";
import { colors } from "../statusline/palette.ts";
import { envelope, glyphs, type Ledger, open, type Open, plain, read } from "./mail.ts";
import { api } from "./rpc.ts";

type Context = Plugin.Context;
type Peer = { id: string; name: string; items: Open[] };

const idle = "\u{f03e6}";
const SETTLE_MS = 300;
const watched = [
  "session.inbox.enqueued",
  "session.inbox.delivered",
  "session.inbox.cancelled",
  "session.deleted",
  "server.connected",
] as const;

function group(items: readonly Open[]): Peer[] {
  const found = new Map<string, Peer>();
  for (const item of items) {
    const peer = found.get(item.peer) ?? { id: item.peer, name: item.name, items: [] };
    peer.items.push(item);
    found.set(item.peer, peer);
  }
  return [...found.values()];
}

function Sidebar(props: { context: Context; sessionID: string; pulse: () => number }) {
  const data = () => props.context.data.session;
  const root = createMemo(() => (data().get(props.sessionID)?.parentID === undefined ? props.sessionID : undefined));
  const [ledger, setLedger] = createSignal<Ledger>({ received: [], sent: [], unreachable: [] });
  let busy = false;
  let again = false;
  const load = async () => {
    const id = root();
    if (!id) return setLedger({ received: [], sent: [], unreachable: [] });
    if (busy) return void (again = true);
    busy = true;
    const directory = data().get(id)?.location.directory;
    try {
      const next = await props.context.client
        .rpc(api)
        .ledger({ sessionID: id }, directory ? { location: { directory } } : undefined);
      if (root() === id) setLedger(next);
    } catch {
      return;
    } finally {
      busy = false;
      if (again) {
        again = false;
        void load();
      }
    }
  };
  createEffect(() => {
    root();
    void load();
  });
  createEffect(() => {
    props.pulse();
    void load();
  });

  const list = createMemo(() => group(open(ledger())));
  const unread = () =>
    list().reduce((count, peer) => count + peer.items.filter((item) => item.state === "unread").length, 0);
  const frame = spinner(createMemo(() => list().some((peer) => data().status(peer.id) === "running")));

  return (
    <Show when={list().length}>
      <Section context={props.context} title="Mail" detail={unread() ? `${unread()} unread` : String(list().length)}>
        <For each={list()}>{(peer) => <Group context={props.context} peer={peer} frame={frame()} />}</For>
      </Section>
    </Show>
  );
}

function Group(props: { context: Context; peer: Peer; frame: string }) {
  const theme = () => props.context.theme.text;
  const session = () => props.context.data.session.get(props.peer.id);
  const name = () => {
    const found = session();
    return found ? plain(found) : props.peer.name;
  };
  const closed = () => !!tag(session()?.title)?.state;
  const state = () => attention(props.context, props.peer.id);
  const running = () => props.context.data.session.status(props.peer.id) === "running";
  const icon = () => {
    const kind = state();
    if (kind) return kind === "permission" ? "!" : "?";
    return running() ? props.frame : idle;
  };
  const tone = () => {
    if (state()) return colors.yellow;
    return running() ? colors.green : theme().muted;
  };
  const go = () => props.context.ui.router.navigate({ type: "session", sessionID: props.peer.id });
  const look = (item: Open) => {
    const glyph = glyphs[item.mail.kind];
    switch (item.state) {
      case "unread":
        return { icon: `  ${glyph}`, tone: colors.pink, detail: timeAgo(item.mail.sent) };
      case "owed":
        return { icon: `  ${glyph}`, tone: colors.yellow, detail: timeAgo(item.mail.sent) };
      case "waiting":
        return { icon: `  → ${glyph}`, tone: theme().muted, detail: envelope.closed, mark: colors.pink };
      case "asked":
        return { icon: `  → ${glyph}`, tone: theme().muted, detail: envelope.open };
    }
  };

  return (
    <box flexDirection="column">
      <Line
        context={props.context}
        icon={icon()}
        tone={tone()}
        text={name()}
        ink={closed() ? theme().muted : theme().base}
        onClick={go}
      />
      <For each={props.peer.items}>
        {(item) => <Line context={props.context} text={item.mail.subject} onClick={go} {...look(item)} />}
      </For>
    </box>
  );
}

function Line(props: {
  context: Context;
  icon: string;
  tone: RGBA;
  text: string;
  ink?: RGBA;
  detail?: string;
  mark?: RGBA;
  onClick: () => void;
}) {
  const [hovered, setHovered] = createSignal(false);
  const [room, setRoom] = createSignal<number>();
  const theme = () => props.context.theme.text;
  const shown = () => {
    const max = room();
    return max === undefined || props.text.length <= max ? props.text : `${props.text.slice(0, Math.max(0, max - 1))}…`;
  };

  return (
    <box
      flexDirection="row"
      onMouseUp={() => props.onClick()}
      onMouseOver={() => setHovered(true)}
      onMouseOut={() => setHovered(false)}
    >
      <text fg={props.tone} wrapMode="none" flexShrink={0}>{`${props.icon} `}</text>
      <box
        flexGrow={1}
        flexShrink={1}
        minWidth={0}
        onSizeChange={function () {
          setRoom(this.width);
        }}
      >
        <text fg={hovered() ? hover : (props.ink ?? theme().muted)} wrapMode="none">
          {shown()}
        </text>
      </box>
      <Show when={props.detail}>
        <text fg={props.mark ?? theme().muted} wrapMode="none" flexShrink={0}>{` ${props.detail}`}</text>
      </Show>
    </box>
  );
}

export default Plugin.define({
  id: "mail",
  setup(context) {
    const [pulse, setPulse] = createSignal(0);
    let timer: ReturnType<typeof setTimeout> | undefined;
    const bump = () => {
      clearTimeout(timer);
      timer = setTimeout(() => setPulse((count) => count + 1), SETTLE_MS);
    };
    const offs = [
      ...watched.map((type) => context.data.on(type, bump)),
      context.data.on("session.tool.success", (event) => {
        if (read(event.data.metadata)) bump();
      }),
    ];
    context.ui.slot({
      append: "sidebar.content",
      render: (props) => <Sidebar context={context} sessionID={props.sessionID} pulse={pulse} />,
    });
    return () => {
      clearTimeout(timer);
      offs.forEach((off) => off());
    };
  },
});
