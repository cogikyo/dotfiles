# Claude subscription accounts

`accounts.ts` fixes the account identities and credential directories.
`anthropic` is Trend at `~/.claude`, `anthropic-personal` is Cogikyo at `~/.claude-cogikyo`, and `anthropic-cullyn` is Cullyn at `~/.claude-cullyn`.
The user logs in or repairs auth with `claude-auth trend`, `claude-auth cogikyo`, or `claude-auth cullyn`.
Agents must not read credential files or change account state.

## Request and credential ownership

`index.ts` registers account-bound HTTP hooks, adds the Claude system identity and one-hour cache hints, and tracks rejected long-context betas per account and model in memory.
`headers.ts` builds request headers from local settings and the vendored model and beta rules.
At startup, `index.ts` detects the CLI version with `claude --version` unless `ANTHROPIC_CLI_VERSION` is already set.
Detection failure logs a warning and keeps the vendored `ccVersion` default.
`claude-auth/` holds request transforms, signing, beta rules, and model configuration from upstream 2.2.1.
Keep vendored files byte-identical when updating them, and put local policy in the wrapper modules.

`credentials.ts` reads `.credentials.json` from the fixed account directory and refreshes expired or rejected tokens.
It locks refresh per account, re-reads disk under the lock, and adopts a token refreshed by another process before exchanging the old one.
Refresh writes preserve other JSON fields and file mode, with an atomic rename only if the prior access token still matches.
Every five minutes, and once at startup, each account refreshes a token that expires within one hour, so idle accounts stay signed in.
The check reads only the local file; when refresh succeeds, network use is one refresh per token lifetime.
A failed early refresh keeps the current token, logs the HTTP status and OAuth error code, and retries on the next check.
When refresh fails for an expired or rejected token, `credentials.ts` runs one tool-free `claude -p` Haiku call with the account's `CLAUDE_CONFIG_DIR`.
If the CLI writes a different, fresh access token to the credential file, the plugin adopts it.
CLI recovery runs at most once per account every 15 minutes across processes.
Refresh locks (`claude-auth-refresh-<digest>.lock`) and recovery markers (`claude-auth-recovery-<digest>`) live in `~/.local/share/opencode/` regardless of XDG overrides.
This fixed path matters when testing with an isolated V2 data directory.

Missing credentials or terminal refresh failures that CLI recovery cannot resolve return an authentication error with the account's login command.
Transient refresh failures return a rate-limit error with a retry hint.
Authentication errors do not move a request to another account.

## Usage caps and failover

Lane picks, `ctrl+m`, and failover share `pool.ts`; `ctrl+m` and root-session failover exclude Trend.
Among uncapped accounts, keep the lane's current account first, then prefer fresh usage data over unknown accounts, then Trend, then the soonest weekly reset.
Trend leads because its 5-hour window limits how fast it can spend weekly usage, so it cannot be drained in a burst later.
Cogikyo and Cullyn have no 5-hour window, so soonest-reset-first uses their weekly capacity before it expires.

Only 429s identified as usage caps trigger failover to other open accounts.
Sessions keep their replacement while the original account's in-memory cap mark lasts, unless the replacement is capped or unavailable.
If every alternative is capped or lacks usable credentials, the original 429 is returned.

## Verification

Run `bun run check` from `config/opencode` for local type, lint, and format checks.
Code run from an agent's shell cannot load Claude credentials through the plugin, so test harnesses see a login failure.
Credential refresh and signed live requests require separate user-approved verification because they can write real account files and refresh locks.
