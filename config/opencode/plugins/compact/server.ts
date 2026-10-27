import type { OpenCodeEvent } from "@opencode/client/promise";
import { Plugin } from "@opencode/plugin";
import { Event } from "@opencode/schema/event";
import { SessionMessage } from "@opencode/schema/session-message";
import { Effect, Fiber, Schema } from "effect";
import { api as drive } from "../drive/rpc.ts";
import { ceiling } from "./limits.ts";

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

const RECOMMENDED = "Load the `recommended` skill and follow it.";

const DRIVE =
  "Drive is armed. Load the `recommended` skill and write its restatement and workflow card, but decide each open item yourself and record it under Decided. Then, in this same turn, continue the armed drive run from the handoff brief or summary.";

const Input = Schema.Struct({
  brief: Schema.String.annotate({
    description:
      "Handoff brief: current work, decisions made, open threads, lanes to keep, HEAD, uncommitted paths with their owner (this session or another), and the intended commit grouping.",
  }),
  reason: Schema.String.annotate({ description: "Why now is a good time to compact." }),
});

const Pending = Schema.Struct({ brief: Schema.String });

const description =
  "Queue compaction of this session with a handoff brief. Compaction runs after the current turn ends, and a follow-up message then resumes the session. Call it when the user asks, after the user approves an offer made with the question tool at a completed-work boundary, or in an armed drive run at a completed-work boundary.";

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

function nudge(tier: number) {
  const head = `Context passed ${label(tiers[tier])} tokens (compaction nudge ${tier + 1}/${tiers.length}).`;
  if (tier + 1 < tiers.length) return `${head} ${BOUNDARY}`;
  return `${head} This session compacts automatically at ${label(ceiling)} tokens, mid-turn, with no handoff brief. ${BOUNDARY}`;
}

function explain(error: unknown) {
  return error instanceof Error ? error.message : JSON.stringify(error);
}

type Request = { id: string; halted: boolean; steered: boolean };

