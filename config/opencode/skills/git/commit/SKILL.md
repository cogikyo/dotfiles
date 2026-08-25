---
name: commit
---

# Commit

Use this procedure to create atomic conventional commits or finish an already-started merge.

## Scope

- Use it only when the active brief approves a commit task or an active merge resolution.
- Collab may follow the full procedure.
- Attended primary Scheme may use Single mode only for approved `.spec/**` artifacts and must refuse merge conflicts.

This procedure does not authorize content edits during an ordinary commit.
Only edit already-conflicted files while resolving a merge.

Stop rather than infer authorization when scope, repository, branch, worktree, or ownership is ambiguous.

## Preflight

Before any Git mutation:

1. Verify the repository root, current branch, selected worktree, and worktree list.
2. Inspect `git status --short --untracked-files=all`, `git diff`, and `git diff --cached`.
3. Check `MERGE_HEAD` and `git diff --name-only --diff-filter=U` for an active merge.
4. Read each approved untracked file before staging it.
5. Inspect `git log --oneline -10` and relevant `git show` output for recent message style.

Identify the exact approved paths and their semantic story or story sequence.
Treat pre-existing staged content as separately owned unless the approved scope includes it or an active merge owns it.
Never silently combine that content with an ordinary commit.

## Choose a mode

Choose the smallest mode that fits the inspected state.
Do not ask for a choice between Single and Partial when the diff makes the answer clear.

- **Single** creates one commit from whole files whose changes all belong to one story.
- **Partial** creates one commit from selected hunks because its files also contain other work.
- **Complex** detangles an approved dirty scope into a sequence of atomic commits.

Single and Partial create exactly one commit.
Complex repeats Single or Partial for one story at a time.
Use Complex only when the active brief approves multiple commits or the full dirty scope.

A clean worktree is not the success condition.
Single and Partial preserve unrelated or later work as dirty state.
Complex commits all approved stories, then reports any unapproved or blocked remainder.

### Find each story

One commit tells one semantic story.
Approved paths bound the search; they do not prove that all changes belong together.

Strong split signals include:

- The subject naturally needs `and`.
- Body bullets describe concerns without one shared intent.
- The scope contains distinct features, fixes, or maintenance work.

In Single or Partial mode, propose a split and stop when more than one story is present.
In Complex mode, use the split as the commit sequence.
Keep one coherent cross-file behavior together even when it touches many files.

### Stage one story

1. Stage a whole file only when every changed hunk belongs to the story.
2. For mixed files, select hunks with `git add -p` or apply a reviewed patch with `git apply --cached`.
3. Inspect `git diff --cached` and confirm that the index contains exactly one complete story.
4. Commit the reviewed index without pathspecs so partial staging remains authoritative.

An index patch must come from the inspected worktree diff and contain only the selected story.
Applying it with `git apply --cached` changes the index without changing the worktree.

- Never use `git add .`, `git add -A`, `git add --all`, `git add -u`, `git commit -a`, or broad directory pathspecs.
- Never use backup files, custom index machinery, stash, reset, or worktree edits to isolate a story.
- Unstage only with explicit path-scoped `git restore --staged` operations.
- Never discard worktree content.

Stop when mixed hunks or pre-existing staged state cannot be isolated with index-only operations.
After a Partial commit and after every Complex commit, inspect all remaining staged and unstaged changes again.

Scheme uses its permitted explicit `.spec/**` path form only after proving that every selected file is wholly approved.

## Write the message

Use `verb(scope/context): short summary`.

### Subject

- Write an imperative, specific summary that explains the story without the diff.
- Derive the scope from the owned path and concern.
- Prefer a concrete two-level scope over a broad top-level label.
- Use `!` only for a real breaking change.
- Never use vague `update`.
- Use `improve` or `adjust` only when a more precise verb does not fit.

