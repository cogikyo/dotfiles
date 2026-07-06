/** @jsxImportSource @opentui/solid */
import type { Message } from "@opencode-ai/sdk/v2";
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from "@opencode-ai/plugin/tui";
import { For, Show, createMemo, createSignal, onCleanup } from "solid-js";
import { colors, pressureColor } from "../../shared/colors.ts";
import { formatTokens, sessionMessages, sessionMeta } from "../../shared/session.ts";
import { SidebarSection } from "../../shared/sidebar-section.tsx";
import { progressIcon } from "../../shared/icons.ts";
import { assessPressure } from "./pressure.ts";
import {
  emptyArtifactHealth,
  ledgerPath,
  projectKey,
  readActiveLocks,
  readLedger,
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

function ContinuityPanel(props: { api: TuiPluginApi; sessionID: string }) {
  const [revision, setRevision] = createSignal(0);
  const refresh = () => setRevision((value) => value + 1);
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
    for (const dispose of disposers) dispose();
  });

  const state = createMemo(() => {
    revision();
    return continuityState(props.api, props.sessionID);
  });

  return (
    <SidebarSection api={props.api} title="Continuity" detail={`${state().pressure.percent.toFixed(0)}% ${state().artifact.status}`} initiallyExpanded={true}>
      <box flexDirection="column" gap={0}>
        <Row api={props.api} label="packet" value={packetLabel(state().artifact)} tone={state().artifact.status === "healthy" ? "good" : "warn"} />
        <Row api={props.api} label="pressure" value={`${progressIcon(state().pressure.percent)} ${formatTokens(state().pressure.tokens)} / ${formatTokens(state().pressure.usable ?? state().pressure.limit ?? 0)}`} color={pressureColor(props.api.theme.current, state().pressure.percent)} />
        <Row api={props.api} label="dirty" value={`${state().dirty.sessionFiles.length}/${state().dirty.files.length} covered · ${state().dirty.percent.toFixed(0)}%`} tone={state().dirty.uncovered.length === 0 ? "good" : "warn"} />
        <Row api={props.api} label="lock" value={lockLabel(state().locks)} tone={state().locks.length > 0 ? "warn" : "muted"} />
        <Row api={props.api} label="renew" value={renewLabel(state().ledger)} tone={state().ledger?.renewal?.targetSessionID ? "good" : "muted"} />
        <Show when={state().dirty.uncovered.length > 0}>
          <For each={state().dirty.uncovered.slice(0, 4)}>
            {(file) => <Row api={props.api} label="open" value={file} tone="muted" />}
          </For>
        </Show>
      </box>
    </SidebarSection>
  );
}

function Row(props: { api: TuiPluginApi; label: string; value: string; tone?: "good" | "warn" | "muted"; color?: ReturnType<typeof pressureColor> }) {
  const c = colors(props.api.theme.current);
  const fg = props.color ?? (props.tone === "good" ? c.green : props.tone === "warn" ? c.yellow : c.muted);
  return (
    <box flexDirection="row" gap={0}>
      <text fg={c.muted} wrapMode="none">{`${props.label}: `}</text>
      <text fg={fg} wrapMode="none">{props.value}</text>
    </box>
  );
}

function continuityState(api: TuiPluginApi, sessionID: string) {
  const projectPath = api.state.path.worktree || api.state.path.directory || process.cwd();
  const key = projectKey(projectPath);
  const ledger = readLedger(key, sessionID);
  const seed = ledgerSeed(api, sessionID, ledger);
  return { ledger, locks: readActiveLocks(key), ...seed };
}

