import { Plugin } from "@opencode/plugin";
import { Effect, Fiber } from "effect";
import { api, pinned } from "./rpc.ts";

const TRIGGER = 2;
const COOLDOWN_MS = 5 * 60_000;
const WORDS = 3;
const OPENING = 2;
const SAMPLED = 4;
const LATEST = 3;
const MESSAGE_CHARS = 400;
const SUMMARY_CHARS = 1_200;
const MAX_CHARS = 36;
const CLOSED = "[CLOSED] ";
const ORCHESTRATOR =
  "Orchestrator mode: the user gave this session an all-caps title. Keep decisions, synthesis, Git, and the user conversation here; send every edit, investigation, data pull, and check to children.";
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

type Message = Awaited<ReturnType<Plugin.Context["session"]["context"]>>[number];
type State = { count: number; wrote: number; running: boolean };

function human(text: string) {
  const plain = text
    .replace(/\[Image \d+\]/gu, " ")
    .replace(/\s+/gu, " ")
    .trim();
  return plain.split(" ").length >= WORDS ? plain : undefined;
}

function clip(text: string, max: number) {
  return text.length > max ? `${text.slice(0, max - 1).trimEnd()}…` : text;
}

function transcript(messages: readonly Message[]) {
  const summary = messages.flatMap((message) =>
    message.type === "compaction" && message.status === "completed" ? [message.summary] : [],
  )[0];
  const said = messages.flatMap((message) => {
    const text = message.type === "user" ? human(message.text) : undefined;
    return text ? [text] : [];
  });
  if (!summary && said.length === 0) return undefined;
  const middle = Math.max(0, said.length - OPENING - LATEST);
  const picks = [
    ...new Set([
      ...Array.from({ length: OPENING }, (_, index) => index),
      ...Array.from({ length: SAMPLED }, (_, index) => OPENING + Math.floor(((index + 0.5) * middle) / SAMPLED)),
      ...Array.from({ length: LATEST }, (_, index) => said.length - LATEST + index),
    ]),
  ]
    .filter((index) => index >= 0 && index < said.length)
    .toSorted((a, b) => a - b);
  const objective = summary?.split(/\n(?=#{1,6} )/u).find((section) => section.trim());
  return [
    ...(objective
      ? ["Summary of the session's earlier work:", "<summary>", clip(objective.trim(), SUMMARY_CHARS), "</summary>"]
      : []),
    ...(picks.length > 0
      ? [
          `The user has sent ${said.length} messages${summary ? " since that summary" : ""}; a selection follows, oldest first, numbered by position:`,
          "<messages>",
          ...picks.map((index) => `#${index + 1} ${clip(said[index] ?? "", MESSAGE_CHARS)}`),
          "</messages>",
        ]
      : []),
  ].join("\n");
}

function request(title: string | undefined, context: string) {
  return [
    "Name this coding session so the user can find it later by scanning or searching a list of sessions.",
    ...(title ? [`Current title: ${title}`] : []),
    context,
    "Write a 2-4 word title in Title Case that names the session's main work by its concrete subject, such as the feature, component, file, or bug, in the language of the user messages.",
    "Judge the whole session: early messages usually state the goal, and later messages usually refine or follow up on it.",
    'Approvals, "continue", formatting or length requests, and instructions about how to reply never name a session.',
    ...(title
      ? [
          "If the current title still names the main work, reply with it unchanged; change it only when the main work has clearly moved to a different task.",
        ]
      : []),
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
  return clip(cased, MAX_CHARS);
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
      const current = states.get(sessionID) ?? { count: 0, wrote: 0, running: false };
      states.set(sessionID, current);
      return current;
    };

    const generate = async (sessionID: string, title: string | undefined) => {
      const context = transcript(await ctx.session.context({ sessionID }));
      if (!context) throw new Error("retitle: the session has no user messages to title");
      const { data: agent } = await ctx.agent.get({ agentID: "title" });
      const { text } = await ctx.generate.text({ prompt: request(title, context), model: agent.model });
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
        if (session.parentID || !session.title || pinned(session.title) || session.title.startsWith(CLOSED)) return;
        current.count = 0;
        const title = await generate(sessionID, session.title);
        if (!title) return;
        const latest = await ctx.session.get({ sessionID });
        if (
          !latest.title ||
          pinned(latest.title) ||
          latest.title.startsWith(CLOSED) ||
          latest.title.toLowerCase() === title.toLowerCase()
        )
          return;
        await write(sessionID, title);
      } finally {
        current.running = false;
      }
    };

    const manual = async (sessionID: string, caps: boolean) => {
      const session = await ctx.session.get({ sessionID });
      if (session.parentID) throw new Error("retitle: child sessions keep their delegated titles");
      const title = await generate(sessionID, undefined);
      if (!title) throw new Error("retitle: the title model returned no title");
      return write(sessionID, caps ? title.toUpperCase() : title);
    };

    await ctx.rpc.register(api, {
      now: async ({ sessionID }) => {
        const { title } = await ctx.session.get({ sessionID });
        return manual(sessionID, pinned(title));
      },
      caps: async ({ sessionID, on }) => {
        if (!on) return manual(sessionID, false);
        const { title } = await ctx.session.get({ sessionID });
        if (!title) throw new Error("retitle: the session has no title to pin");
        return write(sessionID, title.toUpperCase());
      },
    });

    await ctx.session.hook("context", async (event) => {
      try {
        const session = await ctx.session.get({ sessionID: event.sessionID });
        if (!session.parentID && pinned(session.title)) event.system.push({ type: "text", text: ORCHESTRATOR });
      } catch (error) {
        log(Effect.logError("retitle: orchestrator mode failed", { sessionID: event.sessionID, error: String(error) }));
      }
    });

    const watch = async () => {
      for await (const event of ctx.event.subscribe({ signal: controller.signal })) {
        if (
          event.type === "session.inbox.enqueued" &&
          event.data.item.type === "user" &&
          human(event.data.item.payload.text)
        )
          state(event.data.sessionID).count++;
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
