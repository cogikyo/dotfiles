import type { EditorView, OptimizedBuffer, RGBA, Renderable } from "@opentui/core";

type Frame = Renderable & { readonly border: unknown; readonly borderColor: RGBA };
type Draw = (this: Renderable, buffer: OptimizedBuffer, deltaTime: number) => void;
type Editor = Renderable & { readonly plainText: string; readonly editorView: EditorView };

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

function framed(node: Renderable): node is Frame {
  return "borderColor" in node && "border" in node && Array.isArray(node.border) && node.border.join() === "left";
}

function editable(node: Renderable): node is Editor {
  return "plainText" in node && "cursorOffset" in node && "extmarks" in node && "editorView" in node;
}
