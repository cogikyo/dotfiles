# Delegate + usage plugins: finish and close

Goal: execute the runbook below, then delete this doc and `.spec/orchestrate.md`.
Uncommitted at handoff: `plugins/delegate/session.ts` (effort-suffix dedupe + refusal recovery); it gets committed at handoff.

## State (evidence)

Delegate core is committed through `9097587d`; usage providers through `8b72fc71`.

- Verified live: bogus `subagent_type` returns an explicit error listing known agents; effort suffix renders on cards.
- Fixed in `plugins/delegate/session.ts`, typecheck passed, live behavior unverified until restart:
  - Effort suffixes accumulated on `task_id` resume ("· high · high"); trailing effort suffixes are now stripped before appending the current one.
  - Content-filter refusal recovery per orchestrate's refusal policy: `ContentFilterError` and refusal-shaped errors (tolerant classifier) return a normal delegate result with `state="error"` containing `blocked: content_filter`, the child session id, and re-brief advice, instead of throwing. No auto-retry, no taint persistence.
- xai "stuck at 0%" root-caused: adapter and UI are correct; the live billing payload (unified subscription) carries no percent signal, the cache has no `usedPercent`, and the UI renders `--`.
  The observed 0% is attributed to stale attached TUI instances loaded before the plugin edits.
  Evidence: cached JSON and live curl are both percent-free.
- The src-permissions dry-run item is moot; no such commit landed.

## Runbook (drive executes in order)

1. Optional xai hardening in `plugins/usage/xai.ts`: treat `isUnifiedBillingUser: true` with no positive cap/limit as unknown percent, and ignore a future `creditUsagePercent: 0` in that shape so a meaningless constant zero can never render.
   Small change; evidence is in State above. Typecheck + commit.

2. opencode-go iffiness, root cause from scratch (a prior root-cause child was aborted mid-run; findings unknown).
   Investigation brief: inspect the opencode-go entry under `~/.cache/opencode/usage-sidebar/`; reproduce the chain live outside the TUI via a throwaway bun script in `/tmp/opencode` without ever printing the cookie (firefox cookies.sqlite read → /auth/status → workspace via /auth → server-fn id discovery from /usage or /go assets → POST [workspaceID] to /_server → JSON/Seroval parse); prime suspects: cookies.sqlite lock/WAL while Firefox is open, server-fn id drift after site deploys vs the in-memory TTL, Seroval vs JSON parse variance; rank failure modes with evidence, then implement minimal fixes in `plugins/usage/{opencode-go,firefox}.ts`, typecheck, commit.
   Also remove any leftover `/tmp/opencode` scripts from the aborted run.

3. Document the delegate plugin's refusal-recovery result shape and single-effort-suffix behavior wherever the plugin is documented.
   `plugins/README.md` has no delegate section; add a short one or place comments in the plugin, implementer's judgment. Commit.

4. Post-restart confirmations: effort suffix renders exactly once on resumed children; refusal recovery result shape confirmed on the next real refusal (wait-until-encountered is fine).

## Exit

When 1-4 are done, delete this doc and `.spec/orchestrate.md` as commits, surfacing the queued item below in the handoff.

## Queued for user (record only, do not do)

- A collab-mode guidance section on using xai and opencode-go models well, pending real usage signals: xai burn percent still needs the inference SSE `rate_limits.updated` tap (deliberately not built); opencode-go numeric windows depend on runbook item 2.

## Durable context

- The xai adapter reads only Grok CLI auth at `~/.grok/auth.json` (`key` + `expires_at`, never `refresh_token`); opencode's refreshed xai OAuth token got 401, which is why.
  It never refreshes tokens; on `Grok CLI token expired`, run any Grok CLI command to refresh the file.
- opencode-go has no API-key usage route upstream; the approved path is the Firefox `auth` cookie, never stored or logged.
- `delegate.json` lists `xai` and `opencode-go`, so delegating to them no longer errors.
- Rollback is removing the plugin line from `opencode.json`.
- Card effort display is a local description suffix until upstream TUI renders `message.variant`.
