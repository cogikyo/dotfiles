import { MouseButton, type MouseEvent } from "@opentui/core";
import { Plugin } from "@opencode/plugin/tui";
import { createMemo, For, Show } from "solid-js";

type Context = Plugin.Context;
type Session = ReturnType<Context["data"]["session"]["list"]>[number];
type Status = "running" | "idle" | "error" | "interrupted";
type Action = "steer" | "interrupt" | "continue" | "open";

const laneOf = (session: Session) => (typeof session.metadata?.lane === "string" ? session.metadata.lane : undefined);

function status(context: Context, session: Session): Status {
  if (context.data.session.status(session.id) === "running") return "running";
  if (session.outcome === "failed") return "error";
  return session.outcome === "interrupted" ? "interrupted" : "idle";
}

function lanes(context: Context, rootID: string) {
  return context.data.session
    .list()
    .filter((session) => session.parentID === rootID)
    .filter((session) => laneOf(session) !== undefined || status(context, session) !== "idle");
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
  const list = createMemo(() => (root() ? lanes(props.context, props.sessionID) : []));
  const click = (event: MouseEvent, session: Session) => {
    if (event.button === MouseButton.RIGHT) return void act(props.context, session);
    props.context.ui.router.navigate({ type: "session", sessionID: session.id });
  };
  const color = (value: Status) => {
    const text = props.context.theme.text;
    if (value === "error") return text.feedback.error.base;
    if (value === "interrupted") return text.feedback.warning.base;
    return value === "running" ? text.base : text.muted;
  };

  return (
    <Show when={list().length}>
      <box>
        <text fg={props.context.theme.text.base}>
          <b>Lanes</b>
        </text>
        <For each={list()}>
          {(session) => (
            <box flexDirection="row" gap={1} onMouseUp={(event) => click(event, session)}>
              <text fg={color(status(props.context, session))}>{status(props.context, session)}</text>
              <text fg={props.context.theme.text.base}>{laneOf(session) ?? "-"}</text>
              <text fg={props.context.theme.text.muted}>{session.agent ?? ""}</text>
            </box>
          )}
        </For>
      </box>
    </Show>
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
      ],
    }));
  },
});
