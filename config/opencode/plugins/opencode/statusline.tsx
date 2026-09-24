/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule, TuiPromptRef } from "@opencode-ai/plugin/tui";
import { writeFile } from "node:fs/promises";
import {
  Show,
  createComputed,
  createRenderEffect,
  createSignal,
  on,
  onCleanup,
  untrack,
  type Accessor,
} from "solid-js";
import { colors, pressureColor, pressureTier } from "../shared/colors.ts";
import { gitDirtyCount, gitStatus, type GitStatus } from "../shared/git.ts";
import { icons } from "../shared/icons.ts";
import { sessionContextUsage, sessionMeta, shortDir, type SessionUsage } from "../shared/session.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ TUI plugin: cwd, git, and context on the prompt                                               │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

const id = "opencode-statusline";
const REFRESH_MS = 2_000;
const TRACE_GIT_STATUS = process.env.OPENCODE_STATUSLINE_TRACE_GIT === "1";

type SessionPromptProps = {
  api: TuiPluginApi;
  sessionID: string;
  visible?: boolean;
  disabled?: boolean;
  onSubmit?: () => void;
  promptRef?: (ref: TuiPromptRef | undefined) => void;
};

const tui: TuiPlugin = async (api) => {
  api.slots.register({
    order: 100,
    slots: {
      // The core prompt needs these lifecycle props and ref for input focus and submission.
      session_prompt(
        _ctx,
        props: {
          session_id: string;
          visible?: boolean;
          disabled?: boolean;
          on_submit?: () => void;
          ref?: (ref: TuiPromptRef | undefined) => void;
        },
      ) {
        return (
          <SessionPrompt
            api={api}
            sessionID={props.session_id}
            visible={props.visible}
            disabled={props.disabled}
            onSubmit={props.on_submit}
            promptRef={props.ref}
          />
        );
      },
    },
  });
};

const plugin: TuiPluginModule & { id: string } = {
  id,
  tui,
};

/** TUI plugin that shows the session directory, Git status, and context pressure beside the prompt. */
export default plugin;

// ├─ Prompt display ──────────────────────────────────────────────────────────────────────────────┤

function SessionPrompt(props: SessionPromptProps) {
  return (
    <props.api.ui.Prompt
      sessionID={props.sessionID}
      visible={props.visible}
      disabled={props.disabled}
      onSubmit={props.onSubmit}
      ref={props.promptRef}
      hint={<StatusLeft api={props.api} sessionID={props.sessionID} />}
      right={<StatusRight api={props.api} sessionID={props.sessionID} />}
    />
  );
}

function StatusLeft(props: { api: TuiPluginApi; sessionID: string }) {
  const [revision, setRevision] = createSignal(0);
  const [git, setGit] = createSignal<GitStatus | undefined>();
  let refreshID = 0;

  const syncGit = async (seq: number) => {
    const meta = sessionMeta(props.api, props.sessionID);
    const status = await resolveGitStatus(props.api, props.sessionID, meta.cwd);
    if (seq !== refreshID) return;
    if (status) setGit(status);
    else setGit((current) => current ?? fallbackGitStatus(props.api));
  };

  const refresh = () => {
    // Ignore older git requests when a later timer or session event has started another refresh.
    const seq = ++refreshID;
    setRevision((value) => value + 1);
    void syncGit(seq);
  };

  createComputed(on(() => props.sessionID, refresh));
  const timer = setInterval(refresh, REFRESH_MS);
  createRenderEffect(() => {
    const disposers = [
      props.api.event.on("message.updated", (event) => {
        if (event.properties.sessionID === untrack(() => props.sessionID)) untrack(refresh);
      }),
      props.api.event.on("message.removed", (event) => {
        if (event.properties.sessionID === untrack(() => props.sessionID)) untrack(refresh);
      }),
      props.api.event.on("session.status", (event) => {
        if (event.properties.sessionID === untrack(() => props.sessionID)) untrack(refresh);
      }),
      props.api.event.on("session.updated", (event) => {
        if (event.properties.sessionID === untrack(() => props.sessionID)) untrack(refresh);
      }),
      props.api.event.on("vcs.branch.updated", () => untrack(refresh)),
    ];
    onCleanup(() => {
      for (const dispose of disposers) dispose();
    });
  });
  onCleanup(() => {
    clearInterval(timer);
  });

  const meta = () => {
    revision();
    return sessionMeta(props.api, props.sessionID);
  };
  const repoColor = () => {
    revision();
    return agentColor(props.api, props.sessionID);
  };

  return (
    <box flexDirection="row" gap={0}>
      <text fg={repoColor()} wrapMode="none">
        <b>{shortDir(meta().cwd)}</b>
      </text>
      <GitSegment status={git() ?? fallbackGitStatus(props.api)} />
    </box>
  );
}

