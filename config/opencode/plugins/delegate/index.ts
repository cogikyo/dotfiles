import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import { tool } from "@opencode-ai/plugin";
import { decideCapacity } from "./capacity.ts";
import { loadDelegateConfig } from "./config.ts";
import { prepareTask, renderCapacityReport, runChildTask } from "./session.ts";

const DESCRIPTION = [
  "Launch a specialized subagent task.",
  "Use model as provider/model-id to choose a runtime model for this task call.",
  "Use effort for the target model's reasoning variant; invalid efforts fail explicitly.",
  "If model is omitted, the child inherits the current assistant message's model and effort.",
  "Capacity reports mean no child was spawned; pick another provider, lower effort, or ask the user.",
].join(" ");

const id = "delegate-task";

const server: Plugin = async ({ client }) => {
  const config = await loadDelegateConfig();

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
          task_id: tool.schema.string().optional().describe("Existing child session id to resume"),
        },
        async execute(args, ctx) {
          const prepared = await prepareTask(client, ctx, args);
          const capacity = await decideCapacity(prepared.model.providerID, config, ctx.abort);
          if (capacity.action === "report") return renderCapacityReport(args, capacity.report) as never;

          return (await runChildTask({
            client,
            ctx,
            args,
            prepared,
            capacityNotes: capacity.notes,
          })) as never;
        },
      }),
    },
  };
};

export default { id, server } satisfies PluginModule;
