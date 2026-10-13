import { Plugin } from "@opencode/plugin/tui";
import { MouseButton, type BaseRenderable, type TextRenderable } from "@opentui/core";
import { api, pinned } from "./rpc.ts";

const MESSAGES = 6;
const MESSAGE_CHARS = 600;
const REACH = 8;

function isTitle(node: BaseRenderable): node is TextRenderable {
  return "textAlign" in node && "_rename" in node && "pendingTitle" in node;
}

function descendant(node: BaseRenderable, depth: number): TextRenderable | undefined {
  if (isTitle(node)) return node;
  if (depth === 0) return undefined;
  for (const child of node.getChildren()) {
    const found = descendant(child, depth - 1);
    if (found) return found;
  }
  return undefined;
}

function title(anchor: BaseRenderable) {
  let from = anchor;
  for (let at = anchor.parent, step = 0; at && step < REACH; from = at, at = at.parent, step++) {
    for (const child of at.getChildren()) {
      if (child === from) continue;
      const found = descendant(child, 2);
      if (found) return found;
    }
  }
  return undefined;
}

export default Plugin.define({
  id: "retitle",
  setup(context) {
    const rpc = context.client.rpc(api);
    let warned = false;

    const shown = () => {
      const route = context.ui.router.current();
      return route.type === "session" ? route.sessionID : undefined;
    };
    const root = () => {
      const sessionID = shown();
      return sessionID ? context.data.session.root(sessionID) : undefined;
    };
    const recent = (sessionID: string) =>
      context.data.session.message
        .list(sessionID)
        .flatMap((message) => (message.type === "user" ? [message.text.replace(/\s+/gu, " ").trim()] : []))
        .filter(Boolean)
        .slice(-MESSAGES)
        .map((text) => text.slice(0, MESSAGE_CHARS));
    const run = async (sessionID: string | undefined, caps: boolean) => {
      const session = sessionID ? context.data.session.get(sessionID) : undefined;
      if (!session) return;
      const options = { location: { directory: session.location.directory } };
      const input = { sessionID: session.id, recent: recent(session.id) };
      try {
        const result = caps
          ? await rpc.caps({ ...input, on: !pinned(session.title) }, options)
          : await rpc.now(input, options);
        context.ui.toast.show({ message: `Retitled: ${result.title}`, variant: "info" });
      } catch (error) {
        context.ui.toast.show({ title: "Retitle failed", message: String(error), variant: "error" });
      }
    };
    const attach = (anchor: BaseRenderable) => {
      const node = title(anchor);
      if (!node) {
        if (!warned)
          context.ui.toast.show({
            title: "Retitle",
            message: "Sidebar title not found; V2 may have changed its layout. Use <leader>t and <leader>shift+t.",
            variant: "warning",
          });
        warned = true;
        return;
      }
      const box = node.parent;
      if (box) {
        box.alignItems = "center";
        box.paddingRight = 1;
      }
      node.textAlign = "center";
      node.onMouseUp = (event) => void run(shown(), event.button === MouseButton.RIGHT);
    };

    context.ui.slot({
      prepend: "sidebar.content",
      render: () => (
        <box position="absolute" width={0} height={0} ref={(anchor) => queueMicrotask(() => attach(anchor))} />
      ),
    });

    context.keymap.layer(() => ({
      commands: [
        {
          id: "retitle.now",
          title: "Retitle session",
          description: "Generate a new title now; pinned ALL CAPS titles stay in caps",
          group: "Session",
          bind: "<leader>t",
          palette: true,
          slash: { name: "retitle" },
          run: () => run(root(), false),
        },
        {
          id: "retitle.caps",
          title: "Toggle ALL CAPS title",
          description: "Pin a long-running session's title in caps so automatic retitles skip it",
          group: "Session",
          bind: "<leader>shift+t",
          palette: true,
          slash: { name: "caps" },
          run: () => run(root(), true),
        },
      ],
    }));
  },
});
