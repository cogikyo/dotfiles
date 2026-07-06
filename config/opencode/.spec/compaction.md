# Drive continuity and managed sessions

Seeded 2026-07-05 from the user's AFK-but-present correction that Drive should manage sibling OpenCode sessions when durable artifacts coordinate them.
Goal: long Drive runs should preserve error-correction state across compaction, sibling sessions, and interrupted leaves without surfacing user prompts.
End state: continuity plugins are observed in a restarted runtime, Drive session doctrine is settled, `scout/session` is either implemented or explicitly queued, and the TUI instrumentation path is chosen.

## Phase 1: permission prompts and managed session doctrine

Owner files: Drive instructions and OpenCode permission config were the implementation owners; this spec retains only the policy surface.
Status: implemented before this condensation, with runtime effect gated on restarting OpenCode.
Exit evidence: approval-shaped operations should be denied or reported by Drive instead of reaching the TUI as user prompts.

## Phase 2: continuity plugin route

Owner files: `config/opencode/plugins/opencode/continuity/*`, `config/opencode/opencode.json`, `config/opencode/tui.json`, and this spec.
Status: landed as `fb189fb0`.
Exit evidence: server and TUI plugins, state ledger helpers, pressure calculation, manual commands, and config wiring are in git history.
Runtime observation: after restart, continuity ledgers were written for this Drive session and child scouts.
Runtime issue: the TUI sidebar hit an external-directory prompt while reading runtime locks under `/run/user/1000/opencode/continuity/`.
Current fix: allow the continuity runtime directory in `opencode.json` and memoize the sidebar state read per revision.
Remaining verification: restart OpenCode again, then observe one Drive runtime through checkpoint, compact, and renewal paths.

## Phase 3: fleet cleanup packet

Owner files: `config/opencode/.spec/fleet.md`, `config/opencode/.spec/ideas/agent-skills.md`, and this spec.
Status: closed as `fbacb707`.
Exit evidence: the fleet spec was deleted and surviving ideas moved to the ideas packet.
Remaining verification: none in this spec.

## Phase 4: `scout/session`

Owner files: proposed under `config/opencode/agents/scout/`; no agent file should be added before review.
Status: queued proposal; direct file check confirmed no `config/opencode/agents/scout/session.md` exists yet.
Purpose: map OpenCode session state so a parent, sibling, or fresh session can recover context without manually reading raw chat history.
Inputs: structured session commands or storage after source verification, `.spec/` docs, `.learn/` docs, git status, recent commits, and active child-session metadata.
Output: session graph, work threads, recovery packet, and hazards such as prompts, stale specs, overlapping dirty scopes, and compaction pressure.
Non-goals: do not summarize every message, edit files, commit, spawn sessions, or choose the next owner.
First acceptance test: with active specs, unpushed commits, and one closed thread visible through git history, it distinguishes active owners from closed work and flags sessions needing reconciliation.
Runtime need: user confirmed this multiple-session relay is needed for proper compaction and continuity decisions.

## Phase 5: TUI instrumentation

Owner files: upstream OpenCode TUI sources, a local overlay, or a future plugin slot; path decision is still open.
Status: source mapped, not implemented here.
Subagent footer evidence: upstream `packages/tui/src/routes/session/subagent-footer.tsx` already has assistant `providerID`, `modelID`, and `variant` data.
Running task evidence: upstream `packages/tui/src/routes/session/index.tsx` owns `Task()`, `formatSubagentTitle(...)`, and `InlineTool(color=...)`.
Acceptance: leader-down into a child shows agent label, model, non-default effort, context, and cost in the bottom bar.
Acceptance: running tasks show group and effort using both labels and color, with prefixes for scout, build, review, scribe, verify, and primaries.

## Phase 6: continuity sidebar v2

Owner files: `config/opencode/plugins/opencode/continuity/*`, `config/opencode/plugins/shared/sidebar-section.tsx`, and this spec.
Status: implemented in the current continuity WIP, awaiting commit and restarted-runtime observation.

User correction (evidence):
- The current key/value rows `packet:`, `pressure:`, `dirty:`, `lock:`, `renew:` are too noisy.
- `Continuity 14% healthy` is really pressure percent plus artifact status, not a real health score.
- Pressure is model-window based, but the user says ~120k+ tokens is the cognitive dumb zone; model budget like 390k is not the main UI anchor because the usage indicator already owns budget.
- `open:` rows are misleading: they mean ledger/git dirty files not covered by the current session diff, not files opened or read; the concept should become a WIP/sync hazard or be hidden.
- Lock and renewal rows should show only when active.
- The user keeps the `Continuity` name and health idea, and wants compact Nerd Font icon chips plus related sessions sharing a `.spec` packet with clickable sidebar navigation.

UI review proposal (conjecture, to tune):
- One always-visible health rollup, with exceptional rows hidden until they matter.
- Absolute token thresholds around 80k/120k/160k or 72k/96k/120k.
- Related-session rows grouped by shared `.spec` packet.
- `SidebarSection` detail may need JSX or chip support to render icon chips.

