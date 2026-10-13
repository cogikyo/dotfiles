import type { Plugin } from "@opencode/plugin/tui";
import { RGBA, type OptimizedBuffer, type Renderable } from "@opentui/core";
import os from "node:os";
import path from "node:path";
import { createEffect, onCleanup } from "solid-js";

type Context = Plugin.Context;
type Frame = Renderable & { readonly border: unknown; readonly borderColor: RGBA };
type Prompt = {
  readonly frame: Frame;
  readonly cap: Renderable;
  readonly editor: Renderable;
  readonly row: Renderable;
};
type Draw = (this: Renderable, buffer: OptimizedBuffer, deltaTime: number) => void;

export const period = 1_000;
export const tail = 3;
export const tick = 40;
const gap = 2;
const white = RGBA.fromInts(255, 255, 255);
const keys: Readonly<Record<string, string>> = {
  ctrl: "\u{f0633}",
  alt: "\u{2325}",
  shift: "\u{21e7}",
  super: "\u{2318}",
  return: "\u{f0311}",
  enter: "\u{f0311}",
};

let moving = true;

export const motion = () => moving;

export function heat(base: RGBA, amount: number) {
  return mix(base, white, 0.55 * amount);
}

export function Working(props: { context: Context; sessionID?: string }) {
  let host: Renderable | undefined;
  const working = () =>
    props.sessionID !== undefined && props.context.data.session.status(props.sessionID) === "running";

  createEffect(() => {
    if (!working() || !host) return;
    void animations().then((value) => (moving = value));
    const prompt = locate(host);
    if (prompt) onCleanup(start(props.context, prompt));
  });

  return <box position="absolute" ref={(box) => (host = box)} />;
}

function start(context: Context, prompt: Prompt) {
  const column = hook(prompt.cap, (buffer) => bar(buffer, prompt));
  const tip = hook(prompt.row, (buffer) => queue(context, buffer, prompt.editor));
  const timer = setInterval(() => {
    if (moving) context.renderer.requestRender();
  }, tick);
  context.renderer.requestRender();
  return () => {
    clearInterval(timer);
    column();
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

function bar(buffer: OptimizedBuffer, prompt: Prompt) {
  const height = Math.round(prompt.frame.height);
  const head = Math.floor(((performance.now() % period) / period) * (height + 1 + tail));
  const base = prompt.frame.borderColor;
  const x = prompt.frame.screenX;
  for (let row = 0; row <= height; row++) {
    const step = head - row;
    const glow = moving && step >= 0 && step <= tail ? 1 - step / (tail + 1) : 0;
    const y = row < height ? prompt.frame.screenY + row : prompt.cap.screenY;
    buffer.drawText(row < height ? "\u{2590}" : "\u{259d}", x, y, heat(base, glow));
  }
}

function queue(context: Context, buffer: OptimizedBuffer, editor: Renderable) {
  const shortcut = context.keymap.shortcuts("prompt.queue")[0];
  if (!shortcut) return;
  const hint = shortcut
    .split("+")
    .map((key) => keys[key] ?? key.toUpperCase())
    .join(" ");
  const width = Bun.stringWidth(hint);
  const x = editor.screenX + editor.width - width;
  const y = editor.screenY;
  if (x - gap < editor.screenX) return;
  for (let column = x - gap; column < x + width; column++) if (cell(buffer, column, y) !== undefined) return;
  buffer.drawText(hint, x, y, context.theme.text.muted);
}

async function animations() {
  const home = process.env.XDG_CONFIG_HOME || path.join(os.homedir(), ".config");
  const file = await Bun.file(path.join(home, "opencode", "cli.json"))
    .text()
    .catch(() => undefined);
  const flags = [file, process.env.OPENCODE_CLI_CONFIG_CONTENT].map(flag);
  return flags.findLast((value) => value !== undefined) ?? true;
}

function flag(text: string | undefined) {
  if (!text) return undefined;
  try {
    const config: unknown = Bun.JSONC.parse(text);
    if (typeof config !== "object" || config === null || !("animations" in config)) return undefined;
    return typeof config.animations === "boolean" ? config.animations : undefined;
  } catch {
    return undefined;
  }
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
  const [frame, cap] = row?.parent?.getChildren() ?? [];
  if (!row || !frame || !cap || cap === row || !framed(frame)) return;
  const editor = frame.getChildren()[0]?.getChildren().find(editable);
  return editor && { frame, cap, editor, row };
}

function framed(node: Renderable): node is Frame {
  return "borderColor" in node && "border" in node && Array.isArray(node.border) && node.border.join() === "left";
}

function editable(node: Renderable) {
  return "plainText" in node && "cursorOffset" in node && "extmarks" in node;
}
