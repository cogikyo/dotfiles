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
  sharedSpec: boolean;
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
        <StatusRow api={props.api} state={state()} thresholds={props.thresholds} />
        <RelatedSessions api={props.api} sessions={state().related} />
        <MissingPacketNotice api={props.api} state={state()} />
        <LockNotice api={props.api} locks={state().locks} />
        <RenewalNotice api={props.api} ledger={state().ledger} />
      </box>
    </SidebarSection>
  );
}

function Summary(props: { api: TuiPluginApi; state: ContinuityState }) {
  const c = colors(props.api.theme.current);

  return (
    <box flexDirection="row" gap={0}>
      <Show when={props.state.pressure.level === "compact" || props.state.pressure.level === "renew"}>
        <Chip icon={ui.pressure} value="!" color={levelColor(c, props.state.pressure.level)} />
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

function StatusRow(props: { api: TuiPluginApi; state: ContinuityState; thresholds: PressureThresholds }) {
  const c = colors(props.api.theme.current);
  const level = () => props.state.pressure.level;
  const active = () => level() === "compact" || level() === "renew";
  const label = () => {
    const base = `${ui.pressure} ${formatTokens(props.state.pressure.tokens)} · ${level()}`;
    if (level() === "renew") return `${base} ≥ ${formatTokens(props.thresholds.renewTokens)}`;
    if (level() === "compact") return `${base} ≥ ${formatTokens(props.thresholds.compactTokens)}`;
    return base;
  };
  return (
    <text fg={active() ? levelColor(c, level()) : c.muted} wrapMode="none">
      {label()}
    </text>
  );
}

function MissingPacketNotice(props: { api: TuiPluginApi; state: ContinuityState }) {
  return (
    <Show when={props.state.artifact.status !== "healthy" && props.state.pressure.level === "renew"}>
      <Notice api={props.api} icon={ui.packet} color={colors(props.api.theme.current).yellow} text="spec packet missing; renewal disabled" />
    </Show>
  );
}

function LockNotice(props: { api: TuiPluginApi; locks: ActiveLock[] }) {
  return (
    <Show when={props.locks.length > 0}>
      <For each={props.locks.slice(0, 2)}>
        {(lock) => <Notice api={props.api} icon={ui.lock} color={colors(props.api.theme.current).orange} text={lockLabel(lock)} />}
      </For>
    </Show>
  );
}

function RenewalNotice(props: { api: TuiPluginApi; ledger?: ContinuityLedger }) {
  const color = () => props.ledger?.renewal?.error ? colors(props.api.theme.current).red : colors(props.api.theme.current).sky;
  return (
    <Show when={renewalActive(props.ledger)}>
      <Notice api={props.api} icon={ui.renew} color={color()} text={renewalText(props.ledger)} onMouseDown={() => navigateRenewal(props.api, props.ledger)} />
    </Show>
  );
}

function RelatedSessions(props: { api: TuiPluginApi; sessions: RelatedSession[] }) {
  return (
    <Show when={props.sessions.length > 0}>
      <For each={props.sessions}>
        {(session) => <RelatedRow api={props.api} session={session} />}
      </For>
    </Show>
  );
}

function RelatedRow(props: { api: TuiPluginApi; session: RelatedSession }) {
  const c = colors(props.api.theme.current);
  const color = levelColor(c, props.session.level);
  return (
    <box flexDirection="row" gap={0} onMouseDown={() => props.api.route.navigate("session", { sessionID: props.session.sessionID })}>
      <text fg={color} wrapMode="none">{`${props.session.sharedSpec ? ui.packet : ui.related} `}</text>
      <text fg={c.text} wrapMode="none">{sessionLabel(props.session)}</text>
      <text fg={c.muted} wrapMode="none">{` ${ageLabel(props.session.updatedAt)}`}</text>
    </box>
  );
}

function Notice(props: { api: TuiPluginApi; icon: string; color: ReturnType<typeof colors>[keyof ReturnType<typeof colors>]; text: string; onMouseDown?: () => void }) {
  return (
    <box flexDirection="row" gap={0} onMouseDown={props.onMouseDown}>
      <text fg={props.color} wrapMode="none">{`${props.icon} `}</text>
      <text fg={colors(props.api.theme.current).text} wrapMode="none">{props.text}</text>
    </box>
  );
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

function renewalActive(ledger?: ContinuityLedger) {
  return !!(ledger?.renewal?.targetSessionID || ledger?.renewal?.error || ledger?.renewal?.attemptedAt);
}

function renewalChip(ledger?: ContinuityLedger) {
  if (ledger?.renewal?.error) return "!";
  if (ledger?.renewal?.targetSessionID) return "";
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

function sessionLabel(session: RelatedSession) {
  const title = session.title?.trim();
  const base = title || `${titleCase(session.agent)} ${shortID(session.sessionID)}`;
  return base.length > 34 ? `${base.slice(0, 31)}…` : base;
}

function shortID(sessionID: string) {
  return sessionID.slice(0, 8);
}

// Lock purposes embed session ids and spec paths; show basenames and short ids only.
function lockLabel(lock: ActiveLock) {
  const purpose = lock.purpose.replace(/[^\s:|]*\//g, "").replace(/(ses_[a-zA-Z0-9]{8})[a-zA-Z0-9]*/g, "$1…");
  return `${purpose} · ${shortID(lock.holder)}`;
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

// Other root sessions in the project: shared-spec sessions always, otherwise only recently active ones.
// Subagent (leaf) sessions never appear; their agent names carry a "/" and old leaf ledgers match that too.
function relatedSessions(project: string, sessionID: string, specFiles: string[]): RelatedSession[] {
  const specs = new Set(specFiles);
  return readProjectLedgers(project)
    .filter((ledger) => ledger.session.id !== sessionID)
    .filter((ledger) => !ledger.renewal?.completedAt)
    .filter((ledger) => !ledger.session.agent.includes("/"))
    .map((ledger) => ({ ledger, sharedSpec: ledger.artifact.specFiles.some((file) => specs.has(file)) }))
    .filter(({ ledger, sharedSpec }) => sharedSpec || Date.now() - ledger.updatedAt < STALE_SESSION_MS)
    .map(({ ledger, sharedSpec }) => ({
      sessionID: ledger.session.id,
      title: ledger.session.title,
      agent: ledger.session.agent,
      sharedSpec,
      updatedAt: ledger.updatedAt,
      level: relatedLevel(ledger),
    }))
    .sort((left, right) => Number(right.sharedSpec) - Number(left.sharedSpec) || right.updatedAt - left.updatedAt)
    .slice(0, 4);
}

function relatedLevel(ledger: ContinuityLedger): ContinuityLevel {
  if (Date.now() - ledger.updatedAt > STALE_SESSION_MS) return "stale";
  if (ledger.renewal?.error) return "blocked";
  if (ledger.pressure.level !== "low") return ledger.pressure.level;
  if (ledger.artifact.status !== "healthy") return "watch";
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
  const dirty = editedFiles(api, sessionID);
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

function editedFiles(api: TuiPluginApi, sessionID: string) {
  return { files: Array.from(new Set(api.state.session.diff(sessionID).map((file) => file.file))).sort() };
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
