import type { Plugin, PluginInput, PluginModule } from "@opencode-ai/plugin";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { record } from "../shared/record.ts";
import {
  isCompactedPart,
  isCompletedSkillPart,
  isCompletedToolPart,
  isProtectedMarkdownPath,
  persistCompactedPartsHttp,
  persistCompactedSkillParts,
  persistUpdatedPart,
  persistUpdatedPartsHttp,
  stubSkillParts,
  toolMarkdownPath,
  uncompactProtectedParts,
  withoutCompactedTime,
  type PartClient,
  type ProtectRoots,
  type SkillToolPart,
} from "./skill-parts.ts";

const id = "opencode-skill-compact";
const configRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");

const server: Plugin = async ({ client, directory, worktree, serverUrl }) => {
  const compacting = new Set<string>();
  const partClient = partAPI(client);

  const protectRoots = (agent?: string): ProtectRoots => ({
    configRoot,
    projectRoots: [directory, worktree].filter(Boolean),
    agentNames: ["collab", agent].filter((name): name is string => Boolean(name)),
  });

  return {
    // This hook supplies extra context strings to the default compaction prompt.
    "experimental.session.compacting": async (input, output) => {
      compacting.add(input.sessionID);
      output.context.push(
        "Loaded skill bodies were dropped from context. Do not copy skill instructions into the summary. Reload a skill later if its procedure is needed again.",
      );
      try {
        const parts = await sessionSkillParts(client, input.sessionID);
        if (partClient) await persistCompactedSkillParts(partClient, parts);
        else await persistCompactedPartsHttp(serverUrl, directory, parts);
      } catch {
        return;
      }
    },
    // OpenCode exposes transformed messages as { info, parts } entries.
    "experimental.chat.messages.transform": async (_input, output) => {
      const agent = currentAgentFromMessages(output.messages);
      uncompactProtectedParts(
        output.messages.flatMap((message) => (message.parts ?? []) as SkillToolPart[]),
        protectRoots(agent),
      );
      const sessionID = sessionIDFromMessages(output.messages);
      if (!sessionID || !compacting.has(sessionID)) return;
      compacting.delete(sessionID);
      for (const message of output.messages) stubSkillParts(message.parts);
    },
    event: async ({ event }) => {
      if (event.type !== "message.part.updated") return;
      const part = event.properties.part as SkillToolPart;
      if (!isCompletedToolPart(part) || !isCompactedPart(part)) return;
      const filePath = toolMarkdownPath(part);
      if (!filePath || !isProtectedMarkdownPath(filePath, protectRoots())) return;
      const next = withoutCompactedTime(part);
      try {
        if (partClient) await persistUpdatedPart(partClient, next);
        else await persistUpdatedPartsHttp(serverUrl, directory, [next]);
      } catch {
        return;
      }
    },
  };
};

function partAPI(client: unknown): PartClient | undefined {
  const part = record(record(client)?.part);
  const update = part?.update;
  if (typeof update !== "function") return undefined;
  return { part: { update: (args) => Promise.resolve(update.call(part, args)) } };
}

async function sessionSkillParts(client: PluginInput["client"], sessionID: string) {
  const response = await client.session.messages({ path: { id: sessionID } });
  if (response.error || !response.data) throw new Error(`read session ${sessionID} messages failed`);
  return response.data.flatMap((message) => message.parts.filter(isCompletedSkillPart));
}

function currentAgentFromMessages(messages: ReadonlyArray<{ info?: object }>) {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const info = messages[index]?.info;
    const agent = info && "agent" in info ? info.agent : undefined;
    if (typeof agent === "string" && agent) return agent;
  }
  return undefined;
}

function sessionIDFromMessages(
  messages: ReadonlyArray<{ info?: { sessionID?: string }; parts?: Array<{ sessionID?: string }> }>,
) {
  for (const message of messages) {
    if (typeof message.info?.sessionID === "string" && message.info.sessionID) return message.info.sessionID;
    for (const part of message.parts ?? []) {
      if (typeof part.sessionID === "string" && part.sessionID) return part.sessionID;
    }
  }
  return undefined;
}

/** Uses server compaction, message-transform, and message.part.updated hooks to preserve tool parts. */
export default { id, server } satisfies PluginModule;
