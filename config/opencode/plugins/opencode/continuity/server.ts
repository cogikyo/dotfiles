import type { Plugin, PluginInput, PluginModule } from "@opencode-ai/plugin";
import { tool } from "@opencode-ai/plugin";
import { assessPressure, type PressureThresholds, type TokenMessage } from "./pressure.ts";
import { readSettings } from "./settings.ts";
import {
  emptyArtifactHealth,
  ledgerPath,
  projectKey,
  readLedger,
  renderCheckpointSummary,
  renderRenewalPrompt,
  resolveSpecFile,
  TRACKED_SESSION_NAME_MAX_LENGTH,
  TRACKED_SESSION_NAME_PATTERN,
  upsertLedger,
  withLedgerLock,
  withLedgerLockAsync,
  type ArtifactHealth,
  type ContinuityLedger,
  type LedgerSeed,
  type PressureSnapshot,
} from "./state.ts";

const id = "opencode-continuity";
const MIN_AUTOMATION_INTERVAL_MS = 10 * 60_000;

type Client = PluginInput["client"];
type SessionRecord = Record<string, unknown>;
type MessageRecord = { info: Record<string, unknown>; parts: unknown[] };
type ModelRef = { providerID: string; modelID: string };

const server: Plugin = async ({ client, directory, worktree }) => {
  const projectPath = worktree || directory;
  const key = projectKey(projectPath);
  const settings = readSettings();
  let reserved = 10_000;

  return {
    config: async (cfg) => {
      const compaction = object((cfg as Record<string, unknown>).compaction);
      const value = compaction?.reserved;
      if (typeof value === "number" && Number.isFinite(value) && value > 0) reserved = value;
    },
    tool: {
      continuity_track: tool({
        description: "Name this spec-backed root session as a continuity thread jump target. Name must be 3-4 ALL CAPS words and at most 28 chars.",
        args: {
          name: tool.schema.string().describe("3-4 ALL CAPS words, <= 28 chars, words may contain A-Z, 0-9, and hyphen"),
        },
        async execute(args, context) {
          const name = args.name.trim();
          if (!trackedSessionName(name)) throw new Error("continuity_track name must be 3-4 ALL CAPS words, <= 28 chars");
          const targetProjectPath = context.worktree || context.directory || projectPath;
          const tracked = await refreshLedger(client, { projectPath: targetProjectPath, projectKey: projectKey(targetProjectPath), sessionID: context.sessionID, lastEvent: "continuity_track", reserved, thresholds: settings.pressure });
          if (!tracked) throw new Error("continuity_track can only track root sessions with a readable .spec packet");
          if (tracked.artifact.status !== "healthy" || tracked.artifact.specFiles.length === 0) throw new Error("continuity_track requires a .spec packet in this session first");

          await unwrap(client.session.update({ path: { id: context.sessionID }, body: { title: name } } as never), "track continuity session");
          upsertLedger({ ...seedFromLedger(tracked, "continuity_track"), session: { ...tracked.session, title: name } });

          return `continuity tracked as ${name}`;
        },
      }),
    },
    event: async ({ event }) => {
      const sessionID = sessionIDFromEvent(event);
      if (!sessionID || !isRelevantEvent(event.type)) return;

      const ledger = await refreshLedger(client, { projectPath, projectKey: key, sessionID, lastEvent: event.type, reserved, thresholds: settings.pressure });
      if (!ledger || ledger.artifact.status !== "healthy" || !isDriveAgent(ledger.session.agent)) return;

      // Automation runs only at idle so summarize/renew never interrupt a running turn.
      if (event.type !== "session.idle") return;

      if (ledger.pressure.level === "renew") {
        await renewFromLedger(client, ledger, "pressure-renewal");
        return;
      }

      if (ledger.pressure.level === "compact") await summarizeFromLedger(client, ledger);
    },
    "experimental.session.compacting": async (input, output) => {
      const ledger = await refreshLedger(client, { projectPath, projectKey: key, sessionID: input.sessionID, lastEvent: "experimental.session.compacting", reserved, thresholds: settings.pressure });
      if (!ledger) return;

      const summary = renderCheckpointSummary(ledger, "compaction");
      withLedgerLock(ledger.project.key, ledger.session.id, `checkpoint:${ledger.session.id}`, () => {
        upsertLedger(seedFromLedger(ledger, "experimental.session.compacting"), (next) => {
          next.checkpoint = { reason: "compaction", writtenAt: Date.now(), summary };
        });
      });

      output.context.push([
        "Continuity checkpoint before compaction:",
        summary,
        "Treat listed .spec packet(s) as durable truth; raw chat and this ledger are recovery hints only.",
      ].join("\n"));
    },
    "experimental.compaction.autocontinue": async (input, output) => {
      const ledger = await refreshLedger(client, { projectPath, projectKey: key, sessionID: input.sessionID, lastEvent: "experimental.compaction.autocontinue", reserved, thresholds: settings.pressure });
      if (!ledger || ledger.artifact.status !== "healthy" || !isDriveAgent(ledger.session.agent || input.agent)) return;
      if (ledger.pressure.level !== "renew") return;

      const renewed = await renewFromLedger(client, ledger, "post-compaction-renewal");
      if (renewed) output.enabled = false;
    },
  };
};

