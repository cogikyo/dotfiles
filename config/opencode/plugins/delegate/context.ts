import type { SessionUpdateData } from "@opencode-ai/sdk/v2";
import { record } from "../shared/record.ts";
import type { CONTEXT_PRESSURE } from "../shared/session.ts";
import { deny } from "./permission.ts";
import { type Client, finite, string, unwrap } from "./sdk.ts";

export type ContextLimits = typeof CONTEXT_PRESSURE;
export type ContextLimit = {
  level: "hard" | "compaction";
  tokens?: number;
};

export type ContextWarning = "soft" | "medium" | "final";

const SOFT_WARNING_MARKER = "[DELEGATE CONTEXT GOVERNOR: SOFT PRESSURE]";
const MEDIUM_WARNING_MARKER = "[DELEGATE CONTEXT GOVERNOR: MEDIUM PRESSURE]";
const FINAL_WARNING_MARKER = "[DELEGATE CONTEXT GOVERNOR: FINAL WARNING]";
export const contextLimitedSessions = new Set<string>();

export function observeContextLimit(messages: unknown[], limits: ContextLimits) {
  const tokens = maxContextTokens(messages);
  if (messages.some(isAutoCompactionMessage)) {
    return { level: "compaction" as const, tokens };
  }
  if (tokens !== undefined && tokens >= limits.hard) {
    return { level: "hard" as const, tokens };
  }
  return undefined;
}

export function pendingContextWarning(tokens: number | undefined, limits: ContextLimits, spent: Set<ContextWarning>) {
  if (tokens === undefined) return undefined;
  if (tokens >= limits.final && !spent.has("final")) return "final";
  if (tokens >= limits.medium && !spent.has("medium")) return "medium";
  if (tokens >= limits.soft && !spent.has("soft")) return "soft";
  return undefined;
}

export function observeContextWarnings(messages: unknown[], spent: Set<ContextWarning>, observed: Set<ContextWarning>) {
  for (const message of messages) {
    const root = record(message);
    if (record(root?.info)?.role !== "user" || !Array.isArray(root?.parts)) continue;
    for (const value of root.parts) {
      const part = record(value);
      if (part?.type !== "text" || typeof part.text !== "string") continue;
      if (part.text.startsWith(FINAL_WARNING_MARKER)) {
        spent.add("soft");
        spent.add("medium");
        spent.add("final");
        observed.add("final");
      } else if (part.text.startsWith(MEDIUM_WARNING_MARKER)) {
        spent.add("soft");
        spent.add("medium");
        observed.add("medium");
      } else if (part.text.startsWith(SOFT_WARNING_MARKER)) {
        spent.add("soft");
        observed.add("soft");
      }
    }
  }
}

export function maxContextTokens(messages: unknown[]) {
  let result: number | undefined;
  for (const message of messages) {
    const info = record(record(message)?.info);
    if (info?.role !== "assistant") continue;
    const tokens = record(info.tokens);
    if (!tokens) continue;
    const count = tokenCount(tokens);
    if (count <= 0) continue;
    result = Math.max(result ?? 0, count);
  }
  return result;
}

function tokenCount(tokens: Record<string, unknown>) {
  const total = finite(tokens.total);
  return total && total > 0
    ? total
    : (finite(tokens.input) ?? 0) +
        (finite(tokens.output) ?? 0) +
        (finite(record(tokens.cache)?.read) ?? 0) +
        (finite(record(tokens.cache)?.write) ?? 0);
}

function isAutoCompactionMessage(message: unknown) {
  const parts = record(message)?.parts;
  return (
    Array.isArray(parts) &&
    parts.some((value) => {
      const part = record(value);
      return part?.type === "compaction" && part.auto === true;
    })
  );
}

export function finalAssistant(message: Record<string, unknown> | undefined) {
  const finish = string(record(message?.info)?.finish);
  return !!finish && finish !== "tool-calls" && finish !== "unknown";
}

export function contextWarningPrompt(level: ContextWarning, tokens: number | undefined, limits: ContextLimits) {
  if (level === "final") {
    return [
      FINAL_WARNING_MARKER,
      `Observed context: ${tokens ?? "unknown"} tokens; final threshold: ${limits.final}; hard stop: ${limits.hard}.`,
      `Remaining context budget before forced shutdown: ${tokens === undefined ? "unknown" : Math.max(0, limits.hard - tokens)} tokens at this observation.`,
      "A forced context-limited stop is approaching. Finish immediately.",
      "If you are patching, complete only the last edits already in progress. Otherwise, make only final evidence calls.",
      "Return a concise final report now.",
    ].join("\n");
  }
  if (level === "medium") {
    return [
      MEDIUM_WARNING_MARKER,
      `Observed context: ${tokens ?? "unknown"} tokens; medium threshold: ${limits.medium}; final warning: ${limits.final}; hard stop: ${limits.hard}.`,
      "Long-context performance may degrade. Be wary of missed constraints, stale assumptions, and repeated work.",
      "Converge on the assigned boundary and finish soon; verify critical conclusions against source evidence.",
      "Do not expand scope or begin another concern.",
    ].join("\n");
  }
  return [
    SOFT_WARNING_MARKER,
    `Observed context: ${tokens ?? "unknown"} tokens; soft threshold: ${limits.soft}; medium threshold: ${limits.medium}.`,
    `Try to finish before ${limits.medium} tokens if you can, while preserving the assigned acceptance checks.`,
    "Do not expand scope or begin another concern.",
  ].join("\n");
}

export async function sealContextLimited(client: Client, sessionID: string, limit: ContextLimit, signal: AbortSignal) {
  const session = await unwrap<Record<string, unknown>>(
    client.session.get({ path: { id: sessionID }, signal }),
    `read context-limited child session ${sessionID}`,
  );
  const metadata = record(session.metadata) ?? {};
  const delegate = record(metadata.delegate) ?? {};
  const body: NonNullable<SessionUpdateData["body"]> = {
    metadata: {
      ...metadata,
      delegate: {
        ...delegate,
        context: {
          limit: limit.level,
          ...(limit.tokens !== undefined ? { tokens: limit.tokens } : {}),
        },
      },
    },
    permission: [deny("*")],
  };
  await unwrap(
    client.session.update({ path: { id: sessionID }, body, signal }),
    `seal context-limited child session ${sessionID}`,
  );
}
