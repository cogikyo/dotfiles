import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import { tool } from "@opencode-ai/plugin";
import { loadDelegateConfig } from "./config.ts";
import { enforceProviderPolicy } from "./policy.ts";
import { prepareTask, readChildTaskStatus, runChildTask } from "./session.ts";

const DESCRIPTION = [
  "Launch a specialized subagent task.",
  "Use model as provider/model-id to choose a runtime model for this task call.",
  "Use effort for the target model's reasoning variant; invalid efforts fail explicitly.",
  "If model is omitted, the child uses the agent's pinned model when one exists, else the current assistant message's model and effort.",
  "Optional lane names a reusable child within this parent session; without a lane, each call creates a one-shot child.",
  "A lane pins its agent, but model and effort can change between calls; context-limited lanes roll over to a fresh child.",
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
  "List direct task children with lane names, agents, live statuses, and context-limit markers.",
  "Use after an interrupted call to reconcile durable write state before calling the lane again.",
].join(" ");

const id = "delegate-task";

const server: Plugin = async ({ client }) => {
  const config = await loadDelegateConfig();
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
          const key = args.lane?.trim() ? `${ctx.sessionID}\0${args.lane.trim()}` : undefined;
          if (key && activeLanes.has(key)) {
            // TODO: Queue busy lanes when OpenCode 2 background tasks are available.
            throw new Error(`delegate lane ${args.lane} is busy; wait until it is idle`);
          }
          if (key) activeLanes.add(key);
          try {
            const prepared = await prepareTask(client, ctx, args);
            const notes = await enforceProviderPolicy(prepared.model.providerID, config, ctx.abort);
            return (await runChildTask({ client, ctx, args: prepared.args, prepared, notes })) as never;
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
    },
  };
};

export default { id, server } satisfies PluginModule;
