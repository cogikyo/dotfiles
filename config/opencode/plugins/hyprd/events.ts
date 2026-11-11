import { spawn, type ChildProcess } from "node:child_process";
import { createInterface } from "node:readline";
import { Schema } from "effect";

type Listener = { event: (value: unknown) => void; close?: () => void };

type Bus = {
  readonly listeners: Set<Listener>;
  readonly latest: Map<State["event"], unknown>;
  child?: ChildProcess;
  timer?: ReturnType<typeof setTimeout>;
};

const State = Schema.Struct({ event: Schema.Literals(["workspace", "view"]) });
type State = typeof State.Type;

const restart = 5_000;
const shared = globalThis as { [key: symbol]: Bus | undefined };
const key = Symbol.for("hyprd.events");
const bus: Bus = (shared[key] ??= { listeners: new Set(), latest: new Map() });

export function listen(event: Listener["event"], close?: Listener["close"]) {
  const listener = { event, close };
  bus.listeners.add(listener);
  if (!bus.child && !bus.timer) start();
  for (const value of bus.latest.values()) event(value);
  return () => {
    bus.listeners.delete(listener);
    if (bus.listeners.size > 0) return;
    clearTimeout(bus.timer);
    bus.timer = undefined;
    bus.child?.kill();
    bus.child = undefined;
    bus.latest.clear();
  };
}

function start() {
  bus.timer = undefined;
  const child = spawn("hyprd", ["subscribe", "workspace", "view", "navigate"], { stdio: ["ignore", "pipe", "ignore"] });
  bus.child = child;
  createInterface({ input: child.stdout }).on("line", (line) => {
    if (bus.child !== child) return;
    let value: unknown;
    try {
      value = JSON.parse(line);
    } catch {
      return;
    }
    if (Schema.is(State)(value)) bus.latest.set(value.event, value);
    for (const listener of bus.listeners) listener.event(value);
  });
  child.once("error", () => undefined);
  child.once("close", () => {
    if (bus.child !== child) return;
    bus.child = undefined;
    bus.latest.clear();
    for (const listener of bus.listeners) listener.close?.();
    if (bus.listeners.size > 0) bus.timer = setTimeout(start, restart);
  });
}
