import type { ColorInput } from "@opentui/core";
import { Plugin } from "@opencode/plugin/tui";
import { createSignal, For, Show } from "solid-js";
import { usageProviderList } from "./providers.ts";
import { Definition } from "./rpc.ts";
import type { ProviderUsage } from "./types.ts";

const CLOCK_MS = 60_000;
const SETTLE_MS = 5_000;
const BAR_WIDTH = 10;
const DURATION_WIDTH = 7;
const EXACT_WIDTH = 10;
const PERCENT_WIDTH = 3;
const DASH = "--";
const PLACEHOLDER_LABELS = ["H", "W"];
const PRESSURE = [
  "#7492ef",
  "#9db2f4",
  "#95cb79",
  "#9fd883",
  "#f5d599",
  "#f5c069",
  "#f08898",
  "#f29ca9",
  "#ea76c0",
] as const;

type Usage = Pick<ProviderUsage, "id" | "label" | "note" | "noteKind"> & {
  readonly windows: ReadonlyArray<ProviderUsage["windows"][number]>;
  readonly placeholders?: ReadonlyArray<string>;
};

export default Plugin.define({
  id: "usage",
  setup(context) {
    const rpc = context.client.rpc(Definition);
    const [providers, setProviders] = createSignal<readonly Usage[]>([]);
    const [refreshing, setRefreshing] = createSignal<ReadonlySet<string>>(new Set());
    const [now, setNow] = createSignal(Date.now());

    const unavailable = () =>
      setProviders(
        usageProviderList.map(({ id, label }) => ({
          id,
          label,
          windows: [],
          note: "unavailable",
          noteKind: "error" as const,
        })),
      );
    const mark = (providerID: string, active: boolean) =>
      setRefreshing((current) => {
        const next = new Set(current);
        if (active) next.add(providerID);
        else next.delete(providerID);
        return next;
      });
    const refresh = (providerID?: string) => {
      if (providerID && refreshing().has(providerID)) return;
      if (providerID) mark(providerID, true);
      void rpc
        .refresh({ providerID })
        .then((snapshot) => setProviders(snapshot.providers))
        .catch(unavailable)
        .finally(() => providerID && mark(providerID, false));
    };

    let settle: ReturnType<typeof setTimeout> | undefined;
    const schedule = () => {
      if (settle) clearTimeout(settle);
      settle = setTimeout(() => refresh(), SETTLE_MS);
    };

    void rpc
      .list({})
      .then((snapshot) => setProviders(snapshot.providers))
      .catch(unavailable);
    const clock = setInterval(() => setNow(Date.now()), CLOCK_MS);
    const disposers = [
      rpc.events.on("updated", (event) => {
        setProviders(event.data.providers);
      }),
      context.data.on("session.execution.succeeded", schedule),
      context.ui.slot({
        append: "sidebar.content",
        render: (input) => (
          <Show when={!context.data.session.get(input.sessionID)?.parentID}>
            <Dashboard
              context={context}
              providers={providers()}
              activeID={context.data.session.get(input.sessionID)?.model?.providerID ?? ""}
              refreshing={refreshing()}
              now={now()}
              onRefresh={refresh}
            />
          </Show>
        ),
      }),
    ];

    return () => {
      clearInterval(clock);
      if (settle) clearTimeout(settle);
      for (const dispose of disposers) dispose();
    };
  },
});

