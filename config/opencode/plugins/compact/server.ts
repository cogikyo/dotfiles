import { Plugin } from "@opencode/plugin";
import { Effect, Fiber, Schema } from "effect";
import { api as drive } from "../drive/rpc.ts";

const tiers = [150_000, 250_000] as const;

const SUMMARY = [
  "Summary rules for this session:",
  "- Quote the latest real user message verbatim, and mark each of its questions or requests as answered or open.",
  '- Record whether each plan or workflow is approved, proposed, or pending, and quote the user words that approved it. A harness "Continue if you have next steps" message is not approval.',
  '- Keep user decisions in their own words and at their own firmness; keep hedges such as "maybe" or "idk" as hedges.',
  "- Keep standing user instructions verbatim, especially instructions about what to do after compaction.",
  "- List the remaining workflow steps with their status, and state the exact next step.",
  "- List uncommitted work by path, and mark each path as this session's work to commit, another session's work, or work to leave alone.",
  "- Drop items carried from the previous summary that are finished, superseded, or no longer relevant.",
].join("\n");

const CHILD =
  "- This is a delegate child session. Keep its brief's objective, write scope, exclusions, rules, and acceptance checks, as updated by later prompts.";

const CONTINUE = "Continue the armed drive run from the handoff brief.";

const Input = Schema.Struct({
  brief: Schema.String.annotate({
    description:
      "Handoff brief: current work, decisions made, open threads, lanes to keep, HEAD, uncommitted paths with their owner (this session or another), and the intended commit grouping.",
  }),
  reason: Schema.String.annotate({ description: "Why now is a good time to compact." }),
});

const Pending = Schema.Struct({ brief: Schema.String, halted: Schema.optional(Schema.Boolean) });

const description =
  "Queue compaction of this session with a handoff brief. Compaction runs after the current turn ends; the session then waits for the user, or continues from the brief in an armed drive run. Call it when the user asks, after the user approves an offer made with the question tool at a completed-work boundary, or in an armed drive run at a completed-work boundary.";

function key(sessionID: string, inbox = "") {
  return `pending/${sessionID}/${inbox}`;
}

function tierFor(tokens: number) {
  return tiers.findLastIndex((threshold) => tokens >= threshold);
}

function label(tokens: number) {
  return tokens >= 1_000_000 ? `${+(tokens / 1_000_000).toFixed(2)}M` : `${Math.round(tokens / 1_000)}K`;
}

const BOUNDARY = [
  "Offer compaction only at a completed-work boundary: right after a commit lands, or after the task is done and the user deferred the commit.",
  "Hold it mid-task: before approved work is built, while edits, lanes, or repairs are in flight, and when this turn delivers findings, a proposal, or open choices.",
  "At a boundary, ask with the question tool instead of the reply text, and call compact once the user approves.",
  "In an armed drive run, commit finished work, then call compact without asking; the run continues from the brief after compaction.",
  "If the user already declined compaction at this nudge, do not ask again.",
].join(" ");

function nudge(tier: number, window: number) {
  const head = `Context passed ${label(tiers[tier])} tokens (compaction nudge ${tier + 1}/${tiers.length}).`;
  if (tier + 1 < tiers.length) return `${head} ${BOUNDARY}`;
  const limit = window > 0 ? `this model's ${label(window)}-token window` : "the model's context window";
  return `${head} Native auto-compaction fires as the context nears ${limit}, continues the turn, and gets no handoff brief. ${BOUNDARY}`;
}

function explain(error: unknown) {
  return error instanceof Error ? error.message : JSON.stringify(error);
}

