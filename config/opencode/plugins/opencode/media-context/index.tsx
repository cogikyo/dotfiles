/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from "@opencode-ai/plugin/tui";
import type { Message } from "@opencode-ai/sdk/v2";
import { createTextAttributes } from "@opentui/core";
import { spawn, type ChildProcess } from "node:child_process";
import { basename } from "node:path";
import { For, Show, createEffect, createSignal, onCleanup, onMount, untrack } from "solid-js";
import {
  isExistingFile,
  listSessionMedia,
  mediaPart,
  mediaReference,
  registerSessionMedia,
  type MediaRegistryEntry,
} from "./registry";

const id = "opencode-media-context";
const KITTY_PREVIEW = "/home/cullyn/dotfiles/config/xplr/bin/kitty-preview.py";
const IMAGE_ID_BASE = 874_000;
const BOLD = createTextAttributes({ bold: true });
const KITTY_WAIT_TIMEOUT_MS = 2_000;
const MAX_DISCOVERY_MESSAGES = 100;
const MAX_DISCOVERY_REGISTRATIONS = 20;
const RENAME_POLL_INTERVAL_MS = 1_000;
const RENAME_POLL_LIMIT = 30;
let activePreviewToken = 0;
let kittyQueue = Promise.resolve();
const activeDisplays = new Set<ChildProcess>();
type MediaItem = {
  entry: MediaRegistryEntry;
};

type PreviewState = {
  sessionID: string;
  item: MediaItem;
  imageID: number;
};

type TerminalRect = {
  screenX: number;
  screenY: number;
  width: number;
  height: number;
};

function MediaContext(props: {
  api: TuiPluginApi;
  sessionID: string;
  onOpenImage: (preview: PreviewState) => void;
}) {
  const [items, setItems] = createSignal<MediaItem[]>(mediaItems(props.api, props.sessionID));
  const [expanded, setExpanded] = createSignal(true);
  let refreshTimer: ReturnType<typeof setTimeout> | undefined;
  let renamePollTimer: ReturnType<typeof setTimeout> | undefined;
  let renamePolls = 0;
  let unnamedSignature = unnamedImageSignature(items());
  const resetRenamePoll = () => {
    if (renamePollTimer) clearTimeout(renamePollTimer);
    renamePollTimer = undefined;
    renamePolls = 0;
    unnamedSignature = "";
  };
  const refreshForSession = (sessionID: string) => {
    try {
      setItems(mediaItems(props.api, sessionID));
    } catch {
      setItems([]);
    }
    scheduleRenamePoll();
  };
  const refresh = () => refreshForSession(props.sessionID);
  const scheduleRefresh = () => {
    if (refreshTimer) clearTimeout(refreshTimer);
    refreshTimer = setTimeout(() => {
      refreshTimer = undefined;
      refresh();
      props.api.renderer.requestRender();
    }, 50);
  };
  const scheduleRenamePoll = () => {
    const nextUnnamedSignature = unnamedImageSignature(items());
    if (nextUnnamedSignature !== unnamedSignature) {
      unnamedSignature = nextUnnamedSignature;
      renamePolls = 0;
    }
    if (!nextUnnamedSignature) return;
    if (renamePollTimer || renamePolls >= RENAME_POLL_LIMIT) return;
    renamePollTimer = setTimeout(() => {
      renamePollTimer = undefined;
      renamePolls++;
      refresh();
      props.api.renderer.requestRender();
    }, RENAME_POLL_INTERVAL_MS);
  };

  const disposers = [
    props.api.event.on("message.updated", (event) => event.properties.sessionID === props.sessionID && scheduleRefresh()),
    props.api.event.on("message.removed", (event) => event.properties.sessionID === props.sessionID && scheduleRefresh()),
    props.api.event.on("message.part.updated", (event) => event.properties.sessionID === props.sessionID && scheduleRefresh()),
    props.api.event.on("message.part.removed", (event) => event.properties.sessionID === props.sessionID && scheduleRefresh()),
    props.api.event.on("session.compacted", (event) => event.properties.sessionID === props.sessionID && scheduleRefresh()),
  ];

  createEffect(() => {
    const sessionID = props.sessionID;
    untrack(() => {
      resetRenamePoll();
      refreshForSession(sessionID);
      props.api.renderer.requestRender();
    });
  });

  onMount(scheduleRefresh);

  onCleanup(() => {
    if (refreshTimer) clearTimeout(refreshTimer);
    if (renamePollTimer) clearTimeout(renamePollTimer);
    for (const dispose of disposers) dispose();
  });

  const openItem = (item: MediaItem, index: number) => {
    if (!isExistingFile(item.entry.path)) {
      props.api.ui.toast({
        variant: "warning",
        title: `${item.entry.kind === "video" ? "Video" : "Image"} file missing`,
        message: item.entry.path,
      });
      return;
    }

    if (item.entry.kind === "video") {
      openVideo(props.api, item.entry.path);
      return;
    }

    props.onOpenImage({ sessionID: props.sessionID, item, imageID: IMAGE_ID_BASE + index + 1 });
  };

  return (
    <Show when={items().length > 0}>
      <box flexDirection="column" gap={0}>
        <box flexDirection="row" gap={0} onMouseDown={() => setExpanded((value) => !value)}>
          <text fg={props.api.theme.current.text} wrapMode="none">
            {expanded() ? "▼ " : "▶ "}
          </text>
          <text fg={props.api.theme.current.text} attributes={BOLD}>
            Media Context
          </text>
          <text fg={props.api.theme.current.textMuted}>{` ${items().length} media`}</text>
        </box>
        <Show when={expanded()}>
          <For each={items()}>
            {(item, index) => (
              <box flexDirection="row" gap={0} onMouseDown={() => openItem(item, index())}>
                <text fg={mediaItemColor(props.api, item)} attributes={BOLD} wrapMode="none">
                  {item.entry.kind === "video" ? "V " : "I "}
                </text>
                <text fg={mediaItemColor(props.api, item)} wrapMode="none">
                  {mediaItemLabel(item)}
                </text>
              </box>
            )}
          </For>
        </Show>
      </box>
    </Show>
  );
}

