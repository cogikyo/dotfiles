/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from "@opencode-ai/plugin/tui";
import { spawn, type ChildProcess } from "node:child_process";
import { For, Show, createSignal } from "solid-js";
import { SidebarSection } from "../shared/sidebar-section.tsx";

const id = "hyprd-browser-qa";
const MAX_TITLE_LENGTH = 30;
const RESTART_DELAY_MS = 5_000;

type BrowserQA = {
  address: string;
  title: string;
  slot: number;
  workspace: string;
};

type WorkspaceEvent = {
  event?: unknown;
  data?: unknown;
};

function BrowserQASection(props: {
  api: TuiPluginApi;
  entries: BrowserQA[];
  onToggle: (slot: number) => void;
}) {
  return (
    <Show when={props.entries.length > 0}>
      <SidebarSection
        api={props.api}
        title="Browsers"
        detail={`${props.entries.length} ${props.entries.length === 1 ? "window" : "windows"}`}
      >
        <For each={props.entries}>
          {(entry) => (
            <box
              flexDirection="row"
              gap={0}
              onMouseDown={() => props.onToggle(entry.slot)}
            >
              <text fg={props.api.theme.current.primary} wrapMode="none">
                {`#${entry.slot} `}
              </text>
              <text fg={props.api.theme.current.textMuted} wrapMode="none">
                {truncateTitle(entry.title)}
              </text>
            </box>
          )}
        </For>
      </SidebarSection>
    </Show>
  );
}

function toggleWorkspace(
  api: TuiPluginApi,
  slot: number,
  active: () => boolean,
) {
  let child: ReturnType<typeof spawn>;
  try {
    child = spawn("hyprd", ["browser-qa", String(slot)], {
      stdio: ["ignore", "pipe", "pipe"],
    });
  } catch (cause) {
    if (active()) {
      api.ui.toast({
        variant: "error",
        title: "Browser QA toggle failed",
        message: cause instanceof Error ? cause.message : "hyprd could not start",
      });
    }
    return undefined;
  }
  let error = "";
  let spawned = false;
  let reported = false;
  const stdout = child.stdout;
  const stderr = child.stderr;
  if (!stdout || !stderr) {
    child.kill();
    return undefined;
  }

  child.once("spawn", () => {
    spawned = true;
  });
  stdout.setEncoding("utf8");
  stderr.setEncoding("utf8");
  const collect = (chunk: string) => {
    error = `${error}${chunk}`.slice(-1_024);
  };
  stdout.on("data", collect);
  stderr.on("data", collect);
  child.once("error", (cause) => {
    if (!active()) return;
    reported = true;
    api.ui.toast({
      variant: "error",
      title: "Browser QA toggle failed",
      message: cause.message,
    });
  });
  child.once("close", (code) => {
    if (!active() || reported || !spawned || code === 0) return;
    api.ui.toast({
      variant: "error",
      title: "Browser QA toggle failed",
      message: error.trim() || `hyprd exited ${code ?? "without a status"}`,
    });
  });
  return child;
}

function truncateTitle(title: string) {
  const value = title.trim().replace(/\s+/g, " ") || "Untitled";
  if (value.length <= MAX_TITLE_LENGTH) return value;
  return `${value.slice(0, MAX_TITLE_LENGTH - 3)}...`;
}

function browserQA(input: unknown): BrowserQA[] {
  if (!Array.isArray(input)) return [];

  return input
    .filter((entry): entry is BrowserQA => {
      if (!entry || typeof entry !== "object") return false;
      const candidate = entry as Record<string, unknown>;
      return (
        typeof candidate.address === "string" &&
        typeof candidate.title === "string" &&
        typeof candidate.slot === "number" &&
        Number.isSafeInteger(candidate.slot) &&
        candidate.slot > 0 &&
        candidate.workspace === `browser-qa-${candidate.slot}`
      );
    })
    .sort((left, right) => left.slot - right.slot);
}

function workspaceEvent(line: string) {
  try {
    const event = JSON.parse(line) as WorkspaceEvent;
    if (event.event !== "workspace" || !event.data || typeof event.data !== "object") {
      return undefined;
    }
    return browserQA((event.data as Record<string, unknown>).browser_qa);
  } catch {
    return undefined;
  }
}

const tui: TuiPlugin = async (api) => {
  const [entries, setEntries] = createSignal<BrowserQA[]>([]);
  const actions = new Set<ChildProcess>();
  let child: ChildProcess | undefined;
  let restartTimer: ReturnType<typeof setTimeout> | undefined;
  let disposed = false;

  const watch = () => {
    if (disposed) return;
    let watcher: ReturnType<typeof spawn>;
    try {
      watcher = spawn("hyprd", ["subscribe", "workspace"], {
        stdio: ["ignore", "pipe", "ignore"],
      });
    } catch {
      restartTimer = setTimeout(watch, RESTART_DELAY_MS);
      return;
    }
    child = watcher;
    let buffer = "";
    const stdout = watcher.stdout;
    if (!stdout) {
      watcher.kill();
      return;
    }

    stdout.setEncoding("utf8");
    stdout.on("data", (chunk: string) => {
      buffer += chunk;
      const lines = buffer.split("\n");
      buffer = lines.pop() ?? "";
      for (const line of lines) {
        const next = workspaceEvent(line);
        if (!next) continue;
        setEntries(next);
        api.renderer.requestRender();
      }
    });
    watcher.once("error", () => undefined);
    watcher.once("close", () => {
      if (child === watcher) child = undefined;
      if (disposed) return;
      restartTimer = setTimeout(watch, RESTART_DELAY_MS);
    });
  };

  const toggle = (slot: number) => {
    const action = toggleWorkspace(api, slot, () => !disposed);
    if (!action) return;
    actions.add(action);
    action.once("close", () => actions.delete(action));
  };

  watch();
  api.lifecycle.onDispose(() => {
    disposed = true;
    if (restartTimer) clearTimeout(restartTimer);
    if (child && !child.killed) child.kill();
    for (const action of actions) {
      if (!action.killed) action.kill();
    }
    actions.clear();
  });

  api.slots.register({
    order: 150,
    slots: {
      sidebar_content() {
        return (
          <BrowserQASection api={api} entries={entries()} onToggle={toggle} />
        );
      },
    },
  });
};

export default { id, tui } satisfies TuiPluginModule & { id: string };
