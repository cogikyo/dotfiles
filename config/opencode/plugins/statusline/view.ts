import { spawn, type ChildProcess } from "node:child_process";
import { createInterface } from "node:readline";
import { Schema } from "effect";

const View = Schema.Struct({
  event: Schema.Literal("view"),
  data: Schema.Struct({
    panes: Schema.Array(Schema.Struct({ kitty_pid: Schema.Int, kitty_pane_id: Schema.Int, shown: Schema.Boolean })),
  }),
});

type Watch = {
  shown: boolean;
  readonly listeners: Set<() => void>;
  child?: ChildProcess;
  timer?: ReturnType<typeof setTimeout>;
};

const restart = 5_000;
const kitty = Number(process.env.KITTY_PID);
const pane = Number(process.env.KITTY_WINDOW_ID);
// Share the subscription across plugin loads and hot-reload generations.
const shared = globalThis as { [key: symbol]: Watch | undefined };
const key = Symbol.for("statusline.view");
const state = (shared[key] ??= { shown: true, listeners: new Set() });

export const shown = () => state.shown;

export const reveal = () => set(true);

export function watch(listener: () => void) {
  if (!kitty || !pane) return () => undefined;
  state.listeners.add(listener);
  if (!state.child && !state.timer) start();
  return () => {
    state.listeners.delete(listener);
    if (state.listeners.size > 0) return;
    clearTimeout(state.timer);
    state.timer = undefined;
    state.child?.kill();
    state.child = undefined;
    set(true);
  };
}

function set(next: boolean) {
  if (state.shown === next) return;
  state.shown = next;
  for (const listener of state.listeners) listener();
}

function start() {
  state.timer = undefined;
  const child = spawn("hyprd", ["subscribe", "view"], { stdio: ["ignore", "pipe", "ignore"] });
  state.child = child;
  createInterface({ input: child.stdout }).on("line", (line) => {
    const next = parse(line);
    if (next !== undefined) set(next);
  });
  child.once("error", () => undefined);
  child.once("close", () => {
    if (state.child !== child) return;
    state.child = undefined;
    set(true);
    if (state.listeners.size > 0) state.timer = setTimeout(start, restart);
  });
}

function parse(line: string) {
  let value: unknown;
  try {
    value = JSON.parse(line);
  } catch {
    return undefined;
  }
  if (!Schema.is(View)(value)) return undefined;
  return value.data.panes.find((entry) => entry.kitty_pid === kitty && entry.kitty_pane_id === pane)?.shown ?? true;
}
