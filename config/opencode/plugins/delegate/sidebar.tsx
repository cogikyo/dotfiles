/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from "@opencode-ai/plugin/tui";
import type { Message, Session, SessionStatus } from "@opencode-ai/sdk/v2";
import { stringWidth } from "bun";
import { For, Show, createEffect, createMemo, createSignal, untrack, type Accessor } from "solid-js";
import { colors, pressureColor } from "../shared/colors.ts";
import { icons } from "../shared/icons.ts";
import { COMPACTION_LIMIT } from "../shared/session.ts";
import { ActionIcon } from "../shared/action-icon.tsx";
import { SidebarSection } from "../shared/sidebar-section.tsx";
import { closeBody, sessionClosed } from "./closed.ts";
import { errorMessage } from "./sdk.ts";

const id = "delegate-lanes";
const SPIN_MS = 100;
const FAMILIES: [RegExp, string][] = [
  [/^opus/, "opus"],
  [/sol/, "sol"],
  [/luna/, "luna"],
  [/astra/, "astra"],
  [/fable/, "fable"],
  [/grok/, "grok"],
];
const roles: Partial<Record<string, string>> = icons.role;
const scopes: Partial<Record<string, string>> = icons.scope;
const tones: Partial<Record<string, "secondary" | "error" | "success" | "syntaxOperator">> = {
  build: "secondary",
  review: "error",
  verify: "success",
  scout: "syntaxOperator",
};

type Usage = { model: string; tokens: number };
type Status = SessionStatus["type"] | "limited";

function delegate(session: Session): Record<string, unknown> | undefined {
  const value = session.metadata?.delegate;
  return value && typeof value === "object" ? Object.fromEntries(Object.entries(value)) : undefined;
}

function lane(session: Session) {
  const name = delegate(session)?.lane;
  return typeof name === "string" && name ? name : undefined;
}

function tokens(message: Message) {
  if (message.role !== "assistant") return 0;
  const value = message.tokens;
  return value.total || value.input + value.output + value.cache.read + value.cache.write;
}

function usage(message: Message): Usage | undefined {
  if (message.role !== "assistant") return undefined;
  const measured = tokens(message);
  if (!measured) return undefined;
  return { model: message.modelID, tokens: measured };
}

