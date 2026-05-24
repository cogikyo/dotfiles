import type { PluginOptions } from "@opencode-ai/plugin";
import { promises as fs } from "node:fs";
import path from "node:path";
import { updateImageName, type MediaRegistryEntry } from "./registry";

const DEFAULT_OPTIONS: ImageNameOptions = {
  enabled: true,
  timeoutMs: 30_000,
  maxBytes: 8 * 1024 * 1024,
  concurrency: 1,
};
const PROMPT = `Name this image for a developer sidebar and file alias.
Return only 1-3 short concrete words.
Do not include a file extension, quotes, markdown, or a sentence.
Prefer visible subject and role over generic words like image or screenshot.`;
const SYSTEM = "You generate terse lowercase-ish image aliases. Return only the alias words and never call tools.";
const STOP_WORDS = new Set(["a", "an", "the", "image", "photo", "picture", "screenshot"]);

type ImageNameOptions = {
  enabled: boolean;
  timeoutMs: number;
  maxBytes: number;
  concurrency: number;
};

export type NamingModel = {
  providerID: string;
  modelID: string;
};

type Job = {
  sessionID: string;
  handle: string;
  path: string;
  mime: string;
  model: NamingModel;
};

type OpenCodeClient = {
  session: {
    create(input: { body: { title: string } }): unknown;
    prompt(input: { path: { id: string }; body: SessionPromptBody }): unknown;
    delete(input: { path: { id: string } }): unknown;
  };
};

type SessionPromptBody = {
  model: NamingModel;
  system: string;
  tools: Record<string, never>;
  parts: Array<{ type: "text"; text: string } | { type: "file"; mime: string; url: string }>;
};

type CreateImageNamerInput = {
  client: OpenCodeClient;
  options?: PluginOptions;
  ignoredSessions: Set<string>;
};

export function createImageNamer(input: CreateImageNamerInput) {
  const config = parseImageNameOptions(input.options);
  const pending = new Map<string, Job>();
  const running = new Set<string>();
  let active = 0;
  let defaultModel: NamingModel | undefined;

  const startNext = () => {
    if (!config.enabled) return;
    while (active < config.concurrency) {
      const next = firstPendingJob(pending, running);
      if (!next) return;

      const [key, job] = next;
      pending.delete(key);
      running.add(key);
      active++;
      void nameImage(job, config, input.client, input.ignoredSessions).catch((error) => {
        logNameFailure(job, "name", error);
      }).finally(() => {
        running.delete(key);
        active--;
        startNext();
      });
    }
  };

  return {
    setDefaultModel(model: NamingModel | undefined) {
      defaultModel = model;
    },
    enqueue(entry: MediaRegistryEntry, model: NamingModel | undefined) {
      if (!config.enabled || entry.kind !== "image" || entry.name) return;
      const job = {
        sessionID: entry.sessionID,
        handle: entry.handle,
        path: entry.path,
        mime: entry.mime,
        model: model ?? defaultModel,
      };
      if (!job.model) {
        logNameFailure({ ...job, model: { providerID: "unknown", modelID: "unknown" } }, "model", new Error("OpenCode model unavailable for image naming"));
        return;
      }
      const key = jobKey(entry.sessionID, entry.handle);
      if (pending.has(key) || running.has(key)) return;
      pending.set(key, { ...job, model: job.model });
      startNext();
    },
    drain(sessionID: string) {
      if (!config.enabled) return;
      startNext();
    },
    clear(sessionID: string) {
      for (const [key, job] of pending) {
        if (job.sessionID === sessionID) pending.delete(key);
      }
    },
  };
}

export function modelFromValue(value: unknown): NamingModel | undefined {
  if (typeof value === "string") return modelFromString(value);
  const candidate = value as { providerID?: unknown; modelID?: unknown; provider?: unknown; model?: unknown } | undefined;
  if (typeof candidate?.providerID === "string" && typeof candidate.modelID === "string") {
    return cleanModel(candidate.providerID, candidate.modelID);
  }
  if (typeof candidate?.provider === "string" && typeof candidate.model === "string") {
    return cleanModel(candidate.provider, candidate.model);
  }
  return undefined;
}

export function modelFromChatPayload(input: unknown, output: unknown) {
  return firstModel(
    modelAt(output, ["message", "model"]),
    modelAt(output, ["model"]),
    modelAt(input, ["model"]),
    modelAt(input, ["message", "model"]),
    modelAt(input, ["session", "model"]),
  );
}

