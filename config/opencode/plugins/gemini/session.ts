import { existsSync } from "node:fs";
import { mkdir, readFile, rename, rm, writeFile } from "node:fs/promises";
import { homedir } from "node:os";
import { dirname, join } from "node:path";
import { Agent } from "./acp.ts";
import { environment } from "./agy.ts";
import { decide, type Call, type Verdict } from "./policy.ts";

export type Update = {
  sessionUpdate: string;
  toolCallId?: string;
  name?: string;
  kind?: string;
  title?: string;
  status?: string;
  rawInput?: unknown;
  rawOutput?: unknown;
  content?: unknown;
};

export type Usage = {
  inputTokens: number;
  outputTokens: number;
  cachedReadTokens?: number;
  thoughtTokens?: number;
};

export type Result = { stopReason: string; usage?: Usage };

export type Sink = {
  update: (update: Update) => void;
  permission: (call: Call & Update, verdict: Verdict) => void;
};

export type Start = {
  command: string;
  directory: string;
  resume?: string;
};

type Option = { id: string; category?: string; currentValue?: string; options?: { value: string }[] };

type Opened = {
  sessionId: string;
  configOptions?: Option[];
  _meta?: { antigravity?: { tier?: { name?: string } } };
};

export function failure(code: "invalid_request" | "insufficient_quota", message: string) {
  return Object.assign(new Error(message), { code });
}

const paid = /^Google AI (Plus|Pro|Ultra)\b/;
const profile = join(process.env.XDG_STATE_HOME || join(homedir(), ".local", "state"), "opencode", "gemini");
const settings = join(profile, "antigravity-cli", "settings.json");
const strict = `${JSON.stringify({ toolPermission: "strict" })}\n`;
const hooks = join(profile, "config", "hooks.json");
const grace = 5_000;
const stall = 10 * 60_000;

export class Session {
  id = "";
  tier = "";
  resumed = false;
  private readonly agent: Agent;
  private config: Option[] = [];
  private reply: Promise<unknown> = Promise.resolve();
  private timer: Timer | undefined;
  private stalled: ((error: Error) => void) | undefined;

  private constructor(
    private readonly start: Start,
    private readonly sink: Sink,
  ) {
    const env = { ...environment(), GIT_OPTIONAL_LOCKS: "0", GIT_PAGER: "cat", PAGER: "cat" };
    const command = [start.command, "acp", "--sandbox", "--gemini-dir", profile];
    this.agent = new Agent(command, start.directory, env, {
      notify: (method, params) => {
        if (method !== "session/update") return;
        this.touch();
        sink.update(params.update);
      },
      request: (method, params) => {
        this.touch();
        if (method === "session/request_permission") return this.answer(params);
        throw new Error(`${method} is not supported by this client`);
      },
    });
  }

  static async open(start: Start, sink: Sink) {
    const bun = Bun.which("bun");
    if (!bun) throw failure("invalid_request", "bun was not found on PATH; the Gemini read-only hook needs it");
    const unsafe = customizations(start.directory);
    if (unsafe)
      throw failure("invalid_request", `${unsafe} can run code outside the read-only policy; remove it first`);
    const hook = { type: "command", command: `'${bun}' '${join(import.meta.dir, "hook.ts")}'`, timeout: 10 };
    await enforce(settings, strict);
    await enforce(hooks, `${JSON.stringify({ opencode: { PreToolUse: [{ matcher: "*", hooks: [hook] }] } })}\n`);
    const session = new Session(start, sink);
    try {
      await session.handshake();
    } catch (error) {
      session.agent.kill();
      throw error;
    }
    return session;
  }

  async configure(model: string, effort: string | undefined) {
    await this.choose("model", model);
    if (effort !== undefined) await this.choose("thought_level", effort);
  }

