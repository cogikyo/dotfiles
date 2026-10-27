import type { DialogSelectOption } from "@opencode/plugin/tui/context";
import { TextAttributes, type RGBA } from "@opentui/core";
import path from "node:path";
import { createEffect, createMemo, createSignal, Match, Switch, type JSX } from "solid-js";
import { spinner } from "../context/section.tsx";
import { roles } from "../statusline/palette.ts";
import { mix } from "../statusline/working.tsx";
import type { Context, Session, Source } from "./source.ts";
import { tag, title, type State } from "./tags.ts";

export type Kind = "open" | "list";

export type Picker = {
  readonly context: Context;
  readonly source: Source;
  readonly hold: (kind: Kind) => () => void;
  readonly claim: () => () => boolean;
};

/** The host spreads fields omitted from plugin types into DialogSelect for search, titles, and status markers. */
export type Option<Value> = DialogSelectOption<Value> & {
  readonly searchText?: string;
  readonly titleView?: () => JSX.Element;
  readonly gutter?: (color: RGBA) => JSX.Element;
};

type Attention = "permission" | "question" | false;

type Live = {
  readonly active?: () => string[];
  readonly permission: { readonly sessions?: () => string[] };
  readonly form: { readonly sessions?: () => string[] };
};

type Located = { readonly directory: string; readonly project?: { readonly id: string; readonly canonical: string } };

export const located = (context: Context): Located | undefined => context.location;

const TITLE_WIDTH = 61;

export const dialog = (context: Context) => context.theme.surface("dialog");

function members(context: Context, sessionID: string) {
  const family = context.data.session.family(sessionID);
  return family.includes(sessionID) ? family : [sessionID, ...family];
}

export function attention(context: Context, sessionID: string): Attention {
  const ids = members(context, sessionID);
  const { permission, form } = context.data.session;
  if (ids.some((id) => (permission.list(id)?.length ?? 0) > 0)) return "permission";
  if (ids.some((id) => (form.list(id)?.length ?? 0) > 0)) return "question";
  return false;
}

const running = (context: Context, sessionID: string) =>
  members(context, sessionID).some((id) => context.data.session.status(id) === "running");

export function order(context: Context, list: Session[]) {
  const rank = new Map(
    list.map((session) => [session.id, attention(context, session.id) ? 0 : tag(session.title)?.state ? 2 : 1]),
  );
  return list.toSorted((a, b) => (rank.get(a.id) ?? 1) - (rank.get(b.id) ?? 1) || b.time.updated - a.time.updated);
}

export function watch(context: Context, known: () => Session[]) {
  const live = context.data.session as Context["data"]["session"] & Live;
  createEffect(() => {
    const active = live.active?.() ?? [];
    for (const id of active) {
      void live.permission.sync(id).catch(() => undefined);
      void live.form.sync(id).catch(() => undefined);
    }
    const ids = new Set(known().map((session) => session.id));
    const pending = [...(live.permission.sessions?.() ?? []), ...(live.form.sessions?.() ?? []), ...active];
    for (const root of new Set(pending.map((id) => live.root(id)))) {
      if (!ids.has(root) && !live.get(root)) void live.sync(root).catch(() => undefined);
    }
  });
}

function Spin(props: { color: RGBA }) {
  const frame = spinner(() => true);
  return <text fg={props.color}>{frame()}</text>;
}

function Mark(props: { context: Context; sessionID: string; current: boolean; color: RGBA }) {
  const theme = () => dialog(props.context);
  const state = () => attention(props.context, props.sessionID);
  const busy = () => !state() && running(props.context, props.sessionID);
  return (
    <Switch fallback={<text fg={props.color}>{props.current ? "●" : " "}</text>}>
      <Match when={state()}>
        {(kind) => (
          <text
            fg={
              props.color.equals(theme().text.action.primary.focused) ? props.color : theme().text.feedback.warning.base
            }
            attributes={TextAttributes.BOLD}
          >
            {kind() === "permission" ? "!" : "?"}
          </text>
        )}
      </Match>
      <Match when={busy()}>
        <Spin color={props.color} />
      </Match>
    </Switch>
  );
}

export function row<Value>(context: Context, session: Session, value: Value, current: boolean): Option<Value> {
  const text = title(session);
  const [plain, setPlain] = createSignal(true);
  const gutter = (color: RGBA) => {
    setPlain(color.equals(dialog(context).text.base));
    return <Mark context={context} sessionID={session.id} current={current} color={color} />;
  };
  const state = tag(session.title)?.state;
  if (!state) return { title: text, value, gutter };
  const shown = text.length > TITLE_WIDTH ? `${text.slice(0, TITLE_WIDTH - 1)}…` : text;
  return {
    title: text,
    value,
    gutter,
    titleView: () => (plain() ? <span style={{ fg: faded(context, state) }}>{shown}</span> : shown),
  };
}

function faded(context: Context, state: State) {
  const theme = dialog(context);
  if (state === "research" && roles.scout) return mix(roles.scout, theme.background.base, 0.4);
  return theme.text.muted;
}

const signature = (option: Option<unknown>) =>
  [
    JSON.stringify(option.value),
    option.title,
    option.footer ?? "",
    option.category ?? "",
    option.searchText ?? "",
  ].join("\u0000");

export function stable<Value>(build: () => Option<Value>[]) {
  return createMemo(build, [], {
    equals: (a, b) => a.length === b.length && a.every((option, at) => signature(option) === signature(b[at])),
  });
}

export function projectName(project?: { canonical: string; name?: string }) {
  if (!project || project.canonical === "/") return undefined;
  return project.name || path.basename(project.canonical);
}

export function timeAgo(timestamp: number) {
  const minutes = Math.floor((Date.now() - timestamp) / 60_000);
  if (minutes < 1) return "now";
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo`;
  return `${Math.floor(days / 365)}y`;
}

export const width = (value: string) => Bun.stringWidth(value);

export function clip(value: string, max: number) {
  if (max <= 0) return "";
  if (width(value) <= max) return value;
  const root = value.startsWith("/") ? "/" : "";
  const segments = value.slice(root.length).split("/").filter(Boolean);
  const prefix = `${root}…/`;
  let kept = segments.at(-1) ?? value;
  if (segments.length < 2 || width(prefix + kept) > max) return `${value.slice(0, max - 1)}…`;
  for (const segment of segments.slice(0, -1).toReversed()) {
    if (width(`${prefix}${segment}/${kept}`) > max) break;
    kept = `${segment}/${kept}`;
  }
  return prefix + kept;
}

/** DialogSelect focuses a FILTER input after mounting; the plugin API exposes no onFilter hook. */
export function listen(context: Context, set: (query: string) => void) {
  const seen = new WeakSet<object>();
  const editor = context.renderer.currentFocusedEditor;
  if (editor) seen.add(editor);
  let tries = 0;
  const attach = () => {
    const input = context.renderer.currentFocusedEditor;
    if (input && !seen.has(input) && !input.isDestroyed && input.traits.status === "FILTER") {
      set(input.plainText);
      input.on("input", set);
      return;
    }
    if (++tries < 20) setTimeout(attach, 10);
  };
  setTimeout(attach, 0);
}
