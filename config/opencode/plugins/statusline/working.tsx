import type { Plugin } from "@opencode/plugin/tui";
import { RGBA, type OptimizedBuffer, type Renderable } from "@opentui/core";
import { createEffect, onCleanup } from "solid-js";
import { read } from "./toast.tsx";

type Context = Plugin.Context;
type Frame = Renderable & { readonly border: unknown; readonly borderColor: RGBA };
type Prompt = { readonly frame: Frame; readonly editor: Renderable; readonly row: Renderable };
type Draw = (this: Renderable, buffer: OptimizedBuffer, deltaTime: number) => void;

const period = 1_000;
const tail = 3;
const tick = 40;
const gap = 2;
const still = "[⋯]";
const white = RGBA.fromInts(255, 255, 255);

export function Working(props: { context: Context; sessionID?: string }) {
  let host: Renderable | undefined;
  const working = () =>
    props.sessionID !== undefined && props.context.data.session.status(props.sessionID) === "running";

  createEffect(() => {
    if (!working() || !host) return;
    const prompt = locate(host);
    if (prompt) onCleanup(start(props.context, prompt));
  });

  return <box position="absolute" ref={(box) => (host = box)} />;
}

function start(context: Context, prompt: Prompt) {
  let animate = true;
  const border = hook(prompt.frame, (buffer) => {
    animate = !prompt.row.getChildren().some((child) => stilled(child, 4));
    if (animate) pulse(buffer, prompt.frame);
  });
  const tip = hook(prompt.row, (buffer) => queue(context, buffer, prompt.editor));
  const timer = setInterval(() => {
    if (animate) context.renderer.requestRender();
  }, tick);
  context.renderer.requestRender();
  return () => {
    clearInterval(timer);
    border();
    tip();
    context.renderer.requestRender();
  };
}

function hook(node: Renderable, draw: (buffer: OptimizedBuffer) => void) {
  const previous = node.renderAfter;
  const after: Draw = function (buffer, deltaTime) {
    previous?.call(this, buffer, deltaTime);
    draw(buffer);
  };
  node.renderAfter = after;
  return () => {
    if (node.renderAfter === after) node.renderAfter = previous;
  };
}

function pulse(buffer: OptimizedBuffer, frame: Frame) {
  const height = Math.round(frame.height);
  const head = Math.floor(((performance.now() % period) / period) * (height + tail));
  const base = frame.borderColor;
  const bright = mix(base, white, 0.55);
  for (let step = 0; step <= tail; step++) {
    const row = head - step;
    if (row < 0 || row >= height) continue;
    const x = frame.screenX;
    const y = frame.screenY + row;
    const glyph = cell(buffer, x, y);
    if (glyph) buffer.drawText(glyph, x, y, mix(base, bright, 1 - step / (tail + 1)));
  }
}

function queue(context: Context, buffer: OptimizedBuffer, editor: Renderable) {
  const key = context.keymap.shortcuts("prompt.queue")[0]?.replace(/\breturn\b/g, "enter");
  if (!key) return;
  const label = " queue";
  const width = Bun.stringWidth(key + label);
  const x = editor.screenX + editor.width - width;
  const y = editor.screenY;
  if (x - gap < editor.screenX) return;
  for (let column = x - gap; column < x + width; column++) if (cell(buffer, column, y) !== undefined) return;
  const theme = context.theme.text;
  buffer.drawText(key, x, y, theme.base);
  buffer.drawText(label, x + Bun.stringWidth(key), y, theme.muted);
}

function cell(buffer: OptimizedBuffer, x: number, y: number) {
  if (x < 0 || y < 0 || x >= buffer.width || y >= buffer.height) return;
  const code = buffer.buffers.char[y * buffer.width + x] ?? 0;
  if (code === 0 || code === 32) return;
  return code <= 0x10ffff ? String.fromCodePoint(code) : "";
}

function mix(from: RGBA, to: RGBA, amount: number) {
  return RGBA.fromValues(
    from.r + (to.r - from.r) * amount,
    from.g + (to.g - from.g) * amount,
    from.b + (to.b - from.b) * amount,
    from.a,
  );
}

function locate(host: Renderable): Prompt | undefined {
  const row = host.parent;
  const [frame] = row?.parent?.getChildren() ?? [];
  if (!row || !frame || frame === row || !framed(frame)) return;
  const editor = frame.getChildren()[0]?.getChildren().find(editable);
  return editor && { frame, editor, row };
}

function framed(node: Renderable): node is Frame {
  return "borderColor" in node && "border" in node && Array.isArray(node.border) && node.border.join() === "left";
}

function editable(node: Renderable) {
  return "plainText" in node && "cursorOffset" in node && "extmarks" in node;
}

function stilled(node: Renderable, depth: number): boolean {
  if (read(node) === still) return true;
  return depth > 0 && node.getChildren().some((child) => stilled(child, depth - 1));
}
