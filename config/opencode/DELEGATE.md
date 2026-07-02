# Delegate plugin status

Status: implemented and committed through `886a5a4c`, with uncommitted fixes pending in this session.

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
- Confirm `verify/commit` staging behavior under the new permissions.
- If the `src` permissions commit lands first, run the post-restart `verify/source` dry run for the new `src` permissions.

## Next roadmap after smoke

- Resume usage plugin work.
- xai usage should come from `GET https://cli-chat-proxy.grok.com/v1/billing?format=credits` with `Authorization: Bearer <opencode xai oauth access>` and `X-XAI-Token-Auth: xai-grok-cli`.
- The first falsifier is whether opencode's refreshed xai OAuth token is accepted by that endpoint.
- opencode-go usage needs browser-cookie or RPC simulation because there is no public or API-key endpoint.
- Expect hourly, weekly, and monthly draining windows there.
- Once usage signals exist, add xai and opencode-go to `delegate.json` and weave affinities for xai imagegen, x-search, websearch, and opencode-go models into `drive.md`.
- Consider the `@types/node` bump only if TypeScript dependency resolution keeps requiring local `node_modules` hydration.
- The user still needs to install `devtools` with pacman after the `src` package commit.

## Rollback and risk

- Rollback is removing the plugin line from `opencode.json`.
- Shadowing relies on upstream plugin tool override and card name behavior.
- Runtime card effort display is a local description suffix until upstream TUI renders `message.variant`.
