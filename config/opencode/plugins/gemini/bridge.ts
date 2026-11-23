import { randomBytes } from "node:crypto";
import { mkdir, readdir, rm } from "node:fs/promises";
import { createServer, type Server, type Socket } from "node:net";
import { join } from "node:path";
import type { Message } from "./acp.ts";
import { bridges, socketEnv, tokenEnv } from "./agy.ts";

export type Listed = { name: string; description: string; inputSchema: Record<string, unknown> };

export type Content = { type: "text"; text: string } | { type: "image"; data: string; mimeType: string };

export type Called = { content: Content[]; isError?: boolean };

export type Tools = {
  list: () => Promise<Listed[]>;
  call: (name: string, input: Record<string, unknown>) => Promise<Called>;
};

export type Spec = { name: string; command: string; args: string[]; env: { name: string; value: string }[] };

export class Bridge {
  private constructor(
    private readonly server: Server,
    private readonly file: string,
    readonly spec: Spec,
  ) {}

  static async open(bun: string, tools: Tools) {
    await mkdir(bridges, { recursive: true, mode: 0o700 });
    await sweep();
    const file = join(bridges, `${process.pid}-${randomBytes(4).toString("hex")}.sock`);
    const token = randomBytes(32).toString("hex");
    const server = createServer((socket) => serve(socket, token, tools));
    await new Promise<void>((resolve, reject) => {
      server.once("error", reject);
      server.listen(file, () => resolve());
    });
    return new Bridge(server, file, {
      name: "opencode",
      command: bun,
      args: ["--config=/dev/null", "--no-env-file", join(import.meta.dir, "shim.ts")],
      env: [
        { name: socketEnv, value: file },
        { name: tokenEnv, value: token },
      ],
    });
  }

  async close() {
    this.server.close();
    await rm(this.file, { force: true });
  }
}

function serve(socket: Socket, token: string, tools: Tools) {
  let buffer = "";
  let trusted = false;
  socket.setEncoding("utf8");
  socket.on("error", () => socket.destroy());
  socket.on("data", (chunk: string) => {
    buffer += chunk;
    for (let end = buffer.indexOf("\n"); end !== -1; end = buffer.indexOf("\n")) {
      const line = buffer.slice(0, end);
      buffer = buffer.slice(end + 1);
      if (trusted) {
        void respond(socket, line, tools);
        continue;
      }
      if (line !== token) return void socket.destroy();
      trusted = true;
    }
  });
}

async function respond(socket: Socket, line: string, tools: Tools) {
  const reply = await answer(line, tools);
  if (reply && socket.writable) socket.write(`${JSON.stringify({ jsonrpc: "2.0", ...reply })}\n`);
}

async function answer(line: string, tools: Tools): Promise<Message | undefined> {
  let message: Message;
  try {
    message = JSON.parse(line);
  } catch {
    return undefined;
  }
  const { id, method, params } = message;
  if (id === undefined || method === undefined) return undefined;
  try {
    if (method === "initialize")
      return {
        id,
        result: {
          protocolVersion: params?.protocolVersion,
          capabilities: { tools: {} },
          serverInfo: { name: "opencode", version: "0" },
        },
      };
    if (method === "ping") return { id, result: {} };
    if (method === "tools/list") {
      const listed = await tools.list();
      return {
        id,
        result: { tools: listed.map((tool) => Object.assign(tool, { _meta: { "antigravity/alwaysLoad": true } })) },
      };
    }
    if (method === "tools/call")
      return { id, result: await tools.call(String(params?.name), record(params?.arguments)) };
    return { id, error: { code: -32601, message: `${method} is not supported` } };
  } catch (error) {
    return { id, error: { code: -32603, message: String(error) } };
  }
}

async function sweep() {
  const names = await readdir(bridges);
  await Promise.all(names.filter(stale).map((name) => rm(join(bridges, name), { force: true })));
}

function stale(name: string) {
  const pid = Number(name.split("-")[0]);
  if (!Number.isInteger(pid) || pid <= 0) return false;
  try {
    process.kill(pid, 0);
    return false;
  } catch (error) {
    return !(error instanceof Error && "code" in error && error.code === "EPERM");
  }
}

function record(value: unknown): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return {};
  return Object.fromEntries(Object.entries(value));
}
