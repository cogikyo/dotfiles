import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import { errorMessage } from "../shared/error.ts";
import { session, unwrap, type Client } from "../shared/opencode.ts";
import { configRoot, type Loader } from "../shared/root.ts";
import {
  isCompactedPart,
  isCompletedSkillPart,
  isCompletedToolPart,
  isProtectedMarkdownPath,
  persistCompactedPartsHttp,
  persistUpdatedPartsHttp,
  stubSkillParts,
  toolMarkdownPath,
  uncompactProtectedParts,
  withoutCompactedTime,
  type ProtectRoots,
} from "./skill-parts.ts";

const id = "opencode-skill-compact";

const server: Plugin = async ({ client, directory, worktree, serverUrl }) => {
  const compacting = new Set<string>();

  const protectRoots = (loader: Loader): ProtectRoots => ({
    configRoot,
    projectRoots: [directory, worktree].filter(Boolean),
    session: loader,
  });

  const loaderOf = async (sessionID: string, agent?: string): Promise<Loader> => {
    const info = await session(client, sessionID, { label: `${id} read session ${sessionID}` });
    return { agent: agent ?? info.agent, parentID: info.parentID };
  };

  return {
    "experimental.session.compacting": async (input, output) => {
      compacting.add(input.sessionID);
      output.context.push(
        "Loaded skill bodies were dropped from context. Do not copy skill instructions into the summary. Reload a skill later if its procedure is needed again.",
      );
      try {
        const parts = await sessionSkillParts(client, input.sessionID);
        await persistCompactedPartsHttp(serverUrl, directory, parts);
      } catch {
        return;
      }
    },
    "experimental.chat.messages.transform": async (_input, output) => {
      const sessionID = sessionIDFromMessages(output.messages);
      if (!sessionID) return;
      try {
        const loader = await loaderOf(sessionID, currentAgentFromMessages(output.messages));
        uncompactProtectedParts(
          output.messages.flatMap((message) => message.parts),
          protectRoots(loader),
        );
      } catch (error) {
        console.error(`${id}: ${errorMessage(error)}`);
      }
      if (!compacting.has(sessionID)) return;
      compacting.delete(sessionID);
      for (const message of output.messages) stubSkillParts(message.parts);
    },
    event: async ({ event }) => {
      if (event.type !== "message.part.updated") return;
      const part = event.properties.part;
      if (!isCompletedToolPart(part) || !isCompactedPart(part)) return;
      const filePath = toolMarkdownPath(part);
      if (!filePath) return;
      try {
        if (!isProtectedMarkdownPath(filePath, protectRoots(await loaderOf(part.sessionID)))) return;
        await persistUpdatedPartsHttp(serverUrl, directory, [withoutCompactedTime(part)]);
      } catch {
        return;
      }
    },
  };
};

/** Server plugin that compacts completed skill output while keeping project instructions and agent Markdown available. */
export default { id, server } satisfies PluginModule;

async function sessionSkillParts(client: Client, sessionID: string) {
  const messages = await unwrap(
    client.session.messages({ path: { id: sessionID } }),
    `read session ${sessionID} messages`,
  );
  return messages.flatMap((message) => message.parts.filter(isCompletedSkillPart));
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
