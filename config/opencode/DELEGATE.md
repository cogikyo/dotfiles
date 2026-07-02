# DELEGATE: runtime model-delegation plugin for opencode

Build-ready plan for a plugin tool named exactly `task` that shadows the builtin Task tool.
It accepts `{model, effort}` per call, checks provider usage limits before spawning, and renders as the native inline subagent card.
Agent markdown stays fully model-agnostic: model/effort judgment is prose in `drive.md`, provider physics (thresholds, wait budget) live in `delegate.json`.

Rationale: static per-agent model frontmatter causes combinatoric explosion (6+ models × ~5 efforts × ~20 agents).
The same agent legitimately spans the full range (e.g. verify/commit runs mini-fast/low for easy commits and heavy/high for multi-patch detangling).
Only the orchestrator at runtime can judge which model and effort a given delegation deserves.

## Orientation for executor sessions

Each phase below is self-contained; a fresh session, likely a different model, executes one phase cold with no memory beyond committed artifacts.

- Workspace: `~/.config/opencode` is a symlink into `~/dotfiles/config/opencode`; always edit the dotfiles paths.
- Plugin style to match: see `plugins/usage/` and `plugins/hyprd/` (Bun, `.ts`/`.tsx`, small files per concern).
- Server plugins register in `opencode.json`; TUI plugins register in `tui.json` (per `plugins/README.md`). Delegate is a server plugin.
- House rules live in dotfiles `AGENTS.md` files: explicit errors over silent fallbacks, no tests unless asked, one sentence per line in markdown.
- Phases 0–2 are one natural build slice; Phase 3 needs the user's affinity opinions (seeds are hypotheses); Phase 4 is manual TUI verification with the user.

## Source evidence

All anchors verified 2026-07-01 against upstream `github.com/anomalyco/opencode` (formerly sst/opencode), dev branch @ `fc29f99190a95f934d188e4b7acb3f3460cd38fd`, cross-checked against release v1.17.13 (`10c894b`).
Local install: `opencode-bin` (Arch AUR), autoupdate false.
There is NO opencode 2.0 semver; "v2" refers to incremental surfaces inside 1.x (v2 API routes, v2 SDK gen, tui-v2 snapshot channel).
File:line anchors churn with upstream; re-verify against the Phase 0 reference clone before relying on them.

### Builtin Task tool

- Params are only `description`/`prompt`/`subagent_type`/`task_id`/`command`/`background`; no model/variant param, identical on dev and v1.17.13 (`packages/opencode/src/tool/task.ts:43–62`).
- Child spawn: `sessions.create({parentID, title, agent, permission})` at `task.ts:142–158`.
- Model resolution: `next.model ?? {modelID: msg.info.modelID, providerID: msg.info.providerID}` at `task.ts:165–170`.
- Prompt op passes `{messageID, sessionID, model, variant, agent, parts}` at `task.ts:186–198`.
- Abort propagation: `ctx.abort` → session abort (`task.ts:296–333`); resume via `task_id` reuses the child sessionID (`task.ts:121–123`).
- Result = last text part of the returned assistant message, wrapped in `<task id=...><task_result>` XML (`task.ts:199`).

### Server prompt path (the route delegate must use)

- Server prompt schema accepts per-request `model` (`{providerID, modelID}`) and `variant` (string): `PromptInput` at `packages/opencode/src/session/prompt.ts:1498–1519`; served at `POST /session/:sessionID/message` (`server/routes/instance/httpapi/groups/session.ts:70,95`).
- Variant precedence: `input.variant ?? (ag.variant if valid)` and `input.model ?? ag.model ?? currentModel` at `prompt.ts:646–654`.
- v2 `session.prompt` (`POST /api/session/{id}/prompt`) has NO per-request model/variant; model changes only via session-level switchModel. Therefore the delegate tool MUST use the v1-path route.
- v1 SDK typegen omits `variant` from `SessionPromptData.body` (`packages/sdk/js/src/gen/types.gen.ts:2588`); the server accepts it; cast the body (`as any`) or use v2 client types. Runtime-safe since the fetch client passes the body verbatim.

### Variants (effort)

- Variants = named per-model provider-option fragments (`ProviderTransform.variants`, `packages/opencode/src/provider/transform.ts:673`); applied by merge at `packages/opencode/src/session/llm/request.ts:80–91`.
- Anthropic adaptive models (opus ≥4.7, sonnet ≥5, fable-5) accept `low|medium|high|xhigh|max` mapping to `{thinking: {type:"adaptive"}, effort}`.
- OpenAI efforts map to `{reasoningEffort, reasoningSummary:"auto"}` with per-family tiers (`transform.ts:887–946, 519–598`).
- CRITICAL: unknown variant names silently apply NO reasoning options, no error (`request.ts:80–83`). The plugin must validate effort against the model's variant table and error explicitly.

