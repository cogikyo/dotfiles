import { Plugin } from "@opencode/plugin";
import { Effect, Fiber, Schema } from "effect";
import { fences, parse, titled, topic } from "./parse.ts";
import { children } from "./progress.ts";
import { api, type State } from "./rpc.ts";
import { describe, Flow, link, ready, start, status, unlink, update, type Change } from "./state.ts";

const Steps = Schema.Array(Schema.Int);

const Input = Schema.Struct({
  start: Schema.optionalKey(Schema.Union([Schema.Literal(true), Schema.String])).annotate({
    description:
      "Adopt a workflow as a new revision: true takes the latest ```workflow fence in your own messages; a string is the fence body to adopt instead.",
  }),
  done: Schema.optionalKey(Steps).annotate({ description: "Step numbers to mark done." }),
  skip: Schema.optionalKey(Steps).annotate({ description: "Step numbers to mark skipped." }),
  fail: Schema.optionalKey(Steps).annotate({ description: "Step numbers to mark failed." }),
  repair: Schema.optionalKey(Steps).annotate({
    description:
      "Step numbers a gate sends back for repair: reopens them and their downstream steps and spends one round of that gate's repair budget.",
  }),
  check: Schema.optionalKey(Steps).annotate({
    description: "1-based checklist item indexes to tick on `step`.",
  }),
  step: Schema.optionalKey(Schema.Int).annotate({
    description: "Step whose items `check` ticks; a linked lane may omit it and ticks its own step.",
  }),
  end: Schema.optionalKey(Schema.Literal(true)).annotate({ description: "Finish the workflow." }),
});
type Input = typeof Input.Type;

type Message = Awaited<ReturnType<Plugin.Context["session"]["context"]>>[number];

const description = [
  "Read or update this session tree's stored workflow, which keeps step state across compaction.",
  "Call with {} to show status.",
  "Fields combine in one call and apply in order: start, done/skip/fail, repair, check, end.",
  "A lane linked to a step may only show status and `check` items on its own step.",
].join(" ");

const NONE =
  "No workflow is stored for this session. Propose a ```workflow fence, and once it is approved call `workflow` with `start: true`.";

function scans(start: true | string): start is true | "true" {
  return start === true || start.trim() === "true";
}

function latest(messages: readonly Message[]) {
  for (const message of messages.toReversed()) {
    if (message.type !== "assistant") continue;
    for (const part of message.content.toReversed()) {
      if (part.type !== "text") continue;
      const fence = fences(part.text).findLast(({ text, partial }) => !partial && parse(text).steps.length > 0);
      if (fence) return fence.text;
    }
  }
  throw new Error(
    "No complete ```workflow fence with steps found in this session's assistant text; write one or pass its body as `start`.",
  );
}

const QUEUES = Symbol.for("opencode.workflow.queues");

function shared() {
  const host = globalThis as typeof globalThis & { [QUEUES]?: Map<string, Promise<unknown>> };
  return (host[QUEUES] ??= new Map());
}

function unlinked(before: Flow | undefined, after: Flow) {
  const lost = Object.entries(before?.links ?? {}).filter(([child]) => !(child in after.links));
  if (!lost.length) return undefined;
  return `Unlinked, relaunch with \`step\` for a fresh checklist: ${lost.map(([child, n]) => `${child} (was step ${n})`).join(", ")}`;
}

function key(root: string) {
  return `flow/${root}`;
}

function explain(error: unknown) {
  return error instanceof Error ? error.message : JSON.stringify(error);
}

