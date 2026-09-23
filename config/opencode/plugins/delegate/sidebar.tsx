/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from "@opencode-ai/plugin/tui";
import type { Message, Session, SessionStatus } from "@opencode-ai/sdk/v2";
import { For, Show, createEffect, createMemo, createSignal, untrack, type Accessor } from "solid-js";
import { colors, pressureColor } from "../shared/colors.ts";
import { icons } from "../shared/icons.ts";
import { COMPACTION_LIMIT, formatTokens } from "../shared/session.ts";
import { ActionRow } from "../shared/action-row.tsx";
import { SidebarSection } from "../shared/sidebar-section.tsx";

const id = "delegate-lanes";
const DISMISSED = "delegate-lanes.dismissed";
const DISMISSED_TTL = 30 * 24 * 60 * 60 * 1000;

type Usage = { model: string; tokens: number };
type Status = SessionStatus["type"] | "limited";
type Dismissed = Record<string, number>;

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

function running(status: Status) {
  return status === "busy" || status === "retry";
}

function createDismissed(api: TuiPluginApi) {
  const current = () => api.kv.get<Dismissed>(DISMISSED, {});

  function save(next: Dismissed) {
    const cutoff = Date.now() - DISMISSED_TTL;
    api.kv.set(DISMISSED, Object.fromEntries(Object.entries(next).filter(([, time]) => time > cutoff)));
  }

  function dismiss(sessionIDs: string[]) {
    if (!sessionIDs.length) return;
    const now = Date.now();
    save({ ...current(), ...Object.fromEntries(sessionIDs.map((sessionID) => [sessionID, now])) });
  }

  function restore(sessionIDs: string[]) {
    const value = current();
    if (!sessionIDs.some((sessionID) => value[sessionID])) return;
    save(Object.fromEntries(Object.entries(value).filter(([sessionID]) => !sessionIDs.includes(sessionID))));
  }

  return { dismiss, restore, has: (sessionID: string) => current()[sessionID] !== undefined };
}

function createLanes(api: TuiPluginApi) {
  const [sessions, setSessions] = createSignal<Record<string, Session>>({});
  const [usages, setUsages] = createSignal<Record<string, Usage>>({});
  const [statuses, setStatuses] = createSignal<Record<string, SessionStatus["type"]>>({});
  const dismissed = createDismissed(api);
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
    setSessions((current) => ({ ...current, ...Object.fromEntries(children.map((child) => [child.id, child])) }));
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
      dismissed.restore([sessionID]);
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
      if (type === "busy") dismissed.restore([sessionID]);
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
    return newest;
  }

  function status(session: Session): Status {
    if (delegate(session)?.context) return "limited";
    return statuses()[session.id] ?? api.state.session.status(session.id)?.type ?? "idle";
  }

  return { load, forParent, status, dismissed, usage: (sessionID: string) => usages()[sessionID] };
}

type Lanes = ReturnType<typeof createLanes>;

function Row(props: { api: TuiPluginApi; lanes: Lanes; name: string; child: Session }) {
  const theme = () => props.api.theme.current;
  const status = () => props.lanes.status(props.child);
  const measured = () => props.lanes.usage(props.child.id);
  const model = () => (measured()?.model ?? props.child.model?.id ?? "unknown").replace(/^claude-/, "");
  const icon = () => {
    const current = status();
    if (current === "limited") return icons.lane.limited;
    return running(current) ? icons.lane.busy : icons.lane.idle;
  };
  const tone = () => {
    const c = colors(theme());
    const current = status();
    if (current === "limited") return c.red;
    if (current === "retry") return c.yellow;
    return current === "busy" ? c.green : theme().textMuted;
  };
  const spent = () => {
    const value = measured()?.tokens;
    return value ? formatTokens(value) : "";
  };
  const close = { icon: icons.error, run: () => props.lanes.dismissed.dismiss([props.child.id]) };

  return (
    <ActionRow
      api={props.api}
      action={running(status()) ? undefined : close}
      onPress={() => props.api.route.navigate("session", { sessionID: props.child.id })}
      below={
        <text fg={theme().textMuted} wrapMode="none">
          {`  [${props.child.agent ?? "unknown"} • ${model()}]`}
        </text>
      }
    >
      <text fg={tone()} wrapMode="none" flexShrink={0}>{`${icon()} `}</text>
      <text fg={theme().text} wrapMode="none" flexGrow={1} flexShrink={1}>
        {props.name}
      </text>
      <text
        fg={pressureColor(theme(), ((measured()?.tokens ?? 0) / COMPACTION_LIMIT) * 100)}
        wrapMode="none"
        flexShrink={0}
      >
        {spent()}
      </text>
    </ActionRow>
  );
}

function Panel(props: { api: TuiPluginApi; lanes: Lanes; sessionID: string }) {
  createEffect(() => void props.lanes.load(props.sessionID));
  const active = createMemo(() => props.lanes.forParent(props.sessionID));
  const names = createMemo(() =>
    [...active()]
      .filter(([, child]) => !props.lanes.dismissed.has(child.id))
      .map(([name]) => name)
      .toSorted(),
  );

  return (
    <Show when={names().length}>
      <SidebarSection api={props.api} title="Lanes" detail={names().length}>
        <box flexDirection="column">
          <For each={names()}>
            {(name) => (
              <Show when={active().get(name)}>
                {(child: Accessor<Session>) => <Row api={props.api} lanes={props.lanes} name={name} child={child()} />}
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
        run: () => {
          const idle = children().filter((child) => !running(lanes.status(child)) && !lanes.dismissed.has(child.id));
          lanes.dismissed.dismiss(idle.map((child) => child.id));
          api.ui.toast({ message: `Cleared ${idle.length} lane${idle.length === 1 ? "" : "s"}` });
        },
      },
      {
        name: "delegate.lanes.restore",
        title: "Restore cleared lanes",
        category: "Session",
        namespace: "palette",
        run: () => lanes.dismissed.restore(children().map((child) => child.id)),
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
