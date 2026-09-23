import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import path from "node:path";
import { fileURLToPath } from "node:url";
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

type SessionClient = {
  session: {
    messages: (args: unknown) => Promise<unknown>;
  };
};

type SessionMessage = {
  info?: {
    id?: string;
    sessionID?: string;
    agent?: string;
  };
  parts?: SkillToolPart[];
};

const server: Plugin = async ({ client, directory, worktree, serverUrl }) => {
  const compacting = new Set<string>();
  const partClient = client as PartClient;
  const sessionClient = client as SessionClient;

  const protectRoots = (agent?: string): ProtectRoots => ({
    configRoot,
    projectRoots: [directory, worktree].filter(Boolean),
    agentNames: ["collab", agent].filter((name): name is string => Boolean(name)),
  });

  return {
    "experimental.session.compacting": async (input, output) => {
      compacting.add(input.sessionID);
      output.context.push(
        "Loaded skill bodies were dropped from context. Do not copy skill instructions into the summary. Reload a skill later if its procedure is needed again.",
      );
      try {
        const parts = await sessionSkillParts(sessionClient, input.sessionID);
        if (partClient.part?.update) await persistCompactedSkillParts(partClient, parts);
        else await persistCompactedPartsHttp(serverUrl, directory, parts);
      } catch {
        return;
      }
    },
    "experimental.chat.messages.transform": async (_input, output) => {
      const agent = currentAgentFromMessages(output.messages);
      uncompactProtectedParts(
        output.messages.flatMap((message) => (message.parts ?? []) as SkillToolPart[]),
        protectRoots(agent),
      );
      const sessionID = sessionIDFromMessages(output.messages);
      if (!sessionID || !compacting.has(sessionID)) return;
      compacting.delete(sessionID);
      for (const message of output.messages) stubSkillParts(message.parts as SkillToolPart[]);
    },
    event: async ({ event }) => {
      if (event.type !== "message.part.updated") return;
      const part = event.properties.part as SkillToolPart;
      if (!isCompletedToolPart(part) || !isCompactedPart(part)) return;
      const filePath = toolMarkdownPath(part);
      if (!filePath || !isProtectedMarkdownPath(filePath, protectRoots())) return;
      const next = withoutCompactedTime(part);
      try {
        if (partClient.part?.update) await persistUpdatedPart(partClient, next);
        else await persistUpdatedPartsHttp(serverUrl, directory, [next]);
      } catch {
        return;
      }
    },
  };
};

async function sessionSkillParts(client: SessionClient, sessionID: string) {
  const messages = await unwrap<SessionMessage[]>(
    client.session.messages({
      path: { id: sessionID },
    } as never),
    `read session ${sessionID} messages`,
  );
  return messages.flatMap((message) => (message.parts ?? []).filter(isCompletedSkillPart));
}

function currentAgentFromMessages(messages: ReadonlyArray<{ info?: object }>) {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const info = messages[index]?.info;
    const agent = info && "agent" in info ? info.agent : undefined;
    if (typeof agent === "string" && agent) return agent;
  }
  return undefined;
}

function sessionIDFromMessages(messages: ReadonlyArray<{ info?: { sessionID?: string }; parts?: Array<{ sessionID?: string }> }>) {
  for (const message of messages) {
    if (typeof message.info?.sessionID === "string" && message.info.sessionID) return message.info.sessionID;
    for (const part of message.parts ?? []) {
      if (typeof part.sessionID === "string" && part.sessionID) return part.sessionID;
    }
  }
  return undefined;
}

async function unwrap<T>(promise: Promise<unknown>, label: string): Promise<T> {
  const response = await promise;
  const envelope = typeof response === "object" && response !== null ? response as Record<string, unknown> : undefined;
  if (envelope && "error" in envelope && envelope.error !== undefined) throw new Error(`${label} failed`);
  if (envelope && "data" in envelope) return envelope.data as T;
  return response as T;
}

export default { id, server } satisfies PluginModule;
