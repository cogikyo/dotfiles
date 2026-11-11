import path from "node:path";
import type { DialogSelectOption } from "@opencode/plugin/tui/context";
import type { RGBA } from "@opentui/core";
import { createEffect, createMemo, createRoot, createSignal, onCleanup } from "solid-js";
import { desks, show, type Desks } from "../hyprd/open.ts";
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
  urgency,
  watch,
  type Kind,
  type Mailed,
  type Option,
  type Picker,
  type Rank,
} from "./rows.tsx";
import { message, sessions, type Context, type Session } from "./source.ts";
import { tag, title } from "./tags.ts";

const MINUTE = 60_000;
const ages = [15, 30, 60, 120, 240, 1440, 2880, 4320].map((minutes) => minutes * MINUTE);

let last: Desks | undefined;

function seat(desk: Desks): Rank {
  return (session) => {
    const ws = desk.of.get(session.location.directory) ?? 0;
    if (ws <= 0) return Infinity;
    return ws === desk.here ? 0 : ws;
  };
}

function seats(context: Context, mailed: Mailed, loading: Promise<void>, directories: () => string[]) {
  const [desk, setDesk] = createSignal(last);
  const [ready, setReady] = createSignal(false);
  void loading.finally(() => setReady(true));
  let done = false;
  onCleanup(() => {
    done = true;
  });
  const asked = new Set<string>();
  let warned = false;
  const warn = (reason: string) => {
    if (warned) return;
    warned = true;
    context.ui.toast.show({ message: `Could not sort sessions by workspace: ${reason}`, variant: "warning" });
  };
  const forget = () => {
    done = true;
    last = undefined;
    setDesk(undefined);
  };
  const ask = async (fresh: string[]) => {
    try {
      const found = await desks(fresh);
      if (done) return;
      if (!found) return forget();
      setDesk((known) => {
        last = { here: found.here, of: new Map([...(known?.of ?? []), ...found.of]) };
        return last;
      });
      if (found.oversized.length) warn(`directory too long for hyprd: ${found.oversized[0]}`);
    } catch (error) {
      if (done) return;
      forget();
      warn(message(error));
    }
  };
  createEffect(() => {
    if (!ready() || done) return;
    const fresh = [...new Set(directories())].filter((directory) => !asked.has(directory));
    if (!fresh.length) return;
    fresh.forEach((directory) => asked.add(directory));
    void ask(fresh);
  });
  return {
    rank: (): Rank => {
      const known = desk();
      return known ? seat(known) : urgency(context, mailed);
    },
    near: (session: Session) => {
      const known = desk();
      return known && seat(known)(session) === 0;
    },
  };
}

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

function edits(picker: Picker, find: (sessionID: string) => Session | undefined) {
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
      bind: "<leader>r",
      title: "rename",
      onTrigger: (sessionID: string) => {
        const session = find(sessionID);
        if (session) void rename(picker, session);
      },
    },
  ];
}

export async function open(picker: Picker, kind: Kind) {
  const { context, source } = picker;
  const release = picker.hold(kind);
  const [pick, close, lookup, near] = createRoot((dispose) => {
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
    const loading = source.load();
    void context.data.project.sync().catch(() => undefined);
    const list = createMemo(() => all().filter(scoped));
    const live = createMemo(() => list().filter((session) => !tag(session.title)?.state));
    const seated = here
      ? undefined
      : seats(context, mailed, loading, () => live().map((session) => session.location.directory));
    const rank = seated?.rank ?? (() => urgency(context, mailed));
    const row = rows(mailed, seated?.near);
    const option = (category: string, session: Session, footerColor?: RGBA): Option<string> => ({
      ...row(context, session, session.id === current, { ...footer(context, session), footerColor }),
      category,
    });
    const options = stable(() => {
      const closed = list().filter((session) => tag(session.title)?.state);
      const busy = live().filter((session) => running(context, session.id));
      const idle = live().filter((session) => !busy.includes(session));
      const by = rank();
      return [
        ...order(busy, by).map((session) => option("Running", session)),
        ...order(idle, by).map((session) => option("Open", session, pressure(Date.now() - session.time.updated, ages))),
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
        return options() as unknown as DialogSelectOption<string>[];
      },
      // A defined current preserves selection by value as options load, even without a matching row.
      current: current ?? "",
      actions: here ? edits(picker, find) : undefined,
    });
    context.ui.dialog.set({ size: "large" });
    return [selected, dispose, find, seated?.near] as const;
  });
  const sessionID = await pick;
  const session = sessionID ? lookup(sessionID) : undefined;
  const stay = !!session && !!near?.(session);
  release();
  close();
  if (!sessionID) return;
  if (stay) return context.ui.router.navigate({ type: "session", sessionID });
  await show(context.ui, sessionID, session?.location.directory);
}
