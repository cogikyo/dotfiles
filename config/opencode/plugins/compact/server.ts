import { Plugin } from "@opencode/plugin";
import { Effect, Fiber, Schema } from "effect";

const tiers = [100_000, 250_000] as const;
const limit = 300_000;

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

const Input = Schema.Struct({
  brief: Schema.String.annotate({
    description:
      "Handoff brief: current work, decisions made, open threads, lanes to keep, HEAD, uncommitted paths with their owner (this session or another), and the intended commit grouping.",
  }),
  reason: Schema.String.annotate({ description: "Why now is a good time to compact." }),
});

const Pending = Schema.Struct({ brief: Schema.String, inbox: Schema.String });

const description =
  "Queue compaction of this session with a handoff brief. Compaction runs after the current turn ends, and the session then waits for the user. Call it only after the user approves compacting in this conversation, or in an armed drive run at a completed-work boundary.";

function key(sessionID: string) {
  return `pending/${sessionID}`;
}

function tierFor(tokens: number) {
  return tiers.findLastIndex((threshold) => tokens >= threshold);
}

function label(tokens: number) {
  return `${tokens / 1_000}K`;
}

function nudge(tier: number) {
  const head = `Context passed ${label(tiers[tier])} tokens (compaction nudge ${tier + 1}/${tiers.length}).`;
  const declined = "If the user already declined compaction at this nudge, do not ask again.";
  if (tier + 1 < tiers.length)
    return `${head} At the end of this turn, judge whether now is a good time to compact; if so, ask the user and call compact once they approve. ${declined}`;
  return `${head} Native auto-compaction fires near ${label(limit)}, continues the turn, and gets no handoff brief. At the end of this turn, at a completed-work boundary, ask the user to compact and call compact once they approve; good times are right after a commit lands or after the user deferred the commit. In an armed drive run, commit finished work, then call compact without asking. Hold it while edits, lanes, or repairs are in flight, or when this turn delivers findings, a proposal, or open choices. ${declined}`;
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

    const tokens = new Map<string, number>();
    const primaries = new Map<string, Promise<boolean>>();

    const primary = (sessionID: string) => {
      let known = primaries.get(sessionID);
      if (!known) {
        known = ctx.session.get({ sessionID }).then((session) => !session.parentID);
        known.catch(() => primaries.delete(sessionID));
        primaries.set(sessionID, known);
      }
      return known;
    };

    const pending = async (sessionID: string) => {
      const value = await ctx.storage.get(key(sessionID));
      return Schema.is(Pending)(value) ? value : undefined;
    };
    const settle = (sessionID: string, inbox?: string) =>
      (async () => {
        if (inbox !== undefined && (await pending(sessionID))?.inbox !== inbox) return;
        await ctx.storage.remove(key(sessionID));
      })().catch((error: unknown) => fail("clear brief", error));

    await ctx.tool.transform((editor) =>
      editor.add({
        name: "compact",
        description,
        input: Input,
        options: { codemode: false },
        execute: async (input, context) => {
          const admitted = await ctx.session.compact({ sessionID: context.sessionID, delivery: "queue" });
          await ctx.storage.set(key(context.sessionID), { brief: input.brief, inbox: admitted.id });
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
        if (await pending(event.sessionID)) return;
        event.system.push({ type: "text", text: nudge(tier) });
      } catch (error) {
        fail("nudge", error);
      }
    });

    await ctx.session.hook("compaction", async (event) => {
      try {
        const child = !(await primary(event.sessionID));
        event.system.push({ type: "text", text: child ? `${SUMMARY}\n${CHILD}` : SUMMARY });
        const brief = (await pending(event.sessionID))?.brief;
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
        if (event.type === "session.compaction.ended") {
          tokens.delete(event.data.sessionID);
          await settle(event.data.sessionID);
        }
        if (event.type === "session.compaction.failed" && event.data.inputID)
          await settle(event.data.sessionID, event.data.inputID);
        if (event.type === "session.inbox.cancelled") await settle(event.data.sessionID, event.data.inboxID);
        if (event.type === "session.deleted") {
          tokens.delete(event.data.sessionID);
          primaries.delete(event.data.sessionID);
          await settle(event.data.sessionID);
        }
      }
    };
    watch().catch((error: unknown) => {
      if (!controller.signal.aborted) fail("event stream", error);
    });

    return () => controller.abort();
  },
});
