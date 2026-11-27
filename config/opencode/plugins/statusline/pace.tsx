import type { Plugin } from "@opencode/plugin/tui";
import type { CliRenderer, OptimizedBuffer, Renderable, RGBA } from "@opentui/core";
import { createEffect, createSignal, onCleanup } from "solid-js";
import { covers, hook, locate, type Prompt } from "./prompt.ts";
import { animate, clock, motion } from "./working.tsx";

type Context = Plugin.Context;
type Plane = OptimizedBuffer["buffers"]["fg"];

export type Aim = { readonly hue: RGBA | undefined; readonly motion: boolean; readonly pacing: boolean };
export type Painter = {
  /** Paints one frame and returns true when nothing is moving, so the animation loop can stop. */
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
    const { renderer, keymap } = props.context;
    const painter = props.open(prompt);
    const fail = props.fail;
    let seen = 0;
    let painted = false;
    let live = true;
    const unhook = hook(host, () => {
      painted = true;
    });
    const post = (buffer: OptimizedBuffer) => {
      if (!live) return;
      const shown = painted;
      painted = false;
      if (!shown) {
        setStill(true);
        return;
      }
      if (covers.has(keymap.mode.current())) return;
      try {
        // Elapsed ms since the last frame, capped at `stall`; 0 on the first frame or while the shared clock is stopped, Infinity (snap to target) with animations off.
        const beat = clock() && performance.now();
        const elapsed = !motion() ? Infinity : beat && seen ? Math.min(beat - seen, stall) : 0;
        seen = beat;
        const hue = props.hue;
        // The light sweeps only while the user is not typing: the window is blurred or the editor is empty.
        const pacing = motion() && hue !== undefined && (away() || prompt.editor.plainText === "");
        setStill(painter.draw(buffer, { hue, motion: motion(), pacing }, elapsed));
      } catch (error) {
        stop();
        const reason = error instanceof Error ? error.message : String(error);
        queueMicrotask(() => fail(reason));
      }
    };
    const detach = last(renderer, post);
    const stop = () => {
      if (!live) return;
      live = false;
      detach();
      unhook();
      painter.close?.();
      setStill(true);
      renderer.requestRender();
    };
    onCleanup(stop);
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

/** Returns the RGB ints of the latest color passed, holding the previous color when given undefined so a fade-out keeps its hue. */
export function channels() {
  let color: RGBA | undefined;
  let ints: readonly number[] = [0, 0, 0];
  return (next: RGBA | undefined) => {
    if (next && next !== color) {
      color = next;
      ints = next.toInts();
    }
    return ints;
  };
}

// Adds `post` from an animation frame, not at once, so it runs after the post-process functions registered earlier and paints over them.
function last(renderer: CliRenderer, post: (buffer: OptimizedBuffer) => void) {
  const frame = renderer.requestAnimationFrame(() => renderer.addPostProcessFn(post));
  renderer.requestRender();
  return () => {
    renderer.cancelAnimationFrame(frame);
    renderer.removePostProcessFn(post);
  };
}
