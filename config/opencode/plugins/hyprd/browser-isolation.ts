import { tool, type Plugin, type PluginModule } from "@opencode-ai/plugin";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { createInterface } from "node:readline";
import { z } from "zod";

const id = "hyprd-browser-isolation";
const clients = new Map<string, Promise<MCPClient>>();

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Isolated browser tools                                                                        │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

const server: Plugin = async () => {
  const catalog = await MCPClient.connect();
  const tools = await catalog.tools();
  catalog.close();

  return {
    tool: Object.fromEntries(tools.map((item) => [`chrome-devtools_${item.name}`, definition(item)])),
    event: async ({ event }) => {
      if (event.type !== "session.deleted" && event.type !== "session.idle") return;
      const sessionID = event.type === "session.deleted" ? event.properties.info.id : event.properties.sessionID;
      const client = clients.get(sessionID);
      clients.delete(sessionID);
      (await client)?.close();
    },
  };
};

/** Exposes permission-gated Chrome DevTools tools through session-isolated MCP processes. */
export default { id, server } satisfies PluginModule;

// ├─ MCP messages ────────────────────────────────────────────────────────────────────────────────┤

type JSONSchema = {
  type?: string;
  description?: string;
  enum?: Array<string | number | boolean | null>;
  items?: JSONSchema;
  properties?: Record<string, JSONSchema>;
  required?: string[];
};

const label = z.string().optional().catch(undefined);

// Only this JSON Schema subset is translated to OpenCode tool arguments.
const JSONSchema: z.ZodType<JSONSchema> = z.lazy(() =>
  z.object({
    type: label,
    description: label,
    enum: z.array(z.union([z.string(), z.number(), z.boolean(), z.null()])).optional(),
    items: JSONSchema.optional(),
    properties: z.record(z.string(), JSONSchema).optional(),
    required: z.array(z.string()).optional(),
  }),
);

const Tool = z.object({
  name: z.string(),
  description: label,
  inputSchema: z.object({
    properties: z.record(z.string(), JSONSchema).optional(),
    required: z.array(z.string()).optional(),
  }),
});
type Tool = z.infer<typeof Tool>;

const Tools = z.object({ tools: z.array(Tool) });

const Content = z.union([
  z.object({ type: z.literal("text"), text: z.string() }),
  z.object({ type: z.literal("image"), data: z.string(), mimeType: z.string() }),
]);
const Result = z.object({
  content: z.array(z.unknown()).transform((items) =>
    items.flatMap((item) => {
      const parsed = Content.safeParse(item);
      return parsed.success ? [parsed.data] : [];
    }),
  ),
  isError: z.unknown().transform((value) => value === true),
});

// MCP messages are newline-delimited JSON-RPC; replies need a numeric ID.
const Message = z.object({ id: z.number(), method: z.string().optional(), error: z.unknown(), result: z.unknown() });

// ├─ MCP client ──────────────────────────────────────────────────────────────────────────────────┤

class MCPClient {
  private child: ChildProcessWithoutNullStreams;
  private nextID = 1;
  private pending = new Map<number, { resolve: (value: unknown) => void; reject: (cause: Error) => void }>();

  private constructor() {
    this.child = spawn(
      "npx",
      [
        "-y",
        "chrome-devtools-mcp@1.7.0",
        "--isolated",
        "--executablePath=/usr/bin/chromium",
        "--chromeArg=--opencode-browser-qa",
        "--no-usage-statistics",
      ],
      { stdio: ["pipe", "pipe", "pipe"] },
    );
    this.child.stderr.resume();
    createInterface({ input: this.child.stdout }).on("line", (line) => this.receive(line));
    this.child.once("error", (cause) => this.fail(cause));
    this.child.once("close", () => this.fail(new Error("Chrome DevTools MCP exited")));
  }

  static async connect() {
    const client = new MCPClient();
    await client.request("initialize", {
      protocolVersion: "2025-06-18",
      capabilities: { roots: {} },
      clientInfo: { name: id, version: "1" },
    });
    client.send({ jsonrpc: "2.0", method: "notifications/initialized" });
    return client;
  }