function ledgerSeed(api: TuiPluginApi, sessionID: string, ledger?: ContinuityLedger): LedgerSeed {
  const projectPath = api.state.path.worktree || api.state.path.directory || process.cwd();
  const messages = sessionMessages(api, sessionID) as ReadonlyArray<Message>;
  const meta = sessionMeta(api, sessionID);
  const pressure = pressureSnapshot(api, sessionID, messages);
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

function pressureSnapshot(api: TuiPluginApi, sessionID: string, messages: ReadonlyArray<Message>): PressureSnapshot {
  const meta = sessionMeta(api, sessionID);
  const model = api.state.provider.find((provider) => provider.id === meta.providerID)?.models[meta.modelID];
  const reserved = reservedTokens(api);
  return { ...assessPressure({ messages, modelLimit: model?.limit.context, reserved }), updatedAt: Date.now() };
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

function writeCheckpoint(api: TuiPluginApi, sessionID: string, reason: string) {
  const seed = ledgerSeed(api, sessionID, readLedger(projectKey(api.state.path.worktree || api.state.path.directory || process.cwd()), sessionID));
  const filePath = withLedgerLock(seed.project.key, sessionID, `tui-checkpoint:${sessionID}`, () => {
    return upsertLedger(seed, (ledger) => {
      ledger.checkpoint = { reason, writtenAt: Date.now(), summary: renderCheckpointSummary(ledger, reason) };
    });
  });
  if (!filePath) throw new Error("continuity checkpoint lock is busy");
  return filePath;
}

async function compactNow(api: TuiPluginApi, sessionID: string) {
  const filePath = writeCheckpoint(api, sessionID, "manual-compact");
  const model = currentModelRef(api, sessionID);
  if (!model) throw new Error("compact needs a current model reference");
  await unwrap(api.client.session.summarize({ sessionID, ...model } as never), "compact session");
  api.ui.toast({ variant: "success", title: "Continuity checkpoint", message: filePath });
}

async function renewNow(api: TuiPluginApi, sessionID: string) {
  writeCheckpoint(api, sessionID, "manual-renew");
  const seed = ledgerSeed(api, sessionID, readLedger(projectKey(api.state.path.worktree || api.state.path.directory || process.cwd()), sessionID));
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

function registerCommands(api: TuiPluginApi) {
  return api.command?.register(() => [
    {
      title: "Continuity: checkpoint",
      value: "continuity.checkpoint",
      description: "Write the current continuity ledger checkpoint.",
      category: "continuity",
      slash: { name: "continuity-checkpoint" },
      onSelect: () => runCommand(api, async (sessionID) => {
        const filePath = writeCheckpoint(api, sessionID, "manual-checkpoint");
        api.ui.toast({ variant: "success", title: "Continuity checkpoint", message: filePath });
      }),
    },
    {
      title: "Continuity: compact now",
      value: "continuity.compact-now",
      description: "Checkpoint and summarize the current session.",
      category: "continuity",
      slash: { name: "continuity-compact-now" },
      onSelect: () => runCommand(api, (sessionID) => compactNow(api, sessionID)),
    },
    {
      title: "Continuity: renew from artifact",
      value: "continuity.renew-from-artifact",
      description: "Create a fresh root Drive session from the ledger and .spec packet.",
      category: "continuity",
      slash: { name: "continuity-renew-from-artifact" },
      onSelect: () => runCommand(api, (sessionID) => renewNow(api, sessionID)),
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

function packetLabel(artifact: ArtifactHealth) {
  if (artifact.specFiles.length === 0) return artifact.status;
  if (artifact.specFiles.length === 1) return artifact.specFiles[0];
  return `${artifact.specFiles.length} packets`;
}

function lockLabel(locks: ActiveLock[]) {
  if (locks.length === 0) return "free";
  const lock = locks[0];
  const suffix = locks.length > 1 ? ` +${locks.length - 1}` : "";
  return `${lock.purpose} by ${lock.holder}${suffix}`;
}

function renewLabel(ledger?: ContinuityLedger) {
  if (ledger?.renewal?.targetSessionID) return ledger.renewal.targetSessionID;
  if (ledger?.renewal?.error) return "failed";
  return "none";
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
  const disposeCommands = registerCommands(api);
  if (disposeCommands) api.lifecycle.onDispose(disposeCommands);

  api.slots.register({
    order: 128,
    slots: {
      sidebar_content(_ctx, props: { session_id: string }) {
        return <ContinuityPanel api={api} sessionID={props.session_id} />;
      },
    },
  });
};

export default { id, tui } satisfies TuiPluginModule & { id: string };
