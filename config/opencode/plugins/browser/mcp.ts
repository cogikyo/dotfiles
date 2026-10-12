import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { createInterface } from "node:readline";
import { Schema } from "effect";

const Tool = Schema.Struct({
  name: Schema.String,
  description: Schema.optional(Schema.String),
  inputSchema: Schema.JsonObject,
});
export type Tool = typeof Tool.Type;

export const version = "1.7.0";
export const Catalog = Schema.Array(Tool);
const Tools = Schema.Struct({ tools: Catalog });

const Content = Schema.Union([
  Schema.Struct({ type: Schema.Literal("text"), text: Schema.String }),
  Schema.Struct({ type: Schema.Literal("image"), data: Schema.String, mimeType: Schema.String }),
]);
type Content = typeof Content.Type;

const Result = Schema.Struct({ content: Schema.Array(Schema.Unknown), isError: Schema.optional(Schema.Unknown) });

const Message = Schema.Struct({
  id: Schema.Number,
  method: Schema.optional(Schema.String),
  error: Schema.optional(Schema.Unknown),
  result: Schema.optional(Schema.Unknown),
});

export class MCP {
  private readonly child: ChildProcessWithoutNullStreams;
  private next = 1;
  private exit?: Error;
  private readonly done = Promise.withResolvers<void>();
  private readonly pending = new Map<number, { resolve: (value: unknown) => void; reject: (cause: Error) => void }>();

  private constructor(headless: boolean) {
    this.child = spawn(
      "npx",
      [
        "-y",
        `chrome-devtools-mcp@${version}`,
        "--isolated",
        "--executablePath=/usr/bin/chromium",
        "--chromeArg=--opencode-browser-qa",
        "--no-usage-statistics",
        ...(headless ? ["--headless", "--viewport=1920x1080"] : []),
      ],
      { stdio: ["pipe", "pipe", "pipe"] },
    );
    this.child.stderr.resume();
    this.child.stdin.on("error", (cause) => this.fail(cause));
    createInterface({ input: this.child.stdout }).on("line", (line) => this.receive(line));
    this.child.once("error", (cause) => this.fail(cause));
    this.child.once("close", (code) => this.fail(new Error(`Chrome DevTools MCP exited with ${code}`)));
  }

  static async connect(headless: boolean) {
    const client = new MCP(headless);
    try {
      await client.request("initialize", {
        protocolVersion: "2025-06-18",
        capabilities: { roots: {} },
        clientInfo: { name: "opencode-browser", version: "1" },
      });
    } catch (cause) {
      client.close();
      throw cause;
    }
    client.send({ jsonrpc: "2.0", method: "notifications/initialized" });
    return client;
  }

  async tools() {
    return Schema.decodeUnknownSync(Tools)(await this.request("tools/list", {})).tools;
  }

  async call(name: string, args: unknown) {
    const result = Schema.decodeUnknownSync(Result)(await this.request("tools/call", { name, arguments: args }));
    return {
      content: result.content.filter((item): item is Content => Schema.is(Content)(item)),
      isError: result.isError === true,
    };
  }

  get exited() {
    return this.done.promise;
  }

  close() {
    this.child.stdin.end();
    this.child.kill();
  }

  private request(method: string, params: unknown) {
    if (this.exit) return Promise.reject(this.exit);
    const id = this.next++;
    const result = new Promise((resolve, reject) => this.pending.set(id, { resolve, reject }));
    this.send({ jsonrpc: "2.0", id, method, params });
    return result;
  }

  private receive(line: string) {
    let parsed: unknown;
    try {
      parsed = JSON.parse(line);
    } catch {
      return;
    }
    if (!Schema.is(Message)(parsed)) return;
    const { id, method, error, result } = parsed;

    if (method !== undefined) {
      this.send({ jsonrpc: "2.0", id, result: method === "roots/list" ? { roots: [] } : {} });
      return;
    }
    const pending = this.pending.get(id);
    if (!pending) return;
    this.pending.delete(id);
    if (error) pending.reject(new Error(JSON.stringify(error)));
    else pending.resolve(result);
  }

  private send(message: unknown) {
    if (!this.exit) this.child.stdin.write(`${JSON.stringify(message)}\n`);
  }

  private fail(cause: Error) {
    this.exit ??= cause;
    this.done.resolve();
    for (const pending of this.pending.values()) pending.reject(cause);
    this.pending.clear();
  }
}
