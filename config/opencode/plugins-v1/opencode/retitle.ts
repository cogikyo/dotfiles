import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import type { Part } from "@opencode-ai/sdk";
import { errorMessage } from "../shared/error.ts";
import { type Client, create, prompt, type Reply, session, unwrap } from "../shared/opencode.ts";
import { modelFromString, type NamingModel } from "./media-context/naming.ts";

const id = "opencode-retitle";

const AGENT = "title";
const MESSAGES = 6;
const MESSAGE_CHARS = 600;
const TOTAL_CHARS = 3_000;
const TRIGGER = 2;
const COOLDOWN_MS = 5 * 60_000;
const TIMEOUT_MS = 30_000;
const MAX_CHARS = 40;
const INJECTED = "prt_media_context_";

const SYSTEM = "You write terse session titles. Return only the title and never call tools.";

type State = { count: number; recent: string[]; wrote: number; running: boolean };

const server: Plugin = async ({ client }) => {
  const states = new Map<string, State>();
  const hidden = new Set<string>();
  let model: NamingModel | undefined;

  const idle = async (sessionID: string) => {
    const state = states.get(sessionID);
    if (!state || state.running || state.count < TRIGGER) return;
    if (Date.now() - state.wrote < COOLDOWN_MS) return;
    state.running = true;
    try {
      const recent = state.recent.slice();
      state.count = 0;
      await retitle(client, sessionID, recent, model, hidden, () => (state.wrote = Date.now()));
    } catch (error) {
      console.warn(`${id}: retitle ${sessionID} failed: ${errorMessage(error)}`);
    } finally {
      state.running = false;
    }
  };

  return {
    config: async (cfg) => {
      model = cfg.small_model ? modelFromString(cfg.small_model) : undefined;
    },
    "chat.message": async (input, output) => {
      if (input.agent === AGENT || hidden.has(input.sessionID)) return;
      const text = userText(output.parts);
      if (!text) return;
      const state = states.get(input.sessionID) ?? { count: 0, recent: [], wrote: 0, running: false };
      state.count++;
      state.recent = [...state.recent, text].slice(-MESSAGES);
      states.set(input.sessionID, state);
    },
    event: async ({ event }) => {
      if (event.type === "session.deleted") states.delete(event.properties.info.id);
      if (event.type === "session.idle") await idle(event.properties.sessionID);
      if (event.type === "session.status" && event.properties.status.type === "idle")
        await idle(event.properties.sessionID);
    },
  };
};

/** Keeps top-level session titles at 2-4 words that follow the current work, leaving all-caps titles pinned. */
export default { id, server } satisfies PluginModule;

function userText(parts: Part[]) {
  return parts
    .flatMap((part) =>
      part.type === "text" && !part.synthetic && !part.ignored && !part.id.startsWith(INJECTED) ? [part.text] : [],
    )
    .join("\n")
    .replace(/\s+/g, " ")
    .trim()
    .slice(0, MESSAGE_CHARS);
}

function pinned(title: string) {
  return /\p{L}/u.test(title) && !/\p{Ll}/u.test(title);
}

async function retitle(
  client: Client,
  sessionID: string,
  recent: string[],
  model: NamingModel | undefined,
  hidden: Set<string>,
  wrote: () => void,
) {
  const label = `${id} read session ${sessionID}`;
  const info = await session(client, sessionID, { label });
  if (info.parentID || info.agent === AGENT || pinned(info.title)) return;
  if (!model) throw new Error("OpenCode small_model unavailable");

  const raw = await generate(client, info.title, recent, model, hidden);
  const title = clean(raw);
  if (!title) return;

  const current = await session(client, sessionID, { label });
  if (pinned(current.title) || current.title.toLowerCase() === title.toLowerCase()) return;
  await unwrap(
    client.session.update({ path: { id: sessionID }, body: { title } }),
    `${id} update session ${sessionID}`,
  );
  wrote();
}

function request(title: string, recent: string[]) {
  const kept: string[] = [];
  let total = 0;
  for (const text of recent.toReversed()) {
    if (total + text.length > TOTAL_CHARS) break;
    kept.unshift(text);
    total += text.length;
  }
  const lines = kept.map((text, index) => `${index + 1}. ${text}`).join("\n");
  return `Current title: ${title}

Recent user messages, oldest first:
<messages>
${lines}
</messages>

Write a 2-4 word title for what this session is working on now, in the language of the user messages.
If the current title still fits, reply with the current title unchanged.
Reply with the title only.`;
}

async function generate(client: Client, title: string, recent: string[], model: NamingModel, hidden: Set<string>) {
  const temp = await create(
    client,
    { title: "retitle", agent: AGENT, model: { id: model.modelID, providerID: model.providerID } },
    { label: `${id} create temporary session` },
  );
  hidden.add(temp.id);
  let reply: Promise<Reply> | undefined;
  try {
    reply = prompt(
      client,
      temp.id,
      {
        agent: AGENT,
        model,
        system: SYSTEM,
        tools: {},
        parts: [{ type: "text", text: request(title, recent) }],
      },
      { label: `${id} generate title` },
    );
    const response = await timeout(reply);
    return response.parts.flatMap((part) => (part.type === "text" ? [part.text] : [])).join("\n");
  } finally {
    try {
      await client.session.delete({ path: { id: temp.id } });
    } finally {
      const release = () => hidden.delete(temp.id);
      if (reply) void reply.then(release, release);
      else release();
    }
  }
}

async function timeout<T>(promise: Promise<T>) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      promise,
      new Promise<never>((_, reject) => {
        timer = setTimeout(() => reject(new Error(`title request timed out after ${TIMEOUT_MS}ms`)), TIMEOUT_MS);
      }),
    ]);
  } finally {
    clearTimeout(timer);
  }
}

function clean(raw: string) {
  const line =
    raw
      .replace(/<think>[\s\S]*?<\/think>/g, "")
      .split("\n")
      .map((value) => value.trim())
      .find(Boolean) ?? "";
  const title = line
    .replace(/^#+\s*/, "")
    .replace(/^title:\s*/i, "")
    .replace(/[*`]/g, "")
    .replace(/^["'“”‘’«»]+|["'“”‘’«».,;:!?…]+$/g, "")
    .replace(/\s+/g, " ")
    .trim();
  const words = title.split(" ").length;
  if (!title || words < 2 || words > 4 || title.length > MAX_CHARS) return undefined;
  if (pinned(title)) return undefined;
  return title;
}
