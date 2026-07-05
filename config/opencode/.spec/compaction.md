# Drive self-management: compaction, managed sessions, session scout

Seeded 2026-07-05 from the user's AFK-but-present correction that Drive should manage other OpenCode sessions when durable artifacts coordinate them.
Goal: long Drive runs should keep error-correction state across compaction, sibling sessions, and interrupted leaves without surfacing user prompts.
End state: permission-prompt hardening landed, Drive session doctrine updated, a reviewed compaction route selected, and `scout/session` either implemented or queued with clear acceptance criteria.

## Status

- Permission prompt hardening: implemented in the working tree; restart required before running sessions use it.
- Managed session doctrine: implemented in the working tree; restart required before running sessions use it.
- Self-managed compaction: planning; no route selected yet.
- `scout/session`: planning; no agent file should be added until this spec is reviewed.
- TUI instrumentation: source mapped for subagent footer model/effort and running-task group/effort colors.

## Decisions log

- The current live Drive session cannot spawn a new primary because its loaded higher-priority instructions still say not to fork sessions.
- Future Drive should be allowed to spawn managed OpenCode sessions when a `.spec/` packet records objective, ownership, dirty state, recovery, and expected commits.
- Managed sessions are sibling roots, not nested leaf managers; each session stays one hop from a human who can step into it.
- Drive-mode approval prompts are a safety bug; the orchestrator should deny/report approval-shaped operations instead of letting leaf prompts reach the TUI.
- Drive-created `task_id` resumes are disabled because an old child can carry stale ask-shaped permissions; Drive must re-brief a fresh child to apply the current AFK envelope.
- xAI and OpenCode usage rows now appear live from the user's screenshots; the usage adapter now attempts one noninteractive `grok models` refresh before reporting `no auth` or `expired`.
- The Fable block is likely a Fable 5 promotional weekly sub-bucket: Anthropic documents that promo Fable can consume up to 50% of weekly subscription limits and then block while overall weekly usage still has headroom.
- `SubagentFooter` in upstream OpenCode v1.17.13 already has the model and variant data needed to render model and effort in the bottom subagent bar.
- OpenCode auto-compaction triggers from model context overflow math, not a fixed 100k token threshold.
- Manual compaction in the v1.17.13 TUI uses `sdk.client.session.summarize(...)`, which posts to `/session/{id}/summarize` and calls `SessionCompaction.create(...)` before `prompt.loop(...)`.
- OpenCode also exposes a v2 `session.compact` endpoint at `/api/session/{sessionID}/compact`; automation must choose the API surface that the plugin client actually exposes.

## Problem shape

Drive has three different failure modes that look like one entropy leak:

1. Context pressure makes the primary session forget why a phase exists.
2. Parallel or long-running leaves create state that only exists in chat history.
3. Permission prompts from children stall AFK runs even when Drive should have denied the operation.

The repair should make state explicit before context gets hot, like checkpointing a simulation before numerical error dominates.

## Route A: true self-compaction

Objective: keep one Drive session alive by forcing a high-quality self-summary before the context window gets dangerous.

Candidate mechanism:

1. Detect context pressure before about 100k tokens or another empirically safer threshold.
2. Write a compact checkpoint into the owning `.spec/` doc: current objective, landed commits, dirty files, decisions, blockers, active child sessions, and next command.
3. Trigger OpenCode compaction or rely on configured auto-compaction only after the checkpoint exists.
4. Resume from the checkpoint and delete stale details from the spec after commits land.

Open checks:

- Source map found `packages/opencode/src/session/overflow.ts`: usable context is model context or input limit minus `compaction.reserved`, defaulting to a 20k-or-max-output buffer.
- Source map found `packages/opencode/src/session/prompt.ts`: after a finished assistant turn, overflow creates an automatic compaction task before normal continuation.
- Source map found `packages/opencode/src/session/compaction.ts`: `tail_turns`, `preserve_recent_tokens`, and `reserved` control what recent context survives summary.
- Plugin hooks exist in `packages/plugin/src/index.ts`: `experimental.session.compacting` can append context or replace the compaction prompt, and `experimental.compaction.autocontinue` can disable the synthetic continue turn.
- Still open: whether a TUI or server plugin can observe current token usage early enough and call `session.summarize` or `session.compact` before overflow.

