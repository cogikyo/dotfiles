/** @jsxImportSource @opentui/solid */
import type {
  TuiPlugin,
  TuiPluginApi,
  TuiPluginModule,
  TuiPromptRef,
} from "@opencode-ai/plugin/tui";
import { writeFile } from "node:fs/promises";
import { Show, createSignal, onCleanup } from "solid-js";
import { colors, pressureColor, pressureTier } from "../shared/colors.ts";
import { gitDirtyCount, gitStatus, type GitStatus } from "../shared/git.ts";
import { icons } from "../shared/icons.ts";
import {
  sessionContextUsage,
  sessionMeta,
  shortDir,
  type SessionUsage,
} from "../shared/session.ts";

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

function SessionPrompt(props: SessionPromptProps) {
  const Prompt = props.api.ui.Prompt;
  return (
    <Prompt
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

  const refresh = () => {
    const id = ++refreshID;
    setRevision((value) => value + 1);
    const meta = sessionMeta(props.api, props.sessionID);
    void resolveGitStatus(props.api, props.sessionID, meta.cwd).then(
      (status) => {
        if (id !== refreshID) return;
        if (status) setGit(status);
        else setGit((current) => current ?? fallbackGitStatus(props.api));
      },
    );
  };

  refresh();
  const timer = setInterval(refresh, REFRESH_MS);
  const disposers = [
    props.api.event.on("message.updated", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("message.removed", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("session.status", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("session.updated", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("vcs.branch.updated", refresh),
  ];
  onCleanup(() => {
    clearInterval(timer);
    for (const dispose of disposers) dispose();
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
      <GitSegment
        status={git() ?? fallbackGitStatus(props.api)}
        api={props.api}
      />
    </box>
  );
}

function StatusRight(props: { api: TuiPluginApi; sessionID: string }) {
  const [usage, setUsage] = createSignal<SessionUsage | undefined>();

  const refresh = () => {
    const next = sessionContextUsage(props.api, props.sessionID);
    if (next.limit && next.tokens > 0) setUsage(next);
  };

  refresh();
  const timer = setInterval(refresh, REFRESH_MS);
  const disposers = [
    props.api.event.on("message.updated", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("message.removed", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("message.part.updated", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("session.status", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("session.updated", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
  ];
  onCleanup(() => {
    clearInterval(timer);
    for (const dispose of disposers) dispose();
  });

  return <ContextSegment api={props.api} usage={usage()} />;
}

function GitSegment(props: { api: TuiPluginApi; status?: GitStatus }) {
  return (
    <Show when={props.status} keyed>
      {(status) =>
        status.branch ? (
          <box flexDirection="row" gap={0}>
            <text fg={gitStateColor(props.api, status)} wrapMode="none">
              {` ${icons.git.branch}${status.branch}`}
            </text>
            <GitStats api={props.api} status={status} />
          </box>
        ) : null
      }
    </Show>
  );
}

function GitStats(props: { api: TuiPluginApi; status: GitStatus }) {
  const c = colors(props.api.theme.current);
  return (
    <>
      <GitCount
        value={props.status.ahead}
        icon={icons.git.ahead}
        fg={c.green}
      />
      <GitCount
        value={props.status.behind}
        icon={icons.git.behind}
        fg={c.brightRed}
      />
      <GitCount
        value={props.status.modified}
        icon={icons.git.modified}
        fg={c.sky}
      />
      <GitCount
        value={props.status.staged}
        icon={icons.git.staged}
        fg={c.yellow}
      />
      <GitCount
        value={props.status.deleted}
        icon={icons.git.deleted}
        fg={c.red}
      />
      <GitCount
        value={props.status.untracked}
        icon={icons.git.untracked}
        fg={c.yellow}
      />
      <GitCount
        value={props.status.stashed}
        icon={icons.git.stashed}
        fg={c.muted}
      />
      <GitCount
        value={props.status.conflicted}
        icon={icons.git.conflict}
        fg={c.pink}
      />
      <GitCount
        value={props.status.renamed}
        icon={icons.git.renamed}
        fg={c.magenta}
      />
    </>
  );
}

function GitCount(props: {
  value: number;
  icon: string;
  fg: ReturnType<typeof colors>[keyof ReturnType<typeof colors>];
}) {
  return (
    <Show when={props.value > 0}>
      <text fg={props.fg} wrapMode="none">
        {` ${props.icon}${props.value}`}
      </text>
    </Show>
  );
}

function ContextSegment(props: { api: TuiPluginApi; usage?: SessionUsage }) {
  return (
    <Show when={props.usage} keyed>
      {(usage) => (
        <box flexDirection="row" gap={0}>
          <text
            fg={pressureColor(props.api.theme.current, usage.colorPercent)}
            wrapMode="none"
          >
            {icons.context}
            {contextBar(usage.colorPercent)}
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

function Sep(props: { api: TuiPluginApi }) {
  return <text fg={colors(props.api.theme.current).muted}>{icons.sep}</text>;
}

function gitStateColor(api: TuiPluginApi, status: GitStatus) {
  const c = colors(api.theme.current);
  if (status.behind > 0 || status.conflicted > 0) return c.brightRed;
  if (status.modified > 0) return c.sky;
  if (status.staged > 0) return c.yellow;
  if (status.deleted > 0) return c.red;
  if (status.untracked > 0) return c.yellow;
  if (status.ahead > 0) return c.green;
  if (status.renamed > 0) return c.magenta;
  return c.blue;
}

function agentColor(api: TuiPluginApi, sessionID: string) {
  const theme = api.theme.current;
  const agent = currentAgent(api, sessionID);
  const colorName = agent ? api.state.config.agent?.[agent]?.color : undefined;
  if (typeof colorName === "string" && !colorName.startsWith("#")) {
    const color = theme[colorName as keyof typeof theme];
    if (typeof color === "object" && color) return color as typeof theme.text;
  }
  return colors(theme).brightBlue;
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
    complete: false,
  };
}

async function resolveGitStatus(
  api: TuiPluginApi,
  sessionID: string,
  dir: string,
): Promise<GitStatus | undefined> {
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

async function gitStatusFromOpenCode(
  api: TuiPluginApi,
  dir?: string,
): Promise<GitStatus | undefined> {
  const branch = api.state.vcs?.branch;
  if (!branch) return undefined;

  const result = await api.client.file
    .status(dir ? { directory: dir } : undefined)
    .catch(() => undefined);
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
    complete: false,
  };

  for (const file of result.data) {
    const fileStatus = file.status;
    if (fileStatus === "added") status.untracked++;
    if (fileStatus === "modified") status.modified++;
    if (fileStatus === "deleted") status.deleted++;
  }

  return status;
}

function gitStatusFromSessionDiff(
  api: TuiPluginApi,
  sessionID: string,
): GitStatus | undefined {
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
    complete: false,
  };
}

function hasGitCounters(status?: GitStatus) {
  return !!status && gitDirtyCount(status) > 0;
}

async function traceGitStatus(details: unknown) {
  if (!TRACE_GIT_STATUS) return;
  await writeFile(
    "/tmp/opencode-statusline-git.json",
    `${JSON.stringify(details, null, 2)}\n`,
  ).catch(() => undefined);
}

const tui: TuiPlugin = async (api) => {
  api.slots.register({
    order: 100,
    slots: {
      // This wraps the core prompt; lifecycle props/ref must pass through unchanged or input focus/submission breaks.
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

export default plugin;
