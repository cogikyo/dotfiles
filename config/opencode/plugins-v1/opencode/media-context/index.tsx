/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from "@opencode-ai/plugin/tui";
import type { Message } from "@opencode-ai/sdk/v2";
import { createTextAttributes } from "@opentui/core";
import { spawn, type ChildProcess } from "node:child_process";
import { basename } from "node:path";
import { For, Show, createRenderEffect, createSignal, onCleanup, onMount, untrack } from "solid-js";
import { SidebarSection } from "../../shared/sidebar-section.tsx";
import { isExistingFile } from "./files";
import { ImageOverlay, type MediaItem, type PreviewState } from "./preview";
import { listSessionMedia, mediaPart, mediaReference, registerSessionMedia } from "./registry";

const id = "opencode-media-context";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ TUI plugin: session media sidebar and previews                                                │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

const IMAGE_ID_BASE = 874_000;

const BOLD = createTextAttributes({ bold: true });

const MAX_DISCOVERY_MESSAGES = 100;

const MAX_DISCOVERY_REGISTRATIONS = 20;

const RENAME_POLL_INTERVAL_MS = 1_000;

const RENAME_POLL_LIMIT = 30;

// ├─ Hooks and slots ─────────────────────────────────────────────────────────────────────────────┤

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
    api.event.on("message.updated", (event) => untrack(() => closeMissingPreview(event.properties.sessionID))),
    api.event.on("message.removed", (event) => untrack(() => closeMissingPreview(event.properties.sessionID))),
    api.event.on("message.part.updated", (event) => untrack(() => closeMissingPreview(event.properties.sessionID))),
    api.event.on("message.part.removed", (event) => untrack(() => closeMissingPreview(event.properties.sessionID))),
    api.event.on("session.compacted", (event) => untrack(() => closeMissingPreview(event.properties.sessionID))),
  ];

  api.lifecycle.onDispose(() => {
    for (const dispose of disposers) dispose();
    closePreview();
  });

  api.slots.register({
    order: 450,
    slots: {
      app() {
        return (
          <Show when={preview()} keyed>
            {(current: PreviewState) => <ImageOverlay api={api} preview={current} onClose={closePreview} />}
          </Show>
        );
      },
      sidebar_content(_ctx, props: { session_id: string }) {
        return <MediaContext api={api} sessionID={props.session_id} onOpenImage={openPreview} />;
      },
    },
  });
};

/** TUI plugin that lists session images and videos in the sidebar, with Kitty image previews and external video playback. */
export default { id, tui } satisfies TuiPluginModule & { id: string };

// ├─ Sidebar list ────────────────────────────────────────────────────────────────────────────────┤

function MediaContext(props: { api: TuiPluginApi; sessionID: string; onOpenImage: (preview: PreviewState) => void }) {
  const items = createMediaItems(props);

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
      <SidebarSection api={props.api} title="Media Context" detail={`${items().length} media`}>
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
      </SidebarSection>
    </Show>
  );
}

function createMediaItems(props: { api: TuiPluginApi; sessionID: string }) {
  const [items, setItems] = createSignal<MediaItem[]>([]);
  let refreshTimer: ReturnType<typeof setTimeout> | undefined;
  let renamePollTimer: ReturnType<typeof setTimeout> | undefined;
  let renamePolls = 0;
  let unnamedSignature = "";
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

  onSessionMessages(props, scheduleRefresh);

  createRenderEffect(() => {
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
  });

  return items;
}

function onSessionMessages(props: { api: TuiPluginApi; sessionID: string }, scheduleRefresh: () => void) {
  createRenderEffect(() => {
    const disposers = [
      props.api.event.on(
        "message.updated",
        (event) => event.properties.sessionID === untrack(() => props.sessionID) && untrack(scheduleRefresh),
      ),
      props.api.event.on(
        "message.removed",
        (event) => event.properties.sessionID === untrack(() => props.sessionID) && untrack(scheduleRefresh),
      ),
      props.api.event.on(
        "message.part.updated",
        (event) => event.properties.sessionID === untrack(() => props.sessionID) && untrack(scheduleRefresh),
      ),
      props.api.event.on(
        "message.part.removed",
        (event) => event.properties.sessionID === untrack(() => props.sessionID) && untrack(scheduleRefresh),
      ),
      props.api.event.on(
        "session.compacted",
        (event) => event.properties.sessionID === untrack(() => props.sessionID) && untrack(scheduleRefresh),
      ),
    ];
    onCleanup(() => {
      for (const dispose of disposers) dispose();
    });
  });
}

// ├─ Media discovery ─────────────────────────────────────────────────────────────────────────────┤

function unnamedImageSignature(items: MediaItem[]) {
  return items
    .filter((item) => item.entry.kind === "image" && !item.entry.name)
    .map((item) => item.entry.handle)
    .toSorted()
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
    const messages = api.state.session.messages(sessionID);
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
  let registrations = 0;

  const start = Math.max(0, messages.length - MAX_DISCOVERY_MESSAGES);
  for (let index = messages.length - 1; index >= start; index--) {
    const message = messages[index];
    if (!message) continue;

    try {
      for (const part of api.state.part(message.id)) {
        const media = mediaPart(part);
        if (!media) continue;
        registerSessionMedia(sessionID, message.id, media);
        registrations++;
        if (registrations >= MAX_DISCOVERY_REGISTRATIONS) return;
      }
    } catch {
      continue;
    }
  }
}

function previewStillExists(api: TuiPluginApi, current: PreviewState) {
  return mediaItems(api, current.sessionID).some(
    (item) =>
      item.entry.kind === "image" && item.entry.handle === current.item.entry.handle && isExistingFile(item.entry.path),
  );
}

// ├─ External viewers ────────────────────────────────────────────────────────────────────────────┤

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
