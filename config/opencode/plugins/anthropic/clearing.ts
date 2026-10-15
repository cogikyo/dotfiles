import { Effect } from "effect";

const edit = {
  type: "clear_tool_uses_20250919",
  trigger: { type: "input_tokens", value: 150_000 },
  keep: { type: "tool_uses", value: 5 },
  clear_at_least: { type: "input_tokens", value: 50_000 },
};

// Match the wire-name rewrite in claude-auth/transforms.js; mismatches silently lose clearing protection.
const kept = ["mcp_Question", "mcp_Subagent", "mcp_Skill"];

type Block = { type?: string; name?: string };

type Body = {
  tools?: Block[];
  messages?: { content?: unknown }[];
  context_management?: { edits?: unknown[] };
};

type Applied = { cleared_tool_uses?: number; cleared_input_tokens?: number };

function excluded(body: Body) {
  const used = new Set([
    ...(body.tools ?? []).map((tool) => tool.name),
    ...(body.messages ?? []).flatMap((message) =>
      Array.isArray(message.content)
        ? message.content.filter((block: Block) => block.type === "tool_use").map((block: Block) => block.name)
        : [],
    ),
  ]);
  return kept.filter((name) => used.has(name));
}

function total(applied: Applied[], key: keyof Applied) {
  return applied.reduce((sum, entry) => sum + (entry[key] ?? 0), 0);
}

export function clearing(log: (effect: Effect.Effect<void>) => void) {
  const warn = (stage: string, error: unknown) =>
    log(Effect.logWarning("anthropic: tool clearing failed", { stage, error: String(error) }));

  const report = async (sessionID: string, stream: ReadableStream<Uint8Array>) => {
    const decoder = new TextDecoder();
    let text = "";
    let applied: Applied[] = [];
    try {
      for await (const chunk of stream) {
        text += decoder.decode(chunk, { stream: true });
        const lines = text.split("\n");
        text = lines.pop() ?? "";
        for (const line of lines) {
          if (!line.startsWith("data:") || !line.includes('"message_delta"')) continue;
          const data = JSON.parse(line.slice(5));
          if (data.type === "message_delta") applied = data.context_management?.applied_edits ?? [];
        }
      }
    } catch (error) {
      warn("response", error);
      return;
    }
    if (applied.length === 0) return;
    log(
      Effect.logInfo("anthropic: tool results cleared", {
        sessionID,
        toolUses: total(applied, "cleared_tool_uses"),
        inputTokens: total(applied, "cleared_input_tokens"),
      }),
    );
  };

  return {
    inject(kind: string, body: string | undefined) {
      if (kind !== "primary" || !body) return;
      try {
        const parsed: Body = JSON.parse(body);
        const exclude = excluded(parsed);
        parsed.context_management ??= {};
        parsed.context_management.edits = [
          ...(parsed.context_management.edits ?? []),
          exclude.length > 0 ? { ...edit, exclude_tools: exclude } : edit,
        ];
        return JSON.stringify(parsed);
      } catch (error) {
        warn("request", error);
      }
    },

    watch(state: { cleared: boolean }, sessionID: string, response: Response) {
      if (!state.cleared || !response.ok || !response.body) return response;
      try {
        const [mine, theirs] = response.body.tee();
        void report(sessionID, mine);
        return new Response(theirs, {
          status: response.status,
          statusText: response.statusText,
          headers: response.headers,
        });
      } catch (error) {
        warn("response", error);
        return response;
      }
    },
  };
}
