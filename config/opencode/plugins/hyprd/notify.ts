import { Plugin } from "@opencode/plugin/tui";
import { createConnection } from "node:net";
import { tag } from "../sessions/tags.ts";

type Data = Plugin.Context["data"];
type Client = Plugin.Context["client"];
type Session = NonNullable<ReturnType<Data["session"]["get"]>>;
type Asks = {
  permissions: Awaited<ReturnType<Client["permission"]["list"]>>;
  forms: Awaited<ReturnType<Client["session"]["form"]["list"]>>;
};

const SOCKET = "/tmp/hyprd.sock";
const TICK_MS = 60_000;
const ASK_MS = 5 * 60_000;
const IDLE_MS = 15 * 60_000;
const REQUEST_MS = 2_000;
const SYNC_MS = 10_000;
const LINE_MAX = 200; // Keeps the JSON request within hyprd's single 4096-byte Read.

type Kind = "permission" | "question" | "complete" | "idle" | "error" | "lane" | "viewed";

const pane = {
  kitty_pid: Number(process.env.KITTY_PID) || 0,
  kitty_window_id: Number(process.env.KITTY_WINDOW_ID) || 0,
};

const hyprland =
  process.env.XDG_RUNTIME_DIR && process.env.HYPRLAND_INSTANCE_SIGNATURE
    ? `${process.env.XDG_RUNTIME_DIR}/hypr/${process.env.HYPRLAND_INSTANCE_SIGNATURE}/.socket.sock`
    : undefined;

function request(path: string, command: string) {
  return new Promise<string>((resolve) => {
    let body = "";
    const socket = createConnection(path);
    socket.setEncoding("utf8");
    const timer = setTimeout(() => socket.destroy(), REQUEST_MS);
    socket.on("data", (chunk: string) => (body += chunk));
    socket.on("error", () => socket.destroy());
    socket.on("close", () => {
      clearTimeout(timer);
      resolve(body);
    });
    socket.write(command);
  });
}

const line = (text: string) =>
  Array.from(
    text
      .split("\n")
      .map((part) => part.trim())
      .find((part) => /[\p{L}\p{N}]/u.test(part)) ?? "",
  )
    .slice(0, LINE_MAX)
    .join("");

function send(event: Kind, fields: { message?: string; last_assistant_message?: string } = {}) {
  if (!pane.kitty_pid || !pane.kitty_window_id) return;
  const { message, last_assistant_message: last } = fields;
  const body = {
    source: "opencode",
    event,
    message: message && line(message),
    last_assistant_message: last && line(last),
    ...pane,
  };
  void request(SOCKET, `notify ${JSON.stringify(body)}`);
}

async function locked() {
  if (!hyprland) return false;
  const body = await request(hyprland, "j/locked");
  let state: unknown;
  try {
    state = JSON.parse(body);
  } catch {
    state = undefined;
  }
  return typeof state === "object" && state !== null && "locked" in state && state.locked === true;
}

function lane(session: Session) {
  const { lane: name, closed } = session.metadata ?? {};
  if (typeof name === "string") return name;
  if (closed && typeof closed === "object" && !Array.isArray(closed) && typeof closed.lane === "string")
    return closed.lane;
  return session.agent ?? "Lane";
}

const family = (data: Data, id: string) => [...new Set([id, ...data.session.family(id)])];

function reminder(data: Data, id: string, { permissions, forms }: Asks, shown: boolean) {
  const info = data.session.get(id);
  const title = info?.title ?? "OpenCode";
  const members = family(data, id);
  const asks = [...permissions, ...forms].map((ask) => ask.id).toSorted();
  const idles = members.toSorted().map((member) => data.session.get(member)?.time.idle);
  const key = JSON.stringify([asks, idles, data.session.status(id)]);

  if (permissions[0])
    return { key, kind: "permission" as const, after: ASK_MS, message: `${title}: ${permissions[0].action}` };
  if (forms[0]) return { key, kind: "question" as const, after: ASK_MS, message: `${title}: ${forms[0].title}` };
  const busy = members.some(
    (member) =>
      data.session.status(member) === "running" ||
      data.session.pending.list(member).some((item) => item.type !== "synthetic"),
  );
  if (!shown || busy || info?.time.idle === undefined || tag(title)?.state) return { key };
  return { key, kind: "idle" as const, after: IDLE_MS, message: title };
}