export default Plugin.define({
  id: "compact",
  async setup(ctx) {
    const fiber = Fiber.getCurrent();
    if (!fiber) throw new Error("compact: setup started outside the plugin fiber");
    const log = Effect.runForkWith(fiber.context);
    const fail = (what: string, error: unknown) =>
      log(Effect.logError(`compact: ${what} failed`, { error: explain(error) }));
    const guard = <T>(what: string, work: Promise<T>, fallback?: T) =>
      work.catch((error: unknown) => {
        fail(what, error);
        return fallback;
      });

    const tokens = new Map<string, number>();
    const primaries = new Map<string, Promise<boolean>>();
    const compacting = new Set<string>();
    const requests = new Map<string, Request>();

    const primary = (sessionID: string) => {
      let known = primaries.get(sessionID);
      if (!known) {
        known = ctx.session.get({ sessionID }).then((session) => !session.parentID);
        known.catch(() => primaries.delete(sessionID));
        primaries.set(sessionID, known);
      }
      return known;
    };

    const mine = (location?: { directory: string; workspaceID?: string }) =>
      location?.directory === ctx.location.directory && location.workspaceID === ctx.location.workspaceID;

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
      if (entries.length === 0) return;
      const compactions = (await ctx.session.context({ sessionID })).filter((message) => message.type === "compaction");
      const done = entries.filter((entry) => {
        const own = compactions.find((message) => message.id === entry.inbox);
        const later = compactions.some(
          (message) => message.reason === "manual" && message.status === "completed" && message.id > entry.inbox,
        );
        return later || (own !== undefined && own.status !== "running");
      });
      await forget(done.map((entry) => entry.key));
    };

    const resume = async (sessionID: string, request: Request, since: number) => {
      try {
        if (!(await primary(sessionID))) return;
        await ctx.session.wait({ sessionID });
        const session = await ctx.session.get({ sessionID });
        if (request.halted || session.outcome !== "succeeded" || (session.time.idle ?? 0) < since) return;
        const armed = await guard(
          "drive state",
          ctx
            .rpc(drive)
            .state({ sessionID })
            .then((state) => state.armed),
          false,
        );
        const [marker, ...after] = await ctx.session.context({ sessionID });
        if (marker?.id !== request.id || request.steered || after.some((message) => message.type === "user")) return;
        await ctx.session.synthetic({ sessionID, text: armed ? DRIVE : RECOMMENDED, delivery: "queue" });
      } finally {
        if (requests.get(sessionID) === request) requests.delete(sessionID);
      }
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
        event.system.push({ type: "text", text: nudge(tier) });
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

    const cap = (sessionID: string, total: number) => {
      if (total < ceiling || compacting.has(sessionID)) return;
      compacting.add(sessionID);
      void guard(
        "compact at ceiling",
        primary(sessionID).then((root) => (root ? ctx.session.compact({ sessionID, delivery: "steer" }) : undefined)),
      );
    };

    const track = async (event: OpenCodeEvent) => {
      if (event.type === "session.deleted") {
        tokens.delete(event.data.sessionID);
        compacting.delete(event.data.sessionID);
        primaries.delete(event.data.sessionID);
        requests.delete(event.data.sessionID);
        await guard(
          "clear brief",
          stored(event.data.sessionID).then((entries) => forget(entries.map((entry) => entry.key))),
        );
      }
      if (event.type === "session.inbox.enqueued") {
        const { sessionID, inboxID, item } = event.data;
        if (item.type === "compaction") requests.set(sessionID, { id: inboxID, halted: false, steered: false });
        const request = requests.get(sessionID);
        if (item.type === "user" && request) request.steered = true;
      }
      if (event.type === "session.inbox.cancelled") {
        const { sessionID, inboxID } = event.data;
        if (requests.get(sessionID)?.id === inboxID) requests.delete(sessionID);
        await guard("clear brief", ctx.storage.remove(key(sessionID, inboxID)));
      }
      const halted =
        event.type === "session.execution.failed" ||
        (event.type === "session.execution.interrupted" && event.data.reason !== "shutdown");
      const request = halted ? requests.get(event.data.sessionID) : undefined;
      if (request) request.halted = true;
    };

    const act = async (event: OpenCodeEvent) => {
      if (event.type === "session.step.ended") {
        const { sessionID } = event.data;
        const { input, output, reasoning, cache } = event.data.tokens;
        const total = input + output + reasoning + cache.read + cache.write;
        tokens.set(sessionID, total);
        cap(sessionID, total);
      }
      if (event.type === "session.compaction.started") {
        const { sessionID } = event.data;
        const id = event.data.inputID ?? SessionMessage.ID.fromEvent(Event.ID.make(event.id));
        const prior = requests.get(sessionID);
        requests.set(
          sessionID,
          prior?.id === id ? { ...prior, steered: false } : { id, halted: false, steered: false },
        );
        compacting.add(sessionID);
      }
      if (event.type === "session.compaction.ended") {
        const { sessionID } = event.data;
        tokens.delete(sessionID);
        compacting.delete(sessionID);
        await guard("clear brief", sweep(sessionID));
        const request = requests.get(sessionID);
        if (request) void guard("follow up after compaction", resume(sessionID, request, event.created));
      }
      if (event.type === "session.compaction.failed") {
        const { sessionID, inputID } = event.data;
        requests.delete(sessionID);
        if (inputID) await guard("clear brief", ctx.storage.remove(key(sessionID, inputID)));
      }
    };

    const controller = new AbortController();
    const watch = async () => {
      for await (const event of ctx.event.subscribe({ signal: controller.signal })) {
        // Inbox and execution events can lack a location; filtering them would bypass follow-up suppression.
        await track(event);
        if (mine(event.location)) await act(event);
      }
    };
    watch().catch((error: unknown) => {
      if (!controller.signal.aborted) fail("event stream", error);
    });
    void guard("reconcile briefs", reconcile());

    return () => controller.abort();
  },
});
