/** @jsxImportSource @opentui/solid */
import type { Message } from "@opencode-ai/sdk/v2";
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from "@opencode-ai/plugin/tui";
import { For, Show, createMemo, createSignal, onCleanup } from "solid-js";
import { colors } from "../../shared/colors.ts";
import { formatTokens, sessionMessages, sessionMeta } from "../../shared/session.ts";
import { SidebarSection } from "../../shared/sidebar-section.tsx";
import { icons } from "../../shared/icons.ts";
import { assessPressure, type PressureThresholds } from "./pressure.ts";
import { readSettings } from "./settings.ts";
import {
  emptyArtifactHealth,
  ledgerPath,
  projectKey,
  readActiveLocks,
  readLedger,
  readProjectLedgers,
  renderCheckpointSummary,
  renderRenewalPrompt,
  resolveSpecFile,
  upsertLedger,
  withLedgerLock,
  type ActiveLock,
  type ArtifactHealth,
  type ContinuityLedger,
  type DirtyCoverage,
  type LedgerSeed,
  type PressureSnapshot,
} from "./state.ts";

const id = "opencode-continuity-ui";
const REFRESH_MS = 10_000;

type ContinuityLevel = "healthy" | "checkpoint" | "compact" | "renew" | "blocked" | "stale" | "watch";

type RelatedSession = {
  sessionID: string;
  title?: string;
  agent: string;
  sharedSpecFiles: string[];
  updatedAt: number;
  level: ContinuityLevel;
};

type ContinuityState = LedgerSeed & {
  ledger?: ContinuityLedger;
  locks: ActiveLock[];
  related: RelatedSession[];
};

const STALE_SESSION_MS = 24 * 60 * 60_000;
const ui = {
  pressure: icons.context,
  packet: icons.git.staged.trimEnd(),
  wip: icons.git.modified.trimEnd(),
  lock: icons.error,
  renew: icons.effort.auto,
  related: icons.git.branch.trimEnd(),
} as const;

