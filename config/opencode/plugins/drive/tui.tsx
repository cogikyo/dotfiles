import { Plugin } from "@opencode/plugin/tui";
import { createEffect, createSignal, Show } from "solid-js";
import { createStore } from "solid-js/store";
import { colors } from "../statusline/palette.ts";
import { api, type Entry } from "./rpc.ts";
import { title } from "./title.ts";

const poll = 250;

const AWAY =
  "Drive mode is on and the user is away, so nobody will answer this question. Decide yourself, record the decision and your reasons, and continue.";

export default Plugin.define({
  id: "drive",
  setup(context) {
    const rpc = context.client.rpc(api);
    const [armed, setArmed] = createStore<Record<string, boolean>>({});
    const [decorated, setDecorated] = createSignal<string>();
    const decor = title(context.renderer.root);

    const options = (root: string) => {
      const session = context.data.session.get(root);
      return session ? { location: { directory: session.location.directory } } : undefined;
    };
    const current = () => {
      const route = context.ui.router.current();
      return route.type === "session" ? context.data.session.root(route.sessionID) : undefined;
    };
    const mark = () => {
      const root = current();
      if (!root || !armed[root]) {
        decor.hide();
        setDecorated(undefined);
        return;
      }
      setDecorated(decor.show(busy(context, root)) ? root : undefined);
    };
    const sync = async (root: string) => {
      const state = await rpc.state({ sessionID: root }, options(root));
      setArmed(state.root, state.armed);
      mark();
      return state.armed;
    };
    const fail = (error: unknown) =>
      context.ui.toast.show({
        message: `Drive: ${error instanceof Error ? error.message : JSON.stringify(error)}`,
        variant: "error",
      });

    const toggle = async () => {
      const root = current();
      if (!root) return context.ui.toast.show({ message: "Open a session to toggle drive mode", variant: "warning" });
      const state = await rpc.arm({ sessionID: root, armed: !(armed[root] ?? (await sync(root))) }, options(root));
      setArmed(state.root, state.armed);
      mark();
      context.ui.toast.show(
        state.armed
          ? { message: "Drive mode on: a reviewer answers permission requests", variant: "warning" }
          : { message: "Drive mode off", variant: "info" },
      );
      if (!state.armed) return;
      const request = options(state.root);
      await handoff(context, request, (sessions) => rpc.sweep({ sessionID: state.root, sessions }, request));
    };

    const review = async () => {
      const root = current();
      if (!root)
        return context.ui.toast.show({ message: "Open a session to review drive decisions", variant: "warning" });
      const entries = await rpc.journal({ sessionID: root, limit: 100 }, options(root));
      if (entries.length === 0) return context.ui.toast.show({ message: "No drive decisions for this session yet" });
      const entry = await context.ui.dialog.select({
        title: "Drive decisions",
        placeholder: "Search decisions",
        options: entries.map(choice),
      });
      if (entry) await context.ui.dialog.alert({ title: `Drive ${entry.decision}`, message: details(entry) });
    };

    context.ui.slot({
      before: "prompt.footer.status",
      render: (input) => {
        const root = () => (input.sessionID ? context.data.session.root(input.sessionID) : undefined);
        createEffect(() => {
          const id = root();
          if (id && !(id in armed)) sync(id).catch(fail);
        });
        const key = () => root() ?? "";
        return <Badge context={context} visible={(armed[key()] ?? false) && decorated() !== key()} />;
      },
    });

    context.keymap.layer(() => ({
      commands: [
        {
          id: "drive.toggle",
          title: "Toggle drive mode",
          description: "Let a reviewer model answer permission requests for this session tree",
          group: "Session",
          bind: "<leader>d",
          palette: true,
          slash: { name: "drive" },
          run: () => toggle().catch(fail),
        },
        {
          id: "drive.journal",
          title: "Review drive decisions",
          description: "List the reviewer's recent decisions for this session tree",
          group: "Session",
          palette: true,
          slash: { name: "drive-log" },
          run: () => review().catch(fail),
        },
      ],
    }));

    const off = rpc.events.on("changed", (event) => {
      setArmed(event.data.root, event.data.armed);
      mark();
    });
    const timer = setInterval(mark, poll);
    return () => {
      clearInterval(timer);
      off();
      decor.hide();
    };
  },
});

async function handoff(
  context: Plugin.Context,
  request: { location: { directory: string } } | undefined,
  sweep: (sessions: string[]) => Promise<readonly string[]>,
) {
  const [requests, forms] = await Promise.all([
    context.client.permission.request.list(request),
    context.client.form.list(request),
  ]);
  const questions = forms.data.filter((form) => form.metadata?.kind === "question");
  const sessions = [...requests.data, ...questions].map((item) => item.sessionID);
  if (sessions.length === 0) return;
  const tree = new Set(await sweep(sessions));
  await Promise.allSettled(
    questions
      .filter((form) => tree.has(form.sessionID))
      .map((form) => context.client.session.form.cancel({ sessionID: form.sessionID, formID: form.id, message: AWAY })),
  );
}

function busy(context: Plugin.Context, root: string) {
  return [root, ...context.data.session.family(root)].some((id) => context.data.session.status(id) === "running");
}

function Badge(props: { context: Plugin.Context; visible: boolean }) {
  return (
    <Show when={props.visible}>
      <text fg={colors.brightBlue} flexShrink={0}>
        <b>DRIVE</b>
      </text>
    </Show>
  );
}

function choice(entry: Entry) {
  return {
    title: `${entry.decision.padEnd(5)} ${entry.action}: ${entry.resources.join(" ")}`,
    description: entry.reason,
    footer: new Date(entry.time).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }),
    value: entry,
  };
}

function details(entry: Entry) {
  return [
    `time: ${entry.time}`,
    `session: ${entry.session}`,
    `agent: ${entry.agent ?? "unknown"}`,
    `action: ${entry.action}`,
    ...entry.resources.map((resource) => `- ${resource}`),
    "",
    entry.reason,
    "",
    `model: ${entry.model}`,
  ].join("\n");
}