### Tool registry and shadowing

- Plugin hook tools keep their bare key as tool id, no namespacing, no conflict check (`packages/opencode/src/tool/registry.ts:187–192`).
- `all()` returns `[...builtin, ...custom]` (`registry.ts:242–245`); the LLM tool map is built by insertion so later (plugin) wins (`packages/opencode/src/session/tools.ts:48, 89–95`).
- A plugin tool named `task` therefore shadows the builtin in the LLM tool list; internal consumers (subtask parts) still use the builtin via `registry.named()` (`registry.ts:236, 308–311`).
- The subagent list is appended to the tool description keyed on `tool.id === TaskTool.id` string compare, which the shadow also matches (`registry.ts:295`).
- Disabling builtin tools: config `tools: {task: false}` maps to a permission deny keyed by tool id (`config/config.ts:552–562`; filtered at `llm/request.ts:208–214`). Note it would ALSO kill a shadow named `task`; only relevant for the fallback rename path.

### Inline subagent card

- Card dispatch is keyed on tool part name `=== "task"` exactly, in BOTH UIs: TUI toolDisplays set + Match at `packages/tui/src/routes/session/index.tsx:2567–2585, 1758–1760`; v2 session-ui registers name `"task"` at `packages/session-ui/src/components/message-part.tsx:1845–1846`.
- The card reads `metadata.sessionId` for click-through navigation and live child sync (`index.tsx:2216–2298`); `input.description` and `input.subagent_type` render on the card (`2258–2264`).
- A tool named anything else gets GenericTool regardless of metadata.

### Permission inheritance (must replicate)

- The builtin copies the parent session's deny + external_directory rules, then adds `todowrite` and `task` denies unless the subagent's own ruleset permits (`packages/opencode/src/agent/subagent-permissions.ts:14–27`; `task.ts:129–141`).

### Plugin API

- `PluginInput` has the full v1 SDK client + `serverUrl` (`packages/plugin/src/index.ts:56–66`).
- `ToolContext = {sessionID, messageID, agent, directory, worktree, abort, metadata(), ask()}` (`packages/plugin/src/tool.ts:3–20`).
- Tool execute is async (`registry.ts:142`); `ctx.ask` bridges permission prompts (`registry.ts:138`).

### Usage data source

- The user's own usage-sidebar plugin (`dotfiles/config/opencode/plugins/usage/`) polls provider OAuth usage APIs and caches to `~/.cache/opencode/usage-sidebar/{providerID}.json` as `{windows: [{label: "H"|"W", usedPercent, resetAt}]}`.
- See `plugins/usage/cache.ts`, `auth.ts` (`usageCachePath`), `anthropic.ts`, `openai.ts`.
- Delegate reads this cache read-only.

## Decisions made

- Tool name is exactly `task` to shadow the builtin and inherit the inline card, click-through, and description-appended subagent list.
- Per-request model/effort ride the v1-path `POST /session/{id}/message` route because v2 prompt removed per-request model.
- Effort is validated against the model's variant table before prompting; mismatches are explicit tool errors, never silent no-ops.
- Capacity decisions read the existing usage-sidebar cache; delegate owns no polling.
- Model/effort judgment is prose in `drive.md`, edited over time; `delegate.json` carries only provider physics.

## Rejected alternatives

- Static per-agent model frontmatter: combinatoric explosion across models × efforts × agents; the same agent spans the full range per task.
- Tool named `delegate` (non-shadow): loses the inline subagent card in both UIs; kept only as the fallback if upstream adds registry dedupe.
- v2 SDK adoption: nothing required from it, still churning, and v2 prompt actually removed per-request model.
- Auto-fallback routing tables in config: the orchestrator re-picks on capacity reports; judgment stays in prose.

---

## Phase 0 — reference clone

Purpose: a durable clone for re-verifying file:line anchors as upstream churns, replacing the throwaway `/tmp/opencode/opencode-dev` discovery clone.

Prerequisites: none.

Steps:

```bash
git clone https://github.com/anomalyco/opencode ~/repos/opencode
cd ~/repos/opencode
git checkout fc29f99190a95f934d188e4b7acb3f3460cd38fd
```

