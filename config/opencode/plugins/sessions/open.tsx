import path from "node:path";
import { createMemo, createRoot } from "solid-js";
import { pressure } from "../statusline/palette.ts";
import {
  inbox,
  located,
  order,
  projectName,
  rows,
  running,
  stable,
  timeAgo,
  watch,
  type Kind,
  type Option,
  type Picker,
} from "./rows.tsx";
import { message, sessions, type Context, type Session } from "./source.ts";
import { tag, title } from "./tags.ts";

const MINUTE = 60_000;
const ages = [15, 30, 60, 120, 240, 1440, 2880, 4320].map((minutes) => minutes * MINUTE);

function place(context: Context, session: Session) {
  const project = context.data.project.get(session.projectID);
  const name = projectName(project);
  const directory = session.location.directory;
  const base = path.basename(directory);
  const label =
    name && directory !== project?.canonical && name.toLowerCase() !== base.toLowerCase()
      ? `${name} · ${base}`
      : name || base;
  const shown = label.length > 30 ? `${label.slice(0, 29)}…` : label;
  return {
    searchText: `${session.id} ${directory}`,
    footer: `${shown ? `${shown} · ` : ""}${timeAgo(session.time.updated)}`,
  };
}

function worktree(context: Context, session: Session) {
  const directory = session.location.directory;
  const project = context.data.project.get(session.projectID);
  const root = session.subpath ? path.resolve(directory, ...session.subpath.split("/").map(() => "..")) : directory;
  const relative = path.relative(project?.canonical ?? root, root);
  if (!relative.startsWith("..") && !path.isAbsolute(relative)) return undefined;
  const base = path.basename(relative);
  return base.length > 25 ? `${base.slice(0, 24)}…` : base;
}

function spot(context: Context, route: ReturnType<Context["ui"]["router"]["current"]>) {
  return createMemo(() => {
    const session = route.type === "session" ? context.data.session.get(route.sessionID) : undefined;
    if (session) return { directory: session.location.directory, projectID: session.projectID };
    const location = located(context);
    return location && { directory: location.directory, projectID: location.project?.id };
  });
}

function local(context: Context, session: Session) {
  const label = worktree(context, session);
  return { footer: `${label ? `${label} · ` : ""}${timeAgo(session.time.updated)}` };
}

async function remove(picker: Picker, session: Session) {
  const { context, source } = picker;
  const mine = picker.claim();
  const confirmed = await context.ui.dialog.confirm({
    title: "Delete session",
    message: `Delete "${title(session)}"?`,
    label: { confirm: "Delete" },
  });
  if (confirmed === undefined) return;
  if (confirmed) {
    try {
      await context.client.session.remove({ sessionID: session.id });
      source.drop(session.id);
    } catch (error) {
      context.ui.toast.show({ message: `Failed to delete session: ${message(error)}`, variant: "error" });
    }
  }
  if (mine()) await open(picker, "recent");
}

async function rename(picker: Picker, session: Session) {
  const { context, source } = picker;
  const value = await context.ui.dialog.prompt({
    title: "Rename session",
    placeholder: "Session title",
    value: title(session),
  });
  const next = value?.trim();
  if (!next) return;
  try {
    await context.client.session.update({ sessionID: session.id, title: next });
    source.rename(session.id, next);
  } catch (error) {
    context.ui.toast.show({ message: `Failed to rename session: ${message(error)}`, variant: "error" });
  }
}

export async function open(picker: Picker, kind: Kind) {
  const { context, source } = picker;
  const release = picker.hold(kind);
  const [pick, close] = createRoot((dispose) => {
    const route = context.ui.router.current();
    const current = route.type === "session" ? context.data.session.root(route.sessionID) : undefined;
    const here = kind === "recent" ? spot(context, route) : undefined;
    const scoped = (session: Session) => {
      if (!here) return !tag(session.title)?.state;
      const scope = here();
      if (!scope || session.location.directory !== scope.directory) return false;
      return !scope.projectID || session.projectID === scope.projectID;
    };
    const footer = here ? local : place;
    const all = () => sessions(context, source.loaded());
    const find = (sessionID: string) => all().find((session) => session.id === sessionID);
    watch(context, source.loaded);
    const mailed = inbox(context);
    const row = rows(mailed);
    void source.load();
    void context.data.project.sync().catch(() => undefined);
    const option = (category: string, session: Session): Option<string> =>
      Object.assign(row(context, session, session.id, session.id === current), footer(context, session), { category });
    const options = stable(() => {
      const list = all().filter(scoped);
      const closed = list.filter((session) => tag(session.title)?.state);
      const live = list.filter((session) => !closed.includes(session));
      const busy = live.filter((session) => running(context, session.id));
      const idle = live.filter((session) => !busy.includes(session));
      return [
        ...order(context, busy, mailed).map((session) => option("Running", session)),
        ...order(context, idle, mailed).map((session) =>
          Object.assign(option("Open", session), { footerColor: pressure(Date.now() - session.time.updated, ages) }),
        ),
        ...closed.toSorted((a, b) => b.time.updated - a.time.updated).map((session) => option("Closed", session)),
      ];
    });
    const selected = context.ui.dialog.select<string>({
      get title() {
        if (!here) return "Open";
        const projectID = here()?.projectID;
        const label = projectID ? projectName(context.data.project.get(projectID)) : undefined;
        return label ? `Sessions for ${label}` : "Sessions";
      },
      placeholder: "Search sessions…",
      get options() {
        return options();
      },
      // A defined current preserves selection by value as options load, even without a matching row.
      current: current ?? "",
      actions: here
        ? [
            {
              bind: "ctrl+d",
              title: "delete",
              onTrigger: (sessionID: string) => {
                const session = find(sessionID);
                if (session) void remove(picker, session);
              },
            },
            {
              bind: "<leader>r",
              title: "rename",
              onTrigger: (sessionID: string) => {
                const session = find(sessionID);
                if (session) void rename(picker, session);
              },
            },
          ]
        : undefined,
    });
    context.ui.dialog.set({ size: "large" });
    return [selected, dispose] as const;
  });
  const sessionID = await pick;
  release();
  close();
  if (sessionID) context.ui.router.navigate({ type: "session", sessionID });
}
