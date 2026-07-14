import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import { tool } from "@opencode-ai/plugin";
import { appendHeadroomSnapshot } from "../usage/headroom.ts";
import { loadDelegateConfig } from "./config.ts";
import { enforceProviderPolicy } from "./policy.ts";
import { prepareTask, runChildTask } from "./session.ts";

const DESCRIPTION = [
  "Launch a specialized subagent task.",
  "Use model as provider/model-id to choose a runtime model for this task call.",
  "Use effort for the target model's reasoning variant; invalid efforts fail explicitly.",
  "If model is omitted, the child uses the agent's pinned model when one exists, else the current assistant message's model and effort.",
  "If the usage cache shows the provider is exhausted, waits for the reset with no maximum wait.",
  "Delegating to a provider missing from delegate.json errors explicitly.",
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
          const notes = await enforceProviderPolicy(prepared.model.providerID, config, ctx.abort);

          return (await runChildTask({
            client,
            ctx,
            args: prepared.args,
            prepared,
            notes,
          })) as never;
        },
      }),
    },
    "tool.definition": async ({ toolID }, output) => {
      if (toolID !== "task" || !output.description.startsWith(DESCRIPTION)) return;
      output.description = await appendHeadroomSnapshot(output.description);
    },
  };
};

export default { id, server } satisfies PluginModule;
