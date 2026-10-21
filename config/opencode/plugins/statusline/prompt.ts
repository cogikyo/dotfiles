import type { Plugin } from "@opencode/plugin/tui";
import type { OptimizedBuffer, RGBA, Renderable } from "@opentui/core";

type Frame = Renderable & { readonly border: unknown; readonly borderColor: RGBA };
type Draw = (this: Renderable, buffer: OptimizedBuffer, deltaTime: number) => void;
type Editor = Renderable & { readonly plainText: string };

const covers = new Set(["modal", "menu"]);

export type Prompt = {
  readonly frame: Frame;
  readonly cap: Renderable;
  readonly editor: Editor;
  readonly row: Renderable;
};

export function locate(host: Renderable): Prompt | undefined {
  const row = host.parent;
  const [frame, cap] = row?.parent?.getChildren() ?? [];
  if (!row || !frame || !cap || cap === row || !framed(frame)) return;
  const editor = frame.getChildren()[0]?.getChildren().find(editable);
  return editor && { frame, cap, editor, row };
}

export function hook(node: Renderable, draw: (buffer: OptimizedBuffer) => void) {
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

export function overlay(context: Plugin.Context, node: Renderable, draw: (buffer: OptimizedBuffer) => void) {
  let painted = false;
  const unhook = hook(node, () => {
    painted = true;
  });
  const post = (buffer: OptimizedBuffer) => {
    if (painted && !covers.has(context.keymap.mode.current())) draw(buffer);
    painted = false;
  };
  context.renderer.addPostProcessFn(post);
  return () => {
    unhook();
    context.renderer.removePostProcessFn(post);
  };
}

export function cell(buffer: OptimizedBuffer, x: number, y: number) {
  if (x < 0 || y < 0 || x >= buffer.width || y >= buffer.height) return;
  const code = buffer.buffers.char[y * buffer.width + x] ?? 0;
  if (code === 0 || code === 32) return;
  return code <= 0x10ffff ? String.fromCodePoint(code) : "";
}

function framed(node: Renderable): node is Frame {
  return "borderColor" in node && "border" in node && Array.isArray(node.border) && node.border.join() === "left";
}

function editable(node: Renderable): node is Editor {
  return "plainText" in node && "cursorOffset" in node && "extmarks" in node;
}
