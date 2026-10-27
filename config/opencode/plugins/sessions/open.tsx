import path from "node:path";
import { createRoot, createSignal, onCleanup } from "solid-js";
import {
  attention,
  clip,
  dialog,
  listen,
  located,
  order,
  projectName,
  row,
  stable,
  timeAgo,
  watch,
  width,
  type Option,
  type Picker,
} from "./rows.tsx";
import { message, sessions, type Context, type Session } from "./source.ts";

type Target = { type: "session"; sessionID: string } | { type: "project"; directory: string; projectID?: string };
type Worktree = { type: "directory"; directory: string } | { type: "new" };

const RECENT = 8;
const LARGE = 88;

const contentWidth = (context: Context) => Math.min(LARGE, context.renderer.width - 2) - 12;

export function go(context: Context, target: Target | { type: "project"; directory: string }) {
  const router = context.ui.router;
  if (target.type === "session") return router.navigate({ type: "session", sessionID: target.sessionID });
  const location = { directory: target.directory };
  // The host accepts an untyped home location; clear the old object so Home observes the new directory.
  if (router.current().type === "home") router.navigate({ type: "home" });
  router.navigate({ type: "home", location } as { type: "home" });
}

const here = (context: Context) => context.location?.directory ?? context.data.location.default().directory;

function sessionOption(context: Context, session: Session, current: boolean): Option<Target> {
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
    ...row<Target>(context, session, { type: "session", sessionID: session.id }, current),
    searchText: `${session.id} ${directory}`,
    category: "Sessions",
    footer: `${shown ? `${shown} · ` : ""}${timeAgo(session.time.updated)}`,
  };
}

function projectOptions(context: Context): Option<Target>[] {
  const current = here(context);
  const canonical = located(context)?.project?.canonical;
  const theme = dialog(context);
  return context.data.project
    .list()
    .filter((project) => project.canonical !== "/")
    .filter((project, at, list) => list.findIndex((item) => item.canonical === project.canonical) === at)
    .map((project) => {
      const title = projectName(project) ?? path.basename(project.canonical);
      const footer = context.ui.format.path(project.canonical);
      const git = project.vcs === "git";
      const room = contentWidth(context) - width(title) - (git ? 2 : 0);
      const marked = project.canonical === current || project.canonical === canonical;
      return {
        title,
        footer: `${clip(footer, room)}${git ? " →" : ""}`,
        searchText: `${footer} ${projectName(project) ?? ""}`,
        value: { type: "project", directory: project.canonical, ...(git ? { projectID: project.id } : {}) },
        category: "Projects",
        gutter: marked ? () => <text fg={theme.text.formfield.selected}>●</text> : undefined,
      };
    });
}

export async function open(picker: Picker) {
  const { context, source } = picker;
  const release = picker.hold("open");
  const [pick, close] = createRoot((dispose) => {
    const [query, setQuery] = createSignal("");
    const route = context.ui.router.current();
    const current = route.type === "session" ? context.data.session.root(route.sessionID) : undefined;
    watch(context, source.loaded);
    void source.load();
    void context.data.project.sync().catch(() => undefined);
    const options = stable(() => {
      const list = order(context, sessions(context, source.loaded()));
      const waiting = list.filter((session) => attention(context, session.id)).length;
      const shown = query().trim() ? list : list.slice(0, Math.max(RECENT, waiting));
      return [
        ...shown.map((session) => sessionOption(context, session, session.id === current)),
        ...projectOptions(context),
      ];
    });
    listen(context, setQuery);
    const selected = context.ui.dialog.select<Target>({
      title: "Open",
      placeholder: "Search sessions and projects…",
      get options() {
        return options();
      },
      // A defined current preserves selection by value as options load, even without a matching row.
      current: { type: "session", sessionID: current ?? "" },
      actions: [
        {
          bind: "right",
          title: "worktrees",
          onTrigger: (target) => {
            if (target.type === "project" && target.projectID)
              void worktrees(picker, { directory: target.directory, projectID: target.projectID });
          },
        },
      ],
    });
    context.ui.dialog.set({ size: "large" });
    return [selected, dispose] as const;
  });
  const target = await pick;
  release();
  close();
  if (target) go(context, target);
}

