---
name: commit
---

# Commit

## Authority

Create exactly one atomic conventional commit from explicit paths approved by the caller.
This skill also completes an already-started merge by resolving conflicts and creating the merge commit.
Collab is the default owner.
Attended primary Scheme may use this procedure only for approved `.spec/**` artifacts, and never for merge conflicts.
A child Collab may use it when the parent sent a tight commit-sized brief.
Builders do not use this skill.

Do not edit product, documentation, planning, or configuration content except to resolve merge conflicts in already-conflicted files.
Refuse the operation when the approved scope, repository, branch, worktree, or ownership is ambiguous.
Refuse an ordinary commit when it cannot be isolated without changing unrelated index or worktree state.

## Preflight

Resolve and verify the repository root, current branch, selected worktree, and worktree list before mutation.
Inspect all dirty state with `git status --short --untracked-files=all`.
Detect an in-progress merge with `MERGE_HEAD`, unmerged paths, and `git diff --name-only --diff-filter=U`.
Inspect unstaged and staged content with `git diff` and `git diff --cached`.
Read approved untracked files before staging them.
Inspect `git log --oneline -10` and relevant `git show` output for recent message style.

State the exact approved paths and the one semantic story they form.
Treat pre-existing staged content as separately owned unless the caller explicitly included it or an in-progress merge already owns those paths.
Never silently combine pre-existing staged content with an ordinary candidate commit.
Stop when mixed hunks or pre-existing staged state cannot be isolated safely with index-only operations.

## Merge conflicts

Load this skill when a merge is already in progress and conflicts must be resolved.
Do not start a merge, rebase, cherry-pick, or other integration.
If the in-progress operation is a rebase, stop and tell the caller to load `rebase` instead.

Inspect each unmerged path from the merge base, ours, and theirs.
Resolve from those sides plus the caller's semantic authority.
Never choose a whole side blindly.
Keep edits inside the conflicted files and the conflicting concerns.
Do not use the conflict as permission to edit unrelated files.

Stage only the explicit resolved paths.
Never use broad staging.
After every path is resolved, inspect `git diff --cached` and remaining unmerged paths.
Complete the merge with one merge commit.
Keep the existing merge message unless the caller supplied a replacement.

Abort the merge only when that preserves all pre-existing work and the caller asked to abort.
If safe continuation or abort is uncertain, stop with exact OIDs, conflicted paths, and `Questions for parent`.

Scheme must refuse this section.

## Atomic scope

Commit one approved feature, fix, documentation change, planning artifact, merge resolution, or other semantic story.
Do not treat a broad approved path set as proof that every change belongs together.
If the approved scope contains more than one story, stop and return the proposed split instead of creating multiple commits.
Do not split one coherent cross-file behavior merely to reduce file count.

Stage only explicit approved paths and hunks.
Use explicit pathspecs for whole files.
Use `git add -p` or apply a reviewed patch to the index for mixed files when the permission envelope supports it.
Never use `git add .`, `git add -A`, `git add --all`, `git add -u`, `git commit -a`, or a broad directory pathspec.
Use only explicit path-scoped `git restore --staged` operations when unstaging is necessary.
Never discard worktree content.

Inspect `git diff --cached` immediately before committing.
Verify that the candidate contains exactly the approved story and no unrelated staged content.
For ordinary Collab commits, commit the reviewed index without pathspecs so selectively staged hunks remain authoritative.
For Scheme's constrained envelope, use the permitted explicit `.spec/**` path form and first prove that every selected file is wholly approved.

## Message

Use `verb(scope/context): short summary`.
Write an imperative, specific subject that explains the story without requiring the diff.
Derive the scope from the owned path and concern.
Prefer a concrete two-level scope over a broad top-level label.
Use `!` only for a real breaking change.

Prefer these conventional verbs:

| Verb | Use |
| --- | --- |
| `feat` | Major new functionality or an entirely new feature. |
| `add` | A new file, option, component, or small addition. |
| `extend` | An existing feature gains a capability. |
| `improve` | A general quality improvement lacks a more precise verb. |
| `adjust` | A small behavior, permission, ordering, or threshold changes. |
| `edit` | Static content or values change. |
| `fix` | A defect is corrected. |
| `ui` | Visual presentation, layout, styling, or components change. |
| `ux` | User flow, interaction, copy, affordance, or feel changes. |
| `dx` | Developer workflow, tooling, ergonomics, or clarity changes. |
| `refactor` | Internal structure changes while behavior stays the same. |
| `reorg` | Files, directories, modules, or ownership move. |
| `style` | Formatting or whitespace changes only. |
| `docs` | Human-facing documentation changes. |
| `test` | Tests or test artifacts change. |
| `chore` | Build, dependency, or configuration maintenance changes. |
| `ci` | CI or deployment automation changes. |

Never use vague `update`.
Do not default to `improve` or `adjust` when a precise verb explains the change.
A tiny commit may use only the subject.
Otherwise add one or two short bullets about behavior, intent, constraints, or important design choices.
Do not use the body as a file inventory.
Pass each message part with a separate `-m` argument so composition stays non-interactive and auditable.
A merge commit may keep the generated merge subject when it already names both parents.

## Hooks and failures

Run hooks normally.
Never use `--no-verify`, weaken hooks, change Git configuration, or create an empty commit.
After a hook failure, inspect status and both diffs again because the hook may have changed files or the index.
Preserve hook output.
If recovery requires content edits, history changes, or destructive worktree operations, stop and report the exact correction another owner must make.
Retry only when allowed index-only staging or message correction can fix the uncreated commit.

## Boundaries

Do not amend, push, start a merge, rebase, cherry-pick, stash, checkout, switch, reset history, clean, or mutate worktrees.
Do not start a rebase or cherry-pick, and do not commit during those operations.
Completing an already-started merge is allowed.
Do not alter branches, refs, remotes, Git configuration, or unrelated repository file content.
Never load or dispatch another Git workflow.

## Audit and report

Verify the new commit with `git show` and record its OID with `git rev-parse`.
Inspect final status plus staged and unstaged diffs.
Report the repository, branch, worktree, approved paths, committed paths, OID, subject, body choice, merge-conflict resolutions when any, hooks, checks, preserved dirty state, and residual risk.
Return `Questions for parent` when scope, ownership, atomicity, conflict intent, or staged state remains ambiguous.
