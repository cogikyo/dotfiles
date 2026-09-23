---
name: rebase
description: Load before ANY rebase or rebase-conflict work, however small; shared rebase procedure for Collab approval and authorized build/git execution on the resolved current branch, including owned continuation and conflict resolution.
---

# Rebase

## Authority

Collab may plan and execute approved work.
Only attended Collab may launch `build/git`, after presenting repository/worktree, branch and refs, intended mutations, destructive effects, checks, and stop conditions.
The task uses normal ASK semantics, including remembered approvals, with `unattended: true`.
Skill loading grants no execution authority; the worker follows only its approved named workflow and returns missing decisions to Collab.
Use a bounded read-only scout for context-heavy Git archaeology before worker dispatch.
If the remaining work needs a fresh attended session, provide a handoff rather than spawning Collab.

Rebase the already-resolved current branch onto an explicit approved upstream or onto commit.
Require the repository, selected worktree, current branch, upstream or onto OID, expected local tip, and verification commands.
Stop if branch or worktree selection would be required.
Stop if the in-progress operation is a merge; tell the caller to load `commit` instead.

Do not rewrite published history unless the caller explicitly accepted that risk.
Do not squash, drop, skip, reorder, or edit commit messages in this workflow.
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
Resolve both onto and upstream to OIDs; use `git rebase --onto <onto-OID> <upstream-OID>` on the verified current branch.
Inspect effective rebase configuration before starting; stop if it would autostash, update other refs, execute commands, or enable unapproved transforms.
Inspect patch-equivalent and empty commits before starting; stop if replay could silently drop an approved commit.
Do not start a merge.
Keep the operation in this workflow until it completes, safely aborts, or pauses on a semantic decision.

When conflicts appear, inspect each unmerged path from the rebase base, ours, and theirs.
Resolve from those sides plus the caller's semantic authority.
Never choose a whole side blindly.
Stage only the explicit resolved paths.
Continue only after inspecting the staged resolution and remaining conflicts.
Use `GIT_EDITOR=true git rebase --continue` to retain the replayed message without an interactive editor; this does not bypass hooks.

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
Return ambiguous onto choice, published-history risk, or conflict intent to Collab; only attended Collab asks the user.
