import path from "node:path";
import { createRoot } from "solid-js";
import { pressure } from "../statusline/palette.ts";
import { inbox, order, projectName, rows, running, stable, timeAgo, watch, type Option, type Picker } from "./rows.tsx";
import { sessions, type Context, type Session } from "./source.ts";
import { tag } from "./tags.ts";

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

export async function open(picker: Picker) {
  const { context, source } = picker;
  const release = picker.hold("open");
  const [pick, close] = createRoot((dispose) => {
    const route = context.ui.router.current();
    const current = route.type === "session" ? context.data.session.root(route.sessionID) : undefined;
    watch(context, source.loaded);
    const mailed = inbox(context);
    const row = rows(mailed);
    void source.load();
    void context.data.project.sync().catch(() => undefined);
    const option = (category: string, session: Session): Option<string> =>
      Object.assign(row(context, session, session.id, session.id === current), place(context, session), { category });
    const options = stable(() => {
      const list = sessions(context, source.loaded()).filter((session) => !tag(session.title)?.state);
      const busy = list.filter((session) => running(context, session.id));
      const idle = list.filter((session) => !busy.includes(session));
      return [
        ...order(context, busy, mailed).map((session) => option("Running", session)),
        ...order(context, idle, mailed).map((session) =>
          Object.assign(option("Open", session), { footerColor: pressure(Date.now() - session.time.updated, ages) }),
        ),
      ];
    });
    const selected = context.ui.dialog.select<string>({
      title: "Open",
      placeholder: "Search sessions…",
      get options() {
        return options();
      },
      // A defined current preserves selection by value as options load, even without a matching row.
      current: current ?? "",
    });
    context.ui.dialog.set({ size: "large" });
    return [selected, dispose] as const;
  });
  const sessionID = await pick;
  release();
  close();
  if (sessionID) context.ui.router.navigate({ type: "session", sessionID });
}
