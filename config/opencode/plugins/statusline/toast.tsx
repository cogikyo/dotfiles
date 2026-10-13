import type { Plugin } from "@opencode/plugin/tui";
import type { MouseEvent, Renderable, RGBA } from "@opentui/core";
import { createSignal, onCleanup, Show, type ParentProps } from "solid-js";

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

const icon = "\u{f0f3}";
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

  return (
    <box
      border={["left"]}
      borderStyle="heavy"
      borderColor={props.toast.color}
      backgroundColor={theme().background.raised.high}
      paddingLeft={1}
      paddingRight={1}
      onMouseUp={(event) => forward(props.toast.surface, event)}
    >
      <box flexDirection="row" gap={1}>
        <text fg={props.toast.color} wrapMode="none" flexShrink={0}>
          {icon}
        </text>
        <text fg={theme().text.base} wrapMode="word" flexGrow={1} flexShrink={1} minWidth={0}>
          <Show when={props.toast.title} fallback={props.toast.message}>
            {(title) => <b>{title()}</b>}
          </Show>
        </text>
        <text fg={theme().text.muted} wrapMode="none" flexShrink={0}>
          {props.toast.action ? `› ${props.toast.action}` : "✕"}
        </text>
      </box>
      <Show when={props.toast.title}>
        <text fg={theme().text.base} wrapMode="word">
          {props.toast.message}
        </text>
      </Show>
      <Show when={props.toast.pending}>
        {(pending) => (
          <text fg={theme().text.muted} wrapMode="none">
            {pending()}
          </text>
        )}
      </Show>
    </box>
  );
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
  return (
    a.surface === b.surface &&
    a.color.equals(b.color) &&
    a.title === b.title &&
    a.message === b.message &&
    a.action === b.action &&
    a.pending === b.pending
  );
}
