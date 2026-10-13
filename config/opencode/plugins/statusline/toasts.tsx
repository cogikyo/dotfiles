import type { Plugin } from "@opencode/plugin/tui";
import { TextAttributes, type MouseEvent, type Renderable, type RGBA } from "@opentui/core";
import {
  createEffect,
  createMemo,
  createSignal,
  For,
  on,
  onCleanup,
  Show,
  type Accessor,
  type ParentProps,
} from "solid-js";
import { motion, tick } from "./working.tsx";

type Context = Plugin.Context;

type Shown = {
  readonly surface: Renderable;
  readonly color: RGBA;
  readonly title?: string;
  readonly message: string;
  readonly action?: string;
  readonly pending?: string;
};

type Surface = Renderable & { readonly border: unknown; readonly borderColor: RGBA };
type Node = { readonly children: readonly (string | Node)[] };
type Text = Renderable & { readonly textNode: Node };

const estimate = 5_000;
const step = 1_000;
const limit = 3;
const minimum = 3;
const graphemes = new Intl.Segmenter();
const icons = {
  info: "\u{f02fd}",
  success: "\u{f05e1}",
  warning: "\u{f002a}",
  error: "\u{f015a}",
} as const;
const variants = ["info", "success", "warning", "error"] as const;
const affordance = /^(?:x|› (.+))$/s;
const more = /^\+\d+ more$/;

export function mirror(context: Context) {
  const [shown, setShown] = createSignal<Shown>();
  const hosts = new Set<Renderable>();

  const reveal = (keep?: Renderable) => {
    const surface = shown()?.surface;
    if (surface && surface !== keep && !surface.isDestroyed) surface.visible = true;
  };

  const sync = () => {
    const root = context.renderer.root;
    const next = [...hosts].some((host) => onscreen(host, root)) ? find(root) : undefined;
    reveal(next?.surface);
    if (next?.surface.visible) next.surface.visible = false;
    if (!same(shown(), next)) setShown(next);
  };

  const attach = (box: Renderable) => {
    box.onLifecyclePass = sync;
    context.renderer.registerLifecyclePass(box);
    hosts.add(box);
    return () => {
      box.onLifecyclePass = null;
      context.renderer.unregisterLifecyclePass(box);
      hosts.delete(box);
      if (hosts.size > 0) return;
      reveal();
      setShown(undefined);
    };
  };

  return (props: ParentProps) => (
    <box flexDirection="column" ref={(box) => onCleanup(attach(box))}>
      <Show when={shown()} fallback={props.children}>
        {(toast) => <Card context={context} toast={toast()} />}
      </Show>
    </box>
  );
}

function Card(props: { context: Context; toast: Shown }) {
  const theme = () => props.context.theme;
  const clock = countdown(createMemo(() => props.toast, undefined, { equals: alike }));
  const rows = createMemo(() => clip(props.toast.message));
  const variant = () => variants.find((name) => props.toast.color.equals(theme().text.feedback[name].base));
  const footer = () => Boolean(props.toast.pending || props.toast.action);
  const lift = () => {
    const count = (props.toast.title ? 1 : 0) + rows().length + (footer() ? 1 : 0);
    return Math.max(0, Math.ceil((minimum - 1 - count) / 2));
  };
  let bar: Renderable | undefined;

  const hover = (value: boolean, event: MouseEvent) => {
    forward(props.toast.surface, event);
    clock.hover(value);
  };

  return (
    <box
      flexDirection="row"
      minHeight={minimum}
      border={["left"]}
      borderStyle="heavy"
      borderColor={props.toast.color}
      backgroundColor={theme().background.raised.high}
      paddingLeft={1}
      paddingRight={1}
      onMouseOver={(event) => hover(true, event)}
      onMouseOut={(event) => hover(false, event)}
      onMouseUp={(event) => forward(props.toast.surface, event)}
    >
      <Show when={variant()}>
        {(name) => (
          <text fg={props.toast.color} wrapMode="none" flexShrink={0} marginRight={1} marginTop={lift()}>
            {icons[name()]}
          </text>
        )}
      </Show>
      <box flexGrow={1} flexShrink={1} minWidth={0} marginTop={lift()}>
        <Show when={props.toast.title}>{(title) => <Line text={title()} fg={theme().text.base} bold />}</Show>
        <For each={rows()}>{(row) => <Line text={row} fg={theme().text.base} />}</For>
        <Show when={footer()}>
          <box flexDirection="row" gap={1}>
            <text fg={theme().text.muted} wrapMode="none" truncate flexGrow={1} flexShrink={1} minWidth={0}>
              {props.toast.pending ?? ""}
            </text>
            <Show when={props.toast.action}>
              {(action) => (
                <text
                  fg={clock.hovered() ? theme().text.action.primary.base : theme().text.muted}
                  wrapMode="none"
                  flexShrink={0}
                >
                  {`› ${action()}`}
                </text>
              )}
            </Show>
          </box>
        </Show>
        <box flexGrow={1} />
        <Show when={clock.left() > 0}>
          <text fg={props.toast.color} wrapMode="none" height={1} ref={(text) => (bar = text)}>
            {"▁".repeat(Math.ceil((bar?.width ?? 0) * clock.left()))}
          </text>
        </Show>
      </box>
    </box>
  );
}