function StatusRight(props: { api: TuiPluginApi; sessionID: string }) {
  const [usage, setUsage] = createSignal<SessionUsage | undefined>();

  const refresh = () => {
    const next = sessionContextUsage(props.api, props.sessionID);
    if (next.limit && next.tokens > 0) setUsage(next);
  };

  createComputed(
    on(
      () => props.sessionID,
      () => {
        setUsage(undefined);
        refresh();
      },
    ),
  );
  const timer = setInterval(refresh, REFRESH_MS);
  createRenderEffect(() => {
    const disposers = [
      props.api.event.on("message.updated", (event) => {
        if (event.properties.sessionID === untrack(() => props.sessionID)) untrack(refresh);
      }),
      props.api.event.on("message.removed", (event) => {
        if (event.properties.sessionID === untrack(() => props.sessionID)) untrack(refresh);
      }),
      props.api.event.on("message.part.updated", (event) => {
        if (event.properties.sessionID === untrack(() => props.sessionID)) untrack(refresh);
      }),
      props.api.event.on("session.status", (event) => {
        if (event.properties.sessionID === untrack(() => props.sessionID)) untrack(refresh);
      }),
      props.api.event.on("session.updated", (event) => {
        if (event.properties.sessionID === untrack(() => props.sessionID)) untrack(refresh);
      }),
    ];
    onCleanup(() => {
      for (const dispose of disposers) dispose();
    });
  });
  onCleanup(() => {
    clearInterval(timer);
  });

  return <ContextSegment usage={usage()} />;
}

function GitSegment(props: { status?: GitStatus }) {
  return (
    <Show when={props.status} keyed>
      {(status: GitStatus) =>
        status.branch ? (
          <box flexDirection="row" gap={0}>
            <text fg={gitStateColor(status)} wrapMode="none">
              {` ${icons.git.branch}${status.branch}`}
            </text>
            <GitStats status={status} />
          </box>
        ) : null
      }
    </Show>
  );
}

function GitStats(props: { status: GitStatus }) {
  return (
    <>
      <GitCount value={props.status.ahead} icon={icons.git.ahead} fg={colors.green} />
      <GitCount value={props.status.behind} icon={icons.git.behind} fg={colors.brightRed} />
      <GitCount value={props.status.modified} icon={icons.git.modified} fg={colors.sky} />
      <GitCount value={props.status.staged} icon={icons.git.staged} fg={colors.yellow} />
      <GitCount value={props.status.deleted} icon={icons.git.deleted} fg={colors.red} />
      <GitCount value={props.status.untracked} icon={icons.git.untracked} fg={colors.yellow} />
      <GitCount value={props.status.stashed} icon={icons.git.stashed} fg={colors.muted} />
      <GitCount value={props.status.conflicted} icon={icons.git.conflict} fg={colors.pink} />
      <GitCount value={props.status.renamed} icon={icons.git.renamed} fg={colors.magenta} />
    </>
  );
}

function GitCount(props: { value: number; icon: string; fg: (typeof colors)[keyof typeof colors] }) {
  return (
    <Show when={props.value > 0}>
      <text fg={props.fg} wrapMode="none">
        {` ${props.icon}${props.value}`}
      </text>
    </Show>
  );
}

function ContextSegment(props: { usage?: SessionUsage }) {
  return (
    <Show when={props.usage}>
      {(usage: Accessor<SessionUsage>) => (
        <box flexDirection="row" gap={0}>
          <text fg={pressureColor(usage().colorPercent)} wrapMode="none">
            {icons.context}
            {contextBar(usage().percent)}
          </text>
        </box>
      )}
    </Show>
  );
}

function contextBar(percent: number) {
  const filled = pressureTier(percent) + 1;
  return icons.barFilled.repeat(filled) + icons.barEmpty.repeat(9 - filled);
}

