import type { LocationRef } from "@opencode/client/promise";
import { Plugin } from "@opencode/plugin/tui";
import { createEffect, createRoot } from "solid-js";
import fs from "node:fs/promises";
import path from "node:path";
import { prompts } from "../statusline/prompt.ts";

const runtime = process.env.XDG_RUNTIME_DIR;
const file = runtime && path.join(runtime, "opencode", "tui", `${process.pid}.json`);
const interval = 1_000;
const heartbeat = 60_000;
const settle = 300;

type Report = {
  readonly session: string | null;
  readonly draft: boolean | null;
  readonly tabs: number;
  readonly pending: boolean;
};

function isLocation(value: unknown): value is LocationRef {
  return typeof value === "object" && value !== null && "directory" in value && typeof value.directory === "string";
}

async function processStart() {
  const stat = await fs.readFile("/proc/self/stat", "utf8");
  return stat
    .slice(stat.lastIndexOf(")") + 1)
    .trim()
    .split(/\s+/)[19];
}

async function publish(target: string, report: Report) {
  const start = await processStart();
  await fs.mkdir(path.dirname(target), { recursive: true, mode: 0o700 });
  const temp = `${target}.tmp`;
  await fs.writeFile(temp, JSON.stringify({ start, at: Date.now(), ...report }), { mode: 0o600 });
  await fs.rename(temp, target);
}

export default Plugin.define({
  id: "hyprd-context",
  setup(api) {
    const { data, renderer, ui } = api;
    if (!file || !process.env.KITTY_WINDOW_ID) return;
    let published = "";
    let at = 0;
    let writes = Promise.resolve();
    let typing: ReturnType<typeof setTimeout> | undefined;
    let editors: ReturnType<typeof prompts> = [];
    const shown = new Map<string, { readonly session: string; readonly location?: LocationRef }>();

    // Host routes render undefined form and permission lists as empty via ?? [].
    const forms = (session: string, location?: LocationRef) => {
      const list = data.session.form.list(session, location) ?? [];
      const snapshot = location && { directory: location.directory, workspaceID: location.workspaceID };
      for (const form of list) shown.set(form.id, { session, location: snapshot });
      return list;
    };

    const displayed = (route: ReturnType<typeof ui.router.current>) => {
      if (route.type === "plugin") return false;
      if (route.type === "home") {
        const carried = "location" in route ? route.location : undefined;
        return forms("global", isLocation(carried) ? carried : data.location.default()).length > 0;
      }
      const session = data.session.get(route.sessionID);
      const family = session?.parentID ? [] : [...new Set([route.sessionID, ...data.session.family(route.sessionID)])];
      const lists = [
        forms("global", session?.location ?? api.location),
        ...family.flatMap((id) => [data.session.permission.list(id) ?? [], forms(id)]),
      ];
      return lists.some((list) => list.length > 0);
    };

    const pending = (route: ReturnType<typeof ui.router.current>) => {
      const current = displayed(route);
      for (const [id, key] of shown) {
        if (!data.session.form.list(key.session, key.location)?.some((form) => form.id === id)) shown.delete(id);
      }
      return current || shown.size > 0;
    };

    const observe = (): Report => {
      const route = ui.router.current();
      const session = route.type === "session" ? (data.session.root(route.sessionID) ?? route.sessionID) : null;
      // The plugin API exposes no draft reader, so find prompts in the host render tree.
      if (editors.length === 0 || editors.some((editor) => editor.isDestroyed)) editors = prompts(renderer.root);
      return {
        session,
        draft: editors.length === 0 ? null : editors.some((editor) => editor.plainText.trim() !== ""),
        tabs: ui.tabs.enabled() ? ui.tabs.list().length : 1,
        pending: pending(route),
      };
    };

    const sync = () => {
      const report = observe();
      const key = JSON.stringify(report);
      if (key === published && Date.now() - at < heartbeat) return;
      published = key;
      at = Date.now();
      writes = writes.then(() => publish(file, report)).catch(() => undefined);
    };

    const keypress = () => {
      clearTimeout(typing);
      typing = setTimeout(sync, settle);
    };

    // Record forms reactively before navigation; partial answers survive in host memory.
    const record = createRoot((dispose) => {
      createEffect(() => displayed(ui.router.current()));
      return dispose;
    });
    sync();
    const timer = setInterval(sync, interval);
    renderer.keyInput.on("keypress", keypress);
    renderer.on("blur", sync);
    return async () => {
      record();
      clearInterval(timer);
      clearTimeout(typing);
      renderer.keyInput.off("keypress", keypress);
      renderer.off("blur", sync);
      await writes;
      await fs.rm(file, { force: true });
    };
  },
});