function Line(props: { text: string; fg: RGBA; bold?: boolean }) {
  const [width, setWidth] = createSignal(0);

  return (
    <text
      fg={props.fg}
      attributes={props.bold ? TextAttributes.BOLD : TextAttributes.NONE}
      wrapMode="none"
      ref={(node) => {
        node.onSizeChange = () => setWidth(node.width);
      }}
    >
      {fit(props.text, width())}
    </text>
  );
}

function fit(text: string, width: number) {
  if (width <= 0 || Bun.stringWidth(text) <= width) return text;
  let cut = "";
  let used = 0;
  for (const { segment } of graphemes.segment(text)) {
    const size = Bun.stringWidth(segment);
    if (used + size > width - 1) break;
    cut += segment;
    used += size;
  }
  return `${cut.trimEnd()}…`;
}

function countdown(identity: Accessor<Shown>) {
  const [hovered, setHovered] = createSignal(false);
  const [now, setNow] = createSignal(performance.now());
  let since = 0;
  let spent = 0;
  const elapsed = () => spent + (hovered() ? 0 : now() - since);

  createEffect(
    on(identity, () => {
      spent = 0;
      since = performance.now();
      setNow(since);
      const timer = setInterval(
        () => {
          setNow(performance.now());
          if (elapsed() >= estimate) clearInterval(timer);
        },
        motion() ? tick : step,
      );
      onCleanup(() => clearInterval(timer));
    }),
  );

  return {
    hovered,
    left: () => 1 - elapsed() / estimate,
    hover(value: boolean) {
      if (value === hovered()) return;
      const at = performance.now();
      if (value) spent += at - since;
      else since = at;
      setNow(at);
      setHovered(value);
    },
  };
}

function clip(message: string) {
  const rows = message.trimEnd().split("\n");
  if (rows.length <= limit) return rows;
  return [...rows.slice(0, limit - 1), `${rows[limit - 1]}…`];
}

function onscreen(node: Renderable, root: Renderable) {
  for (let current: Renderable | null = node; current; current = current.parent) {
    if (!current.visible) return false;
    if (current === root) return true;
  }
  return false;
}

function forward(surface: Renderable, event: MouseEvent) {
  if (surface.isDestroyed) return;
  const forwarded: MouseEvent = Object.create(event);
  forwarded.stopPropagation();
  surface.processMouseEvent(forwarded);
}

function find(root: Renderable) {
  for (const app of root.getChildren()) {
    const shown = parse(app.getChildren().at(-1));
    if (shown) return shown;
  }
}

function parse(node: Renderable | undefined): Shown | undefined {
  if (!node || node.top !== 1 || node.right !== 2 || !framed(node)) return;
  const [inner, ...extra] = children(node);
  if (!inner || extra.length > 0) return;
  const content = body(children(inner));
  return content && { surface: node, color: node.borderColor, ...content };
}

function body([row, ...rest]: Renderable[]) {
  const cells = row ? children(row) : [];
  const action = affordance.exec(read(cells.at(-1)) ?? "");
  if (!action) return;
  const titled = cells.length === 3 && !textual(cells[1]);
  if (!titled && cells.length !== 2) return;
  const title = titled ? read(cells[0]) : undefined;
  const message = read(titled ? rest.shift() : cells[0]);
  if ((titled && !title) || message === undefined || !counted(rest)) return;
  return { title, message, action: action[1], pending: read(rest[0]) };
}

function counted(rest: Renderable[]) {
  return rest.length === 0 || (rest.length === 1 && more.test(read(rest[0]) ?? ""));
}

function children(node: Renderable) {
  return node.getChildren().filter((child) => child.visible);
}

function framed(node: Renderable): node is Surface {
  return "borderColor" in node && "border" in node && Array.isArray(node.border) && node.border.join() === "left,right";
}

function textual(node: Renderable): node is Text {
  return "textNode" in node;
}

function read(node: Renderable | undefined) {
  if (!node || !textual(node)) return;
  return flatten(node.textNode);
}

function flatten(node: Node): string {
  return node.children.map((child) => (typeof child === "string" ? child : flatten(child))).join("");
}

function same(a: Shown | undefined, b: Shown | undefined) {
  if (!a || !b) return a === b;
  return alike(a, b) && a.pending === b.pending;
}

function alike(a: Shown, b: Shown) {
  return (
    a.surface === b.surface &&
    a.color.equals(b.color) &&
    a.title === b.title &&
    a.message === b.message &&
    a.action === b.action
  );
}
