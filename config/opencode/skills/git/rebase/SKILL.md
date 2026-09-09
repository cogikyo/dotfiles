---
name: rebase
description: Use ONLY in attended Collab for an approved rebase of the already-resolved current branch onto an explicit upstream or onto commit, including continuation of that workflow's in-progress rebase and conflict resolution.
---

# Rebase

## Authority

Only attended Collab follows this procedure and performs its Git mutation; never delegate it.
Keep routine rebases in this session; use a bounded read-only scout for context-heavy Git archaeology.
If the remaining work needs a fresh attended session, provide a handoff rather than spawning Collab.

Rebase the already-resolved current branch onto an explicit approved upstream or onto commit.
Require the repository, selected worktree, current branch, upstream or onto OID, expected local tip, and verification commands.
Stop if branch or worktree selection would be required.
Stop if the in-progress operation is a merge; tell the caller to load `commit` instead.

Do not rewrite published history unless the caller explicitly accepted that risk.
Do not squash, drop, reorder, or edit commit messages unless the caller named those transforms.
Ordinary history editing stays with the user.

## Preflight

Verify the repository root, current branch, selected worktree, worktree list, remotes, dirty state, and active Git operation.
Require a clean starting index and worktree unless this workflow already owns the in-progress rebase.
Never use stash as a fallback.
Inspect local and upstream ancestry before rewriting.
Fetch only when the approved onto ref is remote and the caller authorized the fetch.
Never use `git pull`.

## Rebase

Run an explicit non-interactive rebase onto the approved upstream or onto commit.
Do not start a merge.
Keep the operation in this workflow until it completes, safely aborts, or pauses on a semantic decision.

When conflicts appear, inspect each unmerged path from the rebase base, ours, and theirs.
Resolve from those sides plus the caller's semantic authority.
Never choose a whole side blindly.
Stage only the explicit resolved paths.
Continue only after inspecting the staged resolution and remaining conflicts.

Abort only when that preserves all pre-existing work and the caller asked to abort.
If safe continuation or abort is uncertain, stop with exact OIDs, operation state, conflicted paths, and the decision needed from the user.

### Hook failures

Preserve hook output and inspect status, staged and unstaged diffs, and rebase state before continuing.
Never amend a failed attempt, bypass hooks, or assume hook-created changes are harmless.
Repair only within the active task's approved edit authority and without a new semantic decision.
Run the smallest approved relevant check and stage exact repaired hunks before continuing the still-active rebase.
For substantive, unrelated, or ambiguous failures, stop with the failing hook, affected paths, and a proposed repair owner and check.
Resume only after the repair settles; do not retry continuation if the rebase has already completed.

## Audit and boundaries

Verify the rewritten range against the approved source intent.
Confirm the final tip is the expected descendant of the approved onto commit.
Run the approved checks and confirm that no rebase metadata remains.

Do not push, force, mutate remotes, stash, switch branches, or create or remove worktrees.
Do not start a merge or complete a merge commit.
Do not create an ordinary unrelated commit.
Do not reset, clean, disable hooks, change Git configuration, or edit unrelated files.
Never load or dispatch another Git workflow.

Report preflight and final OIDs, onto ref, conflicts and resolutions, ancestry audit, checks, final status, and residual risk.
Ask the user when onto choice, published-history risk, or conflict intent remains ambiguous.
