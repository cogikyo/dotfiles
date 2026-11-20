import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { Plugin } from "@opencode/plugin";
import { review } from "../guard/review.ts";
import { cages } from "./agy.ts";
import { needs, plan, sandbox, type Call, type Plan } from "./calls.ts";
import type { Update } from "./session.ts";
import type { Transcript } from "./transcript.ts";

export type Owner = { session: string | undefined; agent: string | undefined; directory: string };

type Permission = Plugin.Context["permission"];

export const holds = new Map<string, string>();

const declined = "The user declined this tool call";

export class Gate {
  declined = false;
  stopped = false;
  message = "";
  private refused: string[] = [];
  private readonly pending = new Set<string>();

  constructor(
    private readonly permission: Permission,
    private readonly owner: Owner,
    private readonly transcript: Transcript,
  ) {}

  async judge(call: Call & Update): Promise<boolean> {
    this.transcript.call(call);
    if (this.declined || this.stopped) return this.decline(call);
    const session = this.owner.session;
    if (!session) return this.refuse(call, "this request has no OpenCode session to judge tool calls against");
    const steps = plan(call, this.owner.directory);
    if ("refuse" in steps) return this.refuse(call, steps.refuse);
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
    this.pending.add(call.toolCallId);
    try {
      await needs(steps.tool, checked.input, this.owner.directory).reduce(
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
    } catch (error) {
      if (tag(error) === "Permission.DeclinedError") {
        this.declined = true;
        return this.decline(call);
      }
      return this.refuse(call, error instanceof Error && error.message ? error.message : String(error));
    } finally {
      this.pending.delete(call.toolCallId);
      holds.delete(key);
    }
    return true;
  }

  take() {
    const refused = this.refused;
    this.refused = [];
    return refused;
  }

  async stop() {
    this.stopped = true;
    const session = this.owner.session;
    if (!session || this.pending.size === 0) return;
    const requests = await this.permission.list({ sessionID: session }).catch(() => []);
    await Promise.allSettled(
      requests
        .filter((request) => request.source && this.pending.has(request.source.id))
        .map((request) => this.permission.reply({ sessionID: session, requestID: request.id, decision: "reject" })),
    );
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
}

export async function check(
  steps: Exclude<Plan, { refuse: string }>,
  owner: Owner & { session: string },
): Promise<{ refuse: string } | { input: Record<string, unknown>; push?: string; cage?: ReturnType<typeof sandbox> }> {
  if (!steps.tool) return { input: steps.input };
  const result = await review({
    tool: steps.tool,
    input: steps.input,
    agent: owner.agent,
    directory: owner.directory,
    session: owner.session,
  }).catch((error: unknown) => ({ problem: `the guard could not check this call: ${String(error)}` }));
  if ("problem" in result) return { refuse: result.problem };
  if (!result.cage) return { input: { ...steps.input, ...result.input }, push: result.push };
  const { command, workdir } = result.cage;
  return { input: { command, workdir }, cage: sandbox(String(result.input?.command), workdir) };
}

export async function mark(id: string, cage: Record<string, unknown>) {
  await mkdir(cages, { recursive: true });
  await writeFile(join(cages, id), JSON.stringify(cage));
}

async function wrapped(id: string, cage: Record<string, unknown>) {
  if (!/^[\w-]+$/.test(id)) return false;
  const file = join(cages, id);
  const marked = await readFile(file, "utf8").catch(() => undefined);
  await rm(file, { force: true });
  return marked === JSON.stringify(cage);
}

function tag(error: unknown) {
  return typeof error === "object" && error !== null ? Reflect.get(error, "_tag") : undefined;
}