| Verb       | Use                                                           |
| ---------- | ------------------------------------------------------------- |
| `feat`     | Major new functionality or an entirely new feature.           |
| `add`      | A new file, option, component, or small addition.             |
| `extend`   | An existing feature gains a capability.                       |
| `improve`  | A general quality improvement lacks a more precise verb.      |
| `adjust`   | A small behavior, permission, ordering, or threshold changes. |
| `edit`     | Static content or values change.                              |
| `fix`      | A defect is corrected.                                        |
| `ui`       | Visual presentation, layout, styling, or components change.   |
| `ux`       | User flow, interaction, copy, affordance, or feel changes.    |
| `dx`       | Developer workflow, tooling, ergonomics, or clarity changes.  |
| `refactor` | Internal structure changes while behavior stays the same.     |
| `reorg`    | Files, directories, modules, or ownership move.               |
| `style`    | Formatting or whitespace changes only.                        |
| `docs`     | Human-facing documentation changes.                           |
| `test`     | Tests or test artifacts change.                               |
| `chore`    | Build, dependency, or configuration maintenance changes.      |
| `ci`       | CI or deployment automation changes.                          |

### Body

These are guidelines rather than hard limits:

- A tiny atomic commit may use only the subject.
- Aim for two short bullets about behavior, intent, constraints, or important decisions.
- A normal commit may use up to about six bullets.
- A genuinely large atomic commit may use more.
- Never inventory files or mechanically repeat the subject.

Write the subject and body as one message.
Never pass multiple `-m` arguments because each one creates a separate paragraph.

```bash
git commit -F - <<'EOF'
edit(nvim/editor): tune completion diagnostics

- disable completion ghost text
- guard the diagnostic handler
EOF
```

A merge commit may keep its generated subject when it already names both parents.

## Create each commit

Create the approved commit and run hooks normally.
Only stage and return a command when the active brief explicitly requests manual execution.
If an explicit instruction says to skip hooks, return a command with `--no-verify` and do not run it.

Never execute `--no-verify`, weaken hooks, change Git configuration, or create an empty commit.

### Hook failures

A hook failure does not expand edit authority.

1. Preserve the hook output and inspect status plus both diffs again.
2. Confirm that Git did not create the commit before retrying.
3. Correct message or index-only failures directly, then retry the uncreated commit.
4. Fix a content failure directly only when the active task already authorizes the edit, the fix stays in approved paths, and no semantic decision is required.
5. Run the smallest relevant check, stage the exact repaired hunks, and retry.

Treat hook-created worktree changes as real changes and inspect them before staging.
Never assume they are harmless formatter churn or discard them.

For substantive, unrelated, or ambiguous failures, stop and report the failing hook, affected paths, and smallest likely repair.
Suggest a repair workflow with an owner and falsifying check when the correction exceeds the active task.
Resume committing only after that repair settles.

## Active merge

This skill may finish an active merge, but it must not start any integration operation.
Stop an active rebase and use the `rebase` skill in a separate action.

1. Inspect each conflict from the merge base, ours, and theirs.
2. Resolve from those versions plus the approved intent without choosing a whole side blindly.
3. Edit only conflicted files and their conflicting concerns.
4. Stage only explicit resolved paths.
5. Inspect `git diff --cached` and the remaining unmerged paths after each resolution.
6. Complete the merge with one merge commit.
7. Keep the existing merge message unless the approved brief supplies a replacement.

Abort only when explicitly requested and doing so preserves all pre-existing work.
If safe continuation or abort is uncertain, stop and report the exact OIDs and conflicted paths.

## Boundaries

- Do not amend, push, stash, checkout, switch, reset history, clean, or mutate worktrees.
- Do not alter branches, refs, remotes, Git configuration, or unrelated repository content.
- Do not start a merge, rebase, cherry-pick, or other integration operation.
- Do not commit during a rebase or cherry-pick.
- Do not chain into another Git procedure.

Completing an already-started merge is the only integration exception.

## Finish

Always inspect final status plus staged and unstaged diffs.

- After a commit, verify it with `git show` and record its OID with `git rev-parse`.
- After a handoff, report the staged paths, exact command, and that no OID exists yet.

Summarize the message, affected paths, hooks or checks, preserved dirty state, and residual risk.
Include merge resolutions when applicable.
Surface any unresolved ambiguity without guessing.
