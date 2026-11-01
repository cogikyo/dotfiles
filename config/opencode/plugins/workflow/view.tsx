import type { Plugin } from "@opencode/plugin/tui";
import type { BoxRenderable, Renderable, TextRenderable } from "@opentui/core";
import { createMemo, createRenderEffect, createSignal, onCleanup, Show } from "solid-js";
import { createStore } from "solid-js/store";
import { locate } from "../byline/scroll.ts";
import { spinner } from "../context/section.tsx";
import { card, type Routes } from "./card.ts";
import type { Live } from "./live.ts";
import { palette, styled } from "./style.ts";

type Context = Plugin.Context;

export type View = ReturnType<typeof view>;

const pane = "session-pane";

export function view(context: Context, live: Live, routes: () => Routes | undefined) {
  const [open, setOpen] = createStore<Record<string, boolean>>({});
  context.ui.slot({
    append: "session.composer.top",
    render: (input) => (
      <Show when={open[input.sessionID]}>
        <Card context={context} live={live} sessionID={input.sessionID} routes={routes()} />
      </Show>
    ),
  });
  return {
    open: (sessionID: string) => setOpen(sessionID, true),
    toggle: (sessionID: string) => setOpen(sessionID, !open[sessionID]),
  };
}

function top(node: Renderable) {
  for (let at: Renderable | null = node; at; at = at.parent) if (at.id === pane) return at.y;
  return 0;
}

function hide(node: Renderable) {
  const transcript = locate(node);
  if (!transcript) return;
  // Byline paints after render, so z-index alone cannot cover the transcript.
  transcript.visible = false;
  return () => {
    if (!transcript.isDestroyed) transcript.visible = true;
  };
}

function Card(props: { context: Context; live: Live; sessionID: string; routes?: Routes }) {
  const [room, setRoom] = createSignal(0);
  const [height, setHeight] = createSignal(0);
  let restore: (() => void) | undefined;
  onCleanup(() => restore?.());
  const shown = createMemo(() => props.live.shown(props.sessionID));
  const tones = createMemo(() => palette(props.context.theme, props.context.themeMode));
  const frame = spinner(() => Object.values(shown()?.progress.phases ?? {}).includes("running"));
  const content = createMemo(() => {
    const current = shown();
    if (!current || room() <= 0) return;
    const rows = card(current.workflow, room(), props.routes, { ...current.progress, frame: frame() });
    return styled(rows, tones());
  });

  return (
    <box
      position="absolute"
      bottom="100%"
      left={0}
      width="100%"
      height={height()}
      zIndex={10}
      backgroundColor={props.context.theme.background.base}
      renderBefore={function (this: BoxRenderable) {
        const below = this.parent;
        if (!below) return;
        setHeight(Math.max(0, below.y - top(below)));
        restore ??= hide(this);
      }}
      onSizeChange={function () {
        setRoom(Math.trunc(this.width));
      }}
    >
      <Show
        when={content()}
        fallback={
          <text fg={props.context.theme.text.muted} wrapMode="none">
            No active workflow in this session.
          </text>
        }
      >
        {(text) => (
          <scrollbox
            flexGrow={1}
            minHeight={0}
            verticalScrollbarOptions={{ visible: false }}
            horizontalScrollbarOptions={{ visible: false }}
          >
            <text
              ref={(node: TextRenderable) =>
                createRenderEffect(() => {
                  node.content = text();
                })
              }
              wrapMode="none"
            />
          </scrollbox>
        )}
      </Show>
    </box>
  );
}
