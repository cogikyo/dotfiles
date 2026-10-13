import { Plugin } from "@opencode/plugin/tui";
import { RGBA } from "@opentui/core";
import { createEffect, createSignal, onCleanup, Show, type JSX } from "solid-js";

export const hover = RGBA.fromHex("#6380ec");

const frames = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];

export function spinner(active: () => boolean) {
  const [frame, setFrame] = createSignal(0);
  createEffect(() => {
    if (!active()) return;
    const timer = setInterval(() => setFrame((value) => value + 1), 100);
    onCleanup(() => clearInterval(timer));
  });
  return () => frames[frame() % frames.length];
}

export function Section(props: { context: Plugin.Context; title: string; detail: string; children: JSX.Element }) {
  const [open, setOpen] = createSignal(true);
  const theme = () => props.context.theme.text;

  return (
    <box flexDirection="column">
      <box flexDirection="row" onMouseDown={() => setOpen((value) => !value)}>
        <text fg={theme().base} wrapMode="none">
          {open() ? "▼ " : "▶ "}
        </text>
        <text fg={theme().base} wrapMode="none">
          <b>{props.title}</b>
        </text>
        <text fg={theme().muted} wrapMode="none">{` ${props.detail}`}</text>
      </box>
      <Show when={open()}>{props.children}</Show>
    </box>
  );
}
