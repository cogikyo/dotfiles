import { tool, type Plugin, type PluginModule } from "@opencode-ai/plugin";
import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { createInterface } from "node:readline";
import { z } from "zod";
import { record } from "../shared/record.ts";

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
    const result = record(await this.request("tools/list", {}));
    if (!result || !Array.isArray(result.tools)) throw new Error("Invalid MCP tool list");
    return result.tools.map(parseTool);
  }

  async call(name: string, args: Record<string, unknown>) {
    const result = record(await this.request("tools/call", { name, arguments: args }));
    if (!result || !Array.isArray(result.content)) throw new Error("Invalid MCP tool result");
    return {
      content: result.content.map(record).filter((item) => item !== undefined),
      isError: result.isError === true,
    };
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

  private receive(line: string) {
    let message: Record<string, unknown> | undefined;
    try {
      message = record(JSON.parse(line));
    } catch {
      return;
    }
    if (!message || typeof message.id !== "number") return;

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
      const output = result.content
        .filter(textContent)
        .map((part) => part.text)
        .join("\n\n");
      if (result.isError) throw new Error(output || `${item.name} failed`);
      return {
        output,
        attachments: result.content.filter(imageContent).map((part) => ({
          type: "file",
          mime: part.mimeType,
          url: `data:${part.mimeType};base64,${part.data}`,
        })),
      };
    },
  });
}

function parseTool(value: unknown): Tool {
  const entry = record(value);
  const input = record(entry?.inputSchema);
  if (typeof entry?.name !== "string" || !input) throw new Error("Invalid MCP tool definition");
  const properties = parseProperties(input.properties);
  const required = input.required;
  if (required !== undefined && !stringList(required)) {
    throw new Error("Invalid MCP required properties");
  }
  return {
    name: entry.name,
    ...(typeof entry.description === "string" ? { description: entry.description } : {}),
    inputSchema: {
      ...(properties ? { properties } : {}),
      ...(required ? { required } : {}),
    },
  };
}

function parseSchema(value: unknown): JSONSchema {
  const entry = record(value);
  if (!entry) throw new Error("Invalid MCP input schema");
  const properties = parseProperties(entry.properties);
  const values = entry.enum;
  if (values !== undefined && !literalList(values)) {
    throw new Error("Invalid MCP schema enum");
  }
  const required = entry.required;
  if (required !== undefined && !stringList(required)) {
    throw new Error("Invalid MCP schema required properties");
  }
  return {
    ...(typeof entry.type === "string" ? { type: entry.type } : {}),
    ...(typeof entry.description === "string" ? { description: entry.description } : {}),
    ...(values ? { enum: values } : {}),
    ...(entry.items !== undefined ? { items: parseSchema(entry.items) } : {}),
    ...(properties ? { properties } : {}),
    ...(required ? { required } : {}),
  };
}

function parseProperties(value: unknown): Record<string, JSONSchema> | undefined {
  if (value === undefined) return undefined;
  const entries = record(value);
  if (!entries) throw new Error("Invalid MCP schema properties");
  return Object.fromEntries(Object.entries(entries).map(([key, item]) => [key, parseSchema(item)]));
}

function stringList(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

function literalList(value: unknown): value is Literal[] {
  return Array.isArray(value) && value.every(isLiteral);
}

function isLiteral(value: unknown): value is Literal {
  return value === null || typeof value === "string" || typeof value === "number" || typeof value === "boolean";
}

function textContent(value: Record<string, unknown>): value is Record<string, unknown> & { text: string } {
  return value.type === "text" && typeof value.text === "string";
}

function imageContent(
  value: Record<string, unknown>,
): value is Record<string, unknown> & { data: string; mimeType: string } {
  return value.type === "image" && typeof value.data === "string" && typeof value.mimeType === "string";
}

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

/** Server plugin that registers isolated Chrome DevTools MCP tools and handles `session.idle` and `session.deleted`. */
export default { id, server } satisfies PluginModule;
