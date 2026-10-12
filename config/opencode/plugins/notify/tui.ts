import { Plugin } from "@opencode/plugin/tui";
import { createConnection } from "node:net";

const SOCKET = "/tmp/hyprd.sock";
const IDLE_MS = 10 * 60_000;

type Kind = "permission" | "question" | "complete" | "idle" | "error" | "viewed";

const pane = {
  kitty_pid: Number(process.env.KITTY_PID) || 0,
  kitty_window_id: Number(process.env.KITTY_WINDOW_ID) || 0,
};

function send(event: Kind, fields: { message?: string; last_assistant_message?: string } = {}) {
  if (!pane.kitty_pid || !pane.kitty_window_id) return;
  const socket = createConnection(SOCKET);
  socket.on("error", () => socket.destroy());
  socket.end(`notify ${JSON.stringify({ source: "opencode", event, ...fields, ...pane })}`);
}

export default Plugin.define({
  id: "notify",
  setup(context) {
    const { data, ui } = context;
    const seen = new Set<string>();
    const ended = new Set<string>();
    const failed = new Set<string>();
    const idle = new Map<string, ReturnType<typeof setTimeout>>();
    let focused = false;

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
    const quiet = (id: string) => !open(id) || (focused && shown() === id);
    const title = (id: string) => data.session.get(id)?.title ?? "OpenCode";

    const reply = (id: string) => {
      const last = data.session.message.list(id).findLast((message) => message.type === "assistant");
      if (last?.type !== "assistant") return undefined;
      return (
        last.content
          .flatMap((part) => (part.type === "text" ? [part.text] : []))
          .join("\n")
          .trim() || undefined
      );
    };

    const stop = (id: string) => {
      clearTimeout(idle.get(id));
      idle.delete(id);
    };
    const remind = (id: string) => {
      stop(id);
      idle.set(
        id,
        setTimeout(() => {
          idle.delete(id);
          if (!open(id) || data.session.status(id) !== "idle") return;
          if (!quiet(id)) return remind(id);
          send("idle", { message: title(id) });
          remind(id);
        }, IDLE_MS),
      );
    };

    const ask = (kind: "permission" | "question", id: string, sessionID: string, label: string) => {
      if (seen.has(id)) return;
      seen.add(id);
      void root(sessionID).then((top) => {
        if (quiet(top)) return;
        send(kind, { message: `${title(top)}: ${label}` });
      });
    };

    const finish = (sessionID: string) => {
      if (data.session.get(sessionID)?.parentID || ended.has(sessionID)) return;
      ended.add(sessionID);
      if (failed.delete(sessionID)) return remind(sessionID);
      remind(sessionID);
      if (quiet(sessionID)) return;
      send("complete", { message: title(sessionID), last_assistant_message: reply(sessionID) ?? title(sessionID) });
    };

    const onFocus = () => {
      focused = true;
      for (const id of idle.keys()) stop(id);
      send("viewed");
    };
    const onBlur = () => {
      focused = false;
    };
    context.renderer.on("focus", onFocus);
    context.renderer.on("blur", onBlur);

    const dispose = [
      data.on("permission.asked", (event) => ask("permission", event.data.id, event.data.sessionID, event.data.action)),
      data.on("permission.replied", (event) => seen.delete(event.data.requestID)),
      data.on("form.created", (event) =>
        ask("question", event.data.form.id, event.data.form.sessionID, event.data.form.title ?? "question"),
      ),
      data.on("form.replied", (event) => seen.delete(event.data.id)),
      data.on("form.cancelled", (event) => seen.delete(event.data.id)),
      data.on("session.execution.started", (event) => {
        ended.delete(event.data.sessionID);
        failed.delete(event.data.sessionID);
        stop(event.data.sessionID);
      }),
      data.on("session.execution.succeeded", (event) => finish(event.data.sessionID)),
      data.on("session.execution.interrupted", (event) => {
        if (event.data.reason === "user") return;
        finish(event.data.sessionID);
      }),
      data.on("session.execution.failed", (event) => {
        const sessionID = event.data.sessionID;
        const message = event.data.error.message;
        ui.toast.show({ sessionID, title: "Session failed", message, variant: "error" });
        if (data.session.get(sessionID)?.parentID || ended.has(sessionID) || failed.has(sessionID)) return;
        failed.add(sessionID);
        if (!quiet(sessionID)) send("error", { message: `${title(sessionID)}: ${message}` });
        finish(sessionID);
      }),
    ];

    return () => {
      context.renderer.off("focus", onFocus);
      context.renderer.off("blur", onBlur);
      for (const id of idle.keys()) stop(id);
      dispose.toReversed().forEach((cleanup) => cleanup());
    };
  },
});
