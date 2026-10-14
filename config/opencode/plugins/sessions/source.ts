import type { Plugin } from "@opencode/plugin/tui";
import { createSignal } from "solid-js";

export type Context = Plugin.Context;
export type Session = ReturnType<Context["data"]["session"]["list"]>[number];
export type Source = ReturnType<typeof source>;

const LIMIT = 10_000;

export const message = (error: unknown) => (error instanceof Error ? error.message : JSON.stringify(error));

export function source(context: Context) {
  const [loaded, setLoaded] = createSignal<Session[]>([]);
  const gone = new Set<string>();
  let pending: Promise<void> | undefined;

  const drop = (sessionID: string) => {
    gone.add(sessionID);
    setLoaded((list) => list.filter((session) => session.id !== sessionID));
  };

  const refresh = async () => {
    gone.clear();
    try {
      const page = await context.client.session.list({ limit: LIMIT, order: "desc", parentID: null });
      setLoaded(page.data.filter((session) => !gone.has(session.id)));
    } catch (error) {
      context.ui.toast.show({ title: "Could not refresh sessions", message: message(error), variant: "error" });
    }
  };

  const load = () =>
    (pending ??= refresh().finally(() => {
      pending = undefined;
    }));

  return {
    loaded,
    load,
    drop,
    rename(sessionID: string, title: string) {
      setLoaded((list) => list.map((session) => (session.id === sessionID ? { ...session, title } : session)));
    },
    watch: () => context.data.on("session.deleted", (event) => drop(event.data.sessionID)),
  };
}

export function sessions(context: Context, loaded: Session[]) {
  const seen = new Set<string>();
  return [...context.data.session.list(), ...loaded].filter((session) => {
    if (session.parentID || seen.has(session.id)) return false;
    seen.add(session.id);
    return true;
  });
}
