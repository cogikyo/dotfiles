import { Plugin } from "@opencode/plugin/effect";
import type { Session } from "@opencode/schema/session";
import { Tool } from "@opencode/schema/tool";
import { DateTime, Effect, Option, Schema, Stream } from "effect";
import { readFile } from "node:fs/promises";
import { ranked, reopens, staleAfter, survey } from "../anthropic/pool.ts";
import { api as workflow } from "../workflow/rpc.ts";
import { parseRouting, routeFor, routingPath, type Ref } from "./routing.ts";
import { api, type Outcome } from "./rpc.ts";
import { admit, checklist, cold, coldAfter, loaded } from "./step.ts";

type Status = "running" | "idle" | "error" | "interrupted";
type Input = {
  agent: string;
  description: string;
  prompt: string;
  model?: string;
  sessionID?: Session.ID;
  background?: boolean;
  lane?: string;
  step?: number;
};

const pool = "claude";

const format = (ref: Ref) => `${ref.providerID}/${ref.id}${ref.variant ? `#${ref.variant}` : ""}`;

function parseRef(value: string): Ref {
  const [model, variant] = value.split("#");
  const slash = model.indexOf("/");
  return { providerID: model.slice(0, slash), id: model.slice(slash + 1), variant: variant || undefined };
}

const readRouting = Effect.tryPromise({
  try: async () => parseRouting(await readFile(routingPath, "utf8")),
  catch: (error) =>
    new Tool.Error({
      message: `ROUTING.md at ${routingPath} is unusable (${error instanceof Error ? error.message : String(error)}); pass model explicitly.`,
    }),
});

async function pick(route: Ref, current: Ref | undefined, fallback: Ref | undefined) {
  const list = await survey(route.id);
  const open = ranked(list, current?.providerID);
  const stale = list
    .filter((account) => account.age !== undefined && account.age > staleAfter)
    .map((account) => `${account.id} ${Math.round(account.age! / 60_000)}m old`);
  const notes = stale.length
    ? [`lanes: Claude usage is stale (${stale.join(", ")}); picked by last known numbers.`]
    : [];
  if (!open.length && fallback) {
    const ref = { providerID: fallback.providerID, id: fallback.id };
    notes.push(`lanes: all Claude accounts are capped for ${route.id}; using ${format(ref)} with no effort.`);
    return { ref, note: notes.join("\n") };
  }
  const account = open[0] ?? list.toSorted((a, b) => reopens(a) - reopens(b))[0];
  return { ref: { ...route, providerID: account.id }, note: notes.join("\n") || undefined };
}

const Closed = Schema.Struct({
  lane: Schema.optionalKey(Schema.String),
  by: Schema.Literals(["operator", "agent", "cache"]),
  at: Schema.Number,
});
type Closed = typeof Closed.Type;

const laneOf = (session: Session.Info) =>
  typeof session.metadata?.lane === "string" ? session.metadata.lane : undefined;

const closedOf = (session: Session.Info) =>
  Option.getOrUndefined(Schema.decodeUnknownOption(Closed)(session.metadata?.closed));

const same = (a: Ref | undefined, b: Ref) =>
  a?.providerID === b.providerID && a.id === b.id && (a.variant ?? "default") === (b.variant ?? "default");

function ago(time: DateTime.Utc) {
  const minutes = Math.round((Date.now() - DateTime.toEpochMillis(time)) / 60_000);
  if (minutes < 1) return "now";
  if (minutes < 60) return `${minutes}m ago`;
  return minutes < 1440 ? `${Math.round(minutes / 60)}h ago` : `${Math.round(minutes / 1440)}d ago`;
}

