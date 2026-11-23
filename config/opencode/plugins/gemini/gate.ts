import { Agent, type Plugin } from "@opencode/plugin";
import { Session } from "@opencode/schema/session";
import type { SessionMessage } from "@opencode/schema/session-message";
import { CallID } from "@opencode/schema/tool";
import type { Called } from "./bridge.ts";
import { readOnly } from "../guard/policy.ts";
import { bridged, check, fill, jot, needs, plan, wrapped, type Call } from "./calls.ts";
import type { Update } from "./session.ts";
import { called, decode, failed, listed, type Tool } from "./tools.ts";
import type { Transcript } from "./transcript.ts";

export type Owner = { session: string | undefined; agent: string | undefined; directory: string };

type Permission = Plugin.Context["permission"];

export const holds = new Map<string, string>();

type Source = { type: "tool"; messageID: string; id: string };

const declined = "The user declined this tool call";

export class Gate {
  declined = false;
  stopped = false;
  message = "";
  private refused: string[] = [];
  private readonly pending = new Set<string>();
  private readonly approved: { id: string; tool: string; input: Record<string, unknown> }[] = [];
  private readonly abort = new AbortController();
  private tools: Promise<Map<string, Tool>> | undefined;

  constructor(
    private readonly permission: Permission,
    private readonly owner: Owner,
    private readonly transcript: Transcript,
    private readonly catalog: () => Promise<Map<string, Tool>>,
  ) {}

  async judge(call: Call & Update): Promise<boolean> {
    this.transcript.call(call);
    if (this.declined || this.stopped) return this.decline(call);
    const session = this.owner.session;
    if (!session) return this.refuse(call, "this request has no OpenCode session to judge tool calls against");
    const steps = plan(call, this.owner.directory);
    if ("refuse" in steps) return this.refuse(call, steps.refuse);
    const tool = bridged(call.name);
    if (tool) {
      const input = Object.fromEntries(Object.entries(steps.input).filter(([key]) => key !== "toolSummary"));
      this.approved.push({ id: call.toolCallId, tool, input });
      return true;
    }
    const checked = await check(steps, { ...this.owner, session });
    if ("refuse" in checked) return this.refuse(call, checked.refuse);
    if (checked.cage && !(await wrapped(call.toolCallId, checked.cage)))
      return this.refuse(
        call,
        "read-only agents run shell commands only inside guard's sandbox, and OpenCode's tool hook did not wrap this one",
      );
    const source = { type: "tool" as const, messageID: this.message, id: call.toolCallId };
    const key = `${session} ${source.messageID} ${source.id}`;
    if (checked.push) holds.set(key, checked.push);
    try {
      await this.grant(session, source, needs(steps.tool, checked.input, this.owner.directory));
    } catch (error) {
      if (declines(error)) {
        this.declined = true;
        return this.decline(call);
      }
      return this.refuse(call, explain(error));
    } finally {
      holds.delete(key);
    }
    return true;
  }

  async write(id: string, input: Record<string, unknown>, live: () => boolean) {
    if (this.declined || this.stopped) return declined;
    const session = this.owner.session;
    if (!session || !readOnly(this.owner.agent))
      return "write_file failed: OpenCode writes through the sandbox only for read-only agents";
    this.transcript.call({ sessionUpdate: "tool_call", toolCallId: id, name: "write_file", rawInput: input });
    const owner = { ...this.owner, session };
    const { error, text } = await this.perform(id, input, owner, live).catch((thrown: unknown) =>
      unwritten(`write_file failed: ${explain(thrown)}`),
    );
    this.transcript.settle(id, text, error);
    return text;
  }

  async list() {
    return [...(await this.available()).values()].flatMap((tool) => listed(tool) ?? []);
  }

