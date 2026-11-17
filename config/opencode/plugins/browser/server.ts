import { Plugin } from "@opencode/plugin";
import { Effect, Fiber, Schema } from "effect";
import { Browsers } from "./browsers.ts";
import { Catalog, chrome, engines, MCP, type Engine, type Tool } from "./mcp.ts";
import { leadpier, Preview } from "./preview.ts";
import { redact } from "./redact.ts";

const Target = Schema.Struct({ url: Schema.String });
const navigations = new Set(["navigate_page", "new_page"]);
const web = new Set(["http:", "https:"]);
const hidden = new Set(["chrome-devtools_take_heapsnapshot"]); // Heap snapshots can expose the seeded authentication token.
const redacted = new Set(["firefox-devtools_list_network_requests", "firefox-devtools_get_network_request"]);
const saving = ["saveTo", "preview"];

export default Plugin.define({
  id: "browser",
  async setup(ctx) {
    const fiber = Fiber.getCurrent();
    if (!fiber) throw new Error("browser: setup started outside the plugin fiber");
    const log = Effect.runForkWith(fiber.context);
    const browsers = new Browsers((message, { browser, error }) =>
      log(Effect.logError(message, { browser, error: explain(error) })),
    );
    const preview = new Preview(leadpier);
    const catalogs = new Map<Engine, readonly Tool[]>();

    const close = (sessionID: string) => {
      preview.forget(sessionID);
      browsers.close(sessionID);
    };

    await ctx.tool.transform((editor) => {
      for (const [engine, catalog] of catalogs)
        for (const item of catalog) {
          const name = `${engine.name}_${item.name}`;
          if (hidden.has(name)) continue;
          editor.add({
            name,
            description: item.description ?? item.name,
            input: unsaved(item.inputSchema),
            options: { codemode: false },
            async execute(input, context) {
              check(name, item.name, input);
              const result =
                engine === chrome
                  ? await preview.call(
                      context.sessionID,
                      () => browsers.get(chrome, context.sessionID),
                      item.name,
                      input ?? {},
                    )
                  : await browsers.call(engine, context.sessionID, item.name, input ?? {});
              const content = redacted.has(name)
                ? result.content.map((part) => (part.type === "text" ? { ...part, text: redact(part.text) } : part))
                : result.content;
              const text = content.flatMap((part) => (part.type === "text" ? [part.text] : [])).join("\n\n");
              if (result.isError) throw new Error(text || `${item.name} failed`);
              return {
                content: content.map((part) =>
                  part.type === "text"
                    ? part
                    : { type: "file" as const, uri: `data:${part.mimeType};base64,${part.data}`, mime: part.mimeType },
                ),
              };
            },
          });
        }
    });

    const controller = new AbortController();
    for (const engine of engines)
      discover(ctx, engine)
        .then((tools) => {
          catalogs.set(engine, tools);
          return ctx.tool.reload();
        })
        .catch((error: unknown) =>
          log(Effect.logError("browser: tool discovery failed", { engine: engine.name, error: explain(error) })),
        );

    (async () => {
      for await (const event of ctx.event.subscribe({ signal: controller.signal }))
        if (
          event.type === "session.execution.succeeded" ||
          event.type === "session.execution.failed" ||
          event.type === "session.execution.interrupted" ||
          event.type === "session.deleted"
        )
          close(event.data.sessionID);
    })().catch((error: unknown) => {
      if (!controller.signal.aborted) log(Effect.logError("browser: session watch failed", { error: explain(error) }));
    });

    return async () => {
      controller.abort();
      preview.clear();
      await browsers.clear();
    };
  },
});

function check(name: string, tool: string, input: unknown) {
  if (typeof input === "object" && input !== null && saving.some((key) => key in input))
    throw new Error(`${name} does not save files`);
  const url = navigations.has(tool) && Schema.is(Target)(input) ? input.url : undefined;
  if (url !== undefined && url !== "about:blank" && !web.has(URL.parse(url)?.protocol ?? ""))
    throw new Error(`${tool} only opens http(s) URLs: ${url}`);
}

function unsaved(schema: Tool["inputSchema"]) {
  const { properties } = schema;
  if (properties === null || typeof properties !== "object" || Array.isArray(properties)) return schema;
  return {
    ...schema,
    properties: Object.fromEntries(Object.entries(properties).filter(([key]) => !saving.includes(key))),
  };
}

async function discover(ctx: Plugin.Context, engine: Engine) {
  const key = `catalog/${engine.name}/${Bun.hash(engine.argv.join("\0")).toString(36)}`;
  const cached = await ctx.storage.get(key);
  if (Schema.is(Catalog)(cached)) return cached;
  const client = await MCP.connect(engine);
  try {
    const tools = await client.tools();
    await ctx.storage.set(key, tools);
    return tools;
  } finally {
    client.close();
  }
}

function explain(error: unknown) {
  return error instanceof Error ? error.message : JSON.stringify(error);
}
