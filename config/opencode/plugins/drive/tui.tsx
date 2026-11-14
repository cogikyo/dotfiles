import { Plugin } from "@opencode/plugin/tui";
import { createEffect, Show } from "solid-js";
import { createStore } from "solid-js/store";
import { colors } from "../statusline/palette.ts";
import { Look } from "./look.tsx";
import { api } from "./rpc.ts";

type Context = Plugin.Context;

export default Plugin.define({
  id: "drive",
  setup(context) {
    const drive = connect(context);
    const root = (sessionID?: string) => (sessionID ? context.data.session.root(sessionID) : undefined);
    const busy = (id?: string) =>
      id !== undefined &&
      context.data.session.family(id).some((child) => child !== id && context.data.session.status(child) === "running");

    context.ui.slot({
      before: "prompt.footer.status",
      render: (input) => {
        createEffect(() => {
          const id = root(input.sessionID);
          if (id) drive.sync(id).catch(drive.fail);
        });
        return (
          <Show when={drive.on[root(input.sessionID) ?? ""]}>
            <text fg={colors.brightBlue} flexShrink={0}>
              <b>DRIVE</b>
            </text>
          </Show>
        );
      },
    });

    context.ui.slot({
      append: "prompt.footer",
      render: (input) => (
        <Look
          context={context}
          on={drive.on[root(input.sessionID) ?? ""] ?? false}
          busy={busy(root(input.sessionID))}
          fail={drive.fail}
        />
      ),
    });

    const toggle = () => drive.toggle().catch(drive.fail);
    context.keymap.layer(() => ({
      mode: "global",
      commands: [
        {
          id: "drive.toggle",
          title: "Toggle drive",
          description: "Turn drive on or off for this session tree",
          group: "Session",
          palette: true,
          slash: { name: "drive" },
          run: toggle,
        },
      ],
    }));
    context.keymap.layer(() => ({
      commands: [{ bind: "<leader>d", title: "Toggle drive", group: "Session", run: toggle }],
    }));

    return drive.close;
  },
});

function connect(context: Context) {
  const rpc = context.client.rpc(api);
  const [on, setOn] = createStore<Record<string, boolean>>({});
  const turns = new Map<string, number>();

  const options = (root: string) => {
    const session = context.data.session.get(root);
    return session ? { location: { directory: session.location.directory } } : undefined;
  };
  const fail = (error: unknown) =>
    context.ui.toast.show({
      message: `Drive: ${error instanceof Error ? error.message : JSON.stringify(error)}`,
      variant: "error",
    });
  const settle = async (root: string, request: Promise<{ root: string; on: boolean }>) => {
    const turn = turns.get(root) ?? 0;
    const state = await request;
    if ((turns.get(root) ?? 0) === turn) setOn(state.root, state.on);
    return state.on;
  };
  const sync = (root: string) => settle(root, rpc.state({ sessionID: root }, options(root)));

  const toggle = async () => {
    const route = context.ui.router.current();
    const root = route.type === "session" ? context.data.session.root(route.sessionID) : undefined;
    if (!root) return context.ui.toast.show({ message: "Open a session to toggle drive", variant: "warning" });
    const next = !(await sync(root));
    const state = await settle(root, rpc.set({ sessionID: root, on: next }, options(root)));
    context.ui.toast.show(
      state ? { message: "Drive on", variant: "warning" } : { message: "Drive off", variant: "info" },
    );
  };

  const changed = rpc.events.on("changed", (event) => {
    turns.set(event.data.root, (turns.get(event.data.root) ?? 0) + 1);
    setOn(event.data.root, event.data.on);
  });
  const connected = context.data.on("server.connected", () => {
    for (const root of Object.keys(on)) sync(root).catch(fail);
  });
  const close = () => {
    changed();
    connected();
  };

  return { on, sync, toggle, fail, close };
}
