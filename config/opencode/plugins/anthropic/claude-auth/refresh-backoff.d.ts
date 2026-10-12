/**
 * Transient-vs-terminal classification and per-account backoff for OAuth token
 * refreshes.
 *
 * The token endpoint (`claude.ai/v1/oauth/token`) rate-limits refresh requests
 * with HTTP 429 `rate_limit_error`. That is transient — the refresh token is
 * still valid — but the plugin previously treated every non-OK refresh as a
 * hard failure, surfacing "credentials unavailable. Run `claude`" and then
 * hammering the same endpoint (and the `claude` CLI, which hits it too). This
 * module lets callers tell a transient rate-limit apart from a genuinely dead
 * refresh token (`invalid_grant`), and imposes a cooldown so a rate-limited
 * account is not re-hit until the window has plausibly cleared.
 */
export type RefreshFailureKind = "transient" | "terminal";
/** Base cooldown after the first transient failure (env-overridable). */
export declare const BASE_COOLDOWN_MS: number;
/** Hard ceiling for a single cooldown, regardless of consecutive failures. */
export declare const MAX_COOLDOWN_MS = 60000;
export declare function classifyRefreshFailure(_status: number, oauthError?: string): RefreshFailureKind;
interface BackoffOptions {
    retryAfterMs?: number;
    now?: number;
    rng?: () => number;
}
/**
 * Delay before the next refresh attempt. An explicit `retry-after` from the
 * endpoint wins (still clamped to `MAX_COOLDOWN_MS`); otherwise an exponential
 * schedule (base · 2^(n-1),
 * capped) with jitter in the [50%, 100%] band to desynchronize the several
 * OpenCode instances / CLI invocations that all refresh the same account.
 */
export declare function computeBackoffMs(consecutive: number, opts?: BackoffOptions): number;
/**
 * Record a transient refresh failure for `source` and return the cooldown
 * duration applied. The cooldown escalates with consecutive transient
 * failures and is exposed via {@link isRefreshCooldownActive}.
 */
export declare function noteRefreshTransient(source: string, opts?: BackoffOptions): number;
/** Record a terminal refresh failure (dead refresh token). No cooldown. */
export declare function noteRefreshTerminal(source: string): void;
/** Clear all backoff state for `source` after a successful refresh/adopt. */
export declare function clearRefreshOutcome(source: string): void;
export declare function isRefreshCooldownActive(source: string, now?: number): boolean;
export declare function getRefreshCooldownUntil(source: string): number | null;
export declare function getRefreshFailureKind(source: string): RefreshFailureKind | null;
/** Test seam: drop all in-memory backoff state. */
export declare function resetRefreshBackoffState(): void;
export {};
//# sourceMappingURL=refresh-backoff.d.ts.map