import { Plugin } from "@opencode/plugin/effect";
import type { Session } from "@opencode/schema/session";
import { Tool } from "@opencode/schema/tool";
import { YAML } from "bun";
import { DateTime, Effect, Option, Schema, Stream } from "effect";
import { readFile } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";
import { claudeAccounts } from "../anthropic/accounts.ts";

type Ref = { providerID: string; id: string; variant?: string };
type Status = "running" | "idle" | "error" | "interrupted";
type Window = { label: string; usedPercent?: number; resetAt?: string };
type Account = { id: string; age?: number; known: boolean; capped: number[]; weekly: number };
type Input = {
  agent: string;
  description: string;
  prompt: string;
  model?: string;
  sessionID?: Session.ID;
  background?: boolean;
  lane?: string;
};

const routingPath = path.join(import.meta.dirname, "..", "..", "ROUTING.md");
const pool = "claude";
const accounts = Object.values(claudeAccounts).map(({ id }) => id);
const staleAfter = 10 * 60_000;

const format = (ref: Ref) => `${ref.providerID}/${ref.id}${ref.variant ? `#${ref.variant}` : ""}`;

function parseRef(value: string): Ref {
  const [model, variant] = value.split("#");
  const slash = model.indexOf("/");
  return { providerID: model.slice(0, slash), id: model.slice(slash + 1), variant: variant || undefined };
}

function parseRoute(key: string, value: unknown): Ref {
  const match = typeof value === "string" ? /^([^\s/]+)\/(\S+?)(?:\s+(\S+))?$/u.exec(value.trim()) : null;
  if (!match) throw new Error(`route "${key}" must be "provider/model [effort]"`);
  return { providerID: match[1], id: match[2], variant: match[3] };
}

function parseRoutes(value: unknown) {
  return Object.fromEntries(Object.entries(value ?? {}).map(([key, route]) => [key, parseRoute(key, route)]));
}

const readRouting = Effect.tryPromise({
  try: async () => {
    const match = /^---\r?\n([\s\S]*?)\r?\n---/u.exec(await readFile(routingPath, "utf8"));
    if (!match) throw new Error("no YAML frontmatter");
    const data = YAML.parse(match[1]) as { routes?: unknown; fallbacks?: unknown };
    return { routes: parseRoutes(data.routes), fallbacks: parseRoutes(data.fallbacks) };
  },
  catch: (error) =>
    new Tool.Error({
      message: `ROUTING.md at ${routingPath} is unusable (${error instanceof Error ? error.message : String(error)}); pass model explicitly.`,
    }),
});

function routeFor(routes: Record<string, Ref>, agent: string) {
  if (Object.hasOwn(routes, agent)) return routes[agent];
  const [key] = Object.keys(routes)
    .filter((pattern) => pattern.includes("*") && glob(pattern).test(agent))
    .toSorted((a, b) => b.length - a.length);
  return key === undefined ? undefined : routes[key];
}

function glob(pattern: string) {
  return new RegExp(
    `^${pattern
      .split("*")
      .map((part) => RegExp.escape(part))
      .join(".*")}$`,
    "u",
  );
}

function cacheFile(providerID: string) {
  const xdg = process.env.XDG_CACHE_HOME?.trim();
  return path.join(
    xdg ? path.resolve(xdg) : path.join(homedir(), ".cache"),
    "opencode",
    "usage-sidebar",
    `${providerID}.json`,
  );
}

async function usage(id: string, modelID: string, now: number): Promise<Account> {
  const unknown = { id, known: false, capped: [], weekly: Infinity };
  const data = await readFile(cacheFile(id), "utf8").then(JSON.parse, () => undefined);
  const fetchedAt = data?.fetchedAt;
  const all: Window[] | undefined = data?.usage?.windows;
  if (typeof fetchedAt !== "number" || !Array.isArray(all)) return unknown;
  const age = now - fetchedAt;
  const scoped = modelID
    .split("-")
    .find((part) => part !== "claude")?.[0]
    ?.toUpperCase();
  const windows = all
    .filter(({ label }) => label === "H" || label === "W" || label === scoped)
    .map((window) => {
      const reset = window.resetAt ? Date.parse(window.resetAt) : undefined;
      const stale = reset !== undefined && reset <= now && fetchedAt <= reset;
      return { ...window, reset: reset ?? Infinity, known: !stale && typeof window.usedPercent === "number" };
    });
  if (!windows.length) return { ...unknown, age };
  return {
    id,
    age,
    known: age <= staleAfter && windows.every((window) => window.known),
    capped: windows.filter((window) => window.known && window.usedPercent! >= 100).map((window) => window.reset),
    weekly: windows.find((window) => window.label === "W" && window.known)?.reset ?? Infinity,
  };
}

async function pick(route: Ref, current: Ref | undefined, fallback: Ref | undefined) {
  const now = Date.now();
  const list = await Promise.all(accounts.map((id) => usage(id, route.id, now)));
  const open = list.filter((account) => !account.capped.length);
  const sticky = open.find((account) => account.id === current?.providerID);
  const stale = list
    .filter((account) => account.age !== undefined && account.age > staleAfter)
    .map((account) => `${account.id} ${Math.round(account.age! / 60_000)}m old`);
  const notes = stale.length
    ? [`lanes: Claude usage is stale (${stale.join(", ")}); picked by last known numbers.`]
    : [];
  if (!open.length && fallback) {
    const ref = { providerID: fallback.providerID, id: fallback.id };
    notes.push(`lanes: both Claude accounts are capped for ${route.id}; using ${format(ref)} with no effort.`);
    return { ref, note: notes.join("\n") };
  }
  const [account] =
    sticky !== undefined
      ? [sticky]
      : open.length
        ? open.toSorted((a, b) => Number(b.known) - Number(a.known) || a.weekly - b.weekly)
        : list.toSorted((a, b) => Math.max(...a.capped) - Math.max(...b.capped));
  return { ref: { ...route, providerID: account.id }, note: notes.join("\n") || undefined };
}

const laneOf = (session: Session.Info) =>
  typeof session.metadata?.lane === "string" ? session.metadata.lane : undefined;

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
            if (event.type === "session.inbox.enqueued") {
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

      const run = Effect.fn("lanes.subagent")(function* (
        execute: Tool.Info["execute"],
        raw: Input,
        context: Tool.Context,
      ) {
        const { lane: named, ...input } = raw;
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
          const { model, note } = yield* resolve(input, existing);
          const { model: _, ...rest } = input;
          const call = { ...rest, ...(sessionID ? { sessionID } : {}), ...(model ? { model } : {}) };

          let tagged = false;
          const progress = (update: Tool.Metadata) =>
            Effect.gen(function* () {
              if (!tagged && typeof update.sessionID === "string") {
                tagged = true;
                yield* tag(context.sessionID, update.sessionID, lane);
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
                  laneOf(child) ?? "-",
                  String(child.agent ?? "-"),
                  child.model ? format(child.model) : "-",
                  status(child),
                  ago(child.time.updated),
                  child.id,
                ].join(" | "),
              );
              return { content: ["lane | agent | model | status | last | sessionID", ...rows].join("\n") };
            }),
        });
      });
    }),
});
