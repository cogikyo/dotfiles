import { Plugin } from "@opencode/plugin/effect";
import type { Session } from "@opencode/schema/session";
import type { SessionMessage } from "@opencode/schema/session-message";
import { Tool } from "@opencode/schema/tool";
import { DateTime, Effect, Option, Schema, Stream } from "effect";
import { readFile } from "node:fs/promises";
import { ranked, reopens, staleAfter, survey } from "../anthropic/pool.ts";
import { held, located, opened } from "../mail/store.ts";
import { api as workflow } from "../workflow/rpc.ts";
import { count, measured } from "./label.ts";
import { parseRouting, routeFor, routingPath, type Ref } from "./routing.ts";
import { api, type Outcome } from "./rpc.ts";
import { admit, checklist } from "./step.ts";

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

const inbox = "SELECT EXISTS (SELECT 1 FROM session_inbox WHERE session_id = ?1) AS pending";

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
  by: Schema.Literals(["operator", "agent", "cache", "workflow"]),
  at: Schema.Number,
});
type Closed = typeof Closed.Type;

const laneOf = (session: Session.Info) =>
  typeof session.metadata?.lane === "string" ? session.metadata.lane : undefined;

const closedOf = (session: Session.Info) =>
  Option.getOrUndefined(Schema.decodeUnknownOption(Closed)(session.metadata?.closed));

const find = (list: readonly Session.Info[], key: string) =>
  list.find((session) => session.id === key) ?? list.find((session) => laneOf(session) === key);

const compactable = (messages: readonly SessionMessage.Info[]) =>
  messages.some((message) => {
    switch (message.type) {
      case "user":
      case "synthetic":
      case "skill":
      case "location-switched":
        return true;
      case "shell":
        return message.metadata?.background !== true;
      case "assistant":
        return message.content.some((part) => part.type !== "reasoning");
      default:
        return false;
    }
  });

