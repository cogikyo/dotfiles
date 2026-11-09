import type { OpenCodeEvent } from "@opencode/client/promise";
import { Plugin } from "@opencode/plugin";
import { Event } from "@opencode/schema/event";
import { SessionMessage } from "@opencode/schema/session-message";
import { TokenUsage } from "@opencode/schema/token-usage";
import { Effect, Fiber, Schema } from "effect";
import { api as workflow } from "../workflow/rpc.ts";
import { children } from "../workflow/progress.ts";
import { status } from "../workflow/state.ts";
import { digest, Kept, type Workflow } from "./digest.ts";
import { located } from "../mail/store.ts";
import { history, type Skip } from "./history.ts";
import { ceiling } from "./limits.ts";
import { probe } from "./live.ts";

const threshold = 150_000;
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

const FOLLOW =
  "Compaction finished. Re-check each waiting item in the compaction digest or summary with its recorded command, then load the `recommended` skill and follow it.";

const STOP =
  "Compaction is queued and runs when this turn ends; end the turn now without starting new work. After compaction, `recommended` proposes what comes next.";

const Ends = Schema.Struct({ end: Schema.Literal(true) });

const Input = Schema.Struct({
  brief: Schema.String.annotate({
    description:
      "Handoff brief: in-flight work, each waiting item with its check command, uncommitted paths with their owners and intended commit grouping, and the driven-run marker and scope if a run is active.",
  }),
  reason: Schema.String.annotate({ description: "Why now is a good time to compact." }),
});

const Pending = Schema.Struct({ brief: Schema.optionalKey(Schema.String) });

const description =
  "Queue compaction of this session with a handoff brief. Compaction runs after the current turn ends, and a follow-up message then resumes the session. Call it when the user asks, after the user approves an offer made with the question tool at a completed-work boundary, or without asking at a completed-work boundary in a driven run.";

function key(sessionID: string, inbox = "") {
  return `pending/${sessionID}/${inbox}`;
}

// A stored id marks a pending delivery; delivered records omit it.
const Nudged = Schema.Struct({ epoch: Schema.String, id: Schema.optionalKey(Schema.String) });

function nudged(sessionID = "") {
  return `nudged/${sessionID}`;
}

function label(tokens: number) {
  return tokens >= 1_000_000 ? `${+(tokens / 1_000_000).toFixed(2)}M` : `${Math.round(tokens / 1_000)}K`;
}

const BOUNDARY = [
  "At workflow end, the plugin compacts after this turn unless a brief is pending.",
  "For other compaction offers, wait until a commit lands or the task is done and the user deferred the commit.",
  "Do not offer mid-task while edits, lanes, or repairs are in flight, or while delivering findings, a proposal, or open choices.",
  "Ask through the question tool and call compact after approval.",
  "In a driven run, commit finished work and queue compact without asking, with the active run marker and named scope in the brief so it takes precedence over workflow-end compaction.",
  "If the user already declined compaction at this nudge, do not ask again.",
].join(" ");

const REACHED = `Context reached ${label(threshold)} tokens. At ${label(ceiling)}, forced compaction can run mid-turn without a handoff brief.`;

const NUDGE = `${REACHED} ${BOUNDARY}`;

const FINISH = `${REACHED} A workflow is running: finish it, and the plugin compacts after the turn that ends it. If \`workflow\` headroom shows the remaining steps won't fit before ${label(ceiling)}, end it at the next point with no lanes in flight and no repair pending; after compaction, \`recommended\` resumes the unfinished steps. If the workflow just started after a long planning context, offering compaction now is fine, so it runs on fresh context.`;

function remember<T>(cache: Map<string, Promise<T>>, sessionID: string, load: () => Promise<T>) {
  let known = cache.get(sessionID);
  if (!known) {
    known = load();
    known.catch(() => cache.delete(sessionID));
    cache.set(sessionID, known);
  }
  return known;
}

