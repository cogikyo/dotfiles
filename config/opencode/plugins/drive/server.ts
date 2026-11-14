import { Plugin } from "@opencode/plugin";
import { Effect, Fiber, Schema } from "effect";
import { api } from "./rpc.ts";

const PREFIX = "root/";
const DECIDER = "verify/decide";

const Drive = Schema.Struct({ on: Schema.Boolean, at: Schema.String });
type Drive = typeof Drive.Type;
const valid = Schema.is(Drive);

export function line(drive: Drive) {
  return `Drive is ${drive.on ? "on" : "off"} since ${drive.at}.`;
}

function explain(error: unknown) {
  return error instanceof Error ? error.message : JSON.stringify(error);
}

const missing = Schema.is(Schema.Struct({ _tag: Schema.Literal("Session.NotFoundError") }));

async function sweep(
  ctx: Plugin.Context,
  forget: (sessionID: string) => Promise<unknown>,
  fail: (what: string, error: unknown) => unknown,
  after?: string,
) {
  const page = await ctx.storage.scan({ prefix: PREFIX, after, limit: 100 });
  const ids = page.entries.map(({ key }) => key.slice(PREFIX.length));
  const gone = await Promise.all(
    ids.map((sessionID) =>
      ctx.session.get({ sessionID }).then(
        () => undefined,
        (error: unknown) => {
          if (missing(error)) return sessionID;
          fail(`sweep lookup of ${sessionID}`, error);
          return undefined;
        },
      ),
    ),
  );
  await Promise.all(gone.filter((id) => id !== undefined).map(forget));
  if (page.next) await sweep(ctx, forget, fail, page.next);
}

export default Plugin.define({
  id: "drive",
  async setup(ctx) {
    const fiber = Fiber.getCurrent();
    if (!fiber) throw new Error("drive: setup started outside the plugin fiber");
    const log = Effect.runForkWith(fiber.context);
    const fail = (what: string, error: unknown) =>
      log(Effect.logError(`drive: ${what} failed`, { error: explain(error) }));

    const read = async (root: string) => {
      const value = await ctx.storage.get(PREFIX + root);
      return valid(value) ? value : undefined;
    };

    const root = async (sessionID: string): Promise<string> => {
      const session = await ctx.session.get({ sessionID });
      return session.parentID ? root(session.parentID) : session.id;
    };

    const reader = async (sessionID: string, agent: string) => {
      const session = await ctx.session.get({ sessionID });
      if (!session.parentID) return session.id;
      if (agent !== DECIDER) return;
      const parent = await ctx.session.get({ sessionID: session.parentID });
      return parent.parentID ? undefined : parent.id;
    };

    const rpc = await ctx.rpc.register(api, {
      state: async ({ sessionID }) => {
        const id = await root(sessionID);
        return { root: id, on: (await read(id))?.on ?? false };
      },
      set: async ({ sessionID, on }) => {
        const id = await root(sessionID);
        return { root: id, on: (await set(id, on))?.on ?? false };
      },
    });

    const set = async (id: string, on: boolean) => {
      const current = await read(id);
      if ((current?.on ?? false) === on) return current;
      const next = { on, at: new Date().toISOString().replace(/\.\d+Z$/, "Z") };
      await ctx.storage.set(PREFIX + id, next);
      await rpc.events.emit("changed", { root: id, on });
      return next;
    };

    await ctx.session.hook("context", async (event) => {
      try {
        const id = await reader(event.sessionID, event.agent);
        if (!id) return;
        const drive = await read(id);
        event.system.push({ type: "text", text: drive ? line(drive) : "Drive is off." });
      } catch (error) {
        fail("context", error);
        event.system.push({ type: "text", text: "Drive state is unavailable; treat drive as off." });
      }
    });

    await ctx.tool.transform((editor) =>
      editor.add({
        name: "drive_off",
        description:
          "Turn drive off for this session tree when the user says to stop driving. Only the root session may call it.",
        input: Schema.Struct({}),
        options: { codemode: false },
        execute: async (_input, context) => {
          const session = await ctx.session.get({ sessionID: context.sessionID });
          if (session.parentID) throw new Error("drive_off: only the root session can turn drive off");
          const was = (await read(session.id))?.on ?? false;
          await set(session.id, false);
          return { content: was ? "Drive turned off." : "Drive was already off." };
        },
      }),
    );

    const forget = (sessionID: string) =>
      ctx.storage.remove(PREFIX + sessionID).catch((error: unknown) => fail("forget deleted session", error));

    const controller = new AbortController();
    const watch = async () => {
      for await (const event of ctx.event.subscribe({ types: ["session.deleted"], signal: controller.signal }))
        await forget(event.data.sessionID);
    };
    watch().catch((error: unknown) => {
      if (!controller.signal.aborted) fail("event stream", error);
    });
    sweep(ctx, forget, fail).catch((error: unknown) => fail("sweep deleted sessions", error));

    return () => controller.abort();
  },
});
