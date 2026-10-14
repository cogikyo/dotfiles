import { Plugin } from "@opencode/plugin/tui";
import { list } from "./list.tsx";
import { open } from "./open.tsx";
import type { Kind, Picker } from "./rows.tsx";
import { source } from "./source.ts";

export default Plugin.define({
  id: "sessions",
  setup(context) {
    const [prefs, update] = context.storage.store("list", { initial: { allProjects: true } });
    let shown: { kind: Kind } | undefined;
    let generation = 0;
    const route = () => JSON.stringify(context.ui.router.current());
    const picker: Picker = {
      context,
      source: source(context),
      prefs,
      toggle: () => {
        update((draft) => {
          draft.allProjects = !draft.allProjects;
        }).catch(() => undefined);
      },
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
          bind: "<leader>s",
          title: "Switch session",
          group: "Session",
          run: () => {
            if (shown?.kind !== "list") void list(picker);
          },
        },
      ],
    }));

    return picker.source.watch();
  },
});
