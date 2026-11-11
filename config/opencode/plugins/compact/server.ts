import type { OpenCodeEvent } from "@opencode/client/promise";
import { Plugin } from "@opencode/plugin";
import { Event } from "@opencode/schema/event";
import { SessionMessage } from "@opencode/schema/session-message";
import { TokenUsage } from "@opencode/schema/token-usage";
import { Effect, Fiber, Schema } from "effect";
import { api as workflow } from "../workflow/rpc.ts";
import { ceiling, due, type Level, levels } from "./limits.ts";

const floor = 100_000;

const SUMMARY = [
  "Summary rules for this session:",
  "- Quote the latest real user message verbatim, and mark each of its questions or requests as answered or open.",
  '- Record whether each plan or workflow is approved, proposed, or pending, and quote the user words that approved it. A harness "Continue if you have next steps" message is not approval.',
  '- Keep user decisions in their own words and at their own firmness; keep hedges such as "maybe" or "idk" as hedges.',
  "- Keep standing user instructions verbatim, especially instructions about what to do after compaction.",
  "- List uncommitted work by path, and mark each path as this session's work to commit, another session's work, or work to leave alone.",
  "- Drop items carried from the previous summary that are finished, superseded, or no longer relevant.",
].join("\n");

const CHILD =
  "- This is a delegate child session. Keep its brief's objective, write scope, exclusions, rules, and acceptance checks, as updated by later prompts.";

function recovery(sessionID: string) {
  return `Compaction finished. This session is \`${sessionID}\`. Load the \`recommended\` skill and follow it.`;
}

const WAKE = "Compaction finished while the session was idle; follow the recovery line in the checkpoint summary.";

const STOP =
  "Compaction is queued and runs when this turn ends; end the turn now without starting new work. After compaction, `recommended` proposes what comes next.";

const Ends = Schema.Struct({ end: Schema.Literal(true) });

const description =
  "Queue compaction of this session. Compaction runs after the current turn ends, and a follow-up message then resumes the session. Call it when the user asks or, past 150K, without asking at a clear finished-work point: a commit landed or the task is done, no lanes, edits, or repairs are in flight, and no question is open, including an unanswered user question. Ask through the question tool only if that point is unclear, subject to COLLAB.md's answer-first rule.";

// A stored id marks a pending delivery; delivered records omit it.
const Nudged = Schema.Struct({
  epoch: Schema.String,
  level: Schema.optionalKey(Schema.Number),
  id: Schema.optionalKey(Schema.String),
});

function nudged(sessionID = "") {
  return `nudged/${sessionID}`;
}

function label(tokens: number) {
  return tokens >= 1_000_000 ? `${+(tokens / 1_000_000).toFixed(2)}M` : `${Math.round(tokens / 1_000)}K`;
}

const BOUNDARY = [
  `At workflow end, the plugin compacts after this turn if context is still at least ${label(floor)}.`,
  "For other compaction, past 150K, call compact without asking once a commit lands or the task is done.",
  "The point must be clear: no edits, lanes, or repairs in flight and no open question, including an unanswered user question; do not compact mid-task while delivering findings, a proposal, or open choices.",
  "Ask through the question tool only if that point is unclear, subject to COLLAB.md's answer-first rule, and call compact after approval.",
  "In a driven run, commit finished work and queue compact without asking.",
  "If the user already declined compaction at this nudge, do not ask again.",
].join(" ");