export default Plugin.define({
  id: "workflow",
  async setup(ctx) {
    const fiber = Fiber.getCurrent();
    if (!fiber) throw new Error("workflow: setup started outside the plugin fiber");
    const log = Effect.runForkWith(fiber.context);
    const fail = (what: string, error: unknown) =>
      log(Effect.logError(`workflow: ${what} failed`, { error: explain(error) }));

    const parents = new Map<string, string | undefined>();
    const queues = shared();

    const root = async (sessionID: string): Promise<string> => {
      if (!parents.has(sessionID)) parents.set(sessionID, (await ctx.session.get({ sessionID })).parentID);
      const parent = parents.get(sessionID);
      return parent ? root(parent) : sessionID;
    };

    const load = async (top: string) => {
      const value = await ctx.storage.get(key(top));
      return value === undefined ? undefined : Schema.decodeUnknownSync(Flow)(value);
    };

    const serial = <T>(top: string, work: () => Promise<T>) => {
      const run = (queues.get(top) ?? Promise.resolve()).then(work, work);
      const tail = run.catch(() => undefined);
      queues.set(top, tail);
      void tail.then(() => {
        if (queues.get(top) === tail) queues.delete(top);
      });
      return run;
    };

    const edit = (top: string, change: (flow: Flow | undefined) => Flow | undefined) =>
      serial(top, async () => {
        const before = await load(top);
        const after = change(before);
        if (after === before) return { before, after };
        if (after) await ctx.storage.set(key(top), Schema.encodeSync(Flow)(after));
        else await ctx.storage.remove(key(top));
        await rpc.events.emit("changed", { root: top });
        return { before, after };
      });

    const show = async (flow: Flow) =>
      status(flow, await children((sessionID) => ctx.session.get({ sessionID }), flow.links));

    const state = async (sessionID: string): Promise<State> => {
      const flow = await load(await root(sessionID));
      return flow ? { flow } : {};
    };

    const run = async (input: Input, sessionID: string) => {
      const top = await root(sessionID);
      const child = top !== sessionID;
      const { start: adopt, done, skip, fail: failed, repair, check, step, end } = input;
      const writes = [adopt, done, skip, failed, repair, end].some((field) => field !== undefined);
      if (child && writes)
        throw new Error(
          "Only the root session changes the workflow; a lane may only `check` items on its linked step.",
        );
      if (step !== undefined && check === undefined) throw new Error("`step` only selects where `check` ticks items.");
      if (!writes && check === undefined) {
        const flow = await load(top);
        if (!flow) throw new Error(NONE);
        const own = child ? flow.links[sessionID] : undefined;
        return [await show(flow), own === undefined ? "" : `Your step: ${own}`].filter(Boolean).join("\n");
      }
      const target = (flow: Flow) => {
        if (!child) {
          if (step === undefined) throw new Error("`check` needs `step`.");
          return step;
        }
        const own = flow.links[sessionID];
        if (own === undefined)
          throw new Error(
            "This session is not linked to a workflow step, so it cannot tick items; report your results back to your parent instead.",
          );
        if (step !== undefined && step !== own)
          throw new Error(`This session is linked to step ${own}; it may only tick items on that step.`);
        return own;
      };
      const source = adopt !== undefined && scans(adopt) ? latest(await ctx.session.context({ sessionID })) : adopt;
      let ended: number | undefined;
      const { before, after } = await edit(top, (flow) => {
        let next = source === undefined ? flow : start(flow, source);
        if (source !== undefined) untitled(source);
        if (!next) throw new Error(NONE);
        const change: Change = {
          done,
          skip,
          fail: failed,
          repair,
          check: check && { step: target(next), items: check },
        };
        next = update(next, change);
        if (!end) return next;
        ended = next.rev;
        return undefined;
      });
      if (!after) return `Ended rev ${ended} and deleted the stored workflow; the next \`start\` begins at rev 1.`;
      const flow = after;
      const open = new Set(before && before.rev === flow.rev ? ready(before) : []);
      const fresh = ready(flow).filter((n) => !open.has(n));
      return [
        source !== undefined && `Started rev ${flow.rev}.`,
        source !== undefined && flow.diff && describe(flow.diff),
        source !== undefined && unlinked(before, flow),
        source === undefined && fresh.length > 0 && `Now ready: ${fresh.join(", ")}`,
        await show(flow),
      ]
        .filter((line) => typeof line === "string")
        .join("\n");
    };

    const rpc = await ctx.rpc.register(api, {
      state: ({ sessionID }) => state(sessionID),
      pause: async ({ sessionID }) => {
        const { after } = await edit(await root(sessionID), (flow) =>
          !flow || flow.status === "paused" ? flow : { ...flow, status: "paused" },
        );
        return after ? { flow: after } : {};
      },
      link: async ({ sessionID, step }) => {
        const top = await root(sessionID);
        if (top === sessionID) throw new Error("Only a child session links to a workflow step.");
        await edit(top, (flow) => {
          if (!flow) throw new Error(NONE);
          return flow.links[sessionID] === step ? flow : link(flow, sessionID, step);
        });
        return state(sessionID);
      },
    });

    await ctx.tool.transform((editor) =>
      editor.add({
        name: "workflow",
        description,
        input: Input,
        options: { codemode: false },
        execute: async (input, context) => ({ content: await run(input, context.sessionID) }),
      }),
    );

    const scan = async (prefix: string, after?: string): Promise<{ key: string; value: unknown }[]> => {
      const page = await ctx.storage.scan({ prefix, after, limit: 1000 });
      return page.next ? [...page.entries, ...(await scan(prefix, page.next))] : [...page.entries];
    };

    const forget = async (sessionID: string) => {
      parents.delete(sessionID);
      await edit(sessionID, () => undefined);
      for (const entry of await scan(key(""))) {
        if (!Schema.is(Flow)(entry.value) || !(sessionID in entry.value.links)) continue;
        await edit(entry.key.slice(key("").length), (current) => current && unlink(current, sessionID));
      }
    };

    const controller = new AbortController();
    const watch = async () => {
      for await (const event of ctx.event.subscribe({ signal: controller.signal })) {
        if (event.type !== "session.deleted") continue;
        await forget(event.data.sessionID).catch((error: unknown) => fail("clear session state", error));
      }
    };
    watch().catch((error: unknown) => {
      if (!controller.signal.aborted) fail("event stream", error);
    });

    return () => controller.abort();
  },
});

function untitled(source: string) {
  const { title, steps } = parse(source);
  const heading = { line: source.split("\n").findIndex((text) => text.trimEnd() === title) + 1, headline: title };
  const long = [heading, ...steps].filter(
    ({ headline }) => topic(headline) === headline && Bun.stringWidth(headline) > titled,
  );
  if (!long.length) return;
  throw new Error(
    [
      `Give each long headline, the workflow heading included, a short \`Title: \` of at most ${titled} cells so the sidebar never clips it, as in \`Short Title: the rest of the line\`:`,
      ...long.map(({ line, headline }) => `line ${line}: \`${headline}\` is over ${titled} cells with no title`),
    ].join("\n"),
  );
}
