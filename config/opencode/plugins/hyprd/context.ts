import { Plugin } from "@opencode/plugin/tui";
import fs from "node:fs/promises";
import path from "node:path";

const runtime = process.env.XDG_RUNTIME_DIR;
const file = runtime && path.join(runtime, "opencode", "tui", `${process.pid}.json`);

async function processStart() {
  const stat = await fs.readFile("/proc/self/stat", "utf8");
  return stat
    .slice(stat.lastIndexOf(")") + 1)
    .trim()
    .split(/\s+/)[19];
}

async function publish(target: string, session: string | null) {
  const start = await processStart();
  await fs.mkdir(path.dirname(target), { recursive: true, mode: 0o700 });
  const temp = `${target}.tmp`;
  await fs.writeFile(temp, JSON.stringify({ start, session }), { mode: 0o600 });
  await fs.rename(temp, target);
}

export default Plugin.define({
  id: "hyprd-context",
  setup({ data, ui }) {
    if (!file || !process.env.KITTY_WINDOW_ID) return;
    let shown: string | null | undefined;
    let writes = Promise.resolve();

    const sync = () => {
      const route = ui.router.current();
      const session = route.type === "session" ? (data.session.root(route.sessionID) ?? route.sessionID) : null;
      if (session === shown) return;
      shown = session;
      writes = writes.then(() => publish(file, session)).catch(() => undefined);
    };

    sync();
    const timer = setInterval(sync, 1000);
    return async () => {
      clearInterval(timer);
      await writes;
      await fs.rm(file, { force: true });
    };
  },
});