const ADVICE: Record<Level["kind"], { running: string; idle: string }> = {
  soft: {
    running: `Soft warning: keep the workflow going if its remaining steps fit before ${label(ceiling)}. If this is a natural break with no lanes in flight, no repair pending, and no open question, including an unanswered user question, consider ending it here, re-scoping the rest into a new workflow, and compacting first; after compaction, \`recommended\` resumes the unfinished steps.`,
    idle: `Soft warning. ${BOUNDARY}`,
  },
  strong: {
    running: `Finish up now: start no new steps or lanes, let in-flight lanes land, end the workflow at the first point with no lanes in flight, no repair pending, and no open question, including an unanswered user question, and commit finished work; if context is still at least ${label(floor)}, the plugin compacts after the turn that ends the workflow, and \`recommended\` resumes the unfinished steps.`,
    idle: "Finish up now: start no new work, commit finished work, and queue compact without asking at the next clear finished-work point: a commit landed or the task is done, no lanes, edits, or repairs are in flight, and no question is open, including an unanswered user question. Ask through the question tool only if that point is unclear, subject to COLLAB.md's answer-first rule. In a driven run, queue compact without asking. If the user already declined compaction at this nudge, do not ask again.",
  },
};

function nudge(level: Level, running?: boolean) {
  const advice = ADVICE[level.kind];
  return `Context reached ${label(level.tokens)} tokens; forced compaction runs mid-turn at ${label(ceiling)}. ${running ? advice.running : advice.idle}`;
}