async function refreshLedger(
  client: Client,
  input: { projectPath: string; projectKey: string; sessionID: string; lastEvent: string; reserved: number; thresholds: PressureThresholds },
) {
  try {
    const [session, records] = await Promise.all([
      readSession(client, input.sessionID),
      readMessages(client, input.sessionID),
    ]);
    // Subagent (child) sessions never get ledgers; they would pollute related-session state.
    if (typeof session.parentID === "string" && session.parentID !== "") return undefined;

    const limit = await readModelLimit(client, records);
    const messages = records.map((record) => record.info as TokenMessage);
    const sessionFiles = sessionDiffFiles(records);
    const cwd = latestCwd(records) || string(session.directory) || input.projectPath;
    const artifact = artifactHealth(input.projectPath, cwd, records);
    const dirty = editedFiles(sessionFiles);
    const pressure = pressureSnapshot(messages, input.reserved, limit, input.thresholds);
    const agent = sessionAgent(records) || string(session.agent);
    const seed: LedgerSeed = {
      project: { key: input.projectKey, path: input.projectPath },
      session: { id: input.sessionID, agent, title: string(session.title) || undefined },
      pressure,
      artifact,
      dirty,
      lastEvent: input.lastEvent,
    };

    upsertLedger(seed);
    return readLedger(input.projectKey, input.sessionID) ?? { ...seed, version: 1, schema: "opencode-continuity/v1", updatedAt: Date.now() } satisfies ContinuityLedger;
  } catch (error) {
    await log(client, "warn", `continuity ledger refresh failed for ${input.sessionID}: ${errorMessage(error)}`);
    return undefined;
  }
}

async function summarizeFromLedger(client: Client, ledger: ContinuityLedger) {
  const now = Date.now();
  if (ledger.automation?.lastSummarizeAt && now - ledger.automation.lastSummarizeAt < MIN_AUTOMATION_INTERVAL_MS) return;

  await withLedgerLockAsync(ledger.project.key, ledger.session.id, `summarize:${ledger.session.id}`, async () => {
    const current = readLedger(ledger.project.key, ledger.session.id) ?? ledger;
    if (current.automation?.lastSummarizeAt && now - current.automation.lastSummarizeAt < MIN_AUTOMATION_INTERVAL_MS) return;
    try {
      const model = await readLatestModelRef(client, ledger.session.id);
      if (!model) throw new Error("no model reference available for summarize");
      await unwrap(client.session.summarize({ path: { id: ledger.session.id }, body: model } as never), "summarize session");
      upsertLedger(seedFromLedger(current, "session.summarize"), (next) => {
        next.automation = { ...next.automation, lastSummarizeAt: now };
        next.checkpoint = { reason: "auto-summarize", writtenAt: now, summary: renderCheckpointSummary(current, "auto-summarize") };
      });
    } catch (error) {
      upsertLedger(seedFromLedger(current, "session.summarize.failed"), (next) => {
        next.automation = { ...next.automation, lastSummarizeAt: now };
        next.checkpoint = { reason: "auto-summarize-failed", writtenAt: now, summary: errorMessage(error) };
      });
      await log(client, "warn", `continuity summarize failed for ${ledger.session.id}: ${errorMessage(error)}`);
    }
  });
}

