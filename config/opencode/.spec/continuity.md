# Continuity plugin: idle-safe automation and concise sidebar

Seeded 2026-07-06 while closing the old compaction packet.
Owner: the Drive session landing the current dirty continuity slice; one owner per dirty thread.
Goal: continuity automation must never interrupt a running agent, and the sidebar must be concise.
End state: idle-gated automation and sidebar v4 are committed, typechecked, reviewed, and observed after an OpenCode restart.

The original five-slice commit plan (hyprd, permissions, scout/session, continuity v3, specs) all landed; see the commits in Status and git history for file ownership.

## Phase A: idle-gated automation (server.ts)

Status: committed.
Ownership: `server.ts`.
Recovery note: automation runs only on `session.idle`; streaming events only refresh the ledger.

## Phase B: sidebar v3 (index.tsx)

Status: committed, then superseded by Phase D feedback.
Ownership: `index.tsx` and child-session ledger filtering in `server.ts`.
Recovery note: v3 was still too messy and wordy after restart.

## Phase C: hygiene (state.ts)

Status: committed.
Ownership: `state.ts`.
Recovery note: `DirtyCoverage` became `EditedFiles`, while persisted ledger field `dirty` stayed for compatibility.

## Phase D: sidebar v4 (curation + layout)

Implementation is the current dirty slice.

Curation and membership:

- The Continuity sidebar is open by default.
- Spec packets show basenames only, matching Markdown Context.
- Spec packets use a lightbulb continuity icon, not a git/status icon.
- Jump targets include only named sibling root sessions.
- Membership is title-as-registry: a root session title matching 3-4 ALL-CAPS words, `[A-Z0-9-]+` words separated by spaces, and `<= 28` chars.
- `continuity_track` renames and registers the current root session; Build and Drive prompts nudge agents to use it.
- Recency alone never qualifies.
- Shared-spec auto-membership is removed.
- Media-context root sessions stay hidden unless explicitly renamed into the title pattern.
- Root cause of the bad surfacing: the media-context plugin spawns ephemeral root sessions for image classification/renaming ("media-context image naming"); they carry fresh ledgers and no `parentID`, so the 24h-recency fallback pulled them into the list.

Rows and layout:

- Tracked rows are single-line clickable rows.
- Left side is icon + literal space + name.
- Right side is relative last-updated age, right-justified.
- Busy sessions show a braille spinner from the real TUI session status source.
- Context pressure is removed from Continuity because the built-in footer/statusline cover it.
- Lock rows show only when live locks exist.
- The level-color rainbow may stay only where remaining rows still need it.
- The tracked-sibling count chip is hidden when the count is zero.

## Verification

- Prior static checks were clean before their commits; see Status commits for exact slices.
- Runtime observation still requires an OpenCode restart: confirm idle-only compaction and live sidebar behavior.
- Phase D runtime observation still requires an OpenCode restart and live sidebar check.

## Decisions and deviations

- Phase D uses title-as-registry instead of a separate registry file, so the registry shape question is closed.
- The writer question is closed: `continuity_track` renames/registers the current root session.
- Context pressure left Continuity because built-in footer/statusline own that signal.
- No recent-session fallback and no shared-spec auto-membership remain.

## Questions for parent

- None queued.

## Status

- Phases A-C committed as 7a91be2c, c468a141, c5fc4bcd, 8e082f35; slice 1 (hyprd) committed as 77c08d21.
- OpenCode restarted; user observed sidebar v3 and gave Phase D feedback.
- Phase D implementation is in progress as the current dirty slice.
- Runtime observation after restart is still required.

## Recovery checks

- Reconcile against `git status`; chat is not authority.

## Next steps

1. Finish the Phase D dirty slice if any intended implementation files remain unstaged or incomplete.
2. Restart OpenCode and observe the live sidebar: default-open state, packet basenames, title-gated sibling roots, spinner, ages, and lock visibility.
3. Finish idle-gated runtime observation: confirm summarize fires only at idle; tune 90k/120k/200k thresholds per task if pressure feels wrong.
4. Delete this packet after Phase D lands and runtime observation passes.