  async call(name: string, input: Record<string, unknown>): Promise<Called> {
    const call = this.claim(name, input);
    if (!call)
      return failed(
        this.approved.some((approved) => approved.tool === name)
          ? `the arguments did not match the approved \`${name}\` call, so OpenCode did not run it`
          : `OpenCode approved no pending \`${name}\` call`,
      );
    if (this.declined || this.stopped) return this.fail(call.id, declined);
    const tool = (await this.available()).get(name);
    const { session, agent } = this.owner;
    if (!tool || !session || !agent) return this.fail(call.id, `\`${name}\` is not available to this agent`);
    const checked = await check({ tool: name, input }, { ...this.owner, session });
    if ("refuse" in checked) return this.fail(call.id, checked.refuse);
    const decoded = await decode(tool, checked.input);
    if ("problem" in decoded) return this.fail(call.id, decoded.problem);
    this.pending.add(call.id);
    try {
      const context = {
        sessionID: Session.ID.make(session),
        agent: Agent.ID.make(agent),
        messageID: this.message as SessionMessage.ID,
        id: CallID.make(call.id),
        signal: this.abort.signal,
        progress: async () => {},
      };
      const result = called(await tool.execute(decoded.value, context));
      const shown = result.content.map((item) => (item.type === "text" ? item.text : `[${item.mimeType} image]`));
      this.transcript.settle(call.id, shown.join("\n"), false);
      return result;
    } catch (error) {
      if (!declines(error)) return this.fail(call.id, explain(error));
      this.declined = true;
      return this.fail(call.id, declined);
    } finally {
      this.pending.delete(call.id);
    }
  }

  take() {
    const refused = this.refused;
    this.refused = [];
    return refused;
  }

  async stop() {
    this.stopped = true;
    this.abort.abort();
    const session = this.owner.session;
    if (!session || this.pending.size === 0) return;
    const requests = await this.permission.list({ sessionID: session }).catch(() => []);
    await Promise.allSettled(
      requests
        .filter((request) => request.source && this.pending.has(request.source.id))
        .map((request) => this.permission.reply({ sessionID: session, requestID: request.id, decision: "reject" })),
    );
  }

  private async perform(
    id: string,
    input: Record<string, unknown>,
    owner: Owner & { session: string },
    live: () => boolean,
  ) {
    const job = jot(input, owner);
    if ("refuse" in job) return unwritten(job.refuse);
    const steps = plan({ toolCallId: id, name: "run_command", rawInput: { CommandLine: job.script } }, owner.directory);
    if ("refuse" in steps) return unwritten(`write_file failed: ${steps.refuse}`);
    const checked = await check(steps, owner);
    if ("refuse" in checked) return unwritten(`write_file failed: ${checked.refuse}`);
    if (!checked.cage) return unwritten("write_file failed: the guard did not sandbox this write");
    try {
      const source = { type: "tool" as const, messageID: this.message, id };
      await this.grant(owner.session, source, needs(steps.tool, checked.input, owner.directory));
    } catch (error) {
      if (!declines(error)) return unwritten(`write_file failed: ${explain(error)}`);
      this.declined = true;
      return unwritten(declined);
    }
    if (this.declined || this.stopped) return unwritten(declined);
    if (!live())
      return unwritten(
        "write_file failed: the tool hook stopped waiting before OpenCode approved the write, so nothing was written",
      );
    return fill(checked.cage.CommandLine, checked.cage.Cwd, job.content, job.target);
  }

  private async grant(session: string, source: Source, list: ReturnType<typeof needs>) {
    this.pending.add(source.id);
    try {
      await list.reduce(
        (chain, need) =>
          chain.then(() =>
            this.permission.assert({
              sessionID: session,
              agent: this.owner.agent,
              action: need.action,
              resources: need.resources,
              save: need.save,
              source,
            }),
          ),
        Promise.resolve(),
      );
    } finally {
      this.pending.delete(source.id);
    }
  }

  private refuse(call: Call & Update, reason: string) {
    this.transcript.settle(call.toolCallId, reason, true);
    this.refused.push(`- ${call.title ?? call.name ?? "a tool call"}: ${reason}`);
    return false;
  }

  private decline(call: Call & Update) {
    this.transcript.settle(call.toolCallId, declined, true);
    return false;
  }

  private fail(id: string, reason: string) {
    this.transcript.settle(id, reason, true);
    return failed(reason);
  }

  private available() {
    this.tools ??= this.catalog();
    return this.tools;
  }

  private claim(tool: string, input: Record<string, unknown>) {
    const call = this.approved.find(
      (approved) => approved.tool === tool && Bun.deepEquals(approved.input, input, true),
    );
    if (call) this.approved.splice(this.approved.indexOf(call), 1);
    return call;
  }
}

function declines(error: unknown): boolean {
  if (typeof error !== "object" || error === null) return false;
  return Reflect.get(error, "_tag") === "Permission.DeclinedError" || declines(Reflect.get(error, "error"));
}

function explain(error: unknown) {
  return error instanceof Error && error.message ? error.message : String(error);
}

function unwritten(text: string) {
  return { error: true, text };
}
