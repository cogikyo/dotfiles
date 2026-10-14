import { Plugin } from "@opencode/plugin/tui";
import { list } from "./list.tsx";
import { open } from "./open.tsx";
import type { Kind, Picker } from "./rows.tsx";
import { source } from "./source.ts";

export default Plugin.define({
  id: "sessions",
  setup(context) {
    let shown: { kind: Kind } | undefined;
    let generation = 0;
    const route = () => JSON.stringify(context.ui.router.current());
    const picker: Picker = {
      context,
      source: source(context),
      claim() {
        const mine = ++generation;
        const at = route();
        // Replacing a dialog also resolves undefined, but keeps the host in modal mode.
        return () => generation === mine && route() === at && context.keymap.mode.current() !== "modal";
      },
      hold(kind) {
        generation++;
        const token = { kind };
        shown = token;
        return () => {
          if (shown === token) shown = undefined;
        };
      },
    };

    context.keymap.layer(() => ({
      mode: "global",
      commands: [
        {
          bind: "ctrl+o",
          title: "Open session or project",
          group: "Session",
          run: () => {
            if (shown?.kind !== "open") void open(picker);
          },
        },
        {
          bind: "<leader>r",
          title: "Switch session",
          group: "Session",
          run: () => {
            if (shown?.kind !== "list") void list(picker);
          },
        },
        {
          bind: "<leader>shift+return",
          title: "Finish session with /done",
          group: "Session",
          run: () => {
            const current = context.ui.router.current();
            if (current.type !== "session") return;
            const sessionID = context.data.session.root(current.sessionID);
            context.client.session
              .command({ sessionID, name: "done", text: "" })
              .catch((error: unknown) =>
                context.ui.toast.show({ title: "/done failed", message: String(error), variant: "error" }),
              );
          },
        },
      ],
    }));

    return picker.source.watch();
  },
});
