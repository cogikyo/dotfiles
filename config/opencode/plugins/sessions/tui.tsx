import { Plugin } from "@opencode/plugin/tui";
import { list } from "./list.tsx";
import { go, open } from "./open.tsx";
import type { Kind, Picker } from "./rows.tsx";
import { source, type Context } from "./source.ts";
import { retag, tag, title, type State, type Tag } from "./tags.ts";

async function edit(context: Context, command: string, change: (current?: Tag) => Tag) {
  const current = context.ui.router.current();
  if (current.type !== "session") return undefined;
  const cached = context.data.session.get(context.data.session.root(current.sessionID));
  if (!cached) return undefined;
  try {
    const session = await context.client.session.get({ sessionID: cached.id });
    const next = change(tag(session.title));
    const text = retag(title(session), next);
    if (text !== session.title) await context.client.session.update({ sessionID: session.id, title: text });
    return { session, tag: next };
  } catch (error) {
    context.ui.toast.show({ title: `/${command} failed`, message: String(error), variant: "error" });
    return undefined;
  }
}

async function mark(context: Context, command: string, state: State) {
  const done = await edit(context, command, (current) => ({ state, flags: current?.flags ?? [] }));
  if (done) go(context, { type: "project", directory: done.session.location.directory });
}

async function refusals(context: Context) {
  const done = await edit(context, "refusals", (current) => {
    const flags = current?.flags ?? [];
    const on = flags.includes("refusals");
    return { state: current?.state, flags: on ? flags.filter((flag) => flag !== "refusals") : [...flags, "refusals"] };
  });
  if (!done) return;
  const hidden = done.tag.flags.includes("refusals");
  context.ui.toast.show({
    message: hidden ? "Agents can no longer see this session" : "Agents can see this session again",
    variant: "info",
  });
}

function tagging(context: Context) {
  return [
    {
      id: "sessions.archive",
      title: "Archive session",
      description: "Mark the session [ARCHIVED] without /done, then start a new one here",
      group: "Session",
      palette: true,
      slash: { name: "archive" },
      run: () => mark(context, "archive", "archived"),
    },
    {
      id: "sessions.research",
      title: "Mark session as research",
      description: "Mark the session [RESEARCH] without /done, then start a new one here",
      group: "Session",
      palette: true,
      slash: { name: "research" },
      run: () => mark(context, "research", "research"),
    },
    {
      id: "sessions.refusals",
      title: "Toggle refusals flag",
      description: "Hide or show this session and its children in the agents' sessions tool",
      group: "Session",
      palette: true,
      slash: { name: "refusals" },
      run: () => refusals(context),
    },
  ] as const;
}

export default Plugin.define({
  id: "sessions",
  setup(context) {
    let shown: { kind: Kind } | undefined;
    let generation = 0;
    const route = () => JSON.stringify(context.ui.router.current());
    const picker: Picker = {
      context,
      source: source(context),
      claim() {
        const mine = ++generation;
        const at = route();
        // Replacing a dialog also resolves undefined, but keeps the host in modal mode.
        return () => generation === mine && route() === at && context.keymap.mode.current() !== "modal";
      },
      hold(kind) {
        generation++;
        const token = { kind };
        shown = token;
        return () => {
          if (shown === token) shown = undefined;
        };
      },
    };

    context.keymap.layer(() => ({
      mode: "global",
      commands: [
        {
          bind: "ctrl+o",
          title: "Open session or project",
          group: "Session",
          run: () => {
            if (shown?.kind !== "open") void open(picker);
          },
        },
        {
          bind: "<leader>r",
          title: "Switch session",
          group: "Session",
          run: () => {
            if (shown?.kind !== "list") void list(picker);
          },
        },
        {
          bind: "<leader>shift+return",
          title: "Finish session with /done",
          group: "Session",
          run: () => {
            const current = context.ui.router.current();
            if (current.type !== "session") return;
            const sessionID = context.data.session.root(current.sessionID);
            context.client.session
              .command({ sessionID, name: "done", text: "" })
              .catch((error: unknown) =>
                context.ui.toast.show({ title: "/done failed", message: String(error), variant: "error" }),
              );
          },
        },
        ...tagging(context),
      ],
    }));

    return picker.source.watch();
  },
});
