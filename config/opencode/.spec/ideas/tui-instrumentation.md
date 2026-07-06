# Idea ledger: OpenCode TUI instrumentation

Seeded 2026-07-06 while closing the stale Drive continuity packet.
Goal: preserve the unresolved TUI instrumentation idea without keeping the broad compaction spec alive.
End state: either upstream/plugin slots support the desired signals, or the idea is explicitly rejected.

## Problem

OpenCode already exposes useful session, subagent, model, effort, and running-task data in the TUI internals.
The current plugin surface does not clearly expose all of that data where the user wants it.

## Desired behavior

- Leader-down into a child session shows agent label, model, non-default effort, context, and cost in the bottom bar.
- Running tasks show group and effort using both labels and color.
- Scout, build, review, scribe, verify, and primary sessions have quick visual prefixes.

## Evidence carried forward

- Upstream `packages/tui/src/routes/session/subagent-footer.tsx` has assistant `providerID`, `modelID`, and `variant` data.
- Upstream `packages/tui/src/routes/session/index.tsx` owns `Task()`, `formatSubagentTitle(...)`, and `InlineTool(color=...)`.
- No local implementation path was selected in the closed continuity work.

## Options

1. Wait for or request a plugin slot.
2. Maintain a local overlay against upstream OpenCode TUI.
3. Patch upstream if the change is generally useful.

## Next steps

1. Re-check current upstream TUI slots before any local patch.
2. Choose upstream, overlay, or defer.
3. Delete this idea if the running task and child-session UI becomes good enough upstream.
