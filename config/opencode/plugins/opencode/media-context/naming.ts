import type { PluginOptions } from "@opencode-ai/plugin";
import { promises as fs } from "node:fs";
import path from "node:path";
import { z } from "zod";
import { errorMessage as describe } from "../../shared/error.ts";
import { create, prompt as ask, type Client, type Reply } from "../../shared/opencode.ts";
import { updateImageName } from "./registry";
import type { MediaRegistryEntry } from "./store";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Image naming                                                                                  │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

/** Model selected for temporary image naming. */
export type NamingModel = {
  providerID: string;
  modelID: string;
};

// ├─ Naming options ──────────────────────────────────────────────────────────────────────────────┤

type ImageNameOptions = {
  enabled: boolean;
  timeoutMs: number;
  maxBytes: number;
  concurrency: number;
};

const DEFAULT_OPTIONS: ImageNameOptions = {
  enabled: true,
  timeoutMs: 30_000,
  maxBytes: 8 * 1024 * 1024,
  concurrency: 1,
};

function bounded(fallback: number, min: number, max: number) {
  return z
    .number()
    .catch(fallback)
    .transform((value) => Math.max(min, Math.min(max, Math.trunc(value))));
}

const Options = z
  .object({
    imageNames: z
      .object({
        enabled: z.boolean().catch(DEFAULT_OPTIONS.enabled),
        timeoutMs: bounded(DEFAULT_OPTIONS.timeoutMs, 1_000, 120_000),
        maxBytes: bounded(DEFAULT_OPTIONS.maxBytes, 64 * 1024, 20 * 1024 * 1024),
        concurrency: bounded(DEFAULT_OPTIONS.concurrency, 1, 3),
      })
      .catch(DEFAULT_OPTIONS),
  })
  .catch({ imageNames: DEFAULT_OPTIONS })
  .transform((options): ImageNameOptions => options.imageNames);

// ├─ Queue and concurrency ───────────────────────────────────────────────────────────────────────┤

type Job = {
  sessionID: string;
  handle: string;
  path: string;
  mime: string;
  model: NamingModel;
};

type CreateImageNamerInput = {
  client: Client;
  options?: PluginOptions;
  ignoredSessions: Set<string>;
};

