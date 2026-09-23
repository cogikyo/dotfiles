import type { Plugin, PluginModule } from "@opencode-ai/plugin";
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
import { createImageNamer, modelFromValue } from "./naming";
import { record } from "../../shared/record.ts";

const id = "opencode-media-context-prompt";
let partIDCounter = 0;

/**
 * Uses the `config`, `chat.message`, and `experimental.session.compacting` server hooks.
 * The `event` hook drains naming on `session.idle` or idle `session.status` and clears work on `session.deleted`.
 */
const server: Plugin = async (ctx, options) => {
  const internalSessions = new Set<string>();
  const namer = createImageNamer({ client: ctx.client, options, ignoredSessions: internalSessions });

  return {
    config: async (cfg) => {
      namer.setDefaultModel(modelFromValue(cfg.small_model));
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
      const { type, properties } = event;
      const fields = record(properties);
      const sessionID = fields?.sessionID || record(fields?.info)?.id;
      if (typeof sessionID !== "string" || !sessionID) return;

      if (type === "session.idle" || (type === "session.status" && record(fields?.status)?.type === "idle")) {
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
  const candidate = record(part);
  return candidate?.type === "text" && typeof candidate.text === "string";
}

function isDefined<T>(value: T | undefined): value is T {
  return value !== undefined;
}

/** Server plugin entrypoint for media registration and prompt context. */
export default { id, server } satisfies PluginModule;