function family(model: string) {
  const bare = model.replace(/^.*\//, "").replace(/^claude-/, "");
  return FAMILIES.find(([pattern]) => pattern.test(bare))?.[1] ?? bare;
}

function agentLabel(agent: string) {
  const [role, ...rest] = agent.split("/");
  const glyph = roles[role];
  if (!glyph || !rest.length) return agent;
  const scope = rest.join("/");
  return `${glyph}/${scopes[scope] ?? scope}`;
}

function spentLabel(count: number) {
  const thousands = Math.round(count / 1_000);
  if (thousands >= 1_000) return `${(count / 1_000_000).toFixed(1)}M`;
  return count < 1_000 ? String(count) : `${thousands}K`;
}

function running(status: Status) {
  return status === "busy" || status === "retry";
}

function createSpinner(api: TuiPluginApi, busy: () => boolean) {
  const [frame, setFrame] = createSignal(0);
  let timer: ReturnType<typeof setInterval> | undefined;

  function stop() {
    clearInterval(timer);
    timer = undefined;
  }

  function sync() {
    const active = untrack(busy);
    if (active && !timer) timer = setInterval(() => setFrame((value) => value + 1), SPIN_MS);
    if (!active) stop();
  }

  api.lifecycle.onDispose(stop);
  return { frame, sync };
}

async function closeLane(api: TuiPluginApi, sessionID: string) {
  const current = await api.client.session.get({ sessionID });
  if (!current.data) throw new Error(`read ${sessionID}: ${errorMessage(current.error)}`);
  const updated = await api.client.session.update({ sessionID, ...closeBody(current.data, "operator") });
  if (updated.error) throw new Error(`close ${sessionID}: ${errorMessage(updated.error)}`);
}

async function closeLanes(api: TuiPluginApi, sessionIDs: string[]) {
  const results = await Promise.allSettled(sessionIDs.map((sessionID) => closeLane(api, sessionID)));
  const failures = results.flatMap((result) => (result.status === "rejected" ? [errorMessage(result.reason)] : []));
  if (failures.length) {
    api.ui.toast({
      variant: "error",
      title: `Failed to close ${failures.length} lane${failures.length === 1 ? "" : "s"}`,
      message: failures.join("\n"),
    });
  }
  return sessionIDs.length - failures.length;
}

function createLanes(api: TuiPluginApi) {
  const [sessions, setSessions] = createSignal<Record<string, Session>>({});
  const [usages, setUsages] = createSignal<Record<string, Usage>>({});
  const [statuses, setStatuses] = createSignal<Record<string, SessionStatus["type"]>>({});
  const spinner = createSpinner(api, () => Object.values(sessions()).some((session) => running(status(session))));
  const loaded = new Set<string>();

  const known = (sessionID: string) => untrack(() => sessionID in sessions());
  const put = (session: Session) => setSessions((current) => ({ ...current, [session.id]: session }));
  const measure = (sessionID: string, value: Usage) => setUsages((current) => ({ ...current, [sessionID]: value }));

  async function loadUsage(sessionID: string) {
    const result = await api.client.session.messages({ sessionID, limit: 5 });
    const latest = (result.data ?? []).map((item) => usage(item.info)).findLast(Boolean);
    if (latest) measure(sessionID, latest);
  }

  async function load(parentID: string) {
    if (loaded.has(parentID)) return;
    loaded.add(parentID);
    const response = await api.client.session.children({ sessionID: parentID });
    if (response.error || !response.data) {
      loaded.delete(parentID);
      return;
    }
    const children = response.data.filter((child) => lane(child));
    setSessions((current) => ({ ...Object.fromEntries(children.map((child) => [child.id, child])), ...current }));
    spinner.sync();
    await Promise.all(children.map((child) => loadUsage(child.id)));
  }

  const disposers = [
    api.event.on("session.created", (event) => {
      if (lane(event.properties.info)) put(event.properties.info);
    }),
    api.event.on("session.updated", (event) => {
      if (lane(event.properties.info)) put(event.properties.info);
    }),
    api.event.on("session.deleted", (event) => {
      const sessionID = event.properties.sessionID;
      if (!known(sessionID)) return;
      setSessions(({ [sessionID]: _, ...rest }) => rest);
      spinner.sync();
    }),
    api.event.on("message.updated", (event) => {
      if (!known(event.properties.sessionID)) return;
      const value = usage(event.properties.info);
      if (value) measure(event.properties.sessionID, value);
    }),
    api.event.on("session.status", (event) => {
      const sessionID = event.properties.sessionID;
      const type = event.properties.status.type;
      if (!known(sessionID)) return;
      setStatuses((current) => ({ ...current, [sessionID]: type }));
      spinner.sync();
    }),
  ];
  api.lifecycle.onDispose(() => disposers.forEach((dispose) => dispose()));

  function forParent(parentID: string) {
    const newest = new Map<string, Session>();
    for (const child of Object.values(sessions())) {
      const name = lane(child);
      if (!name || child.parentID !== parentID) continue;
      const seen = newest.get(name);
      if (!seen || child.time.created > seen.time.created) newest.set(name, child);
    }
    for (const [name, child] of newest) if (sessionClosed(child)) newest.delete(name);
    return newest;
  }

  function status(session: Session): Status {
    if (delegate(session)?.context) return "limited";
    return statuses()[session.id] ?? api.state.session.status(session.id)?.type ?? "idle";
  }

  return { load, forParent, status, frame: spinner.frame, usage: (child: string) => usages()[child] };
}

type Lanes = ReturnType<typeof createLanes>;
type Columns = { model: number; spent: number };

function labels(lanes: Lanes, child: Session) {
  const measured = lanes.usage(child.id);
  return {
    model: family(measured?.model ?? child.model?.id ?? "unknown"),
    spent: measured ? spentLabel(measured.tokens) : "",
  };
}

function Row(props: { api: TuiPluginApi; lanes: Lanes; name: string; child: Session; columns: Columns }) {
  const theme = () => props.api.theme.current;
  const status = () => props.lanes.status(props.child);
  const measured = () => props.lanes.usage(props.child.id);
  const icon = () => {
    const current = status();
    if (current === "limited") return icons.lane.limited;
    if (!running(current)) return icons.lane.idle;
    const frames = icons.spinner.braille;
    return frames[props.lanes.frame() % frames.length];
  };
  const tone = () => {
    const c = colors(theme());
    const current = status();
    if (current === "limited") return c.red;
    if (current === "retry") return c.yellow;
    return current === "busy" ? c.green : theme().textMuted;
  };
  const spent = () =>
    props.columns.spent ? ` ${labels(props.lanes, props.child).spent.padStart(props.columns.spent)}` : "";
  const agent = () => props.child.agent ?? "unknown";
  const mode = () => {
    const key = tones[agent().split("/")[0]];
    return key ? theme()[key] : theme().textMuted;
  };
  const model = () => ` ${labels(props.lanes, props.child).model.padStart(props.columns.model)}]`;
  const bracket = () => ` [${agentLabel(agent())}${model()}`;
  const [width, setWidth] = createSignal<number>();
  const [hovered, setHovered] = createSignal(false);
  const name = () => {
    const room = width();
    if (room === undefined) return props.name;
    const budget = room - stringWidth(`${icons.lane.idle} ${bracket()}${spent()}`);
    return stringWidth(props.name) <= budget ? props.name : `${props.name.slice(0, Math.max(0, budget - 1))}…`;
  };
  const close = { icon: icons.error, run: () => void closeLanes(props.api, [props.child.id]) };

  return (
    <box
      flexDirection="row"
      gap={0}
      onMouseDown={() => props.api.route.navigate("session", { sessionID: props.child.id })}
      onMouseOver={() => setHovered(true)}
      onMouseOut={() => setHovered(false)}
      onSizeChange={function () {
        setWidth(this.width);
      }}
    >
      <ActionIcon api={props.api} icon={icon()} fg={tone()} action={running(status()) ? undefined : close} />
      <text fg={hovered() ? theme().secondary : theme().text} wrapMode="none" flexShrink={0} flexGrow={1}>
        {name()}
      </text>
      <text fg={theme().textMuted} wrapMode="none" flexShrink={0}>
        {" ["}
        <span style={{ fg: mode() }}>{agentLabel(agent())}</span>
        {model()}
      </text>
      <text
        fg={pressureColor(theme(), ((measured()?.tokens ?? 0) / COMPACTION_LIMIT) * 100)}
        wrapMode="none"
        flexShrink={0}
      >
        {spent()}
      </text>
    </box>
  );
}

function Panel(props: { api: TuiPluginApi; lanes: Lanes; sessionID: string }) {
  createEffect(() => void props.lanes.load(props.sessionID));
  const active = createMemo(() => props.lanes.forParent(props.sessionID));
  const names = createMemo(() => [...active().keys()].toSorted());
  const columns = createMemo(() => {
    const shown = names()
      .flatMap((name) => active().get(name) ?? [])
      .map((child) => labels(props.lanes, child));
    return {
      model: Math.max(0, ...shown.map((label) => label.model.length)),
      spent: Math.max(0, ...shown.map((label) => label.spent.length)),
    };
  });

  return (
    <Show when={names().length}>
      <SidebarSection api={props.api} title="Lanes" detail={names().length}>
        <box flexDirection="column">
          <For each={names()}>
            {(name) => (
              <Show when={active().get(name)}>
                {(child: Accessor<Session>) => (
                  <Row api={props.api} lanes={props.lanes} name={name} child={child()} columns={columns()} />
                )}
              </Show>
            )}
          </For>
        </box>
      </SidebarSection>
    </Show>
  );
}

function routeParent(api: TuiPluginApi) {
  const route = api.route.current;
  const sessionID = route.name === "session" ? route.params?.sessionID : undefined;
  if (typeof sessionID !== "string") return undefined;
  return api.state.session.get(sessionID)?.parentID ?? sessionID;
}

function registerCommands(api: TuiPluginApi, lanes: Lanes) {
  const children = () => {
    const parentID = routeParent(api);
    return parentID ? [...lanes.forParent(parentID).values()] : [];
  };
  const dispose = api.keymap.registerLayer({
    commands: [
      {
        name: "delegate.lanes.clear",
        title: "Clear idle lanes",
        category: "Session",
        namespace: "palette",
        run: async () => {
          const idle = children().filter((child) => !running(lanes.status(child)));
          const cleared = await closeLanes(
            api,
            idle.map((child) => child.id),
          );
          api.ui.toast({ message: `Cleared ${cleared} lane${cleared === 1 ? "" : "s"}` });
        },
      },
    ],
  });
  api.lifecycle.onDispose(dispose);
}

const tui: TuiPlugin = async (api) => {
  const lanes = createLanes(api);
  registerCommands(api, lanes);
  api.slots.register({
    order: 200,
    slots: {
      sidebar_content(_ctx, props: { session_id: string }) {
        return (
          <Show when={!api.state.session.get(props.session_id)?.parentID}>
            <Panel api={api} lanes={lanes} sessionID={props.session_id} />
          </Show>
        );
      },
    },
  });
};

export default { id, tui } satisfies TuiPluginModule & { id: string };
