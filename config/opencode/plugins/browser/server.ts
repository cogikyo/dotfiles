import { Plugin } from "@opencode/plugin";
import { Schema } from "effect";
import { api as drive } from "../drive/rpc.ts";
import { Catalog, MCP, version, type Tool } from "./mcp.ts";

const Target = Schema.Struct({ url: Schema.String });
const web = new Set(["http:", "https:"]);

export default Plugin.define({
  id: "browser",
  async setup(ctx) {
    const browsers = new Map<string, Promise<MCP>>();
    let catalog: readonly Tool[] = [];

    const forget = (sessionID: string, browser: Promise<MCP>) => {
      if (browsers.get(sessionID) === browser) browsers.delete(sessionID);
    };

    const open = (sessionID: string) => {
      const existing = browsers.get(sessionID);
      if (existing) return existing;
      const opening = ctx
        .rpc(drive)
        .state({ sessionID })
        .then(({ armed }) => MCP.connect(armed));
      browsers.set(sessionID, opening);
      opening.then(
        (browser) => browser.exited.then(() => forget(sessionID, opening)),
        () => forget(sessionID, opening),
      );
      return opening;
    };

    const close = async (sessionID: string) => {
      const browser = browsers.get(sessionID);
      browsers.delete(sessionID);
      (await browser?.catch(() => undefined))?.close();
    };

    await ctx.tool.transform((editor) => {
      for (const item of catalog)
        editor.add({
          name: `chrome-devtools_${item.name}`,
          description: item.description ?? item.name,
          input: item.inputSchema,
          options: { codemode: false },
          async execute(input, context) {
            const url = Schema.is(Target)(input) ? input.url : undefined;
            if (url !== undefined && url !== "about:blank" && !web.has(URL.parse(url)?.protocol ?? ""))
              throw new Error(`${item.name} only opens http(s) URLs: ${url}`);
            const result = await (await open(context.sessionID)).call(item.name, input ?? {});
            const text = result.content.flatMap((part) => (part.type === "text" ? [part.text] : [])).join("\n\n");
            if (result.isError) throw new Error(text || `${item.name} failed`);
            return {
              content: result.content.map((part) =>
                part.type === "text"
                  ? part
                  : { type: "file" as const, uri: `data:${part.mimeType};base64,${part.data}`, mime: part.mimeType },
              ),
            };
          },
        });
    });

    const controller = new AbortController();
    discover(ctx)
      .then((tools) => {
        catalog = tools;
        return ctx.tool.reload();
      })
      .catch((error: unknown) => console.error(`browser: tool discovery failed: ${explain(error)}`));

    (async () => {
      for await (const event of ctx.event.subscribe({ signal: controller.signal }))
        if (event.type === "session.idle" || event.type === "session.deleted") await close(event.data.sessionID);
    })().catch((error: unknown) => {
      if (!controller.signal.aborted) console.error(`browser: session watch failed: ${explain(error)}`);
    });

    return async () => {
      controller.abort();
      await Promise.all([...browsers.keys()].map(close));
    };
  },
});

async function discover(ctx: Plugin.Context) {
  const key = `catalog/${version}`;
  const cached = await ctx.storage.get(key);
  if (Schema.is(Catalog)(cached)) return cached;
  const client = await MCP.connect(true);
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