function modelAt(value: unknown, path: string[]) {
  let current = value;
  for (const key of path) {
    if (!current || typeof current !== "object") return undefined;
    current = (current as Record<string, unknown>)[key];
  }
  return modelFromValue(current);
}

function firstModel(...models: Array<NamingModel | undefined>) {
  return models.find(Boolean);
}

function modelFromString(value: string) {
  const clean = value.trim();
  const slash = clean.indexOf("/");
  if (slash <= 0 || slash === clean.length - 1) return undefined;
  return cleanModel(clean.slice(0, slash), clean.slice(slash + 1));
}

function cleanModel(providerID: string, modelID: string) {
  const provider = providerID.trim();
  const model = modelID.trim();
  return provider && model ? { providerID: provider, modelID: model } : undefined;
}

function parseImageNameOptions(options: PluginOptions | undefined): ImageNameOptions {
  const root = objectOption(options?.imageNames);
  return {
    enabled: booleanOption(root?.enabled, DEFAULT_OPTIONS.enabled),
    timeoutMs: integerOption(root?.timeoutMs, DEFAULT_OPTIONS.timeoutMs, 1_000, 120_000),
    maxBytes: integerOption(root?.maxBytes, DEFAULT_OPTIONS.maxBytes, 64 * 1024, 20 * 1024 * 1024),
    concurrency: integerOption(root?.concurrency, DEFAULT_OPTIONS.concurrency, 1, 3),
  };
}

function jobKey(sessionID: string, handle: string) {
  return `${sessionID}:${handle}`;
}

function firstPendingJob(pending: Map<string, Job>, running: Set<string>) {
  for (const item of pending) {
    if (!running.has(item[0])) return item;
  }
  return undefined;
}

async function nameImage(job: Job, options: ImageNameOptions, client: OpenCodeClient, ignoredSessions: Set<string>) {
  let raw: string;
  try {
    raw = await requestImageName(job, options, client, ignoredSessions);
  } catch (error) {
    throw stageError("request", error);
  }

  const name = slugFromModelText(raw);
  if (!name) throw stageError("sanitize", new Error("empty image name from model"));

  const entry = updateImageName(job.sessionID, job.handle, name, modelSource(job.model));
  if (!entry) throw stageError("registry", new Error("registry rename failed"));
}

async function requestImageName(job: Job, options: ImageNameOptions, client: OpenCodeClient, ignoredSessions: Set<string>) {
  const imageURL = await dataURL(job.path, job.mime, options.maxBytes);
  const session = await client.session.create({ body: { title: "media-context image naming" } });
  const sessionID = sessionIDFromCreateResponse(session);
  if (!sessionID) throw new Error("temporary OpenCode session id unavailable");

  ignoredSessions.add(sessionID);
  let prompt: Promise<unknown> | undefined;

  try {
    prompt = Promise.resolve(client.session.prompt({
      path: { id: sessionID },
      body: {
        model: job.model,
        system: SYSTEM,
        tools: {},
        parts: [
          { type: "text", text: PROMPT },
          { type: "file", mime: imageMime(job.mime, job.path), url: imageURL },
        ],
      },
    }));

    const response = await withTimeout(prompt, options.timeoutMs);
    return assistantText(response);
  } finally {
    try {
      await Promise.resolve(client.session.delete({ path: { id: sessionID } }));
    } finally {
      clearIgnoredSession(ignoredSessions, sessionID, prompt);
    }
  }
}

function clearIgnoredSession(ignoredSessions: Set<string>, sessionID: string, prompt: Promise<unknown> | undefined) {
  if (!prompt) {
    ignoredSessions.delete(sessionID);
    return;
  }

  const timeout = setTimeout(() => ignoredSessions.delete(sessionID), 5_000);
  void prompt.finally(() => {
    clearTimeout(timeout);
    ignoredSessions.delete(sessionID);
  }).catch(() => undefined);
}

function sessionIDFromCreateResponse(value: unknown): string | undefined {
  const candidate = value as { id?: unknown; session?: { id?: unknown }; data?: { id?: unknown; session?: { id?: unknown } } } | undefined;
  const id = candidate?.id ?? candidate?.session?.id ?? candidate?.data?.id ?? candidate?.data?.session?.id;
  return typeof id === "string" && id ? id : undefined;
}