// Read the server: after a TUI restart, the store lacks background tabs' lane asks.
async function asked(client: Client, directories: string[], signal: AbortSignal): Promise<Asks | undefined> {
  const options = { signal: AbortSignal.any([AbortSignal.timeout(SYNC_MS), signal]) };
  const pages = await Promise.allSettled(
    directories.map(async (directory) =>
      Promise.all([
        client.permission.request.list({ location: { directory } }, options),
        client.form.list({ location: { directory } }, options),
      ]),
    ),
  );
  const read = pages.flatMap((page) => (page.status === "fulfilled" ? [page.value] : []));
  if (!read.length) return undefined;
  return {
    permissions: read.flatMap(([permissions]) => permissions.data),
    forms: read.flatMap(([, forms]) => forms.data),
  };
}

function reminders(
  { data, client, ui }: Plugin.Context,
  signal: AbortSignal,
  shown: () => string | undefined,
  quiet: (id: string) => boolean,
) {
  const clocks = new Map<string, { key?: string; elapsed: number }>();
  const roots = () => {
    if (ui.tabs.enabled()) return [...new Set(ui.tabs.list().map((tab) => data.session.root(tab.sessionID)))];
    const id = shown();
    return id ? [id] : [];
  };
  const check = (id: string, paused: boolean, { permissions, forms }: Asks) => {
    const clock = clocks.get(id);
    if (!clock) return;
    const members = new Set(family(data, id));
    const asks = {
      permissions: permissions.filter((ask) => members.has(ask.sessionID)),
      forms: forms.filter((ask) => members.has(ask.sessionID)),
    };
    const due = reminder(data, id, asks, shown() === id);
    if (due.key !== clock.key || !due.kind || quiet(id)) {
      clock.key = due.key;
      clock.elapsed = 0;
      return;
    }
    if (!paused) clock.elapsed += TICK_MS;
    if (clock.elapsed < due.after) return;
    clock.elapsed = 0;
    send(due.kind, { message: due.message });
  };
  const tick = async () => {
    const open = roots();
    for (const id of clocks.keys()) if (!open.includes(id)) clocks.delete(id);
    for (const id of open) if (!clocks.has(id)) clocks.set(id, { elapsed: 0 });
    const directories = open.flatMap((id) =>
      family(data, id).flatMap((member) => data.session.get(member)?.location.directory ?? []),
    );
    if (!directories.length) return;
    const [paused, asks] = await Promise.all([locked(), asked(client, [...new Set(directories)], signal)]);
    if (signal.aborted || !asks) return;
    for (const id of open) check(id, paused, asks);
  };
  const timer = setInterval(() => void tick(), TICK_MS);
  signal.addEventListener("abort", () => clearInterval(timer));

  return () => {
    const id = shown();
    const clock = id ? clocks.get(id) : undefined;
    if (clock) clock.elapsed = 0;
  };
}

function reply(data: Data, id: string) {
  const last = data.session.message.list(id).findLast((message) => message.type === "assistant");
  if (last?.type !== "assistant") return undefined;
  return (
    last.content
      .flatMap((part) => (part.type === "text" ? [part.text] : []))
      .join("\n")
      .trim() || undefined
  );
}

// Completed lanes already sent notices; their root wake turn needs no duplicate.
function woke(data: Data, event: { id: string; data: { sessionID: string } }) {
  const all = data.session.message.list(event.data.sessionID);
  const end = all.findIndex((message) => message.id === event.id.replace(/^evt_/, "msg_"));
  const messages = end < 0 ? all : all.slice(0, end);
  const start = messages.findLastIndex((message) => message.type === "idle");
  const turn = messages.slice(start + 1);
  if (start < 0 || turn.some((message) => message.type === "user")) return false;
  const notices = turn.filter((message) => message.type === "synthetic" && message.metadata?.source === "subagent");
  return notices.length > 0 && notices.every((message) => message.metadata?.state === "completed");
}

