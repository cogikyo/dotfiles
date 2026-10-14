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
    const picker: Picker = {
      context,
      source: source(context),
      prefs,
      toggle: () => {
        update((draft) => {
          draft.allProjects = !draft.allProjects;
        }).catch(() => undefined);
      },
      hold(kind) {
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