No source build is needed; the npm packages `@opencode-ai/plugin` and `@opencode-ai/sdk` carry the types.

Verification: spot-check two anchors from the Source evidence section, e.g. `packages/opencode/src/tool/task.ts:43–62` (builtin params) and `packages/opencode/src/tool/registry.ts:187–192` (bare-key plugin tool ids).
If anchors drift, prefer the clone's current content and note the drift when handing back.

## Phase 1 — delegate plugin

Prerequisites: Phase 0 clone available for anchor re-verification (not a hard build dependency).

Target files (all new) under `dotfiles/config/opencode/plugins/delegate/`:

- `index.ts` — plugin entry; registers the tool named exactly `task`.
- `config.ts` — load and validate `delegate.json`.
- `capacity.ts` — usage cache read + threshold/wait/report decision.
- `session.ts` — child create/prompt/abort/resume wrapper.

Also edit `dotfiles/config/opencode/opencode.json`: add the plugin to the `plugin` array.
Existing entries use `file://` URLs into dotfiles, e.g. `file:///home/cullyn/dotfiles/config/opencode/plugins/delegate/index.ts`.

### Tool schema

Keep the builtin arg names so the inline card renders its fields:

```ts
{ description, prompt, subagent_type, model?, effort?, task_id? }
```

- `model`: string `"provider/model-id"`, split into `{providerID, modelID}` at the boundary.
- `effort`: a variant name for that model (e.g. `low|medium|high|xhigh|max` on Anthropic adaptive models).

### Spawn sequence

