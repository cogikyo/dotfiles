import type { Plugin } from "@opencode/plugin/tui";
import type { OptimizedBuffer, Renderable, RGBA } from "@opentui/core";
import { createEffect, createSignal, onCleanup } from "solid-js";
import { covers, hook, locate, type Prompt } from "./prompt.ts";
import { animate, clock, motion } from "./working.tsx";

type Context = Plugin.Context;
type Plane = OptimizedBuffer["buffers"]["fg"];

export type Aim = { readonly hue: RGBA | undefined; readonly pacing: boolean };
export type Painter = {
  readonly draw: (buffer: OptimizedBuffer, aim: Aim, elapsed: number) => boolean;
  readonly close?: () => void;
};

export const ramp = 600;
const stall = 250;

export function Pace(props: {
  context: Context;
  on: boolean;
  hue: RGBA | undefined;
  open: (prompt: Prompt) => Painter;
  fail: (reason: string) => void;
}) {
  let host: Renderable | undefined;
  const [away, setAway] = createSignal(false);
  const [still, setStill] = createSignal(true);
  const leave = () => setAway(true);
  const back = () => setAway(false);

  createEffect(() => {
    const { renderer } = props.context;
    renderer.on("blur", leave);
    renderer.on("focus", back);
    onCleanup(() => {
      renderer.off("blur", leave);
      renderer.off("focus", back);
    });
  });

  createEffect(() => {
    const prompt = props.on && host ? locate(host) : undefined;
    if (!host || !prompt) return;
    const painter = props.open(prompt);
    let seen = 0;
    onCleanup(() => painter.close?.());
    onCleanup(
      last(
        props.context,
        host,
        (buffer) => {
          const beat = clock() && performance.now();
          const elapsed = !motion() ? Infinity : beat && seen ? Math.min(beat - seen, stall) : 0;
          seen = beat;
          const hue = props.hue;
          const pacing = motion() && hue !== undefined && (away() || prompt.editor.plainText === "");
          setStill(painter.draw(buffer, { hue, pacing }, elapsed));
        },
        props.fail,
      ),
    );
  });
  createEffect(() => {
    void props.hue;
    void away();
    props.context.renderer.requestRender();
  });
  createEffect(() => {
    if (!props.on || still() || covers.has(props.context.keymap.mode.current())) return;
    onCleanup(animate(props.context.renderer));
  });

  return <box position="absolute" ref={(box) => (host = box)} />;
}

export function toward(value: number, target: number, step: number) {
  return value < target ? Math.min(value + step, target) : Math.max(value - step, target);
}

export function smooth(value: number) {
  return value * value * (3 - 2 * value);
}

export function tint(fg: Plane, cell: number, hue: readonly number[], tone: number) {
  for (let channel = 0; channel < 3; channel++) {
    const base = (fg[cell * 4 + channel] ?? 0) & 0xff;
    fg[cell * 4 + channel] = Math.round(base + ((hue[channel] ?? base) - base) * tone);
  }
  fg[cell * 4 + 3] = (fg[cell * 4 + 3] ?? 0) & 0xff;
}

function last(
  context: Context,
  node: Renderable,
  draw: (buffer: OptimizedBuffer) => void,
  fail: (reason: string) => void,
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
      queueMicrotask(() => fail(reason));
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