function ImageOverlay(props: { api: TuiPluginApi; preview: PreviewState; onClose: () => void }) {
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
      onMouseDown={props.onClose}
      onKeyDown={(event) => {
        if (event.name === "escape") props.onClose();
      }}
      onSizeChange={() => props.api.renderer.requestRender()}
    >
      <KittyImageLayer api={props.api} preview={props.preview} />
    </box>
  );
}

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
        props.preview.item.entry.path,
        String(props.preview.imageID),
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
        props.api.ui.toast({
          variant: "warning",
          title: "Image preview failed",
          message: "Kitty graphics helper could not render this image.",
        });
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

  return (
    <box width="100%" height="100%" onSizeChange={scheduleDraw} />
  );
}

function unnamedImageSignature(items: MediaItem[]) {
  return items
    .filter((item) => item.entry.kind === "image" && !item.entry.name)
    .map((item) => item.entry.handle)
    .sort()
    .join("\0");
}

function mediaItemColor(api: TuiPluginApi, item: MediaItem) {
  return item.entry.kind === "video" ? api.theme.current.accent : api.theme.current.secondary;
}

function mediaItemLabel(item: MediaItem) {
  if (item.entry.kind !== "video") return mediaReference(item.entry);
  return basename(item.entry.path) || item.entry.handle;
}

function mediaItems(api: TuiPluginApi, sessionID: string): MediaItem[] {
  try {
    const messages = api.state.session.messages(sessionID) as ReadonlyArray<Message>;
    if (messages.length === 0) return [];

    discoverCurrentSessionMedia(api, sessionID, messages);

    const messageIDs = new Set(messages.map((message) => message.id));
    return listSessionMedia(sessionID)
      .filter((entry) => entry.messageID && messageIDs.has(entry.messageID) && isExistingFile(entry.path))
      .map((entry) => ({ entry }));
  } catch {
    return [];
  }
}

function discoverCurrentSessionMedia(api: TuiPluginApi, sessionID: string, messages: ReadonlyArray<Message>) {
  try {
    let registrations = 0;

    const start = Math.max(0, messages.length - MAX_DISCOVERY_MESSAGES);
    for (let index = messages.length - 1; index >= start; index--) {
      const message = messages[index];
      if (!message) continue;

      try {
        for (const part of api.state.part(message.id)) {
          const media = mediaPart(part);
          if (media) {
            registerSessionMedia(sessionID, message.id, media);
            registrations++;
            if (registrations >= MAX_DISCOVERY_REGISTRATIONS) return;
          }
        }
      } catch {
        continue;
      }
    }
  } catch {
    return;
  }
}

function previewStillExists(api: TuiPluginApi, current: PreviewState) {
  return mediaItems(api, current.sessionID).some(
    (item) => item.entry.kind === "image" && item.entry.handle === current.item.entry.handle && isExistingFile(item.entry.path),
  );
}

function openVideo(api: TuiPluginApi, path: string) {
  let child: ChildProcess;
  try {
    child = spawn("xdg-open", [path], { detached: true, stdio: "ignore" });
  } catch {
    api.ui.toast({
      variant: "warning",
      title: "Video open failed",
      message: path,
    });
    return;
  }

  child.once("error", () => {
    api.ui.toast({
      variant: "warning",
      title: "Video open failed",
      message: path,
    });
  });
  child.once("close", (code) => {
    if (code === 0) return;
    api.ui.toast({
      variant: "warning",
      title: "Video open failed",
      message: `xdg-open exited ${code ?? "without a status"}: ${path}`,
    });
  });
  child.unref();
}

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

const tui: TuiPlugin = async (api) => {
  const [preview, setPreview] = createSignal<PreviewState>();
  const closePreview = () => {
    setPreview(undefined);
    api.renderer.requestRender();
  };
  const openPreview = (next: PreviewState) => {
    setPreview(next);
    api.renderer.requestRender();
  };
  const closeMissingPreview = (sessionID: string) => {
    const current = preview();
    if (current?.sessionID === sessionID && !previewStillExists(api, current)) closePreview();
  };

  const disposers = [
    api.event.on("tui.session.select", closePreview),
    api.event.on("tui.command.execute", (event) => {
      if (event.properties.command.startsWith("session.")) closePreview();
    }),
    api.event.on("message.updated", (event) => closeMissingPreview(event.properties.sessionID)),
    api.event.on("message.removed", (event) => closeMissingPreview(event.properties.sessionID)),
    api.event.on("message.part.updated", (event) => closeMissingPreview(event.properties.sessionID)),
    api.event.on("message.part.removed", (event) => closeMissingPreview(event.properties.sessionID)),
    api.event.on("session.compacted", (event) => closeMissingPreview(event.properties.sessionID)),
  ];

  api.lifecycle.onDispose(() => {
    for (const dispose of disposers) dispose();
    closePreview();
  });

  api.slots.register({
    order: 125,
    slots: {
      app() {
        return (
          <Show when={preview()} keyed>
            {(current) => <ImageOverlay api={api} preview={current} onClose={closePreview} />}
          </Show>
        );
      },
      sidebar_content(_ctx, props: { session_id: string }) {
        return <MediaContext api={api} sessionID={props.session_id} onOpenImage={openPreview} />;
      },
    },
  });
};

export default { id, tui } satisfies TuiPluginModule & { id: string };
