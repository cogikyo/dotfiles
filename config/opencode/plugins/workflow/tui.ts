import { Plugin } from "@opencode/plugin/tui";
import { BoxRenderable, CodeRenderable, TextRenderable, type Renderable } from "@opentui/core";
import { readFileSync } from "node:fs";
import { parseRouting, routingPath } from "../lanes/routing.ts";
import { card, type Routes } from "./card.ts";
import { live } from "./live.ts";
import { parse, type Workflow } from "./parse.ts";
import { replan } from "./replan.ts";
import { sidebar } from "./sidebar.tsx";
import { palette, styled } from "./style.ts";
import { view } from "./view.tsx";

type Room = { width: number };

const inset = 2;

export default Plugin.define({
  id: "workflow",
  setup(context) {
    const room: Room = { width: 80 };
    const failures = new Set<string>();
    context.markdown.registerCodeBlockRenderer("workflow", (token, host) => {
      const fallback = host.defaultRender();
      try {
        return frame(context, { flow: parse(token.text, streaming(token.raw, fallback)), routes: routing() }, room);
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        if (!failures.has(message)) {
          failures.add(message);
          context.ui.toast.show({ variant: "error", title: "Workflow card", message });
        }
        return fallback;
      }
    });

    const store = live(context);
    const session = () => {
      const route = context.ui.router.current();
      return route.type === "session" ? route.sessionID : undefined;
    };
    const overlay = view(context, store, routing);
    const toggle = () => {
      const id = session();
      if (!id) return context.ui.toast.show({ variant: "warning", message: "Open a session to show its workflow" });
      overlay.toggle(id);
    };

    sidebar(context, store, overlay.open);

    context.keymap.layer(() => ({
      mode: "global",
      commands: [
        {
          id: "workflow.view",
          title: "Toggle workflow view",
          description: "Show this session's stored workflow with step and checklist progress",
          group: "Session",
          palette: true,
          bind: "<leader>w",
          run: toggle,
        },
        {
          id: "workflow.replan",
          title: "Replan workflow",
          description: "Pause the workflow and ask Collab to review lanes, compact, and write the next revision",
          group: "Session",
          palette: true,
          slash: { name: "replan" },
          async run() {
            const id = session();
            if (!id) return context.ui.toast.show({ variant: "warning", message: "Open a session to replan" });
            await replan(context, store, id).catch(store.fail);
          },
        },
      ],
    }));

    return store.dispose;
  },
});

function streaming(raw: string, fallback: Renderable | null) {
  const fence = /^ {0,3}(`{3,}|~{3,})/.exec(raw)?.[1];
  if (!(fallback instanceof CodeRenderable && fallback.streaming && fence)) return false;
  return !new RegExp(`\\n {0,3}${fence[0]}{${fence.length},}\\s*$`).test(raw);
}

function routing(): Routes | undefined {
  try {
    return parseRouting(readFileSync(routingPath, "utf8")).routes;
  } catch {
    return undefined;
  }
}

function frame(context: Plugin.Context, { flow, routes }: { flow: Workflow; routes?: Routes }, room: Room) {
  const tones = palette(context.theme, context.themeMode);
  const text = new TextRenderable(context.renderer, { width: "100%", wrapMode: "none" });
  let shown = 0;
  const show = (width: number) => {
    if (width === shown) return;
    shown = width;
    text.content = styled(card(flow, width, routes), tones);
  };
  const box = new BoxRenderable(context.renderer, {
    width: "100%",
    flexShrink: 0,
    marginTop: 0,
    backgroundColor: context.theme.background.raised.base,
    paddingTop: 1,
    paddingBottom: 1,
    paddingLeft: inset,
    paddingRight: inset,
    onSizeChange() {
      const width = Math.trunc(this.width) - 2 * inset;
      if (width <= 0) return;
      room.width = width;
      show(width);
    },
  });
  show(room.width);
  box.add(text);
  return box;
}
