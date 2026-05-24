import type { PluginOptions } from "@opencode-ai/plugin";
import { promises as fs } from "node:fs";
import os from "node:os";
import path from "node:path";
import { updateImageName, type MediaRegistryEntry } from "./registry";

const DEFAULT_OPTIONS: ImageNameOptions = {
  enabled: true,
  model: "openai/gpt-5.4-mini",
  timeoutMs: 30_000,
  maxBytes: 8 * 1024 * 1024,
  concurrency: 1,
};
const PROMPT = `Name this image for a developer sidebar and file alias.
Return only 1-3 short concrete words.
Do not include a file extension, quotes, markdown, or a sentence.
Prefer visible subject and role over generic words like image or screenshot.`;
const STOP_WORDS = new Set(["a", "an", "the", "image", "photo", "picture", "screenshot"]);

type ImageNameOptions = {
  enabled: boolean;
  model: string;
  timeoutMs: number;
  maxBytes: number;
  concurrency: number;
};

type Job = {
  sessionID: string;
  handle: string;
  path: string;
  mime: string;
};

export function createImageNamer(options: PluginOptions | undefined) {
  const config = parseImageNameOptions(options);
  const pending = new Map<string, Job>();
  const running = new Set<string>();
  const readySessions = new Set<string>();
  let active = 0;

  const startNext = () => {
    if (!config.enabled) return;
    while (active < config.concurrency) {
      const next = firstPendingJob(pending, running, readySessions);
      if (!next) return;

      const [key, job] = next;
      pending.delete(key);
      running.add(key);
      active++;
      void nameImage(job, config).catch(() => undefined).finally(() => {
        running.delete(key);
        active--;
        startNext();
      });
    }
  };

  return {
    enqueue(entry: MediaRegistryEntry) {
      if (!config.enabled || entry.kind !== "image" || entry.name) return;
      const key = jobKey(entry.sessionID, entry.handle);
      if (running.has(key)) return;
      pending.set(key, {
        sessionID: entry.sessionID,
        handle: entry.handle,
        path: entry.path,
        mime: entry.mime,
      });
      readySessions.delete(entry.sessionID);
    },
    drain(sessionID: string) {
      if (!config.enabled) return;
      readySessions.add(sessionID);
      startNext();
    },
    clear(sessionID: string) {
      readySessions.delete(sessionID);
      for (const [key, job] of pending) {
        if (job.sessionID === sessionID) pending.delete(key);
      }
    },
  };
}

function parseImageNameOptions(options: PluginOptions | undefined): ImageNameOptions {
  const root = objectOption(options?.imageNames);
  return {
    enabled: booleanOption(root?.enabled, DEFAULT_OPTIONS.enabled),
    model: stringOption(root?.model, DEFAULT_OPTIONS.model),
    timeoutMs: integerOption(root?.timeoutMs, DEFAULT_OPTIONS.timeoutMs, 1_000, 120_000),
    maxBytes: integerOption(root?.maxBytes, DEFAULT_OPTIONS.maxBytes, 64 * 1024, 20 * 1024 * 1024),
    concurrency: integerOption(root?.concurrency, DEFAULT_OPTIONS.concurrency, 1, 3),
  };
}

function jobKey(sessionID: string, handle: string) {
  return `${sessionID}:${handle}`;
}

function firstPendingJob(pending: Map<string, Job>, running: Set<string>, readySessions: Set<string>) {
  for (const item of pending) {
    if (!running.has(item[0]) && readySessions.has(item[1].sessionID)) return item;
  }
  return undefined;
}

async function nameImage(job: Job, options: ImageNameOptions) {
  const raw = await requestImageName(job.path, job.mime, options);
  const name = slugFromModelText(raw);
  if (!name) return;
  updateImageName(job.sessionID, job.handle, name, "openai-responses");
}

async function requestImageName(filePath: string, mime: string, options: ImageNameOptions) {
  const model = openAIModelID(options.model);
  if (!model) throw new Error("media-context image naming only supports openai/* models");

  const bearer = await openAIBearerToken();
  if (!bearer) throw new Error("OpenAI auth unavailable");

  const imageURL = await dataURL(filePath, mime, options.maxBytes);
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), options.timeoutMs);
  try {
    const response = await fetch("https://api.openai.com/v1/responses", {
      method: "POST",
      signal: controller.signal,
      headers: {
        Authorization: `Bearer ${bearer}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        model,
        store: false,
        max_output_tokens: 32,
        input: [
          {
            role: "user",
            content: [
              { type: "input_text", text: PROMPT },
              { type: "input_image", image_url: imageURL, detail: "low" },
            ],
          },
        ],
      }),
    });
    if (!response.ok) throw new Error(`OpenAI image naming HTTP ${response.status}`);
    return outputText(await response.json());
  } finally {
    clearTimeout(timeout);
  }
}

function openAIModelID(value: string) {
  const slash = value.indexOf("/");
  if (slash < 0) return value;
  return value.slice(0, slash) === "openai" ? value.slice(slash + 1) : undefined;
}

async function openAIBearerToken() {
  const auth = await openCodeOpenAIAuth();
  if (auth?.type === "api" && auth.key) return auth.key;
  if (auth?.type === "oauth" && auth.access && !isExpired(auth.expires)) return auth.access;

  const env = process.env.OPENAI_API_KEY?.trim();
  return env || undefined;
}

type OpenAIAuth = {
  type?: string;
  key?: string;
  access?: string;
  expires?: number;
};

async function openCodeOpenAIAuth(): Promise<OpenAIAuth | undefined> {
  try {
    const raw = await fs.readFile(path.join(openCodeDataDir(), "auth.json"), "utf8");
    const parsed = JSON.parse(raw) as { openai?: OpenAIAuth };
    return parsed.openai;
  } catch {
    return undefined;
  }
}

function openCodeDataDir() {
  const xdg = process.env.XDG_DATA_HOME?.trim();
  if (xdg) return path.join(path.resolve(xdg), "opencode");
  return path.join(os.homedir(), ".local", "share", "opencode");
}

function isExpired(expires: number | undefined) {
  return typeof expires === "number" && expires > 0 && expires <= Math.floor(Date.now() / 1000) + 60;
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

function outputText(payload: unknown) {
  const candidate = payload as { output_text?: unknown; output?: Array<{ content?: Array<{ text?: unknown }> }> };
  if (typeof candidate.output_text === "string") return candidate.output_text;
  return candidate.output?.flatMap((item) => item.content ?? []).map((item) => item.text).filter((text): text is string => typeof text === "string").join(" ") ?? "";
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

function stringOption(value: unknown, fallback: string) {
  return typeof value === "string" && value.trim() ? value.trim() : fallback;
}

function integerOption(value: unknown, fallback: number, min: number, max: number) {
  if (typeof value !== "number" || !Number.isFinite(value)) return fallback;
  return Math.max(min, Math.min(max, Math.trunc(value)));
}
