import type { Plugin } from "@opencode/plugin/tui";
import { RGBA, type CliRenderer, type OptimizedBuffer, type Renderable } from "@opentui/core";
import os from "node:os";
import path from "node:path";
import { createEffect, createSignal, onCleanup } from "solid-js";
import { hook, locate, type Prompt } from "./prompt.ts";

type Context = Plugin.Context;
type Hold = { readonly renderer?: CliRenderer };

export const period = 1_000;
export const tail = 3;
export const tick = 100;
const white = RGBA.fromInts(255, 255, 255);
// Hot reload gives each plugin entrypoint its own clock and timer.
const holds = new Set<Hold>();
const [clock, setClock] = createSignal(0);

let moving = true;
let timer: ReturnType<typeof setInterval> | undefined;

export const motion = () => moving;
export { clock };

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
    const prompt = locate(host);
    if (prompt) onCleanup(start(props.context, prompt));
  });

  return <box position="absolute" ref={(box) => (host = box)} />;
}

export async function refresh() {
  moving = await animations();
  return moving;
}

export function animate(renderer?: CliRenderer) {
  const hold: Hold = { renderer };
  let stopped = false;
  const begin = async () => {
    if (!(await refresh()) || stopped) return;
    holds.add(hold);
    timer ??= setInterval(beat, tick);
  };
  void begin();
  renderer?.requestRender();
  return () => {
    stopped = true;
    holds.delete(hold);
    if (holds.size === 0) {
      clearInterval(timer);
      timer = undefined;
      setClock(0);
    }
    renderer?.requestRender();
  };
}

function beat() {
  if (!moving) return setClock(0);
  setClock(performance.now());
  for (const hold of holds) hold.renderer?.requestRender();
}

function start(context: Context, prompt: Prompt) {
  const column = hook(prompt.cap, (buffer) => bar(buffer, prompt));
  const stop = animate(context.renderer);
  return () => {
    stop();
    column();
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
