import { Plugin } from "@opencode/plugin/tui";
import { createEffect, createRoot } from "solid-js";
import { api, type Focus } from "./rpc.ts";

const heartbeat = 60_000;
const timeout = 2_000;

export const store = (context: Plugin.Context) => context.storage.memory("focus", { initial: { focused: false } });

export default Plugin.define({
  id: "hyprd-focus",
  setup(context) {
    const { client, data, ui } = context;
    const rpc = client.rpc(api);
    const [focus] = store(context);
    let sent = "false";
    let failing = false;
    let flight: Promise<void> | undefined;
    let next: Focus | undefined;

    const session = () => {
      const route = ui.router.current();
      return route.type === "session" ? data.session.root(route.sessionID) : "";
    };

    const call = async (input: Focus) => {
      const location = { directory: data.location.default().directory };
      try {
        await rpc.focus(input, { location, signal: AbortSignal.timeout(timeout) });
        failing = false;
      } catch (error) {
        if (failing) return;
        failing = true;
        console.error("hyprd focus report failed", error);
      }
    };

    const drain = async (): Promise<void> => {
      const input = next;
      next = undefined;
      if (!input) {
        flight = undefined;
        return;
      }
      await call(input);
      return drain();
    };

    const report = (focused: boolean) => {
      next = { tui: process.pid, session: session(), focused };
      flight ??= drain();
      return flight;
    };

    const stop = createRoot((dispose) => {
      createEffect(() => {
        const state = focus.focused ? `true:${session()}` : "false";
        if (state === sent) return;
        sent = state;
        void report(focus.focused);
      });
      return dispose;
    });
    const timer = setInterval(() => {
      if (focus.focused) void report(true);
    }, heartbeat);

    return async () => {
      stop();
      clearInterval(timer);
      await report(false);
    };
  },
});
