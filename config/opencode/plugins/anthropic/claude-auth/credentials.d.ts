import { type ClaudeAccount, type ClaudeCredentials } from "./keychain.ts";
import { type RefreshFailureKind } from "./refresh-backoff.ts";
export type { ClaudeAccount } from "./keychain.ts";
export type { ClaudeCredentials } from "./keychain.ts";
export declare function initAccounts(accounts: ClaudeAccount[]): void;
export declare function setActiveAccountSource(source: string): void;
export declare function refreshAccountsList(): ClaudeAccount[];
export declare function getActiveAccount(): ClaudeAccount | null;
export declare function loadPersistedAccountSource(): string | null;
export declare function saveAccountSource(source: string): void;
export declare function syncAuthJson(creds: ClaudeCredentials): void;
export declare const OAUTH_TOKEN_URL = "https://claude.ai/v1/oauth/token";
export declare const OAUTH_CLIENT_ID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e";
export declare function parseOAuthResponse(raw: string, currentRefreshToken: string, now?: number): ClaudeCredentials | null;
/**
 * Extract the non-secret failure reason from an OAuth token-endpoint error
 * body so a refresh failure is diagnosable from the debug log. Handles both the
 * OAuth shape (`{ error, error_description }`) and Anthropic's API error
 * envelope (`{ error: { type, message } }`). Values are truncated and never
 * include tokens; the logger additionally redacts anything JWT-shaped.
 */
export declare function extractOAuthError(raw: string): {
    oauthError?: string;
    oauthErrorDescription?: string;
};
/**
 * Exchanges a refresh token for fresh credentials using the runtime's own
 * fetch.
 *
 * This previously ran the request inside a child process spawned as
 * `process.execPath -e <script>`. That assumed process.execPath is a
 * JavaScript runtime, which does not hold inside OpenCode: the plugin runs
 * in a compiled single-file executable, so process.execPath is the OpenCode
 * binary itself and `-e` is not a script to evaluate. Every refresh exited
 * non-zero with empty stdout and silently fell through to the claude CLI.
 * Node 18+ and Bun both expose a global fetch, so no subprocess is needed.
 */
/**
 * Classified result of an OAuth refresh. A `transient` outcome (429/5xx/network
 * /`rate_limit_error`) means the refresh token is still good and the caller
 * should back off and retry rather than surface a hard error; a `terminal`
 * outcome (`invalid_grant`, ...) means the refresh token is dead.
 */
export type RefreshOutcome = {
    kind: "ok";
    creds: ClaudeCredentials;
} | {
    kind: "transient";
    status: number;
    oauthError?: string;
    retryAfterMs?: number;
} | {
    kind: "terminal";
    status: number;
    oauthError?: string;
};
/**
 * Exchange a refresh token for fresh credentials and classify the result.
 * See {@link RefreshOutcome}. Uses the runtime's own fetch (no subprocess).
 */
export declare function refreshViaOAuthDetailed(refreshToken: string, timeoutMs?: number): Promise<RefreshOutcome>;
/**
 * Backward-compatible wrapper: returns credentials on success, else null.
 * Prefer {@link refreshViaOAuthDetailed} when the transient/terminal
 * distinction matters (cooldown, CLI-fallback gating).
 */
export declare function refreshViaOAuth(refreshToken: string, timeoutMs?: number): Promise<ClaudeCredentials | null>;
/**
 * Refreshes the given (or active) account's credentials if they are within
 * `thresholdMs` of expiry. Defaults to 60s, matching the reactive
 * per-request refresh path. Callers that want a proactive refresh further
 * ahead of expiry (e.g. a background timer) should pass a larger threshold —
 * the account resolution (via getActiveAccount()) stays correct regardless
 * of threshold, so this always operates on the currently active account
 * unless one is explicitly passed in.
 */
export declare function refreshIfNeeded(account?: ClaudeAccount, thresholdMs?: number): Promise<ClaudeCredentials | null>;
export declare function getCredentialsForSync(): ClaudeCredentials | null;
/**
 * Re-read only the active account's credentials from its source (single
 * keychain service read or credentials file) and update them in place,
 * so an externally refreshed token is picked up without a full
 * multi-account keychain rescan.
 *
 * Currently has no call sites: the 401 path uses
 * reloadCredentialsFromSource, which additionally validates the result
 * and refreshes the cache. Wiring this up or deleting it is tracked as a
 * follow-up; until then it must stay consistent with the read paths that
 * are live, hence the configDir below.
 */
export declare function reloadActiveAccount(): void;
/**
 * Refresh the active account's credentials via OAuth even though they
 * still look valid locally. Used on 401 when the source still holds the
 * rejected token (revoked, the claude CLI hasn't refreshed it yet).
 * On success the account, its source, and the cache are all updated.
 * The refresh function is injectable for tests.
 */
export declare function forceRefreshActiveAccount(refresh?: (refreshToken: string) => Promise<ClaudeCredentials | null>): Promise<ClaudeCredentials | null>;
/**
 * Drop the active account's cached credentials so the next
 * getCachedCredentials() call re-reads from the source, bypassing the
 * 30s TTL. Used when the API rejects a token (401) that still looks
 * valid locally.
 */
export declare function invalidateCredentialCache(): void;
export declare function getCachedCredentials(): Promise<ClaudeCredentials | null>;
export interface CredentialWaitOptions {
    maxWaitMs?: number;
    pollMs?: number;
    signal?: AbortSignal;
    now?: () => number;
    sleep?: (ms: number, signal?: AbortSignal) => Promise<void>;
    rng?: () => number;
}
/**
 * Resolve credentials, waiting through a transient refresh rate-limit rather
 * than failing hard. Returns as soon as a token is available — ours refreshed
 * once the cooldown clears, or a sibling OpenCode instance / the `claude` CLI
 * wrote a fresh one to the shared store. Returns null promptly on a terminal
 * failure (dead refresh token) or when the wait budget is exhausted, so the
 * caller can decide between a retryable response and a hard error.
 */
export declare function getCredentialsWithBackoff(opts?: CredentialWaitOptions): Promise<ClaudeCredentials | null>;
/**
 * Whether the active account's most recent refresh failure was transient
 * (rate-limited/retryable) or terminal (dead refresh token), for callers
 * deciding between a retryable response and a hard "re-authenticate" error.
 * An active cooldown implies a transient failure.
 */
export declare function getActiveRefreshFailureKind(): RefreshFailureKind | null;
export declare function reloadCredentialsFromSource(): ClaudeCredentials | null;
//# sourceMappingURL=credentials.d.ts.map