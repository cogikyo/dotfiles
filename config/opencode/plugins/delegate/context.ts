import type { SessionUpdateData } from "@opencode-ai/sdk/v2";
import { type Client, type Message, type Reply, session, unwrap } from "../shared/opencode.ts";
import type { CONTEXT_PRESSURE } from "../shared/session.ts";
import { delegate } from "./metadata.ts";
import { deny } from "./permission.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Context governor                                                                              │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

/** Delegate context thresholds, with a hard stop fixed in `shared/session.ts`. */
export type ContextLimits = typeof CONTEXT_PRESSURE;

/** Recorded child limit; tokens may be absent when no count is available. */
export type ContextLimit = {
  level: "hard" | "compaction";
  tokens?: number;
};

export type ContextWarning = "soft" | "medium" | "final";

/** Process-local child IDs that cannot resume, even if saving their context limit fails. */
export const contextLimitedSessions = new Set<string>();

// ├─ Limit observation ───────────────────────────────────────────────────────────────────────────┤

/** Detects automatic compaction or a hard token stop, preferring compaction when both occur. */
export function observeContextLimit(messages: Message[], limits: ContextLimits) {
  const tokens = maxContextTokens(messages);
  if (messages.some((message) => message.parts.some((part) => part.type === "compaction" && part.auto))) {
    return { level: "compaction" as const, tokens };
  }
  if (tokens !== undefined && tokens >= limits.hard) {
    return { level: "hard" as const, tokens };
  }
  return undefined;
}

/** Finds the highest positive assistant context count, if any. */
export function maxContextTokens(messages: Message[]) {
  let result: number | undefined;
  for (const { info } of messages) {
    if (info.role !== "assistant") continue;
    const { total, input, output, cache } = info.tokens;
    const count = total && total > 0 ? total : input + output + cache.read + cache.write;
    if (count <= 0) continue;
    result = Math.max(result ?? 0, count);
  }
  return result;
}

/** Detects a completed assistant reply so the wait stops sending warnings. */
export function finalAssistant(reply: Reply | undefined) {
  const finish = reply?.info.finish;
  return !!finish && finish !== "tool-calls" && finish !== "unknown";
}

// ├─ Warnings ────────────────────────────────────────────────────────────────────────────────────┤

const SOFT_WARNING_MARKER = "[DELEGATE CONTEXT GOVERNOR: SOFT PRESSURE]";
const MEDIUM_WARNING_MARKER = "[DELEGATE CONTEXT GOVERNOR: MEDIUM PRESSURE]";
const FINAL_WARNING_MARKER = "[DELEGATE CONTEXT GOVERNOR: FINAL WARNING]";

/** Picks the highest unspent warning reached by the observed token count. */
export function pendingContextWarning(tokens: number | undefined, limits: ContextLimits, spent: Set<ContextWarning>) {
  if (tokens === undefined) return undefined;
  if (tokens >= limits.final && !spent.has("final")) return "final";
  if (tokens >= limits.medium && !spent.has("medium")) return "medium";
  if (tokens >= limits.soft && !spent.has("soft")) return "soft";
  return undefined;
}

/** Confirms delivered warning markers and marks their lower levels spent. */
export function observeContextWarnings(messages: Message[], spent: Set<ContextWarning>, observed: Set<ContextWarning>) {
  for (const message of messages) {
    if (message.info.role !== "user") continue;
    for (const part of message.parts) {
      if (part.type !== "text") continue;
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

/** Builds a marked warning prompt whose appearance in history confirms delivery. */
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

// ├─ Seal ────────────────────────────────────────────────────────────────────────────────────────┤

/** Seals a context-limited child with a persistent limit marker and deny-all permissions. */
export async function sealContextLimited(client: Client, sessionID: string, limit: ContextLimit, signal: AbortSignal) {
  const current = await session(client, sessionID, {
    label: `delegate read context-limited child session ${sessionID}`,
    signal,
  });
  const body: NonNullable<SessionUpdateData["body"]> = {
    metadata: {
      ...current.metadata,
      delegate: { ...delegate(current), context: { limit: limit.level, tokens: limit.tokens } },
    },
    permission: [deny("*")],
  };
  await unwrap(
    client.session.update({ path: { id: sessionID }, body, signal }),
    `delegate seal context-limited child session ${sessionID}`,
  );
}
