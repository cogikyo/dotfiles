import { Plugin } from "@opencode/plugin/tui";
import type { Renderable } from "@opentui/core";
import { createMemo, createSignal, For, onCleanup, Show } from "solid-js";

type Context = Plugin.Context;
type Text = Renderable & { readonly textNode: { toChunks(): readonly { readonly text: string }[] } };

const icon = "\u{f051f}";
const limit = 3;
const label = /^\d+ queued(?:$| · )/;
const border = {
  topLeft: "",
  bottomLeft: "",
  vertical: "┃",
  topRight: "",
  bottomRight: "",
  horizontal: " ",
  bottomT: "",
  topT: "",
  cross: "",
  leftT: "",
  rightT: "",
};

const hidden = new WeakSet<Renderable>();

export default Plugin.define({
  id: "queue",
  setup(context) {
    return context.ui.slot({
      append: "session.composer.top",
      render: (input) => <Dock context={context} sessionID={input.sessionID} />,
    });
  },
});

function Dock(props: { context: Context; sessionID: string }) {
  const queued = createMemo(() =>
    props.context.data.session.pending
      .list(props.sessionID)
      .flatMap((item) => (item.type === "user" && item.delivery === "queue" ? [summary(item.payload.text)] : [])),
  );
  const blocked = createMemo(() => {
    const data = props.context.data;
    const session = data.session.get(props.sessionID);
    if (data.session.form.list("global", session?.location ?? props.context.location)?.length) return true;
    if (session?.parentID) return false;
    const ids = [props.sessionID, ...data.session.family(props.sessionID).filter((id) => id !== props.sessionID)];
    return ids.some((id) => data.session.permission.list(id)?.length || data.session.form.list(id)?.length);
  });
  const shown = createMemo(() => queued().length > 0 && !blocked());

  return (
    <box ref={(anchor) => hide(props.context, anchor, shown)} flexShrink={0}>
      <Show when={shown()}>
        <Panel context={props.context} lines={queued()} />
      </Show>
    </box>
  );
}

function Panel(props: { context: Context; lines: readonly string[] }) {
  const [hover, setHover] = createSignal(false);
  const theme = () => props.context.theme;
  const extra = () => Math.max(0, props.lines.length - limit);

  return (
    <box
      marginBottom={1}
      border={["left"]}
      borderColor={theme().border.base}
      customBorderChars={border}
      onMouseOver={() => setHover(true)}
      onMouseOut={() => setHover(false)}
      onMouseUp={() => props.context.keymap.dispatch("session.queued_prompts")}
    >
      <box
        width="100%"
        paddingTop={1}
        paddingBottom={1}
        paddingLeft={2}
        paddingRight={1}
        backgroundColor={hover() ? theme().decrease(theme().background.raised.base) : theme().background.raised.base}
        flexDirection="row"
      >
        <box flexDirection="column" flexGrow={1} flexShrink={1} minWidth={0}>
          <For each={props.lines.slice(0, limit)}>
            {(line, index) => (
              <text fg={index() === 0 ? theme().text.base : theme().text.muted} wrapMode="none" truncate>
                {line}
              </text>
            )}
          </For>
          <Show when={extra() > 0}>
            <text fg={theme().text.muted} wrapMode="none">{`+${extra()} more`}</text>
          </Show>
        </box>
        <text wrapMode="none" flexShrink={0} marginLeft={2}>
          <span style={{ fg: theme().text.feedback.warning.base }}>{icon}</span>
          <span style={{ fg: theme().text.base }}>{` ${props.lines.length}`}</span>
        </text>
      </box>
    </box>
  );
}

function hide(context: Context, anchor: Renderable, shown: () => boolean) {
  let dock: Renderable | undefined;
  anchor.onLifecyclePass = () => {
    dock = find(anchor);
    if (dock) toggle(dock, shown());
  };
  context.renderer.registerLifecyclePass(anchor);
  onCleanup(() => {
    anchor.onLifecyclePass = null;
    context.renderer.unregisterLifecyclePass(anchor);
    if (dock) toggle(dock, false);
  });
}

function toggle(dock: Renderable, shown: boolean) {
  if (!shown) {
    if (hidden.delete(dock) && !dock.isDestroyed) dock.visible = true;
    return;
  }
  if (!dock.visible) return;
  dock.visible = false;
  hidden.add(dock);
}

function summary(text: string) {
  return (
    text
      .split("\n")
      .map((line) => line.trim())
      .find(Boolean) ?? ""
  );
}

function find(anchor: Renderable) {
  const siblings = anchor.parent?.getChildren() ?? [];
  const index = siblings.indexOf(anchor);
  if (index < 0) return;
  return siblings.slice(0, index).findLast(core);
}

function core(node: Renderable) {
  if (!("border" in node) || !Array.isArray(node.border) || node.border.join() !== "left") return false;
  const [inner, ...rest] = node.getChildren();
  if (!inner || rest.length > 0) return false;
  const [text, ...more] = inner.getChildren();
  if (!text || more.length > 0 || !textual(text)) return false;
  return label.test(read(text));
}

function textual(node: Renderable): node is Text {
  return "textNode" in node;
}

function read(node: Text) {
  return node.textNode
    .toChunks()
    .map((chunk) => chunk.text)
    .join("");
}
