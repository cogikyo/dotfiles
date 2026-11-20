import type {
  LanguageModelV3,
  LanguageModelV3CallOptions,
  LanguageModelV3Content,
  LanguageModelV3FinishReason,
  LanguageModelV3GenerateResult,
  LanguageModelV3StreamPart,
  LanguageModelV3StreamResult,
  LanguageModelV3Usage,
} from "@ai-sdk/provider";
import type { Plugin } from "@opencode/plugin";
import { Gate, type Owner } from "./gate.ts";
import { plan } from "./history.ts";
import { add, failure, Session, type Result, type Usage } from "./session.ts";
import { Transcript } from "./transcript.ts";

export type Setup = {
  command: string;
  permission: Plugin.Context["permission"];
  locate: (owner: string | undefined, agent: string | undefined) => Promise<Owner>;
  seen: Map<string, number>;
};

const stops: Record<string, LanguageModelV3FinishReason["unified"]> = {
  end_turn: "stop",
  max_tokens: "length",
  max_turn_requests: "length",
  cancelled: "other",
};

const followups = 2;

export class Gemini implements LanguageModelV3 {
  readonly specificationVersion = "v3";
  readonly provider = "gemini";
  readonly supportedUrls = {};

  constructor(
    readonly modelId: string,
    readonly setup: Setup,
  ) {}

  async doStream(options: LanguageModelV3CallOptions): Promise<LanguageModelV3StreamResult> {
    const owner = options.headers?.["x-opencode-session-id"];
    const effort = options.providerOptions?.gemini?.effort;
    const conversation = plan(options.prompt, owner);
    const located = await this.setup.locate(owner, options.headers?.["x-opencode-agent"]);
    let session: Session | undefined;
    let gate: Gate | undefined;
    let cancelled = false;
    const stream = new ReadableStream<LanguageModelV3StreamPart>({
      start: async (controller) => {
        const emit = (part: LanguageModelV3StreamPart) => {
          if (!cancelled) controller.enqueue(part);
        };
        emit({ type: "stream-start", warnings: [] });
        const transcript = new Transcript(emit);
        const judge = new Gate(this.setup.permission, located, transcript);
        gate = judge;
        try {
          const known = conversation.known;
          const resume = known && (this.setup.seen.get(known.session) ?? 0) <= known.users ? known.session : undefined;
          session = await Session.open(
            { command: this.setup.command, owner: located, resume },
            {
              update: (update) => transcript.update(update),
              permission: (call) => judge.judge(call),
            },
          );
          transcript.meta = conversation.meta(session.id);
          judge.message = session.id;
          await session.configure(this.modelId, typeof effort === "string" ? effort : undefined);
          if (cancelled) return session.kill();
          const first = await session.prompt(session.resumed ? conversation.next : conversation.full);
          if (transcript.meta) this.setup.seen.set(session.id, transcript.meta.users);
          const result = await follow(session, judge, transcript, first);
          if (cancelled) return;
          await session.close();
          transcript.end();
          if (!judge.declined) this.check(result, transcript, session.tier);
          emit({
            type: "finish",
            finishReason: judge.declined
              ? { unified: "stop", raw: "declined" }
              : { unified: stops[result.stopReason] ?? "other", raw: result.stopReason },
            usage: usage(result.usage),
          });
          controller.close();
        } catch (error) {
          await judge.stop();
          session?.kill();
          if (!cancelled) controller.error(error);
        }
      },
      cancel: async () => {
        cancelled = true;
        await gate?.stop();
        await session?.cancel();
      },
    });
    return { stream };
  }

  async doGenerate(options: LanguageModelV3CallOptions): Promise<LanguageModelV3GenerateResult> {
    const { stream } = await this.doStream(options);
    const content: LanguageModelV3Content[] = [];
    let finish: Extract<LanguageModelV3StreamPart, { type: "finish" }> | undefined;
    for await (const part of stream) {
      if (part.type === "text-delta" || part.type === "reasoning-delta") {
        const kind = part.type === "text-delta" ? "text" : "reasoning";
        const last = content.at(-1);
        if (last?.type === kind) last.text += part.delta;
        else content.push({ type: kind, text: part.delta });
      }
      if (part.type === "tool-call" || part.type === "tool-result") content.push(part);
      if (part.type === "finish") finish = part;
    }
    if (!finish) throw new Error("Gemini agent stream ended without a finish");
    return { content, finishReason: finish.finishReason, usage: finish.usage, warnings: [] };
  }

  private check(result: Result, transcript: Transcript, tier: string) {
    if (result.stopReason === "refusal")
      throw failure(
        "insufficient_quota",
        `Gemini agent stopped ${this.modelId} with reason "refusal", its signal for an exhausted ${tier || "Google AI"} quota`,
      );
    if (transcript.text.trim()) return;
    const notice = transcript.notices.at(-1);
    if (notice)
      throw failure(
        notice.includes("RESOURCE_EXHAUSTED") ? "insufficient_quota" : "invalid_request",
        `Gemini agent gave up on ${this.modelId}: ${notice}`,
      );
    throw failure(
      "invalid_request",
      `Gemini agent ended its turn on ${this.modelId} without a reply (stop reason "${result.stopReason}", ${transcript.calls} tool calls)`,
    );
  }
}

async function follow(session: Session, gate: Gate, transcript: Transcript, first: Result): Promise<Result> {
  let result = first;
  for (let left = followups; ; left--) {
    const refused = gate.take();
    if (refused.length === 0 || gate.declined || gate.stopped) return result;
    if (left === 0) {
      transcript.say(
        `OpenCode ended this turn after ${followups} follow-ups because the Gemini agent kept making tool calls it refused. The last refusals:\n${refused.join("\n")}`,
      );
      return result;
    }
    const next = await session.prompt(
      `OpenCode refused these tool calls, so they did not run:\n${refused.join("\n")}\nContinue the task within these limits.`,
    );
    result = { ...next, usage: add(result.usage, next.usage) };
  }
}

function usage(counts: Usage | undefined): LanguageModelV3Usage {
  if (!counts)
    return {
      inputTokens: { total: undefined, noCache: undefined, cacheRead: undefined, cacheWrite: undefined },
      outputTokens: { total: undefined, text: undefined, reasoning: undefined },
    };
  const cached = counts.cachedReadTokens ?? 0;
  const thought = counts.thoughtTokens ?? 0;
  return {
    inputTokens: { total: counts.inputTokens + cached, noCache: counts.inputTokens, cacheRead: cached, cacheWrite: 0 },
    outputTokens: { total: counts.outputTokens, text: counts.outputTokens - thought, reasoning: thought },
  };
}
