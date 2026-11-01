import { Plugin } from "@opencode/plugin/tui";
import { ScrollBoxRenderable, type Renderable } from "@opentui/core";

type Direction = "row" | "row-reverse";

const width = 42;
const poll = 250;

const applied = new WeakMap<Renderable, Direction>();

function place(row: Renderable, pane: Renderable, left: boolean) {
  const sidebar = row.getChildren().some((child) => child !== pane && Math.round(child.width) === width);
  const direction: Direction = left && sidebar ? "row-reverse" : "row";
  if (applied.get(row) === direction) return;
  row.flexDirection = direction;
  applied.set(row, direction);
}

function hide(node: Renderable) {
  if (node.id === "session-pane" || node.id === "session-panel") return;
  if (node instanceof ScrollBoxRenderable) {
    if (node.verticalScrollBar.visible) node.verticalScrollBar.visible = false;
    return;
  }
  for (const child of node.getChildren()) hide(child);
}

function apply(root: Renderable, left: boolean) {
  const pane = root.findDescendantById("session-pane");
  const row = pane?.parent;
  if (!pane || !row || row.isDestroyed) return;
  place(row, pane, left);
  hide(row);
}

export default Plugin.define({
  id: "sidebar",
  setup(context) {
    const [side, update] = context.storage.store("side", { initial: { left: false } });
    const sync = () => apply(context.renderer.root, side.left);

    const flip = async () => {
      await update((draft) => {
        draft.left = !draft.left;
      });
      sync();
      context.renderer.requestRender();
    };
    context.keymap.layer(() => ({
      mode: "global",
      commands: [
        {
          id: "sidebar.side.toggle",
          title: "Toggle sidebar side",
          description: "Move the session sidebar between the left and right of the transcript",
          group: "Session",
          palette: true,
          slash: { name: "sidebar" },
          run: flip,
        },
      ],
    }));
    context.keymap.layer(() => ({
      commands: [{ bind: "<leader>a", title: "Toggle sidebar side", group: "Session", run: flip }],
    }));

    const timer = setInterval(sync, poll);
    return () => clearInterval(timer);
  },
});
