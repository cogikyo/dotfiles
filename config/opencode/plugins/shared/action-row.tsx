import type { TuiPluginApi } from "@opencode-ai/plugin/tui";
import { createSignal, type JSXElement } from "solid-js";

export type RowAction = { icon: string; run: () => void };

export function ActionRow(props: { api: TuiPluginApi; action?: RowAction; onPress: () => void; children: JSXElement }) {
  const [hovered, setHovered] = createSignal(false);
  const shown = () => (hovered() ? props.action : undefined);

  return (
    <box
      flexDirection="row"
      gap={0}
      onMouseOver={() => setHovered(true)}
      onMouseOut={() => setHovered(false)}
      onMouseDown={() => props.onPress()}
    >
      <box flexDirection="row" gap={0} flexGrow={1} flexShrink={1}>
        {props.children}
      </box>
      <text
        fg={props.api.theme.current.textMuted}
        wrapMode="none"
        flexShrink={0}
        onMouseDown={(event) => {
          const action = shown();
          if (!action) return;
          event.stopPropagation();
          action.run();
        }}
      >
        {` ${shown()?.icon ?? " "}`}
      </text>
    </box>
  );
}
