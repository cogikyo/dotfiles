/** How long before a held lock is considered stale (env-overridable). */
export declare const DEFAULT_LOCK_TTL_MS: number;
export interface RefreshLock {
    release(): void;
}
export interface AcquireOptions {
    /** Directory to hold lock files in. Defaults to the OpenCode data dir. */
    dir?: string;
    /** Staleness threshold in ms. Defaults to {@link DEFAULT_LOCK_TTL_MS}. */
    ttlMs?: number;
    now?: () => number;
}
/**
 * Try to acquire the refresh lock for `source`.
 *
 * Returns a {@link RefreshLock} when this process may refresh (either it won the
 * lock, or a filesystem error made the lock unavailable and we degrade to
 * best-effort). Returns null when a live holder currently owns it — the caller
 * should wait and adopt the holder's result instead of refreshing.
 */
export declare function acquireRefreshLock(source: string, opts?: AcquireOptions): RefreshLock | null;
//# sourceMappingURL=refresh-lock.d.ts.map