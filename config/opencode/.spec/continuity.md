# Continuity plugin: idle-safe automation and concise sidebar

Seeded 2026-07-06 while closing the old compaction packet.
Owner: the Drive session landing the current dirty continuity slice; one owner per dirty thread.
Goal: continuity automation must never interrupt a running agent, and the sidebar must be concise.
End state: idle-gated automation and sidebar v3 are committed, typechecked, reviewed, and observed after an OpenCode restart.

## Dirty-state map (commit plan)

1. hyprd agent tabs: `cmds/*`, `config/hypr/binds.conf`, `config/kitty/kitty.conf`; keybind changes are user-confirmed intentional.
2. permissions: `config/opencode/opencode.json` (`/usr` allow, root grep deny), `plugins/delegate/session.ts`, README permissions paragraph only.
3. scout/session: `config/opencode/agents/scout/session.md` plus the `scout/session` rows in `agents/{build,drive,plan}.md`.
4. continuity v3: `plugins/opencode/continuity/{index.tsx,server.ts,state.ts}` plus remaining README continuity hunks.
5. specs: delete `.spec/compaction.md`, add `.spec/ideas/{tui-instrumentation,usage-429}.md`, add this packet.

## Phase A: idle-gated automation (server.ts)

Problem: `event` currently triggers `summarizeFromLedger`/`renewFromLedger` on streaming events, so auto compaction can interrupt an active agent turn.
Fix: refresh the ledger on relevant events, but run automation only when `event.type === "session.idle"`.
Also drop `message.part.updated` and `session.status` from `isRelevantEvent`; per-chunk full message reads are churn.
The `experimental.session.compacting` and `experimental.compaction.autocontinue` hooks stay unchanged.

## Phase B: sidebar v3 (index.tsx)

User verdict on v2: still messy and wordy.
Shape:

- Collapsed chips stay minimal: pressure warn, locks, renewal, related count.
- Expanded always opens with one muted status row: pressure icon, tokens, level.
- Related sessions render as a flat clickable list (max 4), icon colored by level, muted age; drop the per-spec group header rows.
- Related list holds root sessions only: shared-spec always (packet icon), non-shared only if active within 24h; subagents (agent contains `/` or session has `parentID`) excluded, and server.ts skips ledger writes for child sessions.
- Lock rows compact to basenames and short ids.
- Pressure/missing-packet/lock/renewal notice rows stay hidden until active.
- Drop the panel subscription on `message.part.updated`; the 10s timer plus message/diff/compacted events are enough.

## Phase C: hygiene (state.ts)

Rename vestigial `DirtyCoverage` type to `EditedFiles`; keep the persisted ledger field name `dirty` for compatibility and say so in a comment.

## Verification

- `node_modules/.bin/tsc -p config/opencode/tsconfig.json` clean.
- `go vet` + `go test` for `cmds/internal/hyprd/session` before the hyprd commit.
- Runtime observation still requires an OpenCode restart: watch one Drive run hit compact pressure and confirm summarize fires only at idle.

## Status

- Phase A: implemented and reviewed (plus child-session ledger skip); awaiting commit.
- Phase B: implemented and reviewed, review fixes applied (merged status row, dead PressureNotice/pressureText/renewalIsBetter deleted, related-list corrections); awaiting commit.
- Phase C: implemented and reviewed; awaiting commit.
- Slice 1 (hyprd) committed as 77c08d21.

## Recovery checks

- Reconcile against `git status`; chat is not authority.
- If interrupted mid-commit, the slice map above defines ownership; commits are ordered 1, 3, 2, 4, 5 so README hunks split cleanly between slices 2 and 4.

## Next steps

1. Land the five commits above.
2. Restart OpenCode; observe idle-gated compact behavior and sidebar v3.
3. Tune 90k/120k/200k thresholds per task if pressure feels wrong.
4. Delete this packet once runtime observation passes.
