import type { Plugin, PluginModule, ToolContext } from "@opencode-ai/plugin";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { createInterface } from "node:readline";
import { z } from "zod";

const id = "hyprd-browser-isolation";
const clients = new Map<string, Promise<MCPClient>>();

type Tool = {
  name: string;
  description?: string;
  inputSchema: {
    properties?: Record<string, JSONSchema>;
    required?: string[];
  };
};

type Literal = string | number | boolean | null;

type JSONSchema = {
  type?: string;
  description?: string;
  enum?: Literal[];
  items?: JSONSchema;
  properties?: Record<string, JSONSchema>;
  required?: string[];
};

type Content =
  | { type: "text"; text: string }
  | { type: "image"; data: string; mimeType: string }
  | Record<string, unknown>;

type CallResult = { content: Content[]; isError?: boolean };

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
    const result = (await this.request("tools/list", {})) as { tools: Tool[] };
    return result.tools;
  }

  async call(name: string, args: Record<string, unknown>) {
    return (await this.request("tools/call", { name, arguments: args })) as CallResult;
  }

  close() {
    this.child.kill();
  }

  private request(method: string, params: unknown) {
    const id = this.nextID++;
    const result = new Promise((resolve, reject) => this.pending.set(id, { resolve, reject }));
    this.send({ jsonrpc: "2.0", id, method, params });
    return result;
  }

  private receive(line: string) {
    let message: Record<string, unknown>;
    try {
      message = JSON.parse(line) as Record<string, unknown>;
    } catch {
      return;
    }
    if (typeof message.id !== "number") return;

    if (typeof message.method === "string") {
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

function browser(sessionID: string) {
  const existing = clients.get(sessionID);
  if (existing) return existing;

  const opening = MCPClient.connect();
  clients.set(sessionID, opening);
  opening.catch(() => clients.delete(sessionID));
  return opening;
}

function definition(tool: Tool) {
  const required = new Set(tool.inputSchema.required ?? []);
  const args = Object.fromEntries(
    Object.entries(tool.inputSchema.properties ?? {}).map(([name, schema]) => {
      const value = argument(schema);
      return [name, required.has(name) ? value : value.optional()];
    }),
  );
  return {
    description: tool.description ?? tool.name,
    args,
    async execute(args: Record<string, unknown>, ctx: ToolContext) {
      await ctx.ask({
        permission: `chrome-devtools_${tool.name}`,
        patterns: ["*"],
        always: ["*"],
        metadata: {},
      });
      const result = await (await browser(ctx.sessionID)).call(tool.name, args);
      const output = result.content
        .filter((item): item is { type: "text"; text: string } => item.type === "text")
        .map((item) => item.text)
        .join("\n\n");
      if (result.isError) throw new Error(output || `${tool.name} failed`);
      return {
        output,
        attachments: result.content
          .filter((item): item is { type: "image"; data: string; mimeType: string } => item.type === "image")
          .map((item) => ({
            type: "file",
            mime: item.mimeType,
            url: `data:${item.mimeType};base64,${item.data}`,
          })),
      } as never;
    },
  } as never;
}

function argument(schema: JSONSchema): z.ZodType {
  let value: z.ZodType;
  if (schema.enum?.length) {
    value = z.union(schema.enum.map((item) => z.literal(item)) as [z.ZodLiteral<Literal>, ...z.ZodLiteral<Literal>[]]);
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

export default { id, server } satisfies PluginModule;