function gitStateColor(status: GitStatus) {
  if (status.behind > 0 || status.conflicted > 0) return colors.brightRed;
  if (status.modified > 0) return colors.sky;
  if (status.staged > 0) return colors.yellow;
  if (status.deleted > 0) return colors.red;
  if (status.untracked > 0) return colors.yellow;
  if (status.ahead > 0) return colors.green;
  if (status.renamed > 0) return colors.magenta;
  return colors.blue;
}

function agentColor(api: TuiPluginApi, sessionID: string) {
  const theme = api.theme.current;
  const agent = currentAgent(api, sessionID);
  const colorName = agent ? api.state.config.agent?.[agent]?.color : undefined;
  if (typeof colorName === "string" && !colorName.startsWith("#")) {
    const color = Object.entries(theme).find(([name]) => name === colorName)?.[1];
    if (typeof color === "object" && color) return color;
  }
  return colors.brightBlue;
}

function currentAgent(api: TuiPluginApi, sessionID: string) {
  const messages = api.state.session.messages(sessionID);
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if ("agent" in message && message.agent) return message.agent;
  }
  return undefined;
}

function fallbackGitStatus(api: TuiPluginApi): GitStatus | undefined {
  const branch = api.state.vcs?.branch;
  if (!branch) return undefined;

  return {
    branch,
    ahead: 0,
    behind: 0,
    staged: 0,
    modified: 0,
    untracked: 0,
    deleted: 0,
    stashed: 0,
    renamed: 0,
    conflicted: 0,
  };
}

// ├─ Git status ──────────────────────────────────────────────────────────────────────────────────┤

async function resolveGitStatus(api: TuiPluginApi, sessionID: string, dir: string): Promise<GitStatus | undefined> {
  const sessionStatus = gitStatusFromSessionDiff(api, sessionID);
  const directStatus = await gitStatus(dir);
  if (directStatus) {
    void traceGitStatus({ dir, source: "direct", status: directStatus });
    return directStatus;
  }

  const scopedStatus = await gitStatusFromOpenCode(api, dir);
  if (hasGitCounters(scopedStatus)) {
    void traceGitStatus({
      dir,
      source: "opencode-scoped",
      status: scopedStatus,
    });
    return scopedStatus;
  }

  const openCodeStatus = await gitStatusFromOpenCode(api);
  if (hasGitCounters(openCodeStatus)) {
    void traceGitStatus({ dir, source: "opencode", status: openCodeStatus });
    return openCodeStatus;
  }

  const status = sessionStatus ?? scopedStatus ?? openCodeStatus;
  void traceGitStatus({
    dir,
    source: "fallback",
    status,
    sessionStatus,
    scopedStatus,
    openCodeStatus,
  });
  return status;
}

async function gitStatusFromOpenCode(api: TuiPluginApi, dir?: string): Promise<GitStatus | undefined> {
  const branch = api.state.vcs?.branch;
  if (!branch) return undefined;

  const result = await api.client.file.status(dir ? { directory: dir } : undefined).catch(() => undefined);
  if (!result || result.error || !Array.isArray(result.data)) return undefined;

  const status: GitStatus = {
    branch,
    ahead: 0,
    behind: 0,
    staged: 0,
    modified: 0,
    untracked: 0,
    deleted: 0,
    stashed: 0,
    renamed: 0,
    conflicted: 0,
  };

  for (const file of result.data) {
    const fileStatus = file.status;
    if (fileStatus === "added") status.untracked++;
    if (fileStatus === "modified") status.modified++;
    if (fileStatus === "deleted") status.deleted++;
  }

  return status;
}

function gitStatusFromSessionDiff(api: TuiPluginApi, sessionID: string): GitStatus | undefined {
  const branch = api.state.vcs?.branch;
  if (!branch) return undefined;

  const files = api.state.session.diff(sessionID);
  if (files.length === 0) return undefined;

  return {
    branch,
    ahead: 0,
    behind: 0,
    staged: 0,
    modified: files.length,
    untracked: 0,
    deleted: 0,
    stashed: 0,
    renamed: 0,
    conflicted: 0,
  };
}

function hasGitCounters(status?: GitStatus) {
  return !!status && gitDirtyCount(status) > 0;
}

async function traceGitStatus(details: unknown) {
  if (!TRACE_GIT_STATUS) return;
  await writeFile("/tmp/opencode-statusline-git.json", `${JSON.stringify(details, null, 2)}\n`).catch(() => undefined);
}
