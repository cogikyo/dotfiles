# Continuity plugin: idle-safe automation and concise sidebar

Seeded 2026-07-06 while closing the old compaction packet.
Owner: the Drive session landing the current dirty continuity slice; one owner per dirty thread.
Goal: continuity automation must never interrupt a running agent, and the sidebar must be concise.
End state: idle-gated automation and sidebar v4 are committed, typechecked, reviewed, and observed after an OpenCode restart.

The original five-slice commit plan (hyprd, permissions, scout/session, continuity v3, specs) all landed; see the commits in Status and git history for file ownership.

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

## Phase D: sidebar v4 (curation + layout)

Source: user observed sidebar v3 after restart and gave design feedback.
Implementation deferred to the abbott session because visual iteration needs the user present to eyeball the TUI.

Curation over recency:

- Related sessions are curated only; recency alone never qualifies.
- Delete the recent-within-24h fallback in `relatedSessions()` (index.tsx).
- Root cause of the bad surfacing: the media-context plugin spawns ephemeral root sessions for image classification/renaming ("media-context image naming"); they carry fresh ledgers and no `parentID`, so the 24h-recency fallback pulled them into the list.
- Shared-spec detection may stay as one curation input.

Curation mechanism (decided direction, sub-questions open):

- Membership in the related list requires BOTH a short display name AND explicit registration in a shared registry that tracks which sessions are related and managed.
- The registry is the single source of truth for sidebar membership; ledgers keep tracking all sessions for recon.
- The registry likely lives alongside the ledgers in the continuity state dir.
- scout/session reads and writes the same registry so related-session info syncs across channels and sessions.
- Open sub-questions: registry file shape, and who writes it (model TUI command vs plugin tool vs scout/session).

Rows and layout:

- Sessions actively running show a braille spinner.
- Open question: detect running via the session status API vs ledger `lastEvent` freshness.
- The level-color rainbow stays; the problem is discoverability of meanings, not the palette.
- Document each color per `levelColor` in index.tsx: green=healthy, sky=watch, yellow=checkpoint, orange=compact, pink=renew, red=blocked, muted=stale.
- Make the meanings discoverable: README legend at minimum; in-TUI legend optional, decide on abbott.
- Consistent padding across all sidebar rows.
- Related sessions get model-assigned short display names: ALL CAPS, 3-4 words, type-first like commit types (e.g. ADJUST CONTINUITY SIDEBAR), stored so rows fit one line.
- Row layout: icon + short name left, age/duration right-aligned (the current "10h" label), single line, no wrap.

## Verification

- `node_modules/.bin/tsc -p config/opencode/tsconfig.json` clean.
- `go vet` + `go test` for `cmds/internal/hyprd/session` before the hyprd commit.
- Runtime observation still requires an OpenCode restart: watch one Drive run hit compact pressure and confirm summarize fires only at idle.

## Status

- Phases A-C committed as 7a91be2c, c468a141, c5fc4bcd, 8e082f35; slice 1 (hyprd) committed as 77c08d21.
- OpenCode restarted; runtime observation partially done.
- User observed sidebar v3 and gave design feedback, captured as Phase D.
- Phase D: not started; resumes on abbott after a master merge.

## Recovery checks

- Reconcile against `git status`; chat is not authority.

## Next steps

1. Merge master, then resume on abbott.
2. Settle the registry sub-questions: file shape and writer (model TUI command vs plugin tool vs scout/session).
3. Implement Phase D with the user present to eyeball the TUI: registry-gated curation, spinner, color legend (README + optional in-TUI).
4. Finish runtime observation: confirm idle-gated compact behavior; tune 90k/120k/200k thresholds per task if pressure feels wrong.
5. Delete this packet after Phase D lands and runtime observation passes.