API review proposal (conjecture, plugin-feasible in the continuity sidebar):
- Add or derive WIP sync, a health vector, cognitive pressure, and a spec-index reverse map.
- Keep the old `dirty` field for compatibility.
- Mark stale related sessions by last-seen.
- Route navigation via `api.route.navigate('session', {sessionID})`.

Not plugin-feasible: subagent footer and task-row instrumentation have no current published slot, so they still route through Phase 5's upstream or local-patch decision.

Automation facts from landed code (evidence, runtime unobserved until restart):
- Drive sessions with a healthy artifact can auto-summarize or create a fresh root Drive renewal session on shared continuity settings.
- Non-Drive modes receive only the compaction checkpoint hook and get no automatic renewal under current policy.
- Manual TUI renew already exists and should be reviewed before renewal expands beyond Drive.

## Phase 7: pressure threshold settings

Owner files: `config/opencode/plugins/opencode/continuity/pressure.ts`, `config/opencode/plugins/opencode/continuity/settings.*`, server/TUI continuity plugins, and docs.
Status: implemented in the current continuity WIP, awaiting commit and restarted-runtime observation.
User correction: 120k is usually where compaction should happen soon; exact pressure depends on task, but letting Drive cruise much past it should be rare.
Implementation direction: make pressure settings explicit and shared by server and TUI plugins, using absolute token thresholds plus percent/remaining-context guardrails.
Chosen default: checkpoint at 90k, compact at 120k, renew at 200k, with percent guardrails at 75/90/96 and renewal when remaining context falls below 12k.
Non-goal: do not change OpenCode's native `compaction` schema with plugin-specific keys.

## Decisions and deviations

- The current live Drive session cannot spawn a new primary because its loaded higher-priority instructions still say not to fork sessions.
- Future Drive may spawn managed sibling OpenCode sessions only when a `.spec/` packet records objective, ownership, dirty state, recovery, and expected commits.
- Managed sessions remain sibling roots, not nested leaf managers.
- Each managed session stays one hop from a human who can step into it.
- Drive-created `task_id` resumes stay disabled because an old child can carry stale ask-shaped permissions.
- Drive must re-brief a fresh child to apply the current AFK envelope.
- OpenCode auto-compaction is driven by model context overflow math, not a fixed 100k token threshold.
- Chosen continuity route is hybrid checkpointing, not pure Route A or pure Route B.
- The hybrid route writes a machine ledger before compaction, keeps `.spec/*.md` as durable truth, and renews into a fresh root Drive session when renewal is cheaper than in-place continuation.
- Runtime ledgers are mutex/shared state only; raw chat is never silently promoted to authority.
- Continuity runtime locks live under `${XDG_RUNTIME_DIR}/opencode/continuity/`; ledger files stay under `${XDG_STATE_HOME:-~/.local/state}/opencode/continuity/`.
- The TUI continuity sidebar may read runtime locks often enough to need a stable permission rule, but it should not recompute that filesystem state more than once per UI revision.
- Manual compaction in OpenCode v1.17.13 uses `session.summarize(...)`, while v2 also exposes `session.compact`; automation must use the API surface available to the plugin client.
- The resolved Route A/Route B exploratory detail was condensed after `fb189fb0`; its surviving decision is the hybrid route above.
- The fleet cleanup packet is closed after `fbacb707`; future fleet ideas belong in the ideas packet unless they reopen an active implementation phase.
- Continuity sidebar v2 keeps the `Continuity` name and health framing but replaces the noisy key/value rows with one health rollup plus chips, so exceptional rows stay hidden until active.
- Cognitive pressure is anchored to absolute token thresholds near the ~120k dumb zone, not the model budget, because the usage indicator already owns budget.
- Continuity automation now treats 120k as compact pressure by default, with settings available for task-specific tuning.
- Continuity threshold settings live beside the plugin instead of under `opencode.json.compaction` because the upstream config schema rejects unknown compaction keys.

## Open questions for parent

- Should Drive spawn managed sibling sessions automatically once a `.spec/` packet exists, or only after a human-approved seed?
- Should local OpenCode UI changes be patched upstream, maintained as a local overlay, or deferred until a plugin slot exists?
- Should `scout/session` read raw exported transcripts, or only structured session metadata plus durable artifacts?
- Is active-only WIP hazard count enough, or should WIP details move behind a deliberate drilldown command?
- Should manual TUI renew stay Drive-only, or expand to other modes once reviewed?

## Condensed next steps

1. Restart OpenCode so the continuity plugins load.
2. Observe one Drive runtime through checkpoint, compact, and renewal paths, then record the result here.
3. Observe continuity sidebar v2 after restart; keep the header compact and the expanded body calm.
4. Validate the 90k/120k/200k pressure thresholds in a long Drive runtime and tune per task if needed.
5. Choose the upstream, overlay, or plugin-slot path for subagent footer and running-task instrumentation.
6. Review the `scout/session` proposal before any agent file is created.