function Dashboard(props: {
  context: Plugin.Context;
  providers: readonly Usage[];
  activeID: string;
  refreshing: ReadonlySet<string>;
  now: number;
  onRefresh: (providerID: string) => void;
}) {
  const theme = () => props.context.theme;
  return (
    <box flexDirection="column" gap={0} paddingLeft={1}>
      <For each={props.providers}>
        {(provider) => (
          <box flexDirection="column" gap={0}>
            <Header
              context={props.context}
              provider={provider}
              active={provider.id === props.activeID}
              refreshing={props.refreshing.has(provider.id)}
              onRefresh={() => props.onRefresh(provider.id)}
            />
            <Show
              when={provider.windows.length > 0}
              fallback={
                <For each={provider.placeholders ?? PLACEHOLDER_LABELS}>
                  {(label) => (
                    <Row
                      context={props.context}
                      label={label}
                      percent={DASH.padEnd(PERCENT_WIDTH, " ")}
                      percentColor={theme().text.muted}
                      bar={"░".repeat(BAR_WIDTH)}
                      barColor={theme().text.muted}
                      duration={DASH}
                      exact={DASH}
                    />
                  )}
                </For>
              }
            >
              <For each={provider.windows}>
                {(window) => {
                  const reset = () => (window.resetAt ? formatReset(window.resetAt, props.now) : undefined);
                  const pct = window.usedPercent;
                  return (
                    <Row
                      context={props.context}
                      label={window.label}
                      percent={pct !== undefined ? formatPercent(pct) : DASH.padEnd(PERCENT_WIDTH, " ")}
                      percentColor={pct !== undefined ? pressureColor(pct) : theme().text.muted}
                      bar={pct !== undefined ? usageBar(pct) : "░".repeat(BAR_WIDTH)}
                      barColor={pct !== undefined ? pressureColor(pct) : theme().text.muted}
                      marker={paceMarker(window.label, window.resetAt, props.now)}
                      markerColor={paceColor(props.context, window, props.now)}
                      duration={reset()?.duration ?? ""}
                      exact={reset()?.exact ?? ""}
                    />
                  );
                }}
              </For>
            </Show>
          </box>
        )}
      </For>
    </box>
  );
}

function Header(props: {
  context: Plugin.Context;
  provider: Usage;
  active: boolean;
  refreshing: boolean;
  onRefresh: () => void;
}) {
  const theme = () => props.context.theme;
  const heading = () => props.provider.label.match(/^(.*?) (\[[^\]]+\])$/);
  const labelColor = () => (props.refreshing || props.active ? theme().text.action.primary.base : theme().text.base);
  return (
    <box flexDirection="row" gap={0} onMouseDown={() => props.onRefresh()}>
      <text fg={labelColor()}>
        <b>{heading()?.[1] ?? props.provider.label}</b>
      </text>
      <Show when={props.provider.note}>
        <text fg={noteColor(props.context, props.provider)}>{` ${props.provider.note}`}</text>
      </Show>
      <Show when={props.refreshing && !props.provider.note}>
        <text fg={theme().text.action.primary.base}>{` refreshing`}</text>
      </Show>
      <Show when={heading()?.[2]}>
        <box flexGrow={1} minWidth={1} />
        <text fg={labelColor()} flexShrink={0}>
          <b>{heading()?.[2]}</b>
        </text>
      </Show>
    </box>
  );
}

function Row(props: {
  context: Plugin.Context;
  label: string;
  percent: string;
  percentColor: ColorInput;
  bar: string;
  barColor: ColorInput;
  marker?: number;
  markerColor?: ColorInput;
  duration: string;
  exact: string;
}) {
  const muted = () => props.context.theme.text.muted;
  return (
    <box flexDirection="row" gap={0}>
      <text fg={muted()}>{props.label.padEnd(2, " ")}</text>
      <text fg={props.percentColor}>{`${props.percent} `}</text>
      <Show when={props.marker !== undefined} fallback={<text fg={props.barColor}>{`${props.bar} `}</text>}>
        <text fg={props.barColor}>{props.bar.slice(0, props.marker)}</text>
        <text fg={props.markerColor ?? muted()}>┃</text>
        <text fg={props.barColor}>{`${props.bar.slice((props.marker ?? 0) + 1)} `}</text>
      </Show>
      <text fg={muted()}>{`${props.duration.padStart(DURATION_WIDTH, " ")} `}</text>
      <box flexGrow={1} />
      <text fg={muted()}>{props.exact.padStart(EXACT_WIDTH, " ")}</text>
    </box>
  );
}

