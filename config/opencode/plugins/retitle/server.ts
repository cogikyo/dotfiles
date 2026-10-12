import { Plugin } from "@opencode/plugin";
import { Effect, Fiber } from "effect";

const TRIGGER = 2;
const COOLDOWN_MS = 5 * 60_000;
const MESSAGES = 6;
const MESSAGE_CHARS = 600;
const TOTAL_CHARS = 3_000;
const MAX_CHARS = 40;

type State = { count: number; recent: string[]; wrote: number; running: boolean };

function pinned(title: string) {
  return /\p{L}/u.test(title) && !/\p{Ll}/u.test(title);
}

function request(title: string, recent: string[]) {
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
    "Write a 2-4 word title for what this session is working on now, in the language of the user messages.",
    "If the current title still fits, reply with the current title unchanged.",
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
  return title.length > MAX_CHARS ? `${title.slice(0, MAX_CHARS - 1).trimEnd()}…` : title;
}

export default Plugin.define({
  id: "retitle",
  async setup(ctx) {
    const fiber = Fiber.getCurrent();
    if (!fiber) throw new Error("retitle: setup started outside the plugin fiber");
    const log = Effect.runForkWith(fiber.context);
    const states = new Map<string, State>();
    const controller = new AbortController();

    const retitle = async (sessionID: string) => {
      const state = states.get(sessionID);
      if (!state || state.running || state.count < TRIGGER) return;
      if (Date.now() - state.wrote < COOLDOWN_MS) return;
      state.running = true;
      try {
        const session = await ctx.session.get({ sessionID });
        if (session.parentID || !session.title || pinned(session.title)) return;
        const recent = state.recent.slice();
        state.count = 0;
        const { data: agent } = await ctx.agent.get({ agentID: "title" });
        const { text } = await ctx.generate.text({ prompt: request(session.title, recent), model: agent.model });
        const title = clean(text);
        if (!title) return;
        const latest = await ctx.session.get({ sessionID });
        if (!latest.title || pinned(latest.title) || latest.title.toLowerCase() === title.toLowerCase()) return;
        await ctx.session.update({ sessionID, title });
        state.wrote = Date.now();
      } finally {
        state.running = false;
      }
    };

    const watch = async () => {
      for await (const event of ctx.event.subscribe({ signal: controller.signal })) {
        if (event.type === "session.inbox.enqueued" && event.data.item.type === "user") {
          const text = event.data.item.payload.text.replace(/\s+/gu, " ").trim().slice(0, MESSAGE_CHARS);
          if (!text) continue;
          const state = states.get(event.data.sessionID) ?? { count: 0, recent: [], wrote: 0, running: false };
          state.count++;
          state.recent = [...state.recent, text].slice(-MESSAGES);
          states.set(event.data.sessionID, state);
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