function worktreeOptions(context: Context, canonical: string, name: string, list: string[]): Option<Worktree>[] {
  const current = here(context);
  const theme = dialog(context);
  const directories = [...new Set([canonical, ...list])].toSorted((a, b) => {
    if (a === canonical || b === canonical) return a === canonical ? -1 : 1;
    if (a === current || b === current) return a === current ? -1 : 1;
    return 0;
  });
  const titles = directories.map((directory) => (directory === canonical ? name : path.basename(directory)));
  const room = Math.max(0, contentWidth(context) - Math.max(...titles.map(width)));
  return [
    ...directories.map((directory, at): Option<Worktree> => {
      const footer = clip(context.ui.format.path(directory), room);
      return {
        title: titles[at] ?? directory,
        footer: footer + " ".repeat(Math.max(0, room - width(footer))),
        value: { type: "directory", directory },
        gutter: directory === current ? () => <text fg={theme.text.formfield.selected}>●</text> : undefined,
      };
    }),
    { title: "New worktree…", value: { type: "new" } },
  ];
}

async function worktrees(picker: Picker, project: { directory: string; projectID: string }) {
  const { context } = picker;
  const { projectID } = project;
  const name = projectName(context.data.project.get(projectID)) ?? path.basename(project.directory);
  const release = picker.hold("open");
  const [pick, close] = createRoot((dispose) => {
    const [list, setList] = createSignal<string[]>();
    let latest = 0;
    const load = async () => {
      const request = ++latest;
      try {
        const items = await context.client.worktree.list({ projectID });
        if (request === latest) setList(items.map((item) => item.directory));
      } catch (error) {
        if (request !== latest) return;
        context.ui.toast.show({ title: "Loading worktrees failed", message: message(error), variant: "error" });
        setList([]);
      }
    };
    void load().then(() => context.client.worktree.refresh({ projectID }).catch(() => undefined));
    onCleanup(
      context.data.on("worktree.updated", (event) => {
        if (event.data.projectID === projectID) void load();
      }),
    );
    const options = stable(() => worktreeOptions(context, project.directory, name, list() ?? []));
    const selected = context.ui.dialog.select<Worktree>({
      get title() {
        return `${name} / Worktrees${list() === undefined ? " · loading…" : ""}`;
      },
      placeholder: "Search worktrees…",
      get options() {
        return options();
      },
      // A defined current preserves selection by value as options load, even without a matching row.
      current: { type: "directory", directory: "" },
      actions: [{ bind: "left", title: "projects", selection: "none", onTrigger: () => void open(picker) }],
    });
    context.ui.dialog.set({ size: "large" });
    return [selected, dispose] as const;
  });
  const value = await pick;
  release();
  close();
  if (value?.type === "directory") go(context, { type: "project", directory: value.directory });
  if (value?.type === "new") await create(picker, project, name);
}

async function create(picker: Picker, project: { directory: string; projectID: string }, name: string) {
  const { context } = picker;
  const mine = picker.claim();
  const value = await context.ui.dialog.prompt({
    title: `${name} / New worktree`,
    placeholder: "Worktree name (optional)",
    description: "Leave blank for a random name.",
  });
  if (value === undefined) {
    if (mine()) await worktrees(picker, project);
    return;
  }
  context.ui.toast.show({ message: "Creating worktree…", variant: "info" });
  try {
    const created = await context.client.worktree.create({
      projectID: project.projectID,
      ...(value.trim() ? { name: value.trim() } : {}),
    });
    if (mine()) go(context, { type: "project", directory: created.directory });
    else context.ui.toast.show({ message: `Created worktree ${context.ui.format.path(created.directory)}` });
  } catch (error) {
    context.ui.toast.show({ title: "Creating worktree failed", message: message(error), variant: "error" });
  }
}