async function renewFromLedger(client: Client, ledger: ContinuityLedger, reason: string) {
  const now = Date.now();
  if (ledger.automation?.lastRenewAt && now - ledger.automation.lastRenewAt < MIN_AUTOMATION_INTERVAL_MS) return false;
  if (ledger.artifact.status !== "healthy" || !isDriveAgent(ledger.session.agent)) return false;

  return (await withLedgerLockAsync(ledger.project.key, ledger.session.id, `renew:${ledger.artifact.specFiles.join("|") || ledger.session.id}`, async () => {
    const current = readLedger(ledger.project.key, ledger.session.id) ?? ledger;
    if (current.renewal?.completedAt || (current.automation?.lastRenewAt && now - current.automation.lastRenewAt < MIN_AUTOMATION_INTERVAL_MS)) return false;

    try {
      upsertLedger(seedFromLedger(current, "renewal.attempted"), (next) => {
        next.automation = { ...next.automation, lastRenewAt: now };
        next.renewal = { ...next.renewal, oldSessionID: current.session.id, reason, attemptedAt: now };
      });

      const created = await unwrap<Record<string, unknown>>(
        client.session.create({ body: { title: `continuity renewal from ${ledger.session.id}` } } as never),
        "create renewal session",
      );
      const target = string(created.id);
      if (!target) throw new Error("renewal session create response did not include an id");

      const prompt = renderRenewalPrompt(current);
      await promptSession(client, target, prompt);

      upsertLedger(seedFromLedger(current, "renewal.completed"), (next) => {
        next.automation = { ...next.automation, lastRenewAt: now };
        next.renewal = {
          targetSessionID: target,
          targetLedgerPath: ledgerPath(current.project.key, target),
          oldSessionID: current.session.id,
          reason,
          attemptedAt: now,
          completedAt: Date.now(),
        };
      });
      return true;
    } catch (error) {
      upsertLedger(seedFromLedger(current, "renewal.failed"), (next) => {
        next.renewal = { ...next.renewal, oldSessionID: current.session.id, reason, attemptedAt: now, error: errorMessage(error) };
        next.automation = { ...next.automation, lastRenewAt: now };
      });
      await log(client, "warn", `continuity renewal failed for ${ledger.session.id}: ${errorMessage(error)}`);
      return false;
    }
  })) ?? false;
}

async function promptSession(client: Client, sessionID: string, prompt: string) {
  const body = { parts: [{ type: "text", text: prompt }], agent: "drive" };
  const session = client.session as unknown as Record<string, unknown>;
  const promptAsync = session.promptAsync;
  if (typeof promptAsync === "function") {
    await unwrap(promptAsync.call(client.session, { path: { id: sessionID }, body } as never), "prompt renewal session async");
    return;
  }
  await unwrap(client.session.prompt({ path: { id: sessionID }, body } as never), "prompt renewal session");
}

async function readSession(client: Client, sessionID: string) {
  return await unwrap<SessionRecord>(client.session.get({ path: { id: sessionID } } as never), "read session");
}

async function readMessages(client: Client, sessionID: string): Promise<MessageRecord[]> {
  const response = await unwrap<unknown>(client.session.messages({ path: { id: sessionID } } as never), "read messages");
  if (!Array.isArray(response)) return [];
  return response.flatMap((item) => {
    const root = object(item);
    if (!root) return [];
    if (object(root.info)) return [{ info: root.info as Record<string, unknown>, parts: Array.isArray(root.parts) ? root.parts : [] }];
    return [{ info: root, parts: [] }];
  });
}

function pressureSnapshot(messages: TokenMessage[], reserved: number, limit: number | undefined, thresholds: PressureThresholds): PressureSnapshot {
  return { ...assessPressure({ messages, modelLimit: limit, reserved, thresholds }), updatedAt: Date.now() };
}

function artifactHealth(projectPath: string, cwd: string, records: MessageRecord[]): ArtifactHealth {
  const specFiles = Array.from(new Set(specFilesFromParts(projectPath, cwd, records))).sort();
  if (specFiles.length === 0) return emptyArtifactHealth();
  return { status: "healthy", specFiles, notes: ["durable .spec packet exists"], checkedAt: Date.now() };
}

function specFilesFromParts(projectPath: string, cwd: string, records: MessageRecord[]) {
  const files: string[] = [];
  for (const record of records) {
    for (const part of record.parts) {
      const root = object(part);
      const source = object(root?.source);
      const sourcePath = string(source?.path);
      const sourceSpec = sourcePath ? resolveSpecFile(projectPath, sourcePath, cwd) : undefined;
      if (sourceSpec) files.push(sourceSpec);
      const state = object(root?.state);
      const input = object(state?.input);
      for (const value of Object.values(input ?? {})) {
        const spec = typeof value === "string" ? resolveSpecFile(projectPath, value, cwd) : undefined;
        if (spec) files.push(spec);
      }
    }
  }
  return files;
}