function remember<T>(cache: Map<string, Promise<T>>, sessionID: string, load: () => Promise<T>) {
  let known = cache.get(sessionID);
  if (!known) {
    known = load();
    known.catch(() => cache.delete(sessionID));
    cache.set(sessionID, known);
  }
  return known;
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

    const epochs = new Map<string, Promise<string>>();

    const primary = (sessionID: string) =>
      remember(primaries, sessionID, () => ctx.session.get({ sessionID }).then((session) => !session.parentID));

    const epoch = (sessionID: string) =>
      remember(epochs, sessionID, () =>
        ctx.session.context({ sessionID }).then(([first]) => (first?.type === "compaction" ? first.id : "none")),
      );

    const mine = (location?: { directory: string; workspaceID?: string }) =>
      location?.directory === ctx.location.directory && location.workspaceID === ctx.location.workspaceID;

    const scan = async (prefix: string, after?: string): Promise<{ key: string; value: unknown }[]> => {
      const page = await ctx.storage.scan({ prefix, after, limit: 1000 });
      return page.next ? [...page.entries, ...(await scan(prefix, page.next))] : [...page.entries];
    };

    const resume = async (sessionID: string, request: Request, since: number) => {
      try {
        if (!(await primary(sessionID))) return;
        await ctx.session.wait({ sessionID });
        const session = await ctx.session.get({ sessionID });
        if (request.halted || session.outcome !== "succeeded" || (session.time.idle ?? 0) < since) return;
        const [marker, ...after] = await ctx.session.context({ sessionID });
        const continued = after.some((message) => message.type === "user" || message.type === "assistant");
        if (marker?.id !== request.id || request.steered || continued) return;
        await ctx.session.synthetic({ sessionID, text: WAKE, delivery: "queue" });
      } finally {
        if (requests.get(sessionID) === request) requests.delete(sessionID);
      }
    };

    const reconcile = async () => {
      const entries = await scan(nudged());
      await Promise.all(
        entries.map((entry) =>
          ctx.session.get({ sessionID: entry.key.slice(nudged().length) }).then(
            () => undefined,
            () => ctx.storage.remove(entry.key),
          ),
        ),
      );
    };

    const size = async (sessionID: string) => {
      const known = tokens.get(sessionID);
      if (known !== undefined) return known;
      const last = (await ctx.session.context({ sessionID })).findLast(
        (message) => message.type === "assistant" && message.tokens !== undefined,
      );
      const seeded = last?.type === "assistant" && last.tokens ? TokenUsage.total(last.tokens) : 0;
      tokens.set(sessionID, seeded);
      return seeded;
    };

    const conclude = async (sessionID: string) => {
      if (compacting.has(sessionID) || !(await primary(sessionID)) || (await size(sessionID)) < floor) return false;
      await ctx.session.compact({ sessionID, delivery: "queue" });
      return true;
    };

    await ctx.tool.transform((editor) =>
      editor.add({
        name: "compact",
        description,
        input: Schema.Struct({}),
        options: { codemode: false },
        execute: async (_input, context) => {
          await ctx.session.compact({ sessionID: context.sessionID, delivery: "queue" });
          return {
            content: "Compaction queued. It runs when this turn ends; finish the turn without starting new work.",
          };
        },
      }),
    );

    await ctx.session.hook("compaction", async (event) => {
      const root = await guard("compaction session lookup", primary(event.sessionID));
      if (root) {
        event.result = { summary: recovery(event.sessionID), metadata: { keepID: null } };
        return;
      }
      event.system.push({ type: "text", text: root === false ? `${SUMMARY}\n${CHILD}` : SUMMARY });
    });

    await ctx.tool.hook("execute.after", async (event) => {
      if (event.tool !== "workflow" || event.status !== "completed" || !Schema.is(Ends)(event.input)) return;
      if (!(await guard("compact at workflow end", conclude(event.sessionID), false))) return;
      const { content } = event.result;
      event.result = {
        ...event.result,
        content:
          typeof content === "string"
            ? `${content}\n\n${STOP}`
            : [...(content ?? []), { type: "text", text: `\n\n${STOP}` }],
      };
    });

    const cap = (sessionID: string, total: number) => {
      if (total < ceiling || compacting.has(sessionID)) return;
      compacting.add(sessionID);
      void guard(
        "compact at ceiling",
        primary(sessionID).then((root) => (root ? ctx.session.compact({ sessionID, delivery: "steer" }) : undefined)),
      );
    };

    const notify = async (sessionID: string, total: number) => {
      if (total < levels[0].tokens || requests.has(sessionID) || !(await primary(sessionID))) return;
      const [current, record] = await Promise.all([epoch(sessionID), ctx.storage.get(nudged(sessionID))]);
      const sent = Schema.is(Nudged)(record) && record.epoch === current ? record : undefined;
      const level = due(total, sent);
      if (!level) return;
      const id = sent?.id ?? SessionMessage.ID.create();
      if (sent?.id === undefined) await ctx.storage.set(nudged(sessionID), { epoch: current, level: level.tokens, id });
      const running = await guard(
        "workflow status",
        ctx
          .rpc(workflow)
          .state({ sessionID })
          .then(({ flow }) => flow?.status === "active"),
        false,
      );
      await ctx.session.synthetic({
        id: SessionMessage.ID.make(id),
        sessionID,
        text: nudge(level, running),
        description: "Compaction nudge",
        delivery: "steer",
        resume: false,
      });
      await ctx.storage.set(nudged(sessionID), { epoch: current, level: level.tokens });
    };

    const track = async (event: OpenCodeEvent) => {
      if (event.type === "session.deleted") {
        tokens.delete(event.data.sessionID);
        compacting.delete(event.data.sessionID);
        primaries.delete(event.data.sessionID);
        epochs.delete(event.data.sessionID);
        requests.delete(event.data.sessionID);
        await guard("clear session state", ctx.storage.remove(nudged(event.data.sessionID)));
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
      }
      const halted =
        event.type === "session.execution.failed" ||
        (event.type === "session.execution.interrupted" && event.data.reason !== "shutdown");
      if (!halted) return;
      const request = requests.get(event.data.sessionID);
      if (request) request.halted = true;
    };

    const act = async (event: OpenCodeEvent) => {
      if (event.type === "session.step.ended") {
        const { sessionID } = event.data;
        const total = TokenUsage.total(event.data.tokens);
        tokens.set(sessionID, total);
        cap(sessionID, total);
        await guard("nudge", notify(sessionID, total));
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
        epochs.delete(sessionID);
        const request = requests.get(sessionID);
        if (request) void guard("follow up after compaction", resume(sessionID, request, event.created));
      }
      if (event.type === "session.compaction.failed") {
        const { sessionID } = event.data;
        requests.delete(sessionID);
        compacting.delete(sessionID);
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
    void guard("reconcile nudges", reconcile());

    return () => controller.abort();
  },
});
