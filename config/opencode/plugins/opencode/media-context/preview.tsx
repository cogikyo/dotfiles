/** @jsxImportSource @opentui/solid */
import type { TuiPluginApi } from "@opencode-ai/plugin/tui";
import { spawn, type ChildProcess } from "node:child_process";
import { onCleanup, onMount, untrack } from "solid-js";
import type { MediaRegistryEntry } from "./store";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Kitty image overlay                                                                           │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

export type MediaItem = {
  entry: MediaRegistryEntry;
};

/** Session image selected for a Kitty overlay. */
export type PreviewState = {
  sessionID: string;
  item: MediaItem;
  imageID: number;
};

/** Shows a dismissible image overlay in Kitty, clearing the graphic when it unmounts. */
export function ImageOverlay(props: { api: TuiPluginApi; preview: PreviewState; onClose: () => void }) {
  return (
    <box
      width="100%"
      height="100%"
      position="absolute"
      top={0}
      right={0}
      bottom={0}
      left={0}
      zIndex={1000}
      backgroundColor="#000000"
      opacity={0.7}
      focusable
      focused
      onMouseDown={() => props.onClose()}
      onKeyDown={(event) => {
        if (event.name === "escape") props.onClose();
      }}
      onSizeChange={() => props.api.renderer.requestRender()}
    >
      <KittyImageLayer api={props.api} preview={props.preview} />
    </box>
  );
}

// ├─ Overlay draw ────────────────────────────────────────────────────────────────────────────────┤

const KITTY_PREVIEW = "/home/cullyn/dotfiles/config/xplr/bin/kitty-preview.py";
const KITTY_WAIT_TIMEOUT_MS = 2_000;

let activePreviewToken = 0; // Invalidates unfinished draws when the preview changes.
let kittyQueue = Promise.resolve();
const activeDisplays = new Set<ChildProcess>();

function KittyImageLayer(props: { api: TuiPluginApi; preview: PreviewState }) {
  let drawTimer: ReturnType<typeof setTimeout> | undefined;
  let disposed = false;
  let failed = false;

  const draw = () => {
    const target = terminalPreviewFrame(props.api);
    if (disposed || !target || !canAttemptKittyPreview()) return;

    const token = ++activePreviewToken;
    void queueKitty(async () => {
      if (disposed || token !== activePreviewToken) return;
      stopActiveDisplays();
      await runKittyAndWait(["clear"]);
      if (disposed || token !== activePreviewToken) return;

      const child = runKitty([
        "display",
        untrack(() => props.preview.item.entry.path),
        String(untrack(() => props.preview.imageID)),
        String(target.screenX),
        String(target.screenY),
        String(target.width),
        String(target.height),
      ]);
      if (!child) return;

      activeDisplays.add(child);
      child.once("error", () => activeDisplays.delete(child));
      child.once("close", (code) => {
        activeDisplays.delete(child);
        if (code === 0 || disposed || failed || token !== activePreviewToken) return;
        failed = true;
        untrack(() =>
          props.api.ui.toast({
            variant: "warning",
            title: "Image preview failed",
            message: "Kitty graphics helper could not render this image.",
          }),
        );
      });
    });
  };

  const scheduleDraw = () => {
    if (drawTimer) clearTimeout(drawTimer);
    drawTimer = setTimeout(() => {
      drawTimer = undefined;
      draw();
    }, 40);
  };

  onCleanup(() => {
    disposed = true;
    if (drawTimer) clearTimeout(drawTimer);
    void clearKittyOverlay();
  });

  onMount(scheduleDraw);

  return <box width="100%" height="100%" onSizeChange={scheduleDraw} />;
}

// ├─ Frame and process ───────────────────────────────────────────────────────────────────────────┤

type TerminalRect = {
  screenX: number;
  screenY: number;
  width: number;
  height: number;
};

function terminalPreviewFrame(api: TuiPluginApi): TerminalRect | undefined {
  const columns = Math.floor(api.renderer.terminalWidth || api.renderer.width || 0);
  const rows = Math.floor(api.renderer.terminalHeight || api.renderer.height || 0);
  if (columns < 20 || rows < 10) return undefined;

  const width = Math.max(1, Math.floor(columns * 0.9));
  const height = Math.max(1, Math.floor(rows * 0.9));
  return {
    screenX: Math.max(0, Math.floor((columns - width) / 2)),
    screenY: Math.max(0, Math.floor((rows - height) / 2)),
    width,
    height,
  };
}

function canAttemptKittyPreview() {
  return Boolean(process.env.KITTY_WINDOW_ID || process.env.TERM?.toLowerCase().includes("kitty"));
}

async function queueKitty<T>(operation: () => Promise<T> | T) {
  const run = kittyQueue.catch(() => {}).then(operation);
  kittyQueue = run.then(
    () => undefined,
    () => undefined,
  );
  return run;
}

async function clearKittyOverlay() {
  activePreviewToken++;
  await queueKitty(async () => {
    stopActiveDisplays();
    if (canAttemptKittyPreview()) await runKittyAndWait(["clear"]);
  });
}

function stopActiveDisplays() {
  for (const child of activeDisplays) {
    if (!child.killed) child.kill();
  }
  activeDisplays.clear();
}

function runKitty(args: string[]) {
  try {
    const child = spawn("python3", [KITTY_PREVIEW, ...args], { stdio: "ignore" });
    child.once("error", () => {});
    return child;
  } catch {
    return undefined;
  }
}

function runKittyAndWait(args: string[]) {
  const child = runKitty(args);
  if (!child) return Promise.resolve(false);
  return new Promise<boolean>((resolve) => {
    let settled = false;
    let timeout: ReturnType<typeof setTimeout>;
    const finish = (ok: boolean) => {
      if (settled) return;
      settled = true;
      clearTimeout(timeout);
      resolve(ok);
    };
    timeout = setTimeout(() => {
      if (!child.killed) child.kill();
      finish(false);
    }, KITTY_WAIT_TIMEOUT_MS);

    child.once("error", () => finish(false));
    child.once("close", (code) => finish(code === 0));
  });
}