export default Plugin.define({
  id: "compact",
  async setup(ctx) {
    const fiber = Fiber.getCurrent();
    if (!fiber) throw new Error("compact: setup started outside the plugin fiber");
    const log = Effect.runForkWith(fiber.context);
    const fail = (what: string, error: unknown) =>
      log(Effect.logError(`compact: ${what} failed`, { error: explain(error) }));
    const guard = (what: string, work: Promise<unknown>) => work.catch((error: unknown) => fail(what, error));

    const tokens = new Map<string, number>();
    const primaries = new Map<string, Promise<boolean>>();
    const windows = new Map<string, Promise<number>>();

    const primary = (sessionID: string) => {
      let known = primaries.get(sessionID);
      if (!known) {
        known = ctx.session.get({ sessionID }).then((session) => !session.parentID);
        known.catch(() => primaries.delete(sessionID));
        primaries.set(sessionID, known);
      }
      return known;
    };

    const window = (model: { providerID: string; id: string }) => {
      const id = `${model.providerID}/${model.id}`;
      let known = windows.get(id);
      if (!known) {
        known = ctx.model.list().then(({ data }) => {
          const limit = data.find((entry) => entry.providerID === model.providerID && entry.id === model.id)?.limit;
          if (!limit) windows.delete(id);
          return limit ? limit.input || limit.context : 0;
        });
        known.catch(() => windows.delete(id));
        windows.set(id, known);
      }
      return known;
    };

    const scan = async (prefix: string, after?: string): Promise<{ key: string; value: unknown }[]> => {
      const page = await ctx.storage.scan({ prefix, after, limit: 1000 });
      return page.next ? [...page.entries, ...(await scan(prefix, page.next))] : [...page.entries];
    };

    const stored = async (sessionID: string) =>
      (await scan(key(sessionID))).flatMap((entry) =>
        Schema.is(Pending)(entry.value)
          ? [{ key: entry.key, inbox: entry.key.slice(key(sessionID).length), value: entry.value }]
          : [],
      );

    const forget = (keys: readonly string[]) => Promise.all(keys.map((entry) => ctx.storage.remove(entry)));

    const sweep = async (sessionID: string) => {
      const entries = await stored(sessionID);
      if (entries.length === 0) return [];
      const compactions = (await ctx.session.context({ sessionID })).filter((message) => message.type === "compaction");
      const done = entries.flatMap((entry) => {
        const own = compactions.find((message) => message.id === entry.inbox);
        const later = compactions.some(
          (message) => message.reason === "manual" && message.status === "completed" && message.id > entry.inbox,
        );
        if (!later && (!own || own.status === "running")) return [];
        return [{ ...entry, completed: own?.status === "completed" }];
      });
      await forget(done.map((entry) => entry.key));
      return done;
    };

    const halt = async (sessionID: string) =>
      Promise.all(
        (await stored(sessionID)).map((entry) => ctx.storage.set(entry.key, { ...entry.value, halted: true })),
      );

    const resume = async (sessionID: string, since: number) => {
      await ctx.session.wait({ sessionID });
      const session = await ctx.session.get({ sessionID });
      if (session.outcome !== "succeeded" || (session.time.idle ?? 0) < since) return;
      if (!(await ctx.rpc(drive).state({ sessionID })).armed) return;
      await ctx.session.synthetic({ sessionID, text: CONTINUE, delivery: "queue" });
    };

    const reconcile = async () => {
      const entries = await scan("pending/");
      const sessions = new Set<string>();
      const malformed = entries.filter((entry) => {
        const [sessionID, inbox] = entry.key.slice("pending/".length).split("/");
        if (!sessionID || !inbox || !Schema.is(Pending)(entry.value)) return true;
        sessions.add(sessionID);
        return false;
      });
      await forget(malformed.map((entry) => entry.key));
      await Promise.all(
        [...sessions].map((sessionID) =>
          ctx.session.get({ sessionID }).then(
            () => sweep(sessionID),
            async () => forget((await stored(sessionID)).map((entry) => entry.key)),
          ),
        ),
      );
    };

    await ctx.tool.transform((editor) =>
      editor.add({
        name: "compact",
        description,
        input: Input,
        options: { codemode: false },
        execute: async (input, context) => {
          const admitted = await ctx.session.compact({ sessionID: context.sessionID, delivery: "queue" });
          await ctx.storage.set(key(context.sessionID, admitted.id), { brief: input.brief });
          return {
            content: "Compaction queued. It runs when this turn ends; finish the turn without starting new work.",
            metadata: { reason: input.reason, tokens: tokens.get(context.sessionID) ?? 0 },
          };
        },
      }),
    );

    await ctx.session.hook("context", async (event) => {
      try {
        const tier = tierFor(tokens.get(event.sessionID) ?? 0);
        if (tier < 0 || !(await primary(event.sessionID))) return;
        if ((await stored(event.sessionID)).length > 0) return;
        const limit =
          tier + 1 < tiers.length
            ? 0
            : await window(event.model).catch((error: unknown) => {
                fail("model window", error);
                return 0;
              });
        event.system.push({ type: "text", text: nudge(tier, limit) });
      } catch (error) {
        fail("nudge", error);
      }
    });

    await ctx.session.hook("compaction", async (event) => {
      try {
        const child = !(await primary(event.sessionID));
        event.system.push({ type: "text", text: child ? `${SUMMARY}\n${CHILD}` : SUMMARY });
        const brief = (await stored(event.sessionID)).at(-1)?.value.brief;
        if (brief === undefined) return;
        event.system.push({
          type: "text",
          text: `The agent wrote this handoff brief before requesting compaction. Preserve its content in the summary:\n\n${brief}`,
        });
      } catch (error) {
        fail("compaction rules", error);
      }
    });

    const controller = new AbortController();
    const watch = async () => {
      for await (const event of ctx.event.subscribe({ signal: controller.signal })) {
        if (event.type === "session.step.ended") {
          const { input, output, reasoning, cache } = event.data.tokens;
          tokens.set(event.data.sessionID, input + output + reasoning + cache.read + cache.write);
        }
        if (
          event.type === "session.execution.failed" ||
          (event.type === "session.execution.interrupted" && event.data.reason !== "shutdown")
        )
          await guard("halt continuation", halt(event.data.sessionID));
        if (event.type === "session.compaction.ended") {
          const { sessionID } = event.data;
          tokens.delete(sessionID);
          if (event.data.reason !== "manual") continue;
          const done = await sweep(sessionID).catch((error: unknown) => {
            fail("clear brief", error);
            return [];
          });
          if (done.some((entry) => entry.completed && !entry.value.halted))
            void guard("continue after compaction", resume(sessionID, event.created));
        }
        if (event.type === "session.compaction.failed" && event.data.inputID)
          await guard("clear brief", ctx.storage.remove(key(event.data.sessionID, event.data.inputID)));
        if (event.type === "session.inbox.cancelled")
          await guard("clear brief", ctx.storage.remove(key(event.data.sessionID, event.data.inboxID)));
        if (event.type === "session.deleted") {
          tokens.delete(event.data.sessionID);
          primaries.delete(event.data.sessionID);
          await guard(
            "clear brief",
            stored(event.data.sessionID).then((entries) => forget(entries.map((entry) => entry.key))),
          );
        }
      }
    };
    watch().catch((error: unknown) => {
      if (!controller.signal.aborted) fail("event stream", error);
    });
    void guard("reconcile briefs", reconcile());

    return () => controller.abort();
  },
});
