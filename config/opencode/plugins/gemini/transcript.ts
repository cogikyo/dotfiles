import type { LanguageModelV3StreamPart } from "@ai-sdk/provider";
import type { Meta } from "./history.ts";
import type { Update } from "./session.ts";

type Block = { kind: "text" | "reasoning"; id: string };

export class Transcript {
  meta: Meta | undefined;
  text = "";
  calls = 0;
  used: number | undefined;
  readonly notices: string[] = [];
  private open: Block | undefined;
  private blocks = 0;
  private readonly tools = new Map<string, { name: string; settled: boolean }>();

  constructor(private readonly emit: (part: LanguageModelV3StreamPart) => void) {}

  update(update: Update) {
    const kind =
      update.sessionUpdate === "agent_message_chunk"
        ? "text"
        : update.sessionUpdate === "agent_thought_chunk"
          ? "reasoning"
          : undefined;
    if (kind) return this.chunk(kind, update.content);
    if (update.sessionUpdate === "usage_update") {
      this.used = update.used;
      return;
    }
    if (update.sessionUpdate !== "tool_call" && update.sessionUpdate !== "tool_call_update") return;
    if (!update.toolCallId) return;
    this.call(update);
    if (update.status === "completed" || update.status === "failed")
      this.settle(update.toolCallId, result(update), update.status === "failed");
  }

  say(text: string) {
    this.chunk("text", { text });
  }

  end() {
    for (const [id, tool] of this.tools) if (!tool.settled) this.settle(id, "The agent reported no result", true);
    this.close();
  }

  private chunk(kind: Block["kind"], content: unknown) {
    const delta = object(content) && typeof content.text === "string" ? content.text : "";
    if (!delta) return;
    if (kind === "text" && delta.trimStart().startsWith("⚠")) {
      this.notices.push(delta.trim());
      kind = "reasoning";
    }
    if (this.open?.kind !== kind) {
      this.close();
      this.open = { kind, id: String(this.blocks++) };
      this.emit({ type: `${kind}-start`, id: this.open.id, providerMetadata: this.metadata() });
    }
    if (kind === "text") this.text += delta;
    this.emit({ type: `${kind}-delta`, id: this.open.id, delta });
  }

  private close() {
    if (this.open) this.emit({ type: `${this.open.kind}-end`, id: this.open.id });
    this.open = undefined;
  }

  call(update: Update & { toolCallId?: string }) {
    const id = update.toolCallId!;
    if (this.tools.has(id)) return;
    const name = update.name ?? update.kind ?? "tool";
    this.close();
    this.tools.set(id, { name, settled: false });
    this.calls++;
    this.emit({
      type: "tool-call",
      toolCallId: id,
      toolName: name,
      input: JSON.stringify({ title: update.title, ...(object(update.rawInput) ? update.rawInput : undefined) }),
      providerExecuted: true,
      providerMetadata: this.metadata(),
    });
  }

  settle(id: string, value: string, error: boolean) {
    const tool = this.tools.get(id);
    if (!tool || tool.settled) return;
    tool.settled = true;
    this.emit({ type: "tool-result", toolCallId: id, toolName: tool.name, result: value, isError: error });
  }

  private metadata() {
    return this.meta ? { gemini: this.meta } : undefined;
  }
}

function result(update: Update) {
  const pieces = Array.isArray(update.content)
    ? update.content.flatMap((item: { type?: string; content?: { text?: string }; path?: string }) => {
        if (item.type === "content" && item.content?.text) return [item.content.text];
        if (item.type === "diff" && item.path) return [`diff of ${item.path}`];
        return [];
      })
    : [];
  if (pieces.length) return pieces.join("\n");
  if (update.rawOutput !== undefined) return JSON.stringify(update.rawOutput);
  return update.status ?? "done";
}

function object(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
