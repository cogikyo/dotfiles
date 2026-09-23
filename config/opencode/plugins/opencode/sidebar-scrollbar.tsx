/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginModule } from "@opencode-ai/plugin/tui";
import { ScrollBoxRenderable, type BoxRenderable, type Renderable } from "@opentui/core";
import { onMount } from "solid-js";

const id = "opencode-sidebar-scrollbar";

function scrollbox(node: Renderable | null | undefined) {
  while (node && !(node instanceof ScrollBoxRenderable)) node = node.parent;
  return node ?? undefined;
}

function Anchor() {
  let anchor: BoxRenderable | undefined;
  onMount(() => {
    const target = scrollbox(anchor?.parent);
    if (target) target.verticalScrollBar.visible = false;
  });
  return <box ref={(node) => (anchor = node)} visible={false} />;
}

const tui: TuiPlugin = async (api) => {
  api.slots.register({
    order: 0,
    slots: {
      sidebar_content() {
        return <Anchor />;
      },
    },
  });
};

export default { id, tui } satisfies TuiPluginModule & { id: string };