/** Queues unnamed images for temporary naming sessions and saves accepted aliases; pending jobs can be cleared per session. */
export function createImageNamer(input: CreateImageNamerInput) {
  const config = Options.parse(input.options ?? {});
  const pending = new Map<string, Job>();
  const running = new Set<string>();
  let active = 0;
  let defaultModel: NamingModel | undefined;

  const startNext = () => {
    if (!config.enabled) return;
    while (active < config.concurrency) {
      const next = pending.entries().next().value;
      if (!next) return;

      const [key, job] = next;
      pending.delete(key);
      running.add(key);
      active++;
      void nameImage(job, config, input.client, input.ignoredSessions)
        .catch((error) => {
          logNameFailure(job, "name", error);
        })
        .finally(() => {
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
    enqueue(entry: MediaRegistryEntry) {
      if (!config.enabled || entry.kind !== "image" || entry.name) return;
      if (!defaultModel) {
        logNameFailure(
          {
            sessionID: entry.sessionID,
            handle: entry.handle,
            path: entry.path,
            mime: entry.mime,
            model: { providerID: "unknown", modelID: "unknown" },
          },
          "model",
          new Error("OpenCode small_model unavailable for image naming"),
        );
        return;
      }
      const key = jobKey(entry.sessionID, entry.handle);
      if (pending.has(key) || running.has(key)) return;
      pending.set(key, {
        sessionID: entry.sessionID,
        handle: entry.handle,
        path: entry.path,
        mime: entry.mime,
        model: defaultModel,
      });
      startNext();
    },
    clear(sessionID: string) {
      for (const [key, job] of pending) {
        if (job.sessionID === sessionID) pending.delete(key);
      }
    },
  };
}

function jobKey(sessionID: string, handle: string) {
  return `${sessionID}:${handle}`;
}

// ├─ Model selection ─────────────────────────────────────────────────────────────────────────────┤

/** Splits a `provider/model` id at its first slash, or returns undefined for an invalid id. */
export function modelFromString(value: string) {
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

// ├─ Naming request ──────────────────────────────────────────────────────────────────────────────┤

const NAMING_AGENT = "title";
const PROMPT = `Name this image for a developer sidebar and file alias.
Return only 1-3 short concrete words.
Do not include a file extension, quotes, markdown, or a sentence.
Prefer visible subject and role over generic words like image or screenshot.`;
const SYSTEM = "You generate terse lowercase-ish image aliases. Return only the alias words and never call tools.";

async function nameImage(job: Job, options: ImageNameOptions, client: Client, ignoredSessions: Set<string>) {
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

async function requestImageName(job: Job, options: ImageNameOptions, client: Client, ignoredSessions: Set<string>) {
  const imageURL = await dataURL(job.path, job.mime, options.maxBytes);
  const session = await create(
    client,
    {
      title: "media-context image naming",
      agent: NAMING_AGENT,
      model: { id: job.model.modelID, providerID: job.model.providerID },
    },
    { label: "temporary OpenCode session" },
  );
  const sessionID = session.id;

  ignoredSessions.add(sessionID);
  let prompt: Promise<Reply> | undefined;

  try {
    prompt = ask(
      client,
      sessionID,
      {
        agent: NAMING_AGENT,
        model: job.model,
        system: SYSTEM,
        tools: {},
        parts: [
          { type: "text", text: PROMPT },
          { type: "file", mime: imageMime(job.mime, job.path), url: imageURL },
        ],
      },
      { label: "OpenCode image naming" },
    );

    // Keep naming sessions out of media discovery until the prompt settles or the grace period ends.
    const response = await withTimeout(prompt, options.timeoutMs);
    return response.parts.flatMap((part) => (part.type === "text" ? [part.text] : [])).join(" ");
  } finally {
    try {
      await client.session.delete({ path: { id: sessionID } });
    } finally {
      clearIgnoredSession(ignoredSessions, sessionID, prompt);
    }
  }
}

function modelSource(model: NamingModel) {
  return `opencode:${model.providerID}/${model.modelID}`.slice(0, 80);
}

// ├─ Temporary session lifecycle ─────────────────────────────────────────────────────────────────┤

function clearIgnoredSession(ignoredSessions: Set<string>, sessionID: string, prompt: Promise<unknown> | undefined) {
  if (!prompt) {
    ignoredSessions.delete(sessionID);
    return;
  }

  const timeout = setTimeout(() => ignoredSessions.delete(sessionID), 5_000);
  void prompt
    .finally(() => {
      clearTimeout(timeout);
      ignoredSessions.delete(sessionID);
    })
    .catch(() => undefined);
}

async function withTimeout<T>(promise: Promise<T>, timeoutMs: number) {
  let timeout: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      promise,
      new Promise<never>((_, reject) => {
        timeout = setTimeout(
          () => reject(new Error(`OpenCode image naming timed out after ${timeoutMs}ms`)),
          timeoutMs,
        );
      }),
    ]);
  } finally {
    if (timeout) clearTimeout(timeout);
  }
}

// ├─ Failure reporting ───────────────────────────────────────────────────────────────────────────┤

function logNameFailure(job: Job, stage: string, error: unknown) {
  const label = error instanceof NameStageError ? error.stage : stage;
  console.warn(
    `[media-context] image naming failed session=${safeID(job.sessionID)} handle=${job.handle} model=${safeModel(job.model)} stage=${label} error=${errorMessage(error)}`,
  );
}

class NameStageError extends Error {
  constructor(
    readonly stage: string,
    cause: unknown,
  ) {
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
  return sanitizeErrorMessage(describe(error));
}

function sanitizeErrorMessage(message: string) {
  return message
    .replace(/(^|[\s'"])(\/(?:[^\s'",)]+\/?)+)/g, "$1[path]")
    .replace(/\b(?:sk|sess)-[a-zA-Z0-9_-]+/g, "[token]");
}

// ├─ Image payload and alias text ────────────────────────────────────────────────────────────────┤

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

const STOP_WORDS = new Set(["a", "an", "the", "image", "photo", "picture", "screenshot"]);

function slugFromModelText(value: string) {
  const words = value
    .toLowerCase()
    .replace(/\.[a-z0-9]{1,8}$/i, "")
    .match(/[a-z0-9]+/g)
    ?.filter((word) => !STOP_WORDS.has(word))
    .slice(0, 3);
  const slug = words
    ?.join("-")
    .slice(0, 48)
    .replace(/^-+|-+$/g, "");
  return slug || undefined;
}
