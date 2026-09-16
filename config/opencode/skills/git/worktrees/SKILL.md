---
name: worktrees
description: Shared branch/worktree procedure for Collab approval, Orchestrator planning, and authorized build/git execution; resolve path@branch, verify checkouts, and manage named safe lifecycle operations.
---

# Worktrees

## Ownership

Collab may plan and execute approved work; Orchestrator may load this procedure to resolve targets and supervise dependencies without Git mutation.
Only attended Collab may launch `build/git`, after presenting repository/worktree, branch and refs, intended mutations, destructive effects, checks, and stop conditions.
The task uses normal ASK semantics, including remembered approvals, with explicit `authority: "write"` and `unattended: true`.
Orchestrator returns that plan to Collab and cannot launch the worker.
Skill loading grants no execution authority; other children inspect Git read-only and work within their assigned checkout without loading this skill.
Do not repurpose another work thread's checkout.

## Resolve the target

Interpret `<repository-or-worktree-path>@<branch>` as a repository target by default.
Split at the final `@`, resolve the filesystem prefix, then check the suffix as a branch or ref in that repository.
Treat the complete string as a literal path only when the user marks it literal or repository evidence rejects the split.
For example, `~/dotfiles@main` means path `~/dotfiles` and ref `main`; a suffix that is not a Git ref stays part of the path.

Worktrees normally live under `<repository>/.worktrees/<name>`.
Use `git worktree list --porcelain` as authority for their paths and branches; directory names are only hints.
A workspace root may contain several repositories without being a repository itself.
Resolve each repository independently rather than carrying one sibling's branch choice into another.

For each target:

1. Verify the repository root, current branch, worktree list, and staged, unstaged, and untracked changes immediately before editing, delegating, or running branch-sensitive commands.
2. Reuse the worktree holding the requested branch, even when the user supplied the main repository path.
3. If the supplied worktree is on another branch, locate the matching worktree instead of switching that checkout.
4. If no matching worktree exists, return the creation plan to Collab unless the current worker dispatch already approves that exact operation; only attended Collab asks the user.
5. Pass children the resolved repository root, exact worktree path, and verified branch or detached ref, never unresolved `path@branch` notation.
6. Recheck Git state after interruptions, child returns, or signs of concurrent checkout changes.

Use these read-only checks in the resolved checkout:

```bash
git rev-parse --show-toplevel
git branch --show-current
git worktree list --porcelain
git status --short --untracked-files=all
```

An empty branch name means detached HEAD; verify its commit against the requested ref before treating it as the target.

## Create an approved worktree

1. Read the repository's placement and setup rules before choosing a destination.
2. Confirm the existing branch, or the new branch name and explicit starting ref; do not assume the current HEAD is the intended base.
3. Check that the branch is not already attached and the destination is unused, and verify the parent directory before creating any missing directories.
4. Run only the approved creation form below, then verify the result in the new checkout.

Run these commands with the shell tool's working directory set to the resolved repository.
The examples use an absolute `$path`, a local `$branch`, and an explicit approved `$base` OID for a new branch.

Attach an existing local branch:

```bash
git worktree add -- "$path" "$branch"
```

Create a new branch from the approved base:

```bash
git worktree add -b "$branch" -- "$path" "$base"
```

A remote-tracking ref or detached checkout needs an explicit branch/tracking or detached-HEAD choice; do not rely on Git guessing the intended form.
Uncommitted changes stay in their original worktree; the new checkout starts from the selected committed ref.

## Verify readiness

Run the target checks inside the selected worktree and confirm its absolute path, branch or commit, and entry in the worktree list agree.
Identify unresolved merges or unexpected changes rather than declaring the checkout ready because creation returned success.
An existing dirty worktree may be usable when its changes are understood and do not interfere with the approved scope.

Untracked context overlays, local config files, and installed dependencies are not copied from another checkout.
Check the scoped instructions and repository-owned setup requirements before editing or delegating.
If expected context is missing, stop and establish the authorized repair; a workspace-wide relink or install is not implied by worktree creation.

Report the repository and exact worktree path, verified branch or commit, whether it was reused or created, and any setup blocker.
For creation, include the starting ref; distinguish a Git checkout that exists from one ready for the assigned work.

## Recover without losing work

- If checkout reports local changes that ordinary status did not show, inspect the named paths with `git ls-files -v` for `skip-worktree` or `assume-unchanged` flags before choosing a repair.
- On a branch attachment or destination collision, reread the worktree list and inspect the existing path instead of retrying with force.
- After an interrupted creation, reconcile the destination and Git metadata before issuing another add command.
- Do not clear index flags, stash, reset, clean, commit, or delete files to make an operation pass without approval for that action.

Removal requires a named approved path and inspection of tracked, untracked, and ignored files that would be lost.
Require a clean worktree with no files to preserve, no active Git operation, and no other owner using it before `git worktree remove -- <path>`.
Never force removal; preservation work needs a separately approved action.
Deleting the branch is a separate decision.
Use Git's worktree operations rather than deleting a live worktree directory by hand.

## Named branch lifecycle

Create a branch only with its approved name and starting OID, using `git branch -- <name> <base-OID>`; this does not switch the current checkout.
For deletion, verify the named branch's tip, confirm it is not attached to any worktree or owned by another task, and prove its tip is reachable from the approved retained ref.
Use only `git branch -d -- <name>`; if Git refuses, return to Collab without `-D`, force, ref deletion, or configuration changes.
Do not rename branches, switch existing checkouts, or prune or repair worktree metadata under this workflow.
After each mutation, inspect branch refs, worktree list, and status and report what changed and what remains preserved.
