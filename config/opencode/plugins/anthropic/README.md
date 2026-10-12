# Claude subscription accounts

`accounts.ts` fixes the two account identities and credential directories.
`anthropic` is Trend at `~/.claude`, and `anthropic-personal` is Cogikyo at `~/.claude-cogikyo`.
The user logs in or repairs auth with `claude-auth trend` or `claude-auth cogikyo`.
Agents must not read credential files or change account state.

## Request and credential ownership

`index.ts` registers account-bound HTTP hooks, adds the Claude system identity and one-hour cache hints, and tracks rejected long-context betas per account and model in memory.
`headers.ts` builds request headers from local settings and the vendored model and beta rules.
`claude-auth/` holds request transforms, signing, beta rules, model configuration, and HTTP retry code from upstream 2.2.1.
Keep vendored files byte-identical when updating them, and put local policy in the wrapper modules.

`credentials.ts` reads `.credentials.json` from the fixed account directory and refreshes expired or rejected tokens.
It locks refresh per account, re-reads disk under the lock, and adopts a token refreshed by another process before exchanging the old one.
Refresh writes preserve other JSON fields and file mode, with an atomic rename only if the prior access token still matches.
Refresh locks live at `~/.local/share/opencode/claude-auth-refresh-<digest>.lock` regardless of XDG overrides.
This fixed lock path matters when testing with an isolated V2 data directory.

Missing or terminally rejected credentials return an authentication error with the account's login command.
Transient refresh failures return a rate-limit error with a retry hint.
Authentication errors do not trigger account-pool fallback; `lanes/server.ts` owns quota-based account selection.

## Verification

Run `npm run check` from `config/opencode` for local type, lint, and format checks.
Credential refresh and signed live requests require separate user-approved verification because they can write real account files and refresh locks.
