import { Plugin } from "@opencode/plugin/tui";
import { api } from "./rpc.ts";

const THROTTLE_MS = 1_000;

export default Plugin.define({
  id: "question",
  setup(context) {
    const { data, keymap, renderer, ui } = context;
    const rpc = context.client.rpc(api);
    const sent = new Map<string, number>();
    let failing = false;

    const asking = () => {
      if (keymap.mode.current() !== "form") return;
      const route = ui.router.current();
      if (route.type !== "session") return;
      const [form] = data.session.form.list(route.sessionID) ?? [];
      return form?.metadata?.kind === "question" ? route.sessionID : undefined;
    };

    const input = () => {
      const sessionID = asking();
      if (!sessionID) return;
      const now = Date.now();
      if (now - (sent.get(sessionID) ?? 0) < THROTTLE_MS) return;
      sent.set(sessionID, now);
      const directory = data.session.get(sessionID)?.location.directory;
      rpc.typed({ sessionID }, directory ? { location: { directory } } : undefined).then(
        () => (failing = false),
        (error: unknown) => {
          if (failing) return;
          failing = true;
          console.error("question: typing report failed", error);
        },
      );
    };

    renderer.keyInput.prependListener("keypress", input);
    renderer.keyInput.prependListener("paste", input);
    return () => {
      renderer.keyInput.off("keypress", input);
      renderer.keyInput.off("paste", input);
    };
  },
});