function editedFiles(sessionFiles: string[]) {
  return { files: Array.from(new Set(sessionFiles.map(cleanRelative))).sort() };
}

function sessionDiffFiles(records: MessageRecord[]) {
  const files = new Set<string>();
  for (const record of records) {
    const summary = object(record.info.summary);
    const diffs = Array.isArray(summary?.diffs) ? summary.diffs : [];
    for (const diff of diffs) {
      const file = string(object(diff)?.file);
      if (file) files.add(file);
    }
    for (const part of record.parts) {
      const root = object(part);
      if (root?.type === "patch" && Array.isArray(root.files)) {
        for (const file of root.files) if (typeof file === "string") files.add(file);
      }
    }
  }
  return Array.from(files).sort();
}

async function readModelLimit(client: Client, records: MessageRecord[]) {
  const ref = latestModelRef(records);
  if (!ref) return undefined;
  try {
    const response = await unwrap<unknown>(client.provider.list({} as never), "read providers");
    const all = object(response)?.all;
    const providers = Array.isArray(response) ? response : Array.isArray(all) ? all : [];
    for (const provider of providers) {
      const root = object(provider);
      if (root?.id !== ref.providerID) continue;
      const model = object(object(root.models)?.[ref.modelID]);
      const context = object(model?.limit)?.context;
      return typeof context === "number" ? context : undefined;
    }
  } catch {
    return undefined;
  }
  return undefined;
}

async function readLatestModelRef(client: Client, sessionID: string) {
  return latestModelRef(await readMessages(client, sessionID));
}

function latestModelRef(records: MessageRecord[]): ModelRef | undefined {
  for (let index = records.length - 1; index >= 0; index -= 1) {
    const info = records[index].info;
    const assistantProvider = string(info.providerID);
    const assistantModel = string(info.modelID);
    if (assistantProvider && assistantModel) return { providerID: assistantProvider, modelID: assistantModel };
    const userModel = object(info.model);
    const providerID = string(userModel?.providerID);
    const modelID = string(userModel?.modelID);
    if (providerID && modelID) return { providerID, modelID };
  }
  return undefined;
}

function sessionAgent(records: MessageRecord[]) {
  for (let index = records.length - 1; index >= 0; index -= 1) {
    const agent = string(records[index].info.agent);
    if (agent) return agent;
  }
  return "";
}

function latestCwd(records: MessageRecord[]) {
  for (let index = records.length - 1; index >= 0; index -= 1) {
    const cwd = string(object(records[index].info.path)?.cwd);
    if (cwd) return cwd;
  }
  return "";
}

function seedFromLedger(ledger: ContinuityLedger, lastEvent: string): LedgerSeed {
  return { project: ledger.project, session: ledger.session, pressure: ledger.pressure, artifact: ledger.artifact, dirty: ledger.dirty, lastEvent };
}

function sessionIDFromEvent(event: { type: string; properties: unknown }) {
  const props = object(event.properties);
  const info = object(props?.info);
  const part = object(props?.part);
  return string(props?.sessionID) || string(info?.sessionID) || string(part?.sessionID);
}

function isRelevantEvent(type: string) {
  return type === "message.updated" || type === "session.diff" || type === "session.compacted" || type === "session.idle";
}

function isDriveAgent(agent: string) {
  return agent.toLowerCase() === "drive";
}

function trackedSessionName(name: string) {
  return name.length <= TRACKED_SESSION_NAME_MAX_LENGTH && TRACKED_SESSION_NAME_PATTERN.test(name);
}

function cleanRelative(value: string) {
  return value.replace(/^\.\//, "");
}

async function log(client: Client, level: "info" | "warn" | "error", message: string) {
  try {
    await client.app.log({ body: { service: id, level, message } } as never);
  } catch {}
}

async function unwrap<T>(promise: Promise<unknown>, label: string): Promise<T> {
  const response = await promise;
  const envelope = object(response);
  if (envelope && "error" in envelope && envelope.error !== undefined) throw new Error(`${label} failed: ${errorMessage(envelope.error)}`);
  if (envelope && "data" in envelope) return envelope.data as T;
  return response as unknown as T;
}

function object(value: unknown): Record<string, unknown> | undefined {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : undefined;
}

function string(value: unknown) {
  return typeof value === "string" ? value : "";
}

function errorMessage(error: unknown) {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  return JSON.stringify(error);
}

export default { id, server } satisfies PluginModule;