function formatReset(resetAt: string, now: number) {
  const resetDate = new Date(resetAt);
  const ms = resetDate.getTime() - now;
  if (!Number.isFinite(ms)) return { duration: DASH, exact: DASH };
  if (ms <= 0) return { duration: "now", exact: "now" };

  const totalMinutes = Math.ceil(ms / 60_000);
  const days = Math.floor(totalMinutes / (24 * 60));
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60);
  const minutes = totalMinutes % 60;
  const duration =
    days > 0
      ? `${days}d ${String(hours).padStart(2, "0")}h`
      : hours > 0
        ? `${hours}h ${String(minutes).padStart(2, "0")}m`
        : `${String(minutes).padStart(2, "0")}m`;

  const sameDay = resetDate.toDateString() === new Date(now).toDateString();
  const time = new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit", hour12: false }).format(
    resetDate,
  );
  const date = `${resetDate.getMonth() + 1}/${String(resetDate.getDate()).padStart(2, "0")}`;
  return { duration, exact: sameDay ? time : `${date} ${time}` };
}

function usageBar(percent: number) {
  const filled = Math.max(0, Math.min(BAR_WIDTH, Math.round((percent / 100) * BAR_WIDTH)));
  return "█".repeat(filled) + "░".repeat(BAR_WIDTH - filled);
}

function pressureColor(usedPercent: number) {
  const percent = Number.isFinite(usedPercent) ? Math.max(0, Math.min(100, usedPercent)) : 0;
  const tier = [15, 30, 45, 60, 70, 80, 90, 95].findIndex((limit) => percent < limit);
  return PRESSURE[tier === -1 ? PRESSURE.length - 1 : tier];
}

function previousMonth(reset: Date) {
  const start = new Date(reset);
  const day = start.getUTCDate();
  start.setUTCDate(1);
  start.setUTCMonth(start.getUTCMonth() - 1);
  const lastDay = new Date(Date.UTC(start.getUTCFullYear(), start.getUTCMonth() + 1, 0)).getUTCDate();
  start.setUTCDate(Math.min(day, lastDay));
  return start.getTime();
}

function pacePercent(label: string, resetAt: string | undefined, now: number) {
  if (!resetAt) return undefined;
  const reset = new Date(resetAt);
  const end = reset.getTime();
  if (!Number.isFinite(end)) return undefined;

  const start =
    label === "H"
      ? end - 5 * 60 * 60 * 1000
      : label === "M" || label === "C" || label === "O"
        ? previousMonth(reset)
        : end - 7 * 24 * 60 * 60 * 1000;
  const elapsed = (now - start) / (end - start);
  if (!Number.isFinite(elapsed) || elapsed < 0 || elapsed > 1) return undefined;
  return elapsed * 100;
}

function paceMarker(label: string, resetAt: string | undefined, now: number) {
  const percent = pacePercent(label, resetAt, now);
  if (percent === undefined || percent < 10) return undefined;
  return Math.min(BAR_WIDTH - 1, Math.floor((percent / 100) * BAR_WIDTH));
}

function paceColor(context: Plugin.Context, window: Usage["windows"][number], now: number) {
  const expected = pacePercent(window.label, window.resetAt, now);
  const percent = window.usedPercent;
  if (expected === undefined || percent === undefined) return context.theme.text.muted;
  return percent <= expected ? context.theme.text.action.primary.base : context.theme.text.feedback.error.base;
}

function formatPercent(percent: number) {
  const rounded = Math.max(0, Math.min(100, Math.round(percent)));
  return rounded >= 100 ? "100" : `${String(rounded).padStart(2, "0")}%`;
}

function noteColor(context: Plugin.Context, provider: Usage) {
  const { theme } = context;
  if (provider.noteKind === "info") return theme.text.muted;
  if (provider.noteKind === "warn") return theme.text.feedback.warning.base;
  if (provider.noteKind === "error") return theme.text.feedback.error.base;
  if (provider.windows.length > 0 && provider.note?.startsWith("stale ")) return theme.text.muted;
  return theme.text.feedback.error.base;
}