async function withTimeout<T>(promise: Promise<T>, timeoutMs: number) {
  let timeout: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      promise,
      new Promise<never>((_, reject) => {
        timeout = setTimeout(() => reject(new Error(`OpenCode image naming timed out after ${timeoutMs}ms`)), timeoutMs);
      }),
    ]);
  } finally {
    if (timeout) clearTimeout(timeout);
  }
}

function modelSource(model: NamingModel) {
  return `opencode:${model.providerID}/${model.modelID}`.slice(0, 80);
}

function logNameFailure(job: Job, stage: string, error: unknown) {
  const label = error instanceof NameStageError ? error.stage : stage;
  console.warn(`[media-context] image naming failed session=${safeID(job.sessionID)} handle=${job.handle} model=${safeModel(job.model)} stage=${label} error=${errorMessage(error)}`);
}

class NameStageError extends Error {
  constructor(readonly stage: string, cause: unknown) {
    super(errorMessage(cause));
  }
}

function stageError(stage: string, error: unknown) {
  return new NameStageError(stage, error);
}

function safeID(value: string) {
  return value.replace(/[^a-zA-Z0-9_.-]+/g, "_").slice(0, 80) || "session";
}

function safeModel(model: NamingModel) {
  return `${safeID(model.providerID)}/${safeID(model.modelID)}`;
}

function errorMessage(error: unknown) {
  return sanitizeErrorMessage(error instanceof Error ? error.message : String(error));
}

function sanitizeErrorMessage(message: string) {
  return message
    .replace(/(^|[\s'"])(\/(?:[^\s'",)]+\/?)+)/g, "$1[path]")
    .replace(/\b(?:sk|sess)-[a-zA-Z0-9_-]+/g, "[token]");
}

async function dataURL(filePath: string, mime: string, maxBytes: number) {
  const handle = await fs.open(filePath, "r");
  try {
    const stat = await handle.stat();
    if (!stat.isFile() || stat.size <= 0 || stat.size > maxBytes) throw new Error("image is too large for naming");
    const buffer = await handle.readFile();
    return `data:${imageMime(mime, filePath)};base64,${buffer.toString("base64")}`;
  } finally {
    await handle.close();
  }
}

function imageMime(mime: string, filePath: string) {
  if (mime.startsWith("image/")) return mime;
  const ext = path.extname(filePath).toLowerCase();
  if (ext === ".jpg" || ext === ".jpeg") return "image/jpeg";
  if (ext === ".gif") return "image/gif";
  if (ext === ".webp") return "image/webp";
  if (ext === ".png") return "image/png";
  return "image/png";
}

function assistantText(payload: unknown) {
  return assistantTextParts(payload).join(" ");
}

function assistantTextParts(value: unknown): string[] {
  if (Array.isArray(value)) return value.flatMap(assistantTextParts);
  if (!value || typeof value !== "object") return [];

  const candidate = value as Record<string, unknown>;
  const text = candidate.type === "text" && typeof candidate.text === "string" ? [candidate.text] : [];
  if (typeof candidate.output_text === "string") text.push(candidate.output_text);
  for (const key of ["parts", "content", "output"]) {
    const items = candidate[key];
    if (Array.isArray(items)) text.push(...items.flatMap(assistantTextParts));
  }
  for (const key of ["message", "assistant", "data"]) {
    text.push(...assistantTextParts(candidate[key]));
  }
  return text;
}

function slugFromModelText(value: string) {
  const words = value
    .toLowerCase()
    .replace(/\.[a-z0-9]{1,8}$/i, "")
    .match(/[a-z0-9]+/g)
    ?.filter((word) => !STOP_WORDS.has(word))
    .slice(0, 3);
  const slug = words?.join("-").slice(0, 48).replace(/^-+|-+$/g, "");
  return slug || undefined;
}

function objectOption(value: unknown) {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;
}

function booleanOption(value: unknown, fallback: boolean) {
  return typeof value === "boolean" ? value : fallback;
}

function integerOption(value: unknown, fallback: number, min: number, max: number) {
  if (typeof value !== "number" || !Number.isFinite(value)) return fallback;
  return Math.max(min, Math.min(max, Math.trunc(value)));
}