  async prompt(text: string): Promise<Result> {
    const reply = this.agent.request("session/prompt", { sessionId: this.id, prompt: [{ type: "text", text }] });
    this.reply = reply.catch(() => undefined);
    const watchdog = new Promise<never>((_, reject) => {
      this.stalled = reject;
    });
    this.touch();
    try {
      return await Promise.race([reply, watchdog]);
    } finally {
      clearTimeout(this.timer);
      this.stalled = undefined;
    }
  }

  async cancel() {
    if (this.id) this.agent.notify("session/cancel", { sessionId: this.id });
    await Promise.race([this.reply, Bun.sleep(grace)]);
    this.agent.kill();
  }

  close() {
    return this.agent.close(grace);
  }

  kill() {
    this.agent.kill();
  }

  private async handshake() {
    await this.agent.request("initialize", {
      protocolVersion: 1,
      clientCapabilities: { fs: { readTextFile: false, writeTextFile: false }, terminal: false },
      clientInfo: { name: "opencode", version: "0" },
    });
    const base = { cwd: this.start.directory, mcpServers: [] };
    let opened: Opened | undefined;
    if (this.start.resume) {
      opened = await this.agent
        .request("session/resume", { ...base, sessionId: this.start.resume })
        .catch(() => undefined);
      this.resumed = opened !== undefined;
    }
    if (!opened) {
      const created: Opened = await this.agent.request("session/new", base);
      opened = created;
    }
    this.tier = opened._meta?.antigravity?.tier?.name ?? "";
    if (!paid.test(this.tier))
      throw failure(
        "invalid_request",
        `Gemini agent reports plan "${this.tier || "none"}", not a paid Google AI plan; refusing to run on it`,
      );
    this.id = opened.sessionId;
    this.config = opened.configOptions ?? [];
  }

  private async choose(category: string, value: string) {
    const option = this.config.find((item) => item.category === category);
    if (!option) throw failure("invalid_request", `Gemini agent offers no ${category} option`);
    if (option.currentValue === value) return;
    const values = (option.options ?? []).map((item) => item.value);
    if (!values.includes(value))
      throw failure(
        "invalid_request",
        `This Google account does not offer ${category} "${value}"; it offers ${values.join(", ")}`,
      );
    const result = await this.agent.request("session/set_config_option", {
      sessionId: this.id,
      configId: option.id,
      value,
    });
    this.config = result?.configOptions ?? this.config;
  }

  private answer(params: { toolCall: Call & Update; options?: { optionId: string; kind: string }[] }) {
    const verdict = decide(params.toolCall, this.start.directory);
    this.sink.permission(params.toolCall, verdict);
    const wanted = verdict.outcome === "allow" ? "allow_once" : "reject_once";
    const option = verdict.outcome === "cancel" ? undefined : params.options?.find((item) => item.kind === wanted);
    return { outcome: option ? { outcome: "selected", optionId: option.optionId } : { outcome: "cancelled" } };
  }

  private touch() {
    if (!this.stalled) return;
    clearTimeout(this.timer);
    const stalled = this.stalled;
    this.timer = setTimeout(
      () => stalled(new Error(`Gemini agent sent no update for ${stall / 60_000} minutes`)),
      stall,
    );
  }
}

function customizations(directory: string) {
  for (let dir = directory; ; dir = dirname(dir)) {
    const found = ["hooks.json", "mcp_config.json", "plugins.json", "plugins"]
      .map((name) => join(dir, ".agents", name))
      .find((file) => existsSync(file));
    if (found) return found;
    if (existsSync(join(dir, ".git")) || dirname(dir) === dir) return undefined;
  }
}

async function enforce(file: string, content: string) {
  const current = await readFile(file, "utf8").catch((error: NodeJS.ErrnoException) => {
    if (error.code === "ENOENT") return undefined;
    throw error;
  });
  if (current === content) return;
  await mkdir(dirname(file), { recursive: true });
  const temp = `${file}.${process.pid}.${crypto.randomUUID()}`;
  try {
    await writeFile(temp, content);
    await rename(temp, file);
  } catch (error) {
    await rm(temp, { force: true });
    throw error;
  }
}
