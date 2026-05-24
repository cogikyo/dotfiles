import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import {
  isExistingFile,
  listSessionMedia,
  mediaFilePartForEntry,
  mediaPart,
  mediaReference,
  registerSessionMedia,
  resolveMediaReferences,
  videoPathParts,
  type MediaRegistryEntry,
} from "./registry";
import { createImageNamer } from "./naming";

const id = "opencode-media-context-prompt";
let partIDCounter = 0;

const server: Plugin = async (_input, options) => {
  const namer = createImageNamer(options);

  return {
    "chat.message": async (input, output) => {
      const sessionID = input.sessionID;
      const messageID = output.message.id;
      const text = userText(output.parts);
      const registered: MediaRegistryEntry[] = [];

      for (const part of output.parts.map(mediaPart).filter(isDefined)) {
        const entry = registerSessionMedia(sessionID, messageID, part);
        if (entry) {
          registered.push(entry);
          if (entry.kind === "image" && !entry.name && entry.createdAt === entry.updatedAt) namer.enqueue(entry);
        }
      }

      for (const part of videoPathParts(text)) {
        const entry = registerSessionMedia(sessionID, messageID, {
          ...part,
          id: pluginPartID("detected-video"),
          sessionID,
          messageID,
        });
        if (entry) registered.push(entry);
      }

      const resolved = resolveMediaReferences(sessionID, text);
      const injectable = resolved.filter((entry) => entry.kind === "image");
      const localOnly = resolved.filter((entry) => entry.kind === "video");

      if (registered.length > 0) {
        output.parts.push(textPart(sessionID, messageID, `Media references registered: ${formatHandles(registered)}.`));
      }

      if (injectable.length > 0) {
        for (const [index, entry] of injectable.entries()) {
          output.parts.push(mediaFilePartForEntry(entry, sessionID, messageID, index));
        }
        output.parts.push(textPart(sessionID, messageID, `Resolved media handles for provider context: ${formatHandles(injectable)}.`));
      }

      if (localOnly.length > 0) {
        output.parts.push(textPart(sessionID, messageID, `Video handles are local-only and were not sent to the provider: ${formatHandles(localOnly)}.`));
      }
    },
    "experimental.session.compacting": async (input, output) => {
      const entries = listSessionMedia(input.sessionID).filter((entry) => isExistingFile(entry.path));
      if (entries.length === 0) return;
      output.context.push(`Media references available after compaction: ${formatHandles(entries)}.`);
    },
    event: async ({ event }) => {
      const { type, properties } = event as { type?: string; properties?: { sessionID?: string; status?: { type?: string }; info?: { id?: string } } };
      const sessionID = properties?.sessionID || properties?.info?.id;
      if (!sessionID) return;

      if (type === "session.idle" || (type === "session.status" && properties?.status?.type === "idle")) {
        namer.drain(sessionID);
      }
      if (type === "session.deleted") namer.clear(sessionID);
    },
  };
};

function textPart(sessionID: string, messageID: string, text: string) {
  return { id: pluginPartID("note"), sessionID, messageID, type: "text" as const, text };
}

function pluginPartID(label: string) {
  const safeLabel = label.replace(/[^a-zA-Z0-9_-]+/g, "_").replace(/^_+|_+$/g, "") || "part";
  const serial = partIDCounter++;
  return `prt_media_context_${safeLabel}_${Date.now().toString(36)}_${serial.toString(36)}`;
}

function userText(parts: unknown[]) {
  return parts.map((part) => (isTextPart(part) ? part.text : "")).join("\n");
}

function formatHandles(entries: MediaRegistryEntry[]) {
  return entries.map((entry) => `${entry.kind === "video" ? "V" : "I"} ${mediaReference(entry)}`).join(", ");
}

function isTextPart(part: unknown): part is { type: "text"; text: string } {
  const candidate = part as { type?: unknown; text?: unknown } | undefined;
  return candidate?.type === "text" && typeof candidate.text === "string";
}

function isDefined<T>(value: T | undefined): value is T {
  return value !== undefined;
}

export default { id, server } satisfies PluginModule;