  async tools() {
    const result = Tools.safeParse(await this.request("tools/list", {}));
    if (!result.success) throw new Error(`Invalid MCP tool list: ${z.prettifyError(result.error)}`);
    return result.data.tools;
  }

  async call(name: string, args: Record<string, unknown>) {
    const result = Result.safeParse(await this.request("tools/call", { name, arguments: args }));
    if (!result.success) throw new Error("Invalid MCP tool result");
    return result.data;
  }

  close() {
    this.child.kill();
  }

  private request(method: string, params: unknown) {
    const seq = this.nextID++;
    const result = new Promise((resolve, reject) => this.pending.set(seq, { resolve, reject }));
    this.send({ jsonrpc: "2.0", id: seq, method, params });
    return result;
  }

  // MCP server requests get empty responses except `roots/list`, which gets an empty root list.
  private receive(line: string) {
    let parsed;
    try {
      parsed = Message.safeParse(JSON.parse(line));
    } catch {
      return;
    }
    if (!parsed.success) return;
    const message = parsed.data;

    if (message.method !== undefined) {
      const result = message.method === "roots/list" ? { roots: [] } : {};
      this.send({ jsonrpc: "2.0", id: message.id, result });
      return;
    }
    const pending = this.pending.get(message.id);
    if (!pending) return;
    this.pending.delete(message.id);
    if (message.error) pending.reject(new Error(JSON.stringify(message.error)));
    else pending.resolve(message.result);
  }

  private send(message: unknown) {
    this.child.stdin.write(`${JSON.stringify(message)}\n`);
  }

  private fail(cause: Error) {
    for (const pending of this.pending.values()) pending.reject(cause);
    this.pending.clear();
  }
}

// ├─ Session tools ───────────────────────────────────────────────────────────────────────────────┤

// Failed session connections are discarded so a later call can retry.
function browser(sessionID: string) {
  const existing = clients.get(sessionID);
  if (existing) return existing;

  const opening = MCPClient.connect();
  clients.set(sessionID, opening);
  opening.catch(() => clients.delete(sessionID));
  return opening;
}

function definition(item: Tool) {
  const required = new Set(item.inputSchema.required ?? []);
  const shape = Object.fromEntries(
    Object.entries(item.inputSchema.properties ?? {}).map(([name, schema]) => {
      const value = argument(schema);
      return [name, required.has(name) ? value : value.optional()];
    }),
  );
  return tool({
    description: item.description ?? item.name,
    args: shape,
    async execute(args, ctx) {
      await ctx.ask({
        permission: `chrome-devtools_${item.name}`,
        patterns: ["*"],
        always: ["*"],
        metadata: {},
      });
      const result = await (await browser(ctx.sessionID)).call(item.name, args);
      const output = result.content.flatMap((part) => (part.type === "text" ? [part.text] : [])).join("\n\n");
      if (result.isError) throw new Error(output || `${item.name} failed`);
      return {
        output,
        attachments: result.content.flatMap((part) =>
          part.type === "image"
            ? [{ type: "file", mime: part.mimeType, url: `data:${part.mimeType};base64,${part.data}` }]
            : [],
        ),
      };
    },
  });
}

// MCP tool arguments ignore JSON Schema unions and numeric bounds.
function argument(schema: JSONSchema): z.ZodType {
  let value: z.ZodType;
  if (schema.enum?.length) {
    value = z.literal(schema.enum);
  } else {
    switch (schema.type) {
      case "string":
        value = z.string();
        break;
      case "integer":
        value = z.number().int();
        break;
      case "number":
        value = z.number();
        break;
      case "boolean":
        value = z.boolean();
        break;
      case "array":
        value = z.array(schema.items ? argument(schema.items) : z.unknown());
        break;
      case "object": {
        const required = new Set(schema.required ?? []);
        value = z.object(
          Object.fromEntries(
            Object.entries(schema.properties ?? {}).map(([name, item]) => {
              const nested = argument(item);
              return [name, required.has(name) ? nested : nested.optional()];
            }),
          ),
        );
        break;
      }
      default:
        value = z.unknown();
    }
  }
  return schema.description ? value.describe(schema.description) : value;
}