1. `ctx.ask` gate with permission id `task` and the subagent pattern (mirrors the builtin's permission prompt).
2. If `model` is omitted, inherit from the parent's latest assistant message via `client.session.messages` (mirrors `task.ts:165–170`).
3. Derive child permissions from the parent: copy deny + external_directory rules, add `todowrite` and `task` denies unless the subagent's ruleset permits (safety-critical; mirror `subagent-permissions.ts:14–27`).
4. `client.session.create({parentID: ctx.sessionID, title, agent, permission})`, or reuse the child session when `task_id` is supplied.
5. Prompt via v1-path `POST /session/{id}/message` with `{model, variant: effort, agent, parts}`; cast the body for `variant` since v1 typegen omits it.
6. `ctx.metadata({title, metadata: {sessionId, parentSessionId, model}})` so the inline card gets click-through, live sync, and the resolved effort when present.
7. Wire a `ctx.abort` listener → session abort (mirrors `task.ts:296–333`).
8. Return the last text part of the assistant message wrapped in the builtin `<task id=...><task_result>` XML shape.

### Effort validation

Validate `effort` against the model's variant table before prompting.
On mismatch, return an explicit tool error naming the valid efforts for that model.
Never let an unknown variant pass through; upstream silently applies no reasoning options (`request.ts:80–83`).

### Capacity check (before spawning)

Read `~/.cache/opencode/usage-sidebar/{providerID}.json` for the target model's provider.

- Cache missing or stale (older than `staleCacheMinutes`): proceed and annotate the result with the staleness.
- Any capped window (`usedPercent` over its threshold) with an unknown `resetAt` or a reset beyond `maxWaitMinutes`: return a structured capacity report WITHOUT spawning: `{capped, window, usedPercent, resetAt}` so the orchestrator re-picks.
- Otherwise sleep abortably until the LATEST reset among all capped windows, then spawn, so no window remains capped after the wait.
- Capped resets already in the past are excluded from the wait target; if all are past, proceed and annotate the result as stale usage data.

Verification (compile-level; behavioral checks are Phase 4):

```bash
cd ~/dotfiles/config/opencode && bunx tsc --noEmit
```

Then restart opencode and confirm the plugin loads without errors.

## Phase 2 — config

Prerequisites: none (Phase 1 reads this file; either order compiles, but write it before runtime testing).

Target file (new): `dotfiles/config/opencode/delegate.json`.

Physics only, no tiers or routing tables:

```json
{
  "thresholds": { "H": 90, "W": 85 },
  "maxWaitMinutes": 20,
  "staleCacheMinutes": 15,
  "providers": { "anthropic": {}, "openai": {} }
}
```

The weekly threshold sits below the hourly deliberately: W is the scarce rationed budget, H self-heals fast.

Verification: `config.ts` loads it without validation errors on plugin start.

## Phase 3 — drive model-affinity instructions

Prerequisites: Phases 1–2 in place (the instructions reference the tool's model/effort args and capacity reports).
This phase needs the user's opinions; the seed affinities below are hypotheses the user will correct.

Target file: `dotfiles/config/opencode/agents/drive.md` — add a `## Model affinity` section (prose, user-editable over time).

Contents to include:

- Instruct drive to name model + effort on every `task` call.
- Seed affinities:
  - Commits → `gpt-5.4-mini-fast` low; escalate to `gpt-5.5` high for multi-patch detangle stories.
  - Scouts and dirty-state checks → mini low.
  - Frontend builds → anthropic `opus-4-8` high.
  - Plan critique and deep review → `gpt-5.5` xhigh.
  - Orchestration and synthesis stay on the primary (fable); never delegated.
- Capacity report handling: re-pick provider, downgrade effort, or surface to the user.

Also sweep all agent frontmatter under `dotfiles/config/opencode/agents/` to confirm zero `model` keys remain; agents must stay model-agnostic.

```bash
grep -rn "^model" ~/dotfiles/config/opencode/agents/
```

Verification: the grep returns nothing; drive.md renders the new section; user has reviewed the seeds.

## Phase 4 — cutover and verification

Prerequisites: Phases 1–3 complete; run interactively with the user in the TUI.

Shadowing is active as soon as the plugin registers.
Rollback = remove the plugin line from `opencode.json`.

Smoke checks in smallest-falsifier order:

1. Inline card renders + click-through works on a trivial delegation.
2. Child runs the requested model (`message.updated` assistant event carries `modelID`) and effort (message `variant` field).
3. Invalid effort → explicit error naming valid efforts.
4. Permission inheritance: child is denied a bash pattern the parent denies.
5. Capacity path: set a threshold to 0 → report-not-spawn; restore afterward.
6. `task_id` resume works; mid-task abort kills the child.

Re-run this smoke list after every opencode upgrade (autoupdate is false, so breakage only arrives on chosen upgrades).

## Risks

- Shadow fragility: relies on unguarded registry iteration order and the `registry.ts:295` string compare; upstream dedupe would break loudly as a tool conflict. Fallback: rename the tool to `delegate` + config `tools: {task: false}`; loses only the inline card.
- v1 typegen cast for `variant`: runtime-safe today; revisit when the SDK catches up.
- Usage cache format coupling: both plugins are user-owned, so drift is local and self-inflicted.
- Foreground blocking: the child run + wait budget pins the parent turn; keep `maxWaitMinutes` modest.

## Non-goals

- Sleep-until-reset overnight scheduler: needs background subagents + resume; separate project.
- Subagent sidebar: obsoleted by the inline card.
- v2 SDK adoption and auto-fallback routing tables: see Rejected alternatives.

## Roadmap

- Phase 4 TUI smoke remains: verify card click-through and that cards render resolved effort (e.g. "GPT-5.5 medium"); the effort-on-card change is committed but runtime-unverified.
- Robustness fix: `task` calls missing `subagent_type` crash with `undefined is not an object (evaluating '_.replaceAll')`; validate args (subagent_type present and a known agent) and return an explicit tool error.
- Capacity providers: `delegate.json` carries anthropic + openai only, so xai parent sessions error on `task`; the opencode-go provider is newly enabled in `opencode.json` but absent from `delegate.json`; add both once they show real usage signals.
- Usage-plugin phase (deferred by user): xai weekly window via the Grok CLI consumer endpoint `GET https://cli-chat-proxy.grok.com/v1/billing?format=credits` with `Authorization: Bearer <opencode xai oauth access>` + `X-XAI-Token-Auth: xai-grok-cli`.
  - Response: `currentPeriod{type: USAGE_PERIOD_TYPE_WEEKLY, start, end}`, `onDemandCap{val}`, `onDemandUsed{val}`, `prepaidBalance{val}`, optional `creditUsagePercent`.
  - First falsifier: whether opencode's refreshed token is accepted (unconfirmed); no `~/.grok/auth.json` fallback.
  - Widget needs note-plumbing to show draining balances; `cleanUsage` strips notes on the success path.
- opencode-go usage: no public or API-key usage endpoint exists (verified against gateway source + live probe); the console reads its DB via cookie-authed RPC.
  - User direction: cookie-simulation from the browser is the acceptable creative path for opencode-go, and possibly for xai/anthropic-style gaps.
  - opencode-go reportedly has hourly/weekly/monthly windows and is draining-based; design later.
- After usage works: weave xai (imagegen, x-search, websearch) and opencode-go into drive.md model affinities.
