import { Schema } from "effect";
import { listen } from "../hyprd/events.ts";

const View = Schema.Struct({
  event: Schema.Literal("view"),
  data: Schema.Struct({
    panes: Schema.Array(Schema.Struct({ kitty_pid: Schema.Int, kitty_pane_id: Schema.Int, shown: Schema.Boolean })),
  }),
});

type Watch = {
  shown: boolean;
  readonly listeners: Set<() => void>;
  stop?: () => void;
};

const kitty = Number(process.env.KITTY_PID);
const pane = Number(process.env.KITTY_WINDOW_ID);
// Share view state across plugin loads and hot-reload generations.
const shared = globalThis as { [key: symbol]: Watch | undefined };
const key = Symbol.for("statusline.view");
const state = (shared[key] ??= { shown: true, listeners: new Set() });

export const shown = () => state.shown;

export const reveal = () => set(true);

export function watch(listener: () => void) {
  if (!kitty || !pane) return () => undefined;
  state.listeners.add(listener);
  state.stop ??= listen(
    (value) => {
      const next = parse(value);
      if (next !== undefined) set(next);
    },
    () => set(true),
  );
  return () => {
    state.listeners.delete(listener);
    if (state.listeners.size > 0) return;
    state.stop?.();
    state.stop = undefined;
    set(true);
  };
}

function set(next: boolean) {
  if (state.shown === next) return;
  state.shown = next;
  for (const listener of state.listeners) listener();
}

function parse(value: unknown) {
  if (!Schema.is(View)(value)) return undefined;
  return value.data.panes.find((entry) => entry.kitty_pid === kitty && entry.kitty_pane_id === pane)?.shown ?? true;
}