function ContinuityPanel(props: { api: TuiPluginApi; sessionID: string; thresholds: PressureThresholds }) {
  const [revision, setRevision] = createSignal(0);
  const refresh = () => setRevision((value) => value + 1);
  const timer = setInterval(refresh, REFRESH_MS);
  const disposers = [
    props.api.event.on("message.updated", (event) => {
      if (event.properties.info.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("message.part.updated", (event) => {
      if (event.properties.part.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("session.diff", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
    props.api.event.on("session.compacted", (event) => {
      if (event.properties.sessionID === props.sessionID) refresh();
    }),
  ];

  onCleanup(() => {
    clearInterval(timer);
    for (const dispose of disposers) dispose();
  });

  const state = createMemo(() => {
    revision();
    return continuityState(props.api, props.sessionID, props.thresholds);
  });

  return (
    <SidebarSection api={props.api} title="Continuity" detail={<Summary api={props.api} state={state()} />} initiallyExpanded={false}>
      <box flexDirection="column" gap={0}>
        <PressureNotice api={props.api} state={state()} thresholds={props.thresholds} />
        <MissingPacketNotice api={props.api} state={state()} />
        <WipNotice api={props.api} dirty={state().dirty} />
        <LockNotice api={props.api} locks={state().locks} />
        <RenewalNotice api={props.api} ledger={state().ledger} />
        <RelatedSessions api={props.api} sessions={state().related} />
      </box>
    </SidebarSection>
  );
}

function Summary(props: { api: TuiPluginApi; state: ContinuityState }) {
  const c = colors(props.api.theme.current);
  const level = healthLevel(props.state);
  const color = levelColor(c, level);

  return (
    <box flexDirection="row" gap={0}>
      <Chip icon={ui.pressure} value={formatTokens(props.state.pressure.tokens)} color={color} />
      <Show when={props.state.artifact.specFiles.length > 0}>
        <Chip icon={ui.packet} value={String(props.state.artifact.specFiles.length)} color={c.green} />
      </Show>
      <Show when={props.state.artifact.specFiles.length === 0}>
        <Chip icon={ui.packet} value="!" color={c.yellow} />
      </Show>
      <Show when={props.state.dirty.uncovered.length > 0}>
        <Chip icon={ui.wip} value={String(props.state.dirty.uncovered.length)} color={c.yellow} />
      </Show>
      <Show when={props.state.locks.length > 0}>
        <Chip icon={ui.lock} value={String(props.state.locks.length)} color={c.orange} />
      </Show>
      <Show when={renewalActive(props.state.ledger)}>
        <Chip icon={ui.renew} value={renewalChip(props.state.ledger)} color={props.state.ledger?.renewal?.error ? c.red : c.sky} />
      </Show>
      <Show when={props.state.related.length > 0}>
        <Chip icon={ui.related} value={String(props.state.related.length)} color={c.magenta} />
      </Show>
    </box>
  );
}

function Chip(props: { icon: string; value: string; color: ReturnType<typeof colors>[keyof ReturnType<typeof colors>] }) {
  return <text fg={props.color} wrapMode="none">{` ${props.icon}${props.value}`}</text>;
}

function PressureNotice(props: { api: TuiPluginApi; state: ContinuityState; thresholds: PressureThresholds }) {
  return (
    <Show when={props.state.pressure.level === "compact" || props.state.pressure.level === "renew"}>
      <Notice
        api={props.api}
        icon={ui.pressure}
        color={levelColor(colors(props.api.theme.current), props.state.pressure.level)}
        text={pressureText(props.state, props.thresholds)}
      />
    </Show>
  );
}

function MissingPacketNotice(props: { api: TuiPluginApi; state: ContinuityState }) {
  return (
    <Show when={props.state.artifact.status !== "healthy" && props.state.pressure.level !== "low"}>
      <Notice api={props.api} icon={ui.packet} color={colors(props.api.theme.current).yellow} text="spec packet missing; renewal disabled" />
    </Show>
  );
}

function WipNotice(props: { api: TuiPluginApi; dirty: DirtyCoverage }) {
  return (
    <Show when={props.dirty.uncovered.length > 0}>
      <Notice api={props.api} icon={ui.wip} color={colors(props.api.theme.current).yellow} text={`wip elsewhere ${props.dirty.uncovered.length}`} />
    </Show>
  );
}

function LockNotice(props: { api: TuiPluginApi; locks: ActiveLock[] }) {
  return (
    <Show when={props.locks.length > 0}>
      <For each={props.locks.slice(0, 2)}>
        {(lock) => <Notice api={props.api} icon={ui.lock} color={colors(props.api.theme.current).orange} text={`${lock.purpose} held by ${lock.holder}`} />}
      </For>
    </Show>
  );
}

function RenewalNotice(props: { api: TuiPluginApi; ledger?: ContinuityLedger }) {
  const color = () => props.ledger?.renewal?.error ? colors(props.api.theme.current).red : colors(props.api.theme.current).sky;
  return (
    <Show when={renewalActive(props.ledger)}>
      <box flexDirection="row" gap={0} onMouseDown={() => navigateRenewal(props.api, props.ledger)}>
        <text fg={color()} wrapMode="none">{`${ui.renew} `}</text>
        <text fg={colors(props.api.theme.current).text} wrapMode="none">{renewalText(props.ledger)}</text>
      </box>
    </Show>
  );
}

function RelatedSessions(props: { api: TuiPluginApi; sessions: RelatedSession[] }) {
  return (
    <Show when={props.sessions.length > 0}>
      <For each={relatedGroups(props.sessions).slice(0, 2)}>
        {(group) => (
          <box flexDirection="column" gap={0}>
            <Notice api={props.api} icon={ui.packet} color={colors(props.api.theme.current).magenta} text={`${shortSpec(group.specFile)} · ${group.sessions.length}`} />
            <For each={group.sessions.slice(0, 2)}>
              {(session) => <RelatedRow api={props.api} session={session} />}
            </For>
          </box>
        )}
      </For>
    </Show>
  );
}

function RelatedRow(props: { api: TuiPluginApi; session: RelatedSession }) {
  const c = colors(props.api.theme.current);
  const color = levelColor(c, props.session.level);
  return (
    <box flexDirection="row" gap={0} onMouseDown={() => props.api.route.navigate("session", { sessionID: props.session.sessionID })}>
      <text fg={color} wrapMode="none">{`  ${ui.related} `}</text>
      <text fg={color} wrapMode="none">{sessionLabel(props.session)}</text>
      <text fg={c.muted} wrapMode="none">{` ${ageLabel(props.session.updatedAt)}`}</text>
    </box>
  );
}

function Notice(props: { api: TuiPluginApi; icon: string; color: ReturnType<typeof colors>[keyof ReturnType<typeof colors>]; text: string }) {
  return (
    <box flexDirection="row" gap={0}>
      <text fg={props.color} wrapMode="none">{`${props.icon} `}</text>
      <text fg={colors(props.api.theme.current).text} wrapMode="none">{props.text}</text>
    </box>
  );
}

function healthLevel(state: ContinuityState): ContinuityLevel {
  if (state.ledger?.renewal?.error) return "blocked";
  if (state.pressure.level === "renew" && state.artifact.status !== "healthy") return "blocked";
  if (state.pressure.level !== "low") return state.pressure.level;
  if (state.dirty.uncovered.length > 0 || state.artifact.status !== "healthy") return "watch";
  return "healthy";
}

function levelColor(c: ReturnType<typeof colors>, level: ContinuityLevel | PressureSnapshot["level"]) {
  if (level === "healthy") return c.green;
  if (level === "watch") return c.sky;
  if (level === "checkpoint") return c.yellow;
  if (level === "compact") return c.orange;
  if (level === "renew") return c.pink;
  if (level === "blocked") return c.red;
  if (level === "stale") return c.muted;
  return c.green;
}

function pressureText(state: ContinuityState, thresholds: PressureThresholds) {
  const tokens = formatTokens(state.pressure.tokens);
  if (state.pressure.level === "renew") return `renewal pressure ${tokens}; threshold ${formatTokens(thresholds.renewTokens)}`;
  if (state.pressure.level === "compact") return `compact pressure ${tokens}; threshold ${formatTokens(thresholds.compactTokens)}`;
  return `checkpoint pressure ${tokens}; threshold ${formatTokens(thresholds.checkpointTokens)}`;
}

function renewalActive(ledger?: ContinuityLedger) {
  return !!(ledger?.renewal?.targetSessionID || ledger?.renewal?.error || ledger?.renewal?.attemptedAt);
}

function renewalChip(ledger?: ContinuityLedger) {
  if (ledger?.renewal?.error) return "!";
  if (ledger?.renewal?.targetSessionID) return "1";
  return "…";
}

function renewalText(ledger?: ContinuityLedger) {
  if (ledger?.renewal?.error) return `renewal failed ${ledger.renewal.error}`;
  if (ledger?.renewal?.targetSessionID) return `renewed → ${shortID(ledger.renewal.targetSessionID)}`;
  return "renewal pending";
}

function navigateRenewal(api: TuiPluginApi, ledger?: ContinuityLedger) {
  const target = ledger?.renewal?.targetSessionID || ledger?.renewal?.oldSessionID;
  if (target) api.route.navigate("session", { sessionID: target });
}

function relatedGroups(sessions: RelatedSession[]) {
  const groups = new Map<string, RelatedSession[]>();
  for (const session of sessions) {
    for (const specFile of session.sharedSpecFiles) {
      const group = groups.get(specFile) ?? [];
      group.push(session);
      groups.set(specFile, group);
    }
  }
  return Array.from(groups.entries()).map(([specFile, sessions]) => ({ specFile, sessions: sessions.sort((left, right) => right.updatedAt - left.updatedAt) }));
}

function sessionLabel(session: RelatedSession) {
  const title = session.title?.trim();
  const base = title || `${titleCase(session.agent)} ${shortID(session.sessionID)}`;
  return base.length > 34 ? `${base.slice(0, 31)}…` : base;
}

function shortSpec(file: string) {
  const clean = file.replace(/\.md$/u, "");
  const index = clean.indexOf(".spec/");
  if (index >= 0) return clean.slice(index);
  return clean.length > 36 ? `…${clean.slice(-35)}` : clean;
}

function shortID(sessionID: string) {
  return sessionID.slice(0, 8);
}

function ageLabel(updatedAt: number) {
  const age = Math.max(0, Date.now() - updatedAt);
  const minutes = Math.floor(age / 60_000);
  if (minutes < 1) return "now";
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  return `${Math.floor(hours / 24)}d`;
}

function titleCase(value: string) {
  if (!value) return "Session";
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function relatedSessions(project: string, sessionID: string, specFiles: string[]): RelatedSession[] {
  const specs = new Set(specFiles);
  if (specs.size === 0) return [];
  return readProjectLedgers(project)
    .filter((ledger) => ledger.session.id !== sessionID)
    .map((ledger) => ({ ledger, sharedSpecFiles: ledger.artifact.specFiles.filter((file) => specs.has(file)) }))
    .filter((item) => item.sharedSpecFiles.length > 0)
    .map(({ ledger, sharedSpecFiles }) => ({
      sessionID: ledger.session.id,
      title: ledger.session.title,
      agent: ledger.session.agent,
      sharedSpecFiles,
      updatedAt: ledger.updatedAt,
      level: relatedLevel(ledger),
    }))
    .slice(0, 12);
}

function relatedLevel(ledger: ContinuityLedger): ContinuityLevel {
  if (Date.now() - ledger.updatedAt > STALE_SESSION_MS) return "stale";
  if (ledger.renewal?.error) return "blocked";
  if (ledger.pressure.level !== "low") return ledger.pressure.level;
  if (ledger.dirty.uncovered.length > 0 || ledger.artifact.status !== "healthy") return "watch";
  return "healthy";
}

function continuityState(api: TuiPluginApi, sessionID: string, thresholds: PressureThresholds) {
  const projectPath = api.state.path.worktree || api.state.path.directory || process.cwd();
  const key = projectKey(projectPath);
  const ledger = readLedger(key, sessionID);
  const seed = ledgerSeed(api, sessionID, thresholds, ledger);
  return { ledger, locks: readActiveLocks(key), related: relatedSessions(key, sessionID, seed.artifact.specFiles), ...seed };
}

function ledgerSeed(api: TuiPluginApi, sessionID: string, thresholds: PressureThresholds, ledger?: ContinuityLedger): LedgerSeed {
  const projectPath = api.state.path.worktree || api.state.path.directory || process.cwd();
  const messages = sessionMessages(api, sessionID) as ReadonlyArray<Message>;
  const meta = sessionMeta(api, sessionID);
  const pressure = pressureSnapshot(api, sessionID, messages, thresholds);
  const dirty = dirtyCoverage(api, sessionID, ledger);
  const artifact = ledger?.artifact ?? artifactHealth(api, messages);

  return {
    project: { key: projectKey(projectPath), path: projectPath },
    session: { id: sessionID, agent: meta.agent.toLowerCase() || "drive" },
    pressure,
    artifact,
    dirty,
    lastEvent: "tui.render",
  };
}

function pressureSnapshot(api: TuiPluginApi, sessionID: string, messages: ReadonlyArray<Message>, thresholds: PressureThresholds): PressureSnapshot {
  const meta = sessionMeta(api, sessionID);
  const model = api.state.provider.find((provider) => provider.id === meta.providerID)?.models[meta.modelID];
  const reserved = reservedTokens(api);
  return { ...assessPressure({ messages, modelLimit: model?.limit.context, reserved, thresholds }), updatedAt: Date.now() };
}

function reservedTokens(api: TuiPluginApi) {
  const compaction = (api.state.config as Record<string, unknown>).compaction as Record<string, unknown> | undefined;
  const reserved = compaction?.reserved;
  return typeof reserved === "number" && Number.isFinite(reserved) && reserved > 0 ? reserved : undefined;
}

function artifactHealth(api: TuiPluginApi, messages: ReadonlyArray<Message>): ArtifactHealth {
  const files = new Set<string>();
  const projectPath = api.state.path.worktree || api.state.path.directory || process.cwd();
  const cwd = api.state.path.directory || projectPath;
  for (const message of messages) {
    for (const part of api.state.part(message.id)) {
      const source = (part as { source?: { path?: string } }).source;
      const sourceSpec = source?.path ? resolveSpecFile(projectPath, source.path, cwd) : undefined;
      if (sourceSpec) files.add(sourceSpec);
      if (part.type !== "tool") continue;
      const input = part.state.input;
      for (const value of Object.values(input)) {
        const spec = typeof value === "string" ? resolveSpecFile(projectPath, value, cwd) : undefined;
        if (spec) files.add(spec);
      }
    }
  }

  const existing = Array.from(files).sort();
  if (existing.length === 0) return emptyArtifactHealth();
  return { status: "healthy", specFiles: existing, notes: ["durable .spec packet exists"], checkedAt: Date.now() };
}

function dirtyCoverage(api: TuiPluginApi, sessionID: string, ledger?: ContinuityLedger): DirtyCoverage {
  const sessionFiles = api.state.session.diff(sessionID).map((file) => file.file).sort();
  const files = Array.from(new Set([...(ledger?.dirty.files ?? []), ...sessionFiles])).sort();
  const sessionSet = new Set(sessionFiles);
  const uncovered = files.filter((file) => !sessionSet.has(file));
  const percent = files.length === 0 ? 100 : ((files.length - uncovered.length) / files.length) * 100;
  return { files, sessionFiles, uncovered, percent, checkedAt: Date.now() };
}

function writeCheckpoint(api: TuiPluginApi, sessionID: string, thresholds: PressureThresholds, reason: string) {
  const seed = ledgerSeed(api, sessionID, thresholds, readLedger(projectKey(api.state.path.worktree || api.state.path.directory || process.cwd()), sessionID));
  const filePath = withLedgerLock(seed.project.key, sessionID, `tui-checkpoint:${sessionID}`, () => {
    return upsertLedger(seed, (ledger) => {
      ledger.checkpoint = { reason, writtenAt: Date.now(), summary: renderCheckpointSummary(ledger, reason) };
    });
  });
  if (!filePath) throw new Error("continuity checkpoint lock is busy");
  return filePath;
}

async function compactNow(api: TuiPluginApi, sessionID: string, thresholds: PressureThresholds) {
  const filePath = writeCheckpoint(api, sessionID, thresholds, "manual-compact");
  const model = currentModelRef(api, sessionID);
  if (!model) throw new Error("compact needs a current model reference");
  await unwrap(api.client.session.summarize({ sessionID, ...model } as never), "compact session");
  api.ui.toast({ variant: "success", title: "Continuity checkpoint", message: filePath });
}

async function renewNow(api: TuiPluginApi, sessionID: string, thresholds: PressureThresholds) {
  writeCheckpoint(api, sessionID, thresholds, "manual-renew");
  const seed = ledgerSeed(api, sessionID, thresholds, readLedger(projectKey(api.state.path.worktree || api.state.path.directory || process.cwd()), sessionID));
  const ledger = readLedger(seed.project.key, sessionID);
  if (!ledger || ledger.artifact.status !== "healthy") throw new Error("renewal needs a healthy .spec packet in the continuity ledger");

  const created = await unwrap<Record<string, unknown>>(api.client.session.create({ title: `continuity renewal from ${sessionID}` } as never), "create renewal session");
  const target = typeof created.id === "string" ? created.id : "";
  if (!target) throw new Error("renewal session create response did not include an id");
  await promptSession(api, target, renderRenewalPrompt(ledger));
  upsertLedger(seed, (next) => {
    next.renewal = { targetSessionID: target, targetLedgerPath: ledgerPath(seed.project.key, target), oldSessionID: sessionID, reason: "manual-renew", attemptedAt: Date.now(), completedAt: Date.now() };
  });
  api.route.navigate("session", { sessionID: target });
  api.ui.toast({ variant: "success", title: "Continuity renewed", message: target });
}

async function promptSession(api: TuiPluginApi, sessionID: string, prompt: string) {
  const body = { sessionID, parts: [{ type: "text", text: prompt }], agent: "drive" };
  const session = api.client.session as unknown as { promptAsync?: (input: unknown) => Promise<unknown>; prompt: (input: unknown) => Promise<unknown> };
  if (typeof session.promptAsync === "function") {
    await unwrap(session.promptAsync(body), "prompt renewal session async");
    return;
  }
  await unwrap(session.prompt(body), "prompt renewal session");
}

function registerCommands(api: TuiPluginApi, thresholds: PressureThresholds) {
  return api.command?.register(() => [
    {
      title: "Continuity: checkpoint",
      value: "continuity.checkpoint",
      description: "Write the current continuity ledger checkpoint.",
      category: "continuity",
      slash: { name: "continuity-checkpoint" },
      onSelect: () => runCommand(api, async (sessionID) => {
        const filePath = writeCheckpoint(api, sessionID, thresholds, "manual-checkpoint");
        api.ui.toast({ variant: "success", title: "Continuity checkpoint", message: filePath });
      }),
    },
    {
      title: "Continuity: compact now",
      value: "continuity.compact-now",
      description: "Checkpoint and summarize the current session.",
      category: "continuity",
      slash: { name: "continuity-compact-now" },
      onSelect: () => runCommand(api, (sessionID) => compactNow(api, sessionID, thresholds)),
    },
    {
      title: "Continuity: renew from artifact",
      value: "continuity.renew-from-artifact",
      description: "Create a fresh root Drive session from the ledger and .spec packet.",
      category: "continuity",
      slash: { name: "continuity-renew-from-artifact" },
      onSelect: () => runCommand(api, (sessionID) => renewNow(api, sessionID, thresholds)),
    },
  ]);
}

async function runCommand(api: TuiPluginApi, action: (sessionID: string) => void | Promise<void>) {
  const sessionID = currentSessionID(api);
  if (!sessionID) {
    api.ui.toast({ variant: "warning", title: "Continuity", message: "No active session." });
    return;
  }
  try {
    await action(sessionID);
  } catch (error) {
    api.ui.toast({ variant: "error", title: "Continuity failed", message: errorMessage(error) });
  }
}

function currentSessionID(api: TuiPluginApi) {
  const route = api.route.current;
  return route.name === "session" && typeof route.params?.sessionID === "string" ? route.params.sessionID : undefined;
}

function currentModelRef(api: TuiPluginApi, sessionID: string) {
  const meta = sessionMeta(api, sessionID);
  if (!meta.providerID || !meta.modelID) return undefined;
  return { providerID: meta.providerID, modelID: meta.modelID };
}

async function unwrap<T>(promise: Promise<unknown>, label: string): Promise<T> {
  const response = await promise;
  const envelope = typeof response === "object" && response !== null ? (response as Record<string, unknown>) : undefined;
  if (envelope && "error" in envelope && envelope.error !== undefined) throw new Error(`${label} failed: ${errorMessage(envelope.error)}`);
  if (envelope && "data" in envelope) return envelope.data as T;
  return response as T;
}

function errorMessage(error: unknown) {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  return JSON.stringify(error);
}

const tui: TuiPlugin = async (api) => {
  const settings = readSettings();
  const disposeCommands = registerCommands(api, settings.pressure);
  if (disposeCommands) api.lifecycle.onDispose(disposeCommands);

  api.slots.register({
    order: 128,
    slots: {
      sidebar_content(_ctx, props: { session_id: string }) {
        return <ContinuityPanel api={api} sessionID={props.session_id} thresholds={settings.pressure} />;
      },
    },
  });
};

export default { id, tui } satisfies TuiPluginModule & { id: string };
