import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import type { Part } from "@opencode-ai/sdk";
import { isExistingFile, videoPathParts } from "./files";
import {
  listSessionMedia,
  mediaFilePartForEntry,
  mediaPart,
  mediaReference,
  registerSessionMedia,
  resolveMediaReferences,
} from "./registry";
import type { MediaRegistryEntry } from "./store";
import { createImageNamer, modelFromString } from "./naming";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Server plugin: register and resolve session media                                             │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

const id = "opencode-media-context-prompt";

let partIDCounter = 0;

const server: Plugin = async (ctx, options) => {
  const internalSessions = new Set<string>();
  const namer = createImageNamer({ client: ctx.client, options, ignoredSessions: internalSessions });

  return {
    config: async (cfg) => {
      namer.setDefaultModel(cfg.small_model ? modelFromString(cfg.small_model) : undefined);
    },
    "chat.message": async (input, output) => {
      const sessionID = input.sessionID;
      if (internalSessions.has(sessionID)) return;

      const messageID = output.message.id;
      const text = userText(output.parts);
      const registered: MediaRegistryEntry[] = [];

      for (const part of output.parts.map(mediaPart).filter(isDefined)) {
        const entry = registerSessionMedia(sessionID, messageID, part);
        if (entry) {
          registered.push(entry);
          if (entry.kind === "image" && !entry.name) namer.enqueue(entry);
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
        output.parts.push(
          textPart(sessionID, messageID, `Resolved media handles for provider context: ${formatHandles(injectable)}.`),
        );
      }

      if (localOnly.length > 0) {
        output.parts.push(
          textPart(
            sessionID,
            messageID,
            `Video handles are local-only and were not sent to the provider: ${formatHandles(localOnly)}.`,
          ),
        );
      }
    },
    "experimental.session.compacting": async (input, output) => {
      const entries = listSessionMedia(input.sessionID).filter((entry) => isExistingFile(entry.path));
      if (entries.length === 0) return;
      output.context.push(`Media references available after compaction: ${formatHandles(entries)}.`);
    },
    event: async ({ event }) => {
      if (event.type === "session.deleted") namer.clear(event.properties.info.id);
    },
  };
};

/** Server plugin that registers session media, resolves image handles for provider context, and keeps video handles local. */
export default { id, server } satisfies PluginModule;

// ├─ Synthetic parts ─────────────────────────────────────────────────────────────────────────────┤

function textPart(sessionID: string, messageID: string, text: string) {
  return { id: pluginPartID("note"), sessionID, messageID, type: "text" as const, text };
}

function pluginPartID(label: string) {
  const safeLabel = label.replace(/[^a-zA-Z0-9_-]+/g, "_").replace(/^_+|_+$/g, "") || "part";
  const serial = partIDCounter++;
  return `prt_media_context_${safeLabel}_${Date.now().toString(36)}_${serial.toString(36)}`;
}

function userText(parts: Part[]) {
  return parts.map((part) => (part.type === "text" ? part.text : "")).join("\n");
}

function formatHandles(entries: MediaRegistryEntry[]) {
  return entries.map((entry) => `${entry.kind === "video" ? "V" : "I"} ${mediaReference(entry)}`).join(", ");
}

function isDefined<T>(value: T | undefined): value is T {
  return value !== undefined;
}