export default Plugin.define({
  id: "lanes",
  effect: (ctx) =>
    Effect.gen(function* () {
      const live = new Map<string, Status>();
      const creating = new Set<string>();
      const prompting = new Map<string, string>();

      yield* ctx.event.subscribe().pipe(
        Stream.runForEach((event) =>
          Effect.sync(() => {
            if (event.type === "session.execution.started") live.set(event.data.sessionID, "running");
            if (event.type === "session.execution.succeeded") live.set(event.data.sessionID, "idle");
            if (event.type === "session.execution.failed") live.set(event.data.sessionID, "error");
            if (event.type === "session.execution.interrupted") live.set(event.data.sessionID, "interrupted");
            if (event.type === "session.inbox.enqueued" && event.data.item.type === "user") {
              const key = prompting.get(event.data.sessionID);
              if (key !== undefined) creating.delete(key);
              prompting.delete(event.data.sessionID);
            }
          }),
        ),
        Effect.tapCause(Effect.logError),
        Effect.ignore,
        Effect.forkScoped,
      );

      const status = (session: Session.Info): Status =>
        live.get(session.id) ??
        (session.outcome === "failed" ? "error" : session.outcome === "interrupted" ? "interrupted" : "idle");

      const get = (sessionID: string) =>
        ctx.session.get({ sessionID: sessionID as Session.ID }).pipe(Effect.option, Effect.map(Option.getOrUndefined));

      const children = Effect.fn("lanes.children")(function* (parentID: string) {
        const prefix = `child/${parentID}/`;
        const ids: string[] = [];
        let after: string | undefined;
        do {
          const page = yield* ctx.storage.scan({ prefix, after, limit: 1000 });
          ids.push(...page.entries.map((entry) => entry.key.slice(prefix.length)));
          after = page.next;
        } while (after);
        const sessions = yield* Effect.forEach(ids, get, { concurrency: "unbounded" });
        return sessions.filter((session) => session !== undefined);
      });

      const tag = Effect.fn("lanes.tag")(function* (parentID: string, childID: string, lane: string | undefined) {
        yield* ctx.storage.set(`child/${parentID}/${childID}`, true);
        if (!lane) return;
        const child = yield* get(childID);
        if (!child || laneOf(child) === lane) return;
        yield* ctx.session
          .update({ sessionID: child.id, metadata: { ...child.metadata, lane } })
          .pipe(Effect.tapCause(Effect.logError), Effect.ignore);
      });

      const seal = Effect.fn("lanes.seal")(function* (child: Session.Info, by: Closed["by"]) {
        const lane = laneOf(child);
        const { lane: _, ...metadata } = child.metadata ?? {};
        yield* ctx.session
          .update({
            sessionID: child.id,
            metadata: { ...metadata, closed: { ...(lane ? { lane } : {}), by, at: Date.now() } },
            permissions: [...(child.permissions ?? []), { action: "*", resource: "*", effect: "deny" }],
          })
          .pipe(
            Effect.mapError(
              (error) =>
                new Tool.Error({
                  message: `Closing ${lane ? `lane "${lane}"` : `session ${child.id}`} failed: ${error instanceof Error ? error.message : String(error)}`,
                }),
            ),
          );
      });

      const close = Effect.fn("lanes.close")(function* (parentID: string, names: readonly string[], by: Closed["by"]) {
        const list = yield* children(parentID);
        const outcomes: Outcome[] = [];
        for (const lane of new Set(names.map((name) => name.trim()).filter(Boolean))) {
          const child = list.find((session) => laneOf(session) === lane);
          if (!child) {
            const closed = list.some((session) => closedOf(session)?.lane === lane);
            outcomes.push({ lane, skipped: closed ? "already closed" : "missing" });
            continue;
          }
          if (status(child) === "running" || creating.has(`${parentID}/${lane}`)) {
            outcomes.push({ lane, skipped: "running" });
            continue;
          }
          yield* seal(child, by);
          outcomes.push({ lane });
        }
        return outcomes;
      });

      yield* ctx.rpc
        .register(api, {
          close: ({ sessionID, lanes }) => close(sessionID, lanes, "operator").pipe(Effect.orDie),
        })
        .pipe(Effect.orDie);

      const expire = Effect.fn("lanes.expire")(function* (child: Session.Info, parentID: string) {
        if (child.parentID !== parentID || status(child) === "running" || !child.time.idle) return;
        const now = Date.now();
        if (now - DateTime.toEpochMillis(child.time.idle) <= coldAfter) return;
        const messages = yield* ctx.session.context({ sessionID: child.id }).pipe(
          Effect.mapError(
            (error) =>
              new Tool.Error({
                message: `Reading session ${child.id} failed: ${error instanceof Error ? error.message : String(error)}`,
              }),
          ),
        );
        const latest = messages.at(-1);
        if (latest?.type !== "idle") return;
        const idle = now - DateTime.toEpochMillis(latest.time.created);
        const step = messages.findLast((message) => message.type === "assistant" && message.tokens !== undefined);
        const size = loaded(step?.type === "assistant" ? step.tokens : undefined);
        if (!cold(idle, size)) return;
        yield* seal(child, "cache");
        const lane = laneOf(child);
        return yield* new Tool.Error({
          message: [
            `${lane ? `Lane "${lane}"` : `Session ${child.id}`} was idle ${Math.round(idle / 3_600_000)}h with ${Math.round(size / 1000)}K context, so its prompt cache had expired and it is now closed.`,
            lane
              ? "Naming it again starts a fresh child, so send a full brief instead of a delta."
              : "Start a fresh child with a full brief instead of a delta.",
          ].join(" "),
        });
      });

      const resolve = Effect.fn("lanes.resolve")(function* (input: Input, existing: Session.Info | undefined) {
        const explicit = input.model ? parseRef(input.model) : undefined;
        if (explicit && explicit.providerID !== pool) return { model: input.model, note: undefined };
        const routing = yield* readRouting;
        const route = explicit ?? routeFor(routing.routes, input.agent);
        if (!route) return { model: undefined, note: undefined };
        const { ref, note } =
          route.providerID === pool
            ? yield* Effect.promise(() => pick(route, existing?.model, routing.fallbacks[`${pool}/${route.id}`]))
            : { ref: route, note: undefined };
        if (!explicit && same(existing?.model, ref)) return { model: undefined, note };
        return { model: format(ref), note };
      });

      const flowOf = (sessionID: string) =>
        ctx
          .rpc(workflow)
          .state({ sessionID })
          .pipe(
            Effect.catchCause((cause) =>
              Effect.logError("lanes: workflow state failed", cause).pipe(Effect.as(undefined)),
            ),
          );

      const bind = (childID: string, step: number) =>
        ctx
          .rpc(workflow)
          .link({ sessionID: childID, step })
          .pipe(
            Effect.asVoid,
            Effect.catchCause((cause) => Effect.logError(`lanes: linking ${childID} to step ${step} failed`, cause)),
          );

      const gate = Effect.fn("lanes.gate")(function* (
        parentID: string,
        agent: string,
        n: number | undefined,
        existing: Session.Info | undefined,
      ) {
        const build = agent.startsWith("build/");
        if (n === undefined && !build) return {};
        const state = yield* flowOf(parentID);
        const linked = existing ? state?.flow?.links[existing.id] : undefined;
        if (n === undefined)
          return linked === undefined && state?.flow?.status === "active"
            ? { nudge: "lanes: a workflow is active; does this work belong on a `step`, or in a new revision?" }
            : {};
        const verdict = admit(state, n, linked === n);
        if ("error" in verdict) return yield* new Tool.Error({ message: verdict.error });
        return linked === n ? {} : { brief: checklist(verdict.step), target: n };
      });

      const run = Effect.fn("lanes.subagent")(function* (
        execute: Tool.Info["execute"],
        raw: Input,
        context: Tool.Context,
      ) {
        const { lane: named, step: n, ...input } = raw;
        const lane = named?.trim() || undefined;
        const key = `${context.sessionID}/${lane}`;
        if (lane && creating.has(key))
          return yield* new Tool.Error({
            message: `Lane "${lane}" is still starting; name it again once it has started.`,
          });
        if (lane) creating.add(key);

        return yield* Effect.gen(function* () {
          const found = lane ? (yield* children(context.sessionID)).find((child) => laneOf(child) === lane) : undefined;
          if (found) creating.delete(key);
          if (found && input.sessionID && input.sessionID !== found.id)
            return yield* new Tool.Error({
              message: `Lane "${lane}" is session ${found.id}, not ${input.sessionID}; pass one of lane or sessionID.`,
            });
          const sessionID = input.sessionID ?? found?.id;
          const existing = found ?? (sessionID ? yield* get(sessionID) : undefined);
          const closed = existing && closedOf(existing);
          if (closed)
            return yield* new Tool.Error({
              message: `Session ${existing.id} is ${closed.lane ? `closed lane "${closed.lane}"` : "closed"} and cannot resume; start a fresh lane by name instead.`,
            });
          const { brief, target, nudge } = yield* gate(context.sessionID, input.agent, n, existing);
          if (existing) yield* expire(existing, context.sessionID);
          const { model, note: routed } = yield* resolve(input, existing);
          const note = [routed, nudge].filter(Boolean).join("\n") || undefined;
          const { model: _, ...rest } = input;
          const call = {
            ...rest,
            ...(brief ? { prompt: `${rest.prompt}\n\n${brief}` } : {}),
            ...(sessionID ? { sessionID } : {}),
            ...(model ? { model } : {}),
          };

          let tagged = false;
          const progress = (update: Tool.Metadata) =>
            Effect.gen(function* () {
              if (!tagged && typeof update.sessionID === "string") {
                tagged = true;
                yield* tag(context.sessionID, update.sessionID, lane);
                if (target !== undefined) yield* bind(update.sessionID, target);
                if (lane && creating.has(key)) prompting.set(update.sessionID, key);
              }
              yield* context.progress(update);
            });

          const result = yield* execute(call, { ...context, progress }).pipe(
            Effect.mapError((error) =>
              note === undefined
                ? error
                : new Tool.Error({
                    message: `${error.message}\n\n${note}`,
                    error: error.error,
                    metadata: error.metadata,
                  }),
            ),
          );
          return {
            ...result,
            content:
              note === undefined
                ? result.content
                : typeof result.content === "string"
                  ? `${result.content}\n\n${note}`
                  : [...(result.content ?? []), { type: "text" as const, text: note }],
            metadata: { ...result.metadata, ...(lane ? { lane } : {}) },
          };
        }).pipe(
          Effect.ensuring(
            Effect.sync(() => {
              if (!lane) return;
              creating.delete(key);
              for (const [childID, pending] of prompting) if (pending === key) prompting.delete(childID);
            }),
          ),
        );
      });

      yield* ctx.tool.transform((editor) => {
        editor.update("subagent", (tool) => {
          const execute = tool.execute;
          const input = tool.input as unknown as Schema.Struct<Schema.Struct.Fields>;
          tool.input = Schema.Struct({
            ...input.fields,
            lane: Schema.optionalKey(Schema.String).annotate({
              description:
                "Stable name for this subagent conversation. Reusing a lane name continues the same child session; naming a running child steers it.",
            }),
            step: Schema.optionalKey(Schema.Int.check(Schema.isGreaterThanOrEqualTo(1))).annotate({
              description: "The workflow step this child carries out; links the child to that step.",
            }),
          });
          tool.execute = (raw, context) => run(execute, raw as Input, context);
        });

        editor.add({
          name: "lanes",
          description:
            "List this session's subagent children with lane name, agent, model, status, last activity, and sessionID.",
          input: Schema.Struct({}),
          options: { codemode: false },
          execute: (_input, context) =>
            Effect.gen(function* () {
              const list = (yield* children(context.sessionID)).toSorted(
                (a, b) => DateTime.toEpochMillis(b.time.updated) - DateTime.toEpochMillis(a.time.updated),
              );
              if (!list.length) return { content: "No subagent children." };
              const rows = list.map((child) =>
                [
                  laneOf(child) ?? closedOf(child)?.lane ?? "-",
                  String(child.agent ?? "-"),
                  child.model ? format(child.model) : "-",
                  closedOf(child) ? "closed" : status(child),
                  ago(child.time.updated),
                  child.id,
                ].join(" | "),
              );
              return { content: ["lane | agent | model | status | last | sessionID", ...rows].join("\n") };
            }),
        });

        editor.add({
          name: "lanes_close",
          description:
            "Close named lanes under this session so they cannot resume; running lanes are skipped. Naming a closed lane again starts a fresh child.",
          input: Schema.Struct({
            lanes: Schema.Array(Schema.String).annotate({ description: "Lane names to close." }),
          }),
          options: { codemode: false },
          execute: (input, context) =>
            close(context.sessionID, input.lanes, "agent").pipe(
              Effect.map((outcomes) => ({
                content:
                  outcomes
                    .map(({ lane, skipped }) => (skipped ? `skipped ${lane}: ${skipped}` : `closed ${lane}`))
                    .join("\n") || "No lane names given.",
              })),
            ),
        });
      });
    }),
});