const skip = (key: string, why: string) => Effect.succeed(`skipped ${key}: ${why}`);

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
      const file = located(ctx.app.channel);
      const live = new Map<string, Status>();
      const creating = new Set<string>();
      const prompting = new Map<string, string>();

      const status = (session: Session.Info): Status =>
        live.get(session.id) ??
        (session.outcome === "failed" ? "error" : session.outcome === "interrupted" ? "interrupted" : "idle");

      const get = (sessionID: string) =>
        ctx.session.get({ sessionID: sessionID as Session.ID }).pipe(Effect.option, Effect.map(Option.getOrUndefined));

      const keys = Effect.fn("lanes.keys")(function* (prefix: string) {
        const ids: string[] = [];
        let after: string | undefined;
        do {
          const page = yield* ctx.storage.scan({ prefix, after, limit: 1000 });
          ids.push(...page.entries.map((entry) => entry.key.slice(prefix.length)));
          after = page.next;
        } while (after);
        return ids;
      });

      const children = Effect.fn("lanes.children")(function* (parentID: string) {
        const ids = yield* keys(`child/${parentID}/`);
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

      const busy = (parentID: string, child: Session.Info) => {
        const lane = laneOf(child);
        return (
          status(child) === "running" ||
          held(child.id, file) ||
          (lane !== undefined && creating.has(`${parentID}/${lane}`))
        );
      };

      const close = Effect.fn("lanes.close")(function* (parentID: string, names: readonly string[], by: Closed["by"]) {
        const list = yield* children(parentID);
        const outcomes: Outcome[] = [];
        const sealed = new Set<string>();
        for (const key of new Set(names.map((name) => name.trim()).filter(Boolean))) {
          const child = find(list, key);
          const closed = child
            ? sealed.has(child.id) || closedOf(child) !== undefined
            : list.some((session) => closedOf(session)?.lane === key);
          if (!child || closed) {
            outcomes.push({ lane: key, skipped: closed ? "already closed" : "missing" });
            continue;
          }
          if (busy(parentID, child)) {
            outcomes.push({ lane: key, skipped: "running" });
            continue;
          }
          yield* seal(child, by);
          sealed.add(child.id);
          outcomes.push({ lane: key });
        }
        return outcomes;
      });

      const size = (child: Session.Info) =>
        ctx.session.context({ sessionID: child.id }).pipe(Effect.map((messages) => measured(messages)?.tokens));

      const queued = (sessionID: string) => {
        try {
          const pending = opened(file, (db) => db.query<{ pending: number }, [string]>(inbox).get(sessionID)?.pending);
          return pending === 0 ? undefined : "pending input";
        } catch {
          return "inbox unreadable";
        }
      };

      const blocked = (parentID: string, child: Session.Info) =>
        closedOf(child) ? "closed" : busy(parentID, child) ? "running" : queued(child.id);

      const shrink = (parentID: string, key: string, child: Session.Info) =>
        Effect.gen(function* () {
          if (!compactable(yield* ctx.session.context({ sessionID: child.id })))
            return `skipped ${key}: nothing to compact`;
          const current = yield* get(child.id);
          const why = current ? blocked(parentID, current) : "missing";
          if (why) return `skipped ${key}: ${why}`;
          const admitted = yield* ctx.session.compact({ sessionID: child.id, delivery: "queue" });
          yield* ctx.session.wait({ sessionID: child.id });
          const marker = (yield* ctx.session.context({ sessionID: child.id })).find(
            (message) => message.id === admitted.id,
          );
          if (marker?.type !== "compaction" || marker.status === "running")
            return `failed ${key}: compaction did not complete`;
          return marker.status === "completed" ? `compacted ${key}` : `failed ${key}: ${marker.error.message}`;
        }).pipe(
          Effect.catch((error) =>
            Effect.succeed(`failed ${key}: ${error instanceof Error ? error.message : String(error)}`),
          ),
        );

      const compact = Effect.fn("lanes.compact")(function* (parentID: string, names: readonly string[]) {
        const list = yield* children(parentID);
        const runs: Effect.Effect<string>[] = [];
        const seen = new Set<string>();
        for (const key of names.map((name) => name.trim()).filter(Boolean)) {
          const child = find(list, key);
          const twice = seen.has(key) || (child !== undefined && seen.has(child.id));
          seen.add(key);
          if (child) seen.add(child.id);
          if (twice) {
            runs.push(skip(key, "listed twice"));
            continue;
          }
          if (!child) {
            runs.push(skip(key, list.some((session) => closedOf(session)?.lane === key) ? "closed" : "missing"));
            continue;
          }
          const why = blocked(parentID, child);
          runs.push(why ? skip(key, why) : shrink(parentID, key, child));
        }
        return yield* Effect.all(runs, { concurrency: "unbounded" });
      });

      const retire = (childID: string) =>
        Effect.gen(function* () {
          const key = `retire/${childID}`;
          // Register retirement before reading the child, or a settle event can miss the request and leave it open.
          yield* ctx.storage.set(key, true);
          const child = yield* get(childID);
          const [outcome] = child?.parentID ? yield* close(child.parentID, [child.id], "workflow") : [];
          if (outcome?.skipped !== "running") yield* ctx.storage.remove(key);
        }).pipe(
          Effect.catchCause((cause) => Effect.logError(`lanes: closing ${childID} at workflow end failed`, cause)),
        );

      yield* ctx.event.subscribe().pipe(
        Stream.runForEach((event) =>
          Effect.gen(function* () {
            if (event.type === "session.execution.started") live.set(event.data.sessionID, "running");
            if (event.type === "session.execution.succeeded") live.set(event.data.sessionID, "idle");
            if (event.type === "session.execution.failed") live.set(event.data.sessionID, "error");
            if (event.type === "session.execution.interrupted") live.set(event.data.sessionID, "interrupted");
            if (event.type === "session.inbox.enqueued" && event.data.item.type === "user") {
              const key = prompting.get(event.data.sessionID);
              if (key !== undefined) creating.delete(key);
              prompting.delete(event.data.sessionID);
            }
            const settled =
              event.type === "session.execution.succeeded" ||
              event.type === "session.execution.failed" ||
              event.type === "session.execution.interrupted";
            if (settled && (yield* ctx.storage.get(`retire/${event.data.sessionID}`)) !== undefined)
              yield* retire(event.data.sessionID);
          }),
        ),
        Effect.tapCause(Effect.logError),
        Effect.ignore,
        Effect.forkScoped,
      );

      yield* ctx
        .rpc(workflow)
        .events.subscribe("changed")
        .pipe(
          Stream.runForEach(({ data }) => Effect.forEach(Object.keys(data.ended ?? {}), retire, { discard: true })),
          Effect.tapCause(Effect.logError),
          Effect.ignore,
          Effect.forkScoped,
        );

      yield* keys("retire/").pipe(
        Effect.flatMap((ids) => Effect.forEach(ids, retire, { discard: true })),
        Effect.tapCause(Effect.logError),
        Effect.ignore,
        Effect.forkScoped,
      );

      yield* ctx.rpc
        .register(api, {
          close: ({ sessionID, lanes }) => close(sessionID, lanes, "operator").pipe(Effect.orDie),
        })
        .pipe(Effect.orDie);

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
        if (n === undefined && !build && !existing) return {};
        const state = yield* flowOf(parentID);
        const linked = existing ? state?.flow?.links[existing.id] : undefined;
        const step = n ?? linked;
        if (step === undefined)
          return build && state?.flow?.status === "active"
            ? { nudge: "lanes: a workflow is active; does this work belong on a `step`, or in a new revision?" }
            : {};
        const verdict = admit(state, step, linked === step);
        if ("error" in verdict) return yield* new Tool.Error({ message: verdict.error });
        return linked === step ? {} : { brief: checklist(verdict.step), target: step };
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
            "List this session's subagent children with lane name, agent, model, status, context tokens, last activity, and sessionID.",
          input: Schema.Struct({}),
          options: { codemode: false },
          execute: (_input, context) =>
            Effect.gen(function* () {
              const list = (yield* children(context.sessionID)).toSorted(
                (a, b) => DateTime.toEpochMillis(b.time.updated) - DateTime.toEpochMillis(a.time.updated),
              );
              if (!list.length) return { content: "No subagent children." };
              const sizes = yield* Effect.forEach(
                list,
                (child) => size(child).pipe(Effect.option, Effect.map(Option.getOrUndefined)),
                { concurrency: "unbounded" },
              );
              const rows = list.map((child, at) =>
                [
                  laneOf(child) ?? closedOf(child)?.lane ?? "-",
                  String(child.agent ?? "-"),
                  child.model ? format(child.model) : "-",
                  closedOf(child) ? "closed" : status(child),
                  sizes[at] === undefined ? "-" : count(sizes[at]),
                  ago(child.time.updated),
                  child.id,
                ].join(" | "),
              );
              return { content: ["lane | agent | model | status | tokens | last | sessionID", ...rows].join("\n") };
            }),
        });

        editor.add({
          name: "lanes_compact",
          description:
            "Compact idle subagent children under this session and wait for each summary; running and closed children are skipped. Pass lane names, or session IDs for unnamed children. The child is not prompted afterward; naming the lane again resumes it from the summary.",
          input: Schema.Struct({
            lanes: Schema.Array(Schema.String).annotate({ description: "Lane names or child session IDs to compact." }),
          }),
          options: { codemode: false },
          execute: (input, context) =>
            compact(context.sessionID, input.lanes).pipe(
              Effect.map((lines) => ({ content: lines.join("\n") || "No lane names or session IDs given." })),
            ),
        });

        editor.add({
          name: "lanes_close",
          description:
            "Close subagent children under this session so they cannot resume; running children are skipped. Pass lane names, or session IDs for unnamed children. Naming a closed lane again starts a fresh child.",
          input: Schema.Struct({
            lanes: Schema.Array(Schema.String).annotate({ description: "Lane names or child session IDs to close." }),
          }),
          options: { codemode: false },
          execute: (input, context) =>
            close(context.sessionID, input.lanes, "agent").pipe(
              Effect.map((outcomes) => ({
                content:
                  outcomes
                    .map(({ lane, skipped }) => (skipped ? `skipped ${lane}: ${skipped}` : `closed ${lane}`))
                    .join("\n") || "No lane names or session IDs given.",
              })),
            ),
        });
      });
    }),
});
