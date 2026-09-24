import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import { tool } from "@opencode-ai/plugin";
import { loadDelegateConfig } from "./config.ts";
import { enforceProviderPolicy } from "./policy.ts";
import { closeLane, readChildTaskStatus } from "./lane.ts";
import { errorMessage } from "../shared/error.ts";
import type { Client } from "../shared/opencode.ts";
import { prepareTask, runChildTask } from "./session.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Delegate tools                                                                                │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

const DESCRIPTION = [
  "Launch a specialized subagent task.",
  "Use model as provider/model-id to choose a runtime model for this task call.",
  "Use effort for the target model's reasoning variant; invalid efforts fail explicitly.",
  "If model is omitted, the child uses the agent's pinned model when one exists, else the current assistant message's model and effort.",
  "Optional lane names a reusable child within this parent session; without a lane, each call creates a one-shot child.",
  "A lane pins its agent, but model and effort can change between calls; context-limited and closed lanes roll over to a fresh child.",
  "A closed lane name can come back as a different agent.",
  "compact: true requires an existing idle lane and summarizes it before sending the prompt.",
  "unattended defaults to true; it converts ask to deny for the child and descendants, and children cannot use question.",
  "build/git requires explicit unattended true and an attended primary Collab parent.",
  "Collab presents the named Git plan before invoking build/git through normal task ask permissions, including remembered approvals.",
  "Never launch collab.",
  "Do not make attended children under an unattended parent.",
  "If an interrupted call hides its result, use task_status to reconcile its durable state before calling that lane again.",
  "If the usage cache shows the provider is exhausted, waits for the reset with no maximum wait.",
  "Delegating to a provider missing from delegate.json errors explicitly.",
].join(" ");

const STATUS_DESCRIPTION = [
  "List direct task children with lane names, agents, live statuses, and context-limit and closed markers.",
  "Use after an interrupted call to reconcile durable write state before calling the lane again.",
].join(" ");

const CLOSE_DESCRIPTION = [
  "Close idle named lanes under this session so the sidebar hides them and the names can be reused.",
  "Busy, missing, and already closed lanes are skipped with a reason.",
  "The next task call with a closed lane name starts a fresh child.",
].join(" ");

const id = "delegate-task";

const server: Plugin = async ({ client }) => {
  const config = await loadDelegateConfig();
  // Serialize calls to the same named lane within this plugin instance.
  const activeLanes = new Set<string>();

  return {
    tool: {
      task: tool({
        description: DESCRIPTION,
        args: {
          description: tool.schema.string().describe("A short (3-5 words) description of the task"),
          prompt: tool.schema.string().describe("The task for the agent to perform"),
          subagent_type: tool.schema.string().describe("The type of specialized agent to use for this task"),
          model: tool.schema.string().optional().describe("Optional runtime model as provider/model-id"),
          effort: tool.schema.string().optional().describe("Optional reasoning effort variant for the target model"),
          lane: tool.schema
            .string()
            .optional()
            .describe("Named reusable child within this parent session; omit for a one-shot child"),
          compact: tool.schema
            .boolean()
            .optional()
            .describe("Summarize an existing idle lane before sending this prompt"),
          unattended: tool.schema
            .boolean()
            .optional()
            .describe(
              "Defaults to true; ask becomes deny for the child and descendants. build/git requires explicit true",
            ),
        },
        async execute(args, ctx) {
          const key = args.lane?.trim() ? laneKey(ctx.sessionID, args.lane.trim()) : undefined;
          if (key && activeLanes.has(key)) {
            throw new Error(`delegate lane ${args.lane} is busy; wait until it is idle`);
          }
          if (key) activeLanes.add(key);
          try {
            const prepared = await prepareTask(client, ctx, args);
            const notes = await enforceProviderPolicy(prepared.model.providerID, config, ctx.abort);
            return runChildTask({ client, ctx, prepared, notes });
          } finally {
            if (key) activeLanes.delete(key);
          }
        },
      }),
      task_status: tool({
        description: STATUS_DESCRIPTION,
        args: {},
        async execute(_args, ctx) {
          await ctx.ask({
            permission: "task_status",
            patterns: ["*"],
            always: [],
            metadata: {},
          });
          return readChildTaskStatus(client, ctx.sessionID, ctx.abort);
        },
      }),
      task_close: tool({
        description: CLOSE_DESCRIPTION,
        args: {
          lanes: tool.schema.array(tool.schema.string()).describe("Lane names under this session to close"),
        },
        async execute(args, ctx) {
          const lanes = [...new Set(args.lanes.map((lane) => lane.trim()).filter(Boolean))];
          if (!lanes.length) throw new Error("task_close requires at least one lane name");
          await ctx.ask({
            permission: "task_close",
            patterns: lanes,
            always: [],
            metadata: { lanes },
          });
          return closeLanes({ client, active: activeLanes, sessionID: ctx.sessionID, lanes, signal: ctx.abort });
        },
      }),
    },
  };
};

/** Registers task tools after validating the provider allowlist. */
export default { id, server } satisfies PluginModule;

// ├─ Lane close lock ─────────────────────────────────────────────────────────────────────────────┤

// Closes named lanes in parallel, reporting per-lane failures unless the call is aborted.
async function closeLanes(input: {
  client: Client;
  active: Set<string>;
  sessionID: string;
  lanes: string[];
  signal: AbortSignal;
}) {
  const { client, active, sessionID, signal } = input;
  const lines = await Promise.all(
    input.lanes.map(async (lane) => {
      const key = laneKey(sessionID, lane);
      if (active.has(key)) return `${lane}: skipped (task call in flight)`;
      active.add(key);
      try {
        return `${lane}: ${await closeLane(client, sessionID, lane, signal)}`;
      } catch (error) {
        if (signal.aborted) throw error;
        return `${lane}: failed (${errorMessage(error)})`;
      } finally {
        active.delete(key);
      }
    }),
  );
  return lines.join("\n");
}

// NUL separates parent and lane names without key collisions.
function laneKey(sessionID: string, lane: string) {
  return `${sessionID}\0${lane}`;
}
