export declare const SYSTEM_IDENTITY = "You are Claude Code, Anthropic's official CLI for Claude.";
type ContentBlock = {
    type?: string;
    text?: string;
} & Record<string, unknown>;
type Message = {
    role?: string;
    content?: string | ContentBlock[];
};
/**
 * Strategy for reconciling `tool_use` / `tool_result` adjacency that OpenCode's
 * automatic compaction can break (issues #212/#226/#261):
 *
 * - `placeholder` (default): never delete blocks. For any `tool_use` left
 *   without an adjacent `tool_result`, synthesize a paired placeholder result.
 *   Because assistant `content[]` is never rewritten, `thinking` /
 *   `redacted_thinking` blocks stay byte-identical, sidestepping Anthropic's
 *   thinking-preservation contract (issue #261).
 * - `drop`: remove orphaned blocks (the upstream behavior), but omit an entire
 *   assistant turn when it carries `thinking` blocks and an orphaned `tool_use`
 *   rather than partially rewriting it (which #261 forbids).
 */
export type ToolRepairMode = "placeholder" | "drop";
/** Content used for a synthesized `tool_result` whose real output was compacted away. */
export declare const TOOL_RESULT_PLACEHOLDER = "Tool result unavailable (removed during context compaction).";
/**
 * Resolve the repair strategy from the environment. Defaults to `placeholder`,
 * the lossless strategy. Set `OPENCODE_CLAUDE_AUTH_TOOL_REPAIR=drop` to opt into
 * the drop strategy.
 */
export declare function resolveToolRepairMode(env?: Record<string, string | undefined>): ToolRepairMode;
/**
 * Drop-strategy repair. Iterated to a fixed point so that cascades — e.g. an
 * omitted thinking turn orphaning the result that followed it — are fully
 * reconciled rather than left half-repaired.
 */
export declare function repairToolPairs(messages: Message[]): Message[];
/**
 * Placeholder-strategy repair (default). Guarantees `tool_use` ↔ `tool_result`
 * adjacency without ever deleting a block from an assistant turn, so thinking
 * blocks are preserved exactly. Two passes:
 *
 *  1. Remove `tool_result` blocks that have no adjacent preceding `tool_use`
 *     (these live in user turns, so no thinking block is affected).
 *  2. Synthesize a placeholder `tool_result`, adjacent, for every `tool_use`
 *     that still lacks one.
 */
export declare function synthesizeMissingToolResults(messages: Message[]): Message[];
/** Dispatch to the configured repair strategy. */
export declare function applyToolRepair(messages: Message[], mode: ToolRepairMode): Message[];
export declare function transformBody(body: BodyInit | null | undefined, mode?: ToolRepairMode): BodyInit | null | undefined;
export declare function stripToolPrefix(text: string): string;
export declare function transformResponseStream(response: Response): Response;
export {};
//# sourceMappingURL=transforms.d.ts.map