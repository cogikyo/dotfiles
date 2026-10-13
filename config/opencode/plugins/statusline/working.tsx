import type { Plugin } from "@opencode/plugin/tui";
import { RGBA, type OptimizedBuffer, type Renderable } from "@opentui/core";
import os from "node:os";
import path from "node:path";
import { createEffect, onCleanup } from "solid-js";
import { hook, locate, type Prompt } from "./prompt.ts";

type Context = Plugin.Context;

export const period = 1_000;
export const tail = 3;
export const tick = 40;
const white = RGBA.fromInts(255, 255, 255);

let moving = true;

export const motion = () => moving;

export function heat(base: RGBA, amount: number) {
  return mix(base, white, 0.55 * amount);
}

export function mix(from: RGBA, to: RGBA, amount: number) {
  return RGBA.fromValues(
    from.r + (to.r - from.r) * amount,
    from.g + (to.g - from.g) * amount,
    from.b + (to.b - from.b) * amount,
    from.a,
  );
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
  const timer = setInterval(() => {
    if (moving) context.renderer.requestRender();
  }, tick);
  context.renderer.requestRender();
  return () => {
    clearInterval(timer);
    column();
    context.renderer.requestRender();
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
