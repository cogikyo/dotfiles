import type { Plugin } from "@opencode/plugin/tui";
import type { OptimizedBuffer, Renderable } from "@opentui/core";
import { createEffect, onCleanup } from "solid-js";
import { hook, locate } from "../statusline/prompt.ts";
import { animate, clock } from "../statusline/working.tsx";
import { arrows } from "./arrows.ts";
import { wheel } from "./wheel.ts";

type Context = Plugin.Context;

const covers = new Set(["modal", "menu"]);

export function Look(props: { context: Context; on: boolean; busy: boolean; fail: (error: unknown) => void }) {
  let host: Renderable | undefined;

  createEffect(() => {
    const prompt = props.on && host ? locate(host) : undefined;
    if (!host || !prompt) return;
    const icon = wheel(props.context.renderer);
    onCleanup(icon.close);
    onCleanup(
      last(
        props.context,
        host,
        (buffer) => {
          const cursor = props.context.renderer.getCursorState();
          icon.draw(buffer, prompt.editor, clock());
          arrows(buffer, prompt, cursor.visible ? { x: cursor.x - 1, y: cursor.y - 1 } : undefined);
        },
        props.fail,
      ),
    );
  });
  createEffect(() => {
    if (props.on && props.busy) onCleanup(animate(props.context.renderer));
  });

  return <box position="absolute" ref={(box) => (host = box)} />;
}

function last(
  context: Context,
  node: Renderable,
  draw: (buffer: OptimizedBuffer) => void,
  fail: (error: unknown) => void,
) {
  const renderer = context.renderer;
  let painted = false;
  let live = true;
  const unhook = hook(node, () => {
    painted = true;
  });
  const post = (buffer: OptimizedBuffer) => {
    if (!live) return;
    try {
      if (painted && !covers.has(context.keymap.mode.current())) draw(buffer);
    } catch (error) {
      live = false;
      const reason = error instanceof Error ? error.message : String(error);
      queueMicrotask(() => fail(new Error(`look off, ${reason}`)));
    }
    painted = false;
    renderer.removePostProcessFn(post);
    if (live) renderer.addPostProcessFn(post);
  };
  renderer.addPostProcessFn(post);
  renderer.requestRender();
  return () => {
    live = false;
    unhook();
    renderer.removePostProcessFn(post);
    renderer.requestRender();
  };
}