Risks:

- A model-authored summary can preserve the wrong invariants if no spec checkpoint exists first.
- A compaction hook that runs too late is indistinguishable from memory loss.
- Hidden state in child sessions still requires a session scout even if the parent compacts well.

## Route B: managed session renewal

Objective: move a bounded phase into a fresh Drive session before compaction pressure becomes the bottleneck.

Candidate mechanism:

1. Parent Drive writes or updates the owning `.spec/` packet.
2. Parent records current git status, upstream divergence, active child sessions, and the exact next phase owner.
3. Parent starts a new Drive session with the spec path and bounded phase brief.
4. New session runs scout ──▶ build ──▶ review ──▶ scribe ──▶ commit against that packet.
5. Parent or the next human session reconciles by reading the spec, git commits, and `scout/session` output.

Acceptance criteria:

- No phase depends on hidden chat memory from its parent.
- Each managed session has one durable owner packet and one dirty-state thread.
- The parent can die after spawning and the new session can still converge from `.spec/` plus git.
- A session that hits a permission prompt treats it as a bug in the permission envelope, not a user decision point.

Risks:

- Multiple Drive sessions can trample the same file unless the spec records ownership and dirty-state scope.
- Token savings become coordination debt if sessions are spawned for tiny slices.
- Current OpenCode CLI/session APIs need source verification before automating spawn commands.

## `scout/session` proposal

Purpose: map OpenCode session state so a parent, sibling, or fresh session can recover context without reading raw chat history manually.

Inputs to inspect read-only:

- `opencode session` and `opencode export` output for named or recent sessions.
- OpenCode session storage or DB only after source verification of the local path and schema.
- `.spec/` docs, `.learn/` docs, git status, recent commits, and active child-session IDs from delegate tool metadata.

Report shape:

- Session graph: parent, children, siblings, agent names, models, variants, and last activity.
- Work threads: spec owner, files touched, dirty files, staged files, commits landed, and likely owner.
- Recovery packet: last durable objective, decisions, blockers, next commands, and prompts that need re-briefing.
- Hazards: permission prompts, refusal-tainted children, stale specs, overlapping dirty scopes, and sessions past compaction pressure.

Non-goals:

- Do not summarize every message.
- Do not edit, commit, spawn sessions, or choose the next owner.
- Do not become a coordinator; it maps state for a primary to decide.

First acceptance test:

- Given this repo with 18 unpushed commits and `fleet.md`, `scout/session` can say which session/thread owns `fleet.md`, which commits landed, which tasks remain, and whether any running child sessions need reconciliation.

## TUI instrumentation track

Subagent footer model and effort:

- Source map points to upstream `packages/tui/src/routes/session/subagent-footer.tsx` in OpenCode v1.17.13.
- The footer already has access to assistant `providerID`, `modelID`, and `variant`; no backend change is needed.
- Acceptance: leader-down into a child shows agent label, model, non-default effort, context, and cost in the bottom bar.

Running task colors:

- Source map points to upstream `packages/tui/src/routes/session/index.tsx`, especially `Task()`, `formatSubagentTitle(...)`, and `InlineTool(color=...)`.
- Agent group colors should derive from the group prefix: scout, build, review, scribe, verify, plus primaries.
- Effort color should ramp by intensity: low muted, medium/info, high/warn, xhigh/max danger or accent.
- Avoid encoding semantics only by color; preserve text labels for accessibility and logs.

## Next steps

1. Verify and land the delegate permission hardening plus Grok CLI refresh automation.
2. Choose an upstream, overlay, or plugin-slot path for the TUI instrumentation.
3. Draft `scout/session` as a proposed leaf in this spec, then review before editing agent files.
4. Choose Route A, Route B, or a hybrid: checkpoint before compaction, spawn managed session when the checkpoint exceeds a bounded phase.

## Queued for user

- Review whether Drive may spawn managed sibling sessions automatically once a `.spec/` packet exists, or only after a human-approved seed.
- Choose whether local OpenCode UI changes should be patched upstream, maintained as a local package overlay, or deferred until a plugin slot exists.
- Decide whether `scout/session` should read raw exported transcripts or only structured session metadata plus durable artifacts.
