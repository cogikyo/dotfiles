import type { TuiPluginApi } from "@opencode-ai/plugin/tui";
import { createTextAttributes } from "@opentui/core";
import { Show, createSignal, type JSXElement } from "solid-js";

const TITLE_ATTRIBUTES = createTextAttributes({ bold: true });

export function SidebarSection(props: {
  api: TuiPluginApi;
  title: string;
  detail?: string | number | JSXElement;
  initiallyExpanded?: boolean;
  children: JSXElement;
}) {
  const [expanded, setExpanded] = createSignal(props.initiallyExpanded ?? true);

  return (
    <box flexDirection="column" gap={0}>
      <box flexDirection="row" gap={0} onMouseDown={() => setExpanded((value) => !value)}>
        <text fg={props.api.theme.current.text} wrapMode="none">
          {expanded() ? "▼ " : "▶ "}
        </text>
        <text fg={props.api.theme.current.text} attributes={TITLE_ATTRIBUTES}>
          {props.title}
        </text>
        <Detail api={props.api} detail={props.detail} />
      </box>
      <Show when={expanded()}>{props.children}</Show>
    </box>
  );
}

function Detail(props: { api: TuiPluginApi; detail?: string | number | JSXElement }) {
  if (props.detail === undefined || props.detail === "") return null;
  if (typeof props.detail === "string" || typeof props.detail === "number") {
    return <text fg={props.api.theme.current.textMuted}>{` ${props.detail}`}</text>;
  }
  return (
    <box flexDirection="row" gap={0}>
      <text fg={props.api.theme.current.textMuted}> </text>
      {props.detail}
    </box>
  );
}