function carried(messages: readonly { metadata?: Record<string, unknown> }[]) {
  for (const message of messages) if (typeof message.metadata?.digest === "string") return message.metadata.digest;
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

    const file = located(ctx.app.channel);
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

    const tracked = (sessionID: string): Promise<Workflow> =>
      ctx
        .rpc(workflow)
        .state({ sessionID })
        .then(
          async ({ flow }) => ({
            text: flow && status(flow, await children((id) => ctx.session.get({ sessionID: id }), flow.links)),
          }),
          (error: unknown) => {
            fail("workflow status", error);
            return { failed: true as const };
          },
        );

    const resume = async (sessionID: string, request: Request, since: number) => {
      try {
        if (!(await primary(sessionID))) return;
        await ctx.session.wait({ sessionID });
        const session = await ctx.session.get({ sessionID });
        if (request.halted || session.outcome !== "succeeded" || (session.time.idle ?? 0) < since) return;
        const [marker, ...after] = await ctx.session.context({ sessionID });
        if (marker?.id !== request.id || request.steered || after.some((message) => message.type === "user")) return;
        await ctx.session.synthetic({ sessionID, text: FOLLOW, delivery: "queue" });
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
      for (const entry of await scan(nudged())) sessions.add(entry.key.slice(nudged().length));
      await Promise.all(
        [...sessions].map((sessionID) =>
          ctx.session.get({ sessionID }).then(
            () => sweep(sessionID),
            () => clear(sessionID),
          ),
        ),
      );
    };

    const clear = async (sessionID: string) =>
      forget([...(await stored(sessionID)).map((entry) => entry.key), nudged(sessionID)]);

    const rules = async (sessionID: string, text: string) => {
      const brief = (await stored(sessionID)).at(-1)?.value.brief;
      return [
        text,
        ...(brief === undefined
          ? []
          : [
              `The agent wrote this handoff brief before requesting compaction. Preserve its content in the summary:\n\n${brief}`,
            ]),
      ].map((part) => ({ type: "text" as const, text: part }));
    };

    const skipped = (sessionID: string) => (row: Skip) =>
      log(Effect.logWarning("compact: skipped an undecodable history row", { sessionID, ...row }));

    const summarize = async (sessionID: string) => {
      const context = await ctx.session.context({ sessionID });
      const running = context.findLast((message) => message.type === "compaction" && message.status === "running");
      if (!running) throw new Error("no running compaction in the session context");
      const [first] = context;
      const prior =
        first?.type === "compaction" && first.status === "completed" && Schema.is(Kept)(first.metadata?.brief)
          ? first.metadata.brief
          : undefined;
      const entry = await ctx.storage.get(key(sessionID, running.id));
      const pending = Schema.is(Pending)(entry) ? entry : undefined;
      const kept = pending?.brief === undefined ? prior : { text: pending.brief, at: running.time.created };
      const session = { id: sessionID, messages: history(sessionID, file, skipped(sessionID)) };
      const [flow, live] = await Promise.all([tracked(sessionID), probe(ctx, sessionID, file)]);
      const text = digest(session, { kept, fresh: kept !== prior, ended: pending !== undefined }, flow, live);
      return { digest: text, ...(kept ? { brief: kept } : {}) };
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
      const admitted = await ctx.session.compact({ sessionID, delivery: "queue" });
      if (!Schema.is(Pending)(await ctx.storage.get(key(sessionID, admitted.id))))
        await ctx.storage.set(key(sessionID, admitted.id), {});
      return true;
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

    await ctx.session.hook("compaction", async (event) => {
      try {
        if (!(await primary(event.sessionID))) {
          event.system.push(...(await rules(event.sessionID, `${SUMMARY}\n${CHILD}`)));
          return;
        }
        event.result = { summary: "", metadata: { keepID: null, ...(await summarize(event.sessionID)) } };
      } catch (error) {
        fail("compaction digest", error);
        const prior = carried(event.messages);
        event.system.push(
          ...((await guard("compaction rules", rules(event.sessionID, SUMMARY))) ?? []),
          ...(prior ? [{ type: "text" as const, text: `Digest from the previous compaction:\n\n${prior}` }] : []),
        );
      }
    });

    await ctx.session.hook("context", (event) => {
      const text = carried(event.messages);
      if (text) event.system.push({ type: "text", text });
    });

    await ctx.tool.hook("execute.after", async (event) => {
      if (event.tool !== "workflow" || event.status !== "completed" || !Schema.is(Ends)(event.input)) return;
      if (!(await guard("compact at workflow end", conclude(event.sessionID), false))) return;
      const { content } = event.result;
      event.result = {
        ...event.result,
        content:
          typeof content === "string" ? `${content}\n\n${STOP}` : [...(content ?? []), { type: "text", text: STOP }],
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
      if (total < threshold || requests.has(sessionID) || !(await primary(sessionID))) return;
      const [current, record] = await Promise.all([epoch(sessionID), ctx.storage.get(nudged(sessionID))]);
      const sent = Schema.is(Nudged)(record) && record.epoch === current ? record : undefined;
      if (sent && sent.id === undefined) return;
      if ((await stored(sessionID)).length > 0) return;
      const id = sent?.id ?? SessionMessage.ID.create();
      if (!sent) await ctx.storage.set(nudged(sessionID), { epoch: current, id });
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
        text: running ? FINISH : NUDGE,
        description: "Compaction nudge",
        delivery: "steer",
        resume: false,
      });
      await ctx.storage.set(nudged(sessionID), { epoch: current });
    };

    const track = async (event: OpenCodeEvent) => {
      if (event.type === "session.deleted") {
        tokens.delete(event.data.sessionID);
        compacting.delete(event.data.sessionID);
        primaries.delete(event.data.sessionID);
        epochs.delete(event.data.sessionID);
        requests.delete(event.data.sessionID);
        await guard("clear session state", clear(event.data.sessionID));
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
        await guard("clear brief", sweep(sessionID));
        const request = requests.get(sessionID);
        if (request) void guard("follow up after compaction", resume(sessionID, request, event.created));
      }
      if (event.type === "session.compaction.failed") {
        const { sessionID, inputID } = event.data;
        requests.delete(sessionID);
        compacting.delete(sessionID);
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
