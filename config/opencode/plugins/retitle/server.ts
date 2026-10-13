import { Plugin } from "@opencode/plugin";
import { Effect, Fiber } from "effect";
import { api, pinned } from "./rpc.ts";

const TRIGGER = 2;
const COOLDOWN_MS = 5 * 60_000;
const MESSAGES = 6;
const MESSAGE_CHARS = 600;
const TOTAL_CHARS = 3_000;
const MAX_CHARS = 36;
const MINOR = new Set([
  "a",
  "an",
  "and",
  "as",
  "at",
  "but",
  "by",
  "for",
  "in",
  "nor",
  "of",
  "on",
  "or",
  "the",
  "to",
  "vs",
  "via",
  "with",
]);

type State = { count: number; recent: string[]; wrote: number; running: boolean };

function request(title: string, recent: readonly string[], force: boolean) {
  const kept: string[] = [];
  let total = 0;
  for (const text of recent.toReversed()) {
    if (total + text.length > TOTAL_CHARS) break;
    kept.unshift(text);
    total += text.length;
  }
  return [
    `Current title: ${title}`,
    "Recent user messages, oldest first:",
    "<messages>",
    ...kept.map((text, index) => `${index + 1}. ${text}`),
    "</messages>",
    "Write a 2-4 word title in Title Case for what this session is working on now, in the language of the user messages.",
    ...(force ? [] : ["If the current title still fits, reply with the current title unchanged."]),
    "Reply with the title only.",
  ].join("\n");
}

function clean(raw: string) {
  const line =
    raw
      .replace(/<think>[\s\S]*?<\/think>/gu, "")
      .split("\n")
      .map((value) => value.trim())
      .find(Boolean) ?? "";
  const title = line
    .replace(/^#+\s*/u, "")
    .replace(/^title:\s*/iu, "")
    .replace(/[*`]/gu, "")
    .replace(/^["'“”‘’]+|["'“”‘’.]+$/gu, "")
    .trim();
  const cased = titleCase(title);
  return cased.length > MAX_CHARS ? `${cased.slice(0, MAX_CHARS - 1).trimEnd()}…` : cased;
}

function titleCase(title: string) {
  const words = title.split(" ");
  return words
    .map((word, index) => {
      if (/\p{Lu}/u.test(word)) return word;
      if (index > 0 && index < words.length - 1 && MINOR.has(word)) return word;
      return word.replace(/\p{Ll}/u, (letter) => letter.toUpperCase());
    })
    .join(" ");
}

export default Plugin.define({
  id: "retitle",
  async setup(ctx) {
    const fiber = Fiber.getCurrent();
    if (!fiber) throw new Error("retitle: setup started outside the plugin fiber");
    const log = Effect.runForkWith(fiber.context);
    const states = new Map<string, State>();
    const controller = new AbortController();

    const state = (sessionID: string) => {
      const current = states.get(sessionID) ?? { count: 0, recent: [], wrote: 0, running: false };
      states.set(sessionID, current);
      return current;
    };

    const generate = async (title: string, recent: readonly string[], force: boolean) => {
      const { data: agent } = await ctx.agent.get({ agentID: "title" });
      const { text } = await ctx.generate.text({ prompt: request(title, recent, force), model: agent.model });
      return clean(text);
    };

    const write = async (sessionID: string, title: string) => {
      await ctx.session.update({ sessionID, title });
      const current = state(sessionID);
      current.wrote = Date.now();
      current.count = 0;
      return { title };
    };

    const retitle = async (sessionID: string) => {
      const current = states.get(sessionID);
      if (!current || current.running || current.count < TRIGGER) return;
      if (Date.now() - current.wrote < COOLDOWN_MS) return;
      current.running = true;
      try {
        const session = await ctx.session.get({ sessionID });
        if (session.parentID || !session.title || pinned(session.title)) return;
        const recent = current.recent.slice();
        current.count = 0;
        const title = await generate(session.title, recent, false);
        if (!title) return;
        const latest = await ctx.session.get({ sessionID });
        if (!latest.title || pinned(latest.title) || latest.title.toLowerCase() === title.toLowerCase()) return;
        await write(sessionID, title);
      } finally {
        current.running = false;
      }
    };

    const manual = async (sessionID: string, recent: readonly string[], caps: boolean) => {
      const session = await ctx.session.get({ sessionID });
      if (session.parentID) throw new Error("retitle: child sessions keep their delegated titles");
      const title = await generate(session.title ?? "", recent, true);
      if (!title) throw new Error("retitle: the title model returned no title");
      return write(sessionID, caps ? title.toUpperCase() : title);
    };

    await ctx.rpc.register(api, {
      now: async ({ sessionID, recent }) => {
        const { title } = await ctx.session.get({ sessionID });
        return manual(sessionID, recent, pinned(title));
      },
      caps: async ({ sessionID, recent, on }) => {
        if (!on) return manual(sessionID, recent, false);
        const { title } = await ctx.session.get({ sessionID });
        if (!title) throw new Error("retitle: the session has no title to pin");
        return write(sessionID, title.toUpperCase());
      },
    });

    const watch = async () => {
      for await (const event of ctx.event.subscribe({ signal: controller.signal })) {
        if (event.type === "session.inbox.enqueued" && event.data.item.type === "user") {
          const text = event.data.item.payload.text.replace(/\s+/gu, " ").trim().slice(0, MESSAGE_CHARS);
          if (!text) continue;
          const current = state(event.data.sessionID);
          current.count++;
          current.recent = [...current.recent, text].slice(-MESSAGES);
        }
        if (event.type === "session.deleted") states.delete(event.data.sessionID);
        if (event.type === "session.execution.succeeded") {
          retitle(event.data.sessionID).catch((error: unknown) =>
            log(Effect.logError("retitle: failed", { sessionID: event.data.sessionID, error: String(error) })),
          );
        }
      }
    };
    watch().catch((error: unknown) => log(Effect.logError("retitle: event stream failed", { error: String(error) })));

    return () => controller.abort();
  },
});
