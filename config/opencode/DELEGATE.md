# Delegate plugin status

Status: implemented and committed through `9097587d`.

## Done

- `task` shadows the builtin task tool and accepts `{model, effort}`.
- It uses the v1 prompt route so each request can set model and variant.
- It validates effort against model variants and returns explicit errors for bad effort.
- It reads the usage-sidebar cache for capacity and returns capacity reports without spawning when capped.
- It mirrors child permission inheritance, abort, resume, and the builtin XML result shape.
- Cross-provider delegation was verified after restart, including an anthropic parent spawning openai/gpt-5.5, mini-fast, and fable children.
- Card click-through works via `metadata.sessionId`.
- The effort-on-card root cause was the TUI ignoring metadata except `sessionId` and `background`.
- The current fix appends effort to the rendered task description.
- Explicit validation now covers missing, empty, and unknown `subagent_type`, plus missing or empty description and prompt.

## Still required after restart

- Confirm the effort suffix is visible on new cards and child titles.
- Confirm missing or unknown `subagent_type` returns an explicit error instead of a `replaceAll` crash.
- `verify/commit` staging smoke passed when it created `20cefb37` and `9097587d` from exact scopes.
- If the `src` permissions commit lands first, run the post-restart `verify/source` dry run for the new `src` permissions.

## Usage plugin status

Status: xai and opencode-go usage work is currently uncommitted, pending this session's commit. Runtime verification remains pending.

- xai and opencode-go adapters are implemented in `plugins/usage/`, and both are registered in `index.tsx`.
- The falsifier resolved: opencode's refreshed xai OAuth token got 401, so the adapter reads only the Grok CLI auth at `~/.grok/auth.json` (`key` + `expires_at`, never `refresh_token`).
- The Grok CLI billing endpoint for this unified subscription returns tier and current period but no consumption percent, so xai renders an info reset/tier note with no windows; the true burn percent waits on a live inference SSE `rate_limits.updated` tap that is deliberately not built yet.
- opencode-go has no API-key usage route upstream; the console `queryLiteSubscription` is browser-session `/_server` only, and browser cookie replay is deferred pending explicit live approval. The adapter shows a warn no-route note.
- `delegate.json` now lists `xai` and `opencode-go` so delegating to those connected providers no longer errors. Neither yields numeric windows yet, so capacity proceeds ungated for both.

## Next roadmap after smoke

- Tap the xai inference SSE `rate_limits.updated` stream for a real burn percent, then promote the xai note to a numeric window.
- Revisit opencode-go browser cookie replay only after explicit live approval; expect hourly, weekly, and monthly draining windows there.
- Once numeric signals exist, weave affinities for xai imagegen, x-search, websearch, and opencode-go models into `drive.md`.
- Consider the `@types/node` bump only if TypeScript dependency resolution keeps requiring local `node_modules` hydration.

## Rollback and risk

- Rollback is removing the plugin line from `opencode.json`.
- Shadowing relies on upstream plugin tool override and card name behavior.
- Runtime card effort display is a local description suffix until upstream TUI renders `message.variant`.