export default Plugin.define({
  id: "notify",
  setup(context) {
    const { data, ui } = context;
    const seen = new Set<string>();
    const ended = new Set<string>();
    const [focus, setFocus] = context.storage.memory("focus", { initial: { focused: false } });
    const abort = new AbortController();

    const root = async (sessionID: string): Promise<string> => {
      if (!data.session.get(sessionID)) await data.session.sync(sessionID).catch(() => undefined);
      const parent = data.session.get(sessionID)?.parentID;
      return parent ? root(parent) : sessionID;
    };
    const shown = () => {
      const route = ui.router.current();
      return route.type === "session" ? data.session.root(route.sessionID) : undefined;
    };
    const open = (id: string) =>
      ui.tabs.enabled() ? ui.tabs.list().some((tab) => tab.sessionID === id) : shown() === id;
    const quiet = (id: string) => !open(id) || (focus.focused && shown() === id);
    const title = (id: string) => data.session.get(id)?.title ?? "OpenCode";
    const reset = reminders(context, abort.signal, shown, quiet);

    const ask = async (kind: "permission" | "question", id: string, sessionID: string, label: string) => {
      if (seen.has(id)) return;
      seen.add(id);
      const top = await root(sessionID);
      if (!abort.signal.aborted && !quiet(top)) send(kind, { message: `${title(top)}: ${label}` });
    };

    const finish = async (sessionID: string, outcome: "succeeded" | "woke" | "failed", error = "") => {
      if (ended.has(sessionID)) return;
      ended.add(sessionID);
      const top = await root(sessionID);
      if (abort.signal.aborted || quiet(top)) return;
      if (top !== sessionID) {
        const child = data.session.get(sessionID);
        if (child && outcome !== "failed") send("lane", { message: `${lane(child)} done` });
        return;
      }
      if (outcome === "failed") return send("error", { message: `${title(top)}: ${error}` });
      if (outcome === "woke") return;
      send("complete", { message: title(top), last_assistant_message: reply(data, top) ?? title(top) });
    };

    const onFocus = () => {
      setFocus((draft) => {
        draft.focused = true;
      });
      reset();
      send("viewed");
    };
    const onBlur = () =>
      setFocus((draft) => {
        draft.focused = false;
      });
    context.renderer.on("focus", onFocus);
    context.renderer.on("blur", onBlur);

    const dispose = [
      data.on("permission.asked", (event) => {
        void ask("permission", event.data.id, event.data.sessionID, event.data.action);
      }),
      data.on("permission.replied", (event) => seen.delete(event.data.requestID)),
      data.on("form.created", (event) => {
        void ask("question", event.data.form.id, event.data.form.sessionID, event.data.form.title);
      }),
      data.on("form.replied", (event) => seen.delete(event.data.id)),
      data.on("form.cancelled", (event) => seen.delete(event.data.id)),
      data.on("session.execution.started", (event) => ended.delete(event.data.sessionID)),
      data.on("session.execution.succeeded", (event) => {
        // Read before any await: later events in the host batch can change the store.
        void finish(event.data.sessionID, woke(data, event) ? "woke" : "succeeded");
      }),
      data.on("session.execution.failed", (event) => {
        const sessionID = event.data.sessionID;
        const message = event.data.error.message;
        ui.toast.show({ sessionID, title: "Session failed", message, variant: "error" });
        void finish(sessionID, "failed", message);
      }),
    ];

    return () => {
      abort.abort();
      context.renderer.off("focus", onFocus);
      context.renderer.off("blur", onBlur);
      dispose.toReversed().forEach((cleanup) => cleanup());
    };
  },
});
