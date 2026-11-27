import { Plugin } from "@opencode/plugin/tui";
import { watch, type FSWatcher } from "node:fs";
import path from "node:path";
import { build, source } from "./build.ts";

export function watchTheme(from: string, onError: (message: string) => void) {
  let last: string | undefined;
  let failed = false;
  let disposed = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let queue = Promise.resolve();
  let watcher: FSWatcher | undefined;

  const fail = (error: unknown) => {
    if (!disposed) onError(error instanceof Error ? error.message : String(error));
  };
  const run = async () => {
    try {
      const { content, transcript, written } = await build(from);
      const state = JSON.stringify([content, transcript]);
      const changed = written || failed || (last !== undefined && state !== last);
      last = state;
      failed = false;
      if (!disposed && changed && process.listenerCount("SIGUSR2") > 0) process.kill(process.pid, "SIGUSR2");
    } catch (error) {
      failed = true;
      fail(error);
    }
  };
  const enqueue = () => {
    queue = queue.then(run);
  };

  try {
    watcher = watch(path.dirname(from), (_event, filename) => {
      if (filename !== path.basename(from)) return;
      clearTimeout(timer);
      timer = setTimeout(enqueue, 100);
    });
    watcher.on("error", (error) => {
      watcher?.close();
      fail(error);
    });
  } catch (error) {
    fail(error);
  }
  enqueue();

  return () => {
    disposed = true;
    clearTimeout(timer);
    watcher?.close();
  };
}

export default Plugin.define({
  id: "vagari",
  setup(context) {
    return watchTheme(source, (message) => context.ui.toast.show({ variant: "error", title: "Vagari theme", message }));
  },
});
