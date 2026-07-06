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
Status: queued proposal.
Purpose: map OpenCode session state so a parent, sibling, or fresh session can recover context without manually reading raw chat history.
Inputs: structured session commands or storage after source verification, `.spec/` docs, `.learn/` docs, git status, recent commits, and active child-session metadata.
Output: session graph, work threads, recovery packet, and hazards such as prompts, stale specs, overlapping dirty scopes, and compaction pressure.
Non-goals: do not summarize every message, edit files, commit, spawn sessions, or choose the next owner.
First acceptance test: with active specs, unpushed commits, and one closed thread visible through git history, it distinguishes active owners from closed work and flags sessions needing reconciliation.

## Phase 5: TUI instrumentation

Owner files: upstream OpenCode TUI sources, a local overlay, or a future plugin slot; path decision is still open.
Status: source mapped, not implemented here.
Subagent footer evidence: upstream `packages/tui/src/routes/session/subagent-footer.tsx` already has assistant `providerID`, `modelID`, and `variant` data.
Running task evidence: upstream `packages/tui/src/routes/session/index.tsx` owns `Task()`, `formatSubagentTitle(...)`, and `InlineTool(color=...)`.
Acceptance: leader-down into a child shows agent label, model, non-default effort, context, and cost in the bottom bar.
Acceptance: running tasks show group and effort using both labels and color, with prefixes for scout, build, review, scribe, verify, and primaries.

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

## Open questions for parent

- Should Drive spawn managed sibling sessions automatically once a `.spec/` packet exists, or only after a human-approved seed?
- Should local OpenCode UI changes be patched upstream, maintained as a local overlay, or deferred until a plugin slot exists?
- Should `scout/session` read raw exported transcripts, or only structured session metadata plus durable artifacts?

## Condensed next steps

1. Restart OpenCode so the continuity plugins load.
2. Observe one Drive runtime through checkpoint, compact, and renewal paths, then record the result here.
3. Choose the upstream, overlay, or plugin-slot path for subagent footer and running-task instrumentation.
4. Review the `scout/session` proposal before any agent file is created.
