import { Plugin } from "@opencode/plugin/tui";
import { spawn, type ChildProcess } from "node:child_process";
import { createInterface } from "node:readline";
import { Schema } from "effect";
import { createSignal, For, Show } from "solid-js";

const Window = Schema.Struct({
  address: Schema.String,
  title: Schema.String,
  slot: Schema.Int.check(Schema.isGreaterThan(0)),
  workspace: Schema.String,
});
type Window = typeof Window.Type;

const Workspace = Schema.Struct({
  event: Schema.Literal("workspace"),
  data: Schema.Struct({ browser_qa: Schema.NullOr(Schema.Array(Schema.Unknown)) }),
});

const restart = 5_000;
const width = 30;

export default Plugin.define({
  id: "browser",
  setup(context) {
    const [windows, setWindows] = createSignal<readonly Window[]>([]);
    let watcher: ChildProcess | undefined;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let disposed = false;

    const watch = () => {
      const child = spawn("hyprd", ["subscribe", "workspace"], { stdio: ["ignore", "pipe", "ignore"] });
      watcher = child;
      createInterface({ input: child.stdout }).on("line", (line) => {
        const next = parse(line);
        if (next) setWindows(next);
      });
      child.once("error", () => undefined);
      child.once("close", () => {
        if (watcher === child) watcher = undefined;
        if (!disposed) timer = setTimeout(watch, restart);
      });
    };

    const fail = (message: string) => {
      if (!disposed) context.ui.toast.show({ title: "Browser QA", message, variant: "error" });
    };

    const focus = (slot: number) => {
      const child = spawn("hyprd", ["browser-qa", String(slot)], { stdio: ["ignore", "pipe", "pipe"] });
      let output = "";
      const collect = (chunk: Buffer) => {
        output = `${output}${chunk.toString()}`.slice(-1_024);
      };
      child.stdout.on("data", collect);
      child.stderr.on("data", collect);
      child.once("error", (cause) => fail(cause.message));
      child.once("close", (code) => {
        if (code !== 0 && code !== null) fail(output.trim() || `hyprd exited ${code}`);
      });
    };

    watch();

    context.ui.slot({
      after: "sidebar.content",
      render: () => {
        const theme = context.theme;
        return (
          <Show when={windows().length > 0}>
            <box>
              <text fg={theme.text.base}>
                <b>Browsers</b>
                <span style={{ fg: theme.text.muted }}>
                  {` ${windows().length} ${windows().length === 1 ? "window" : "windows"}`}
                </span>
              </text>
              <For each={windows()}>
                {(window) => (
                  <box flexDirection="row" onMouseUp={() => focus(window.slot)}>
                    <text fg={theme.text.action.primary.base} wrapMode="none" flexShrink={0}>
                      {`#${window.slot} `}
                    </text>
                    <text fg={theme.text.muted} wrapMode="none">
                      {clip(window.title)}
                    </text>
                  </box>
                )}
              </For>
            </box>
          </Show>
        );
      },
    });

    return () => {
      disposed = true;
      clearTimeout(timer);
      watcher?.kill();
    };
  },
});

function parse(line: string) {
  let value: unknown;
  try {
    value = JSON.parse(line);
  } catch {
    return undefined;
  }
  if (!Schema.is(Workspace)(value)) return undefined;
  return (value.data.browser_qa ?? [])
    .filter((item): item is Window => Schema.is(Window)(item) && item.workspace === `browser-qa-${item.slot}`)
    .toSorted((left, right) => left.slot - right.slot);
}

function clip(title: string) {
  const value = title.trim().replace(/\s+/g, " ") || "Untitled";
  return value.length <= width ? value : `${value.slice(0, width - 1)}…`;
}
