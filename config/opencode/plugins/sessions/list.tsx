import path from "node:path";
import { createRoot } from "solid-js";
import {
  attention,
  closed,
  located,
  projectName,
  row,
  stable,
  title,
  watch,
  type Option,
  type Picker,
} from "./rows.tsx";
import { message, sessions, type Context, type Session } from "./source.ts";

function worktree(context: Context, session: Session) {
  const directory = session.location.directory;
  const project = context.data.project.get(session.projectID);
  const root = session.subpath ? path.resolve(directory, ...session.subpath.split("/").map(() => "..")) : directory;
  const relative = path.relative(project?.canonical ?? root, root);
  if (!relative.startsWith("..") && !path.isAbsolute(relative)) return undefined;
  const base = path.basename(relative);
  return base.length > 25 ? `${base.slice(0, 24)}…` : base;
}

function option(context: Context, session: Session, current: boolean, today: string): Option<string> {
  const date = new Date(session.time.updated).toDateString();
  return {
    ...row(context, session, session.id, current),
    category: date === today ? "Today" : date,
    footer: worktree(context, session),
  };
}

const day = (session: Session) => new Date(session.time.updated).setHours(0, 0, 0, 0);

function arrange(context: Context, scope: Session[]) {
  const rank = new Map(
    scope.map((session) => [session.id, closed(session) ? 2 : attention(context, session.id) ? 0 : 1]),
  );
  return scope.toSorted(
    (a, b) => day(b) - day(a) || (rank.get(a.id) ?? 1) - (rank.get(b.id) ?? 1) || b.time.updated - a.time.updated,
  );
}

async function remove(picker: Picker, session: Session) {
  const { context, source } = picker;
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
  await list(picker);
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

export async function list(picker: Picker) {
  const { context, source } = picker;
  const release = picker.hold("list");
  const [pick, close] = createRoot((dispose) => {
    const route = context.ui.router.current();
    const current = route.type === "session" ? context.data.session.root(route.sessionID) : undefined;
    const location = located(context);
    const directory = location?.directory ?? context.data.location.default().directory;
    const scoped = (session: Session) =>
      picker.prefs.allProjects ||
      (session.location.directory === directory && (!location?.project || session.projectID === location.project.id));
    const all = () => sessions(context, source.loaded());
    const find = (sessionID: string) => all().find((session) => session.id === sessionID);
    watch(context, source.loaded);
    void source.load();
    void context.data.project.sync().catch(() => undefined);
    const options = stable(() => {
      const today = new Date().toDateString();
      return arrange(context, all().filter(scoped)).map((session) =>
        option(context, session, session.id === current, today),
      );
    });
    const project = location?.project;
    const name = () => (project ? projectName(context.data.project.get(project.id)) : undefined);
    const selected = context.ui.dialog.select<string>({
      get title() {
        const label = name();
        return !picker.prefs.allProjects && label ? `Sessions for ${label}` : "Sessions";
      },
      get options() {
        return options();
      },
      current,
      get actions() {
        return [
          {
            bind: "ctrl+d",
            title: "delete",
            onTrigger: (sessionID: string) => {
              const session = find(sessionID);
              if (session) void remove(picker, session);
            },
          },
          {
            bind: "ctrl+r",
            title: "rename",
            onTrigger: (sessionID: string) => {
              const session = find(sessionID);
              if (session) void rename(picker, session);
            },
          },
          {
            bind: "ctrl+a",
            title: picker.prefs.allProjects ? "current directory" : "all projects",
            side: "right" as const,
            selection: "none" as const,
            onTrigger: picker.toggle,
          },
        ];
      },
    });
    context.ui.dialog.set({ size: "large" });
    return [selected, dispose] as const;
  });
  const sessionID = await pick;
  release();
  close();
  if (sessionID) context.ui.router.navigate({ type: "session", sessionID });
}
