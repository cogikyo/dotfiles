import type { TuiPluginApi } from "@opencode-ai/plugin/tui";
import type { RGBA } from "@opentui/core";
import { createSignal } from "solid-js";

export type IconAction = { icon: string; run: () => void };

export function ActionIcon(props: { api: TuiPluginApi; icon: string; fg: RGBA; action?: IconAction }) {
  const [hovered, setHovered] = createSignal(false);
  const shown = () => (hovered() ? props.action : undefined);

  return (
    <text
      fg={shown() ? props.api.theme.current.textMuted : props.fg}
      wrapMode="none"
      flexShrink={0}
      onMouseOver={() => setHovered(true)}
      onMouseOut={() => setHovered(false)}
      onMouseDown={(event) => {
        const action = shown();
        if (!action) return;
        event.stopPropagation();
        action.run();
      }}
    >
      {`${shown()?.icon ?? props.icon} `}
    </text>
  );
}
