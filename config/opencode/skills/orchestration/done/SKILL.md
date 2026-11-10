---
name: done
description: Use on /done, a request to finish and close, or COLLAB.md's automatic finish point; Collab runs comments, commits, retro judgment, and cleanup, then closes finished lanes and tags the session.
---

# Done

Collab runs this procedure in the current session, in the order below, on `/done`, a request to finish and close, or the finish point in `COLLAB.md`.
Either trigger approves the comment pass, retro handling below, removing landed worktrees, deleting this session tree's own scratch and stopping processes running from it, and closing finished lanes and the session.
It approves nothing else; Git operations follow Collab's `Git` rules.

## Comments

When this session changed code, send `build/scribe` a comment-only audit of exactly those changed hunks.
Fix or delete stale comments; add one only where a reader would otherwise form a wrong belief.
Skip when no code changed or a scribe pass already ran after the last edit.

## Commit

Load `commit` and commit this session's reviewed work.
Ask one question with the `question` tool only when ownership of a path or hunk is unclear.
Leave other sessions' work dirty and skip when nothing is left.
After committing, push new commits under `COLLAB.md`'s `Git` rules where pushes need no yes.
Leave pushes that need the user's yes unpushed and name them in the report.

## Retro

Judge in-session, without subagents, using the Qualify bar in `../../improve/retro/SKILL.md` over this session and its children.
Drop candidates this session already fixed.
On an automatic run, skip live retro; when any candidate qualifies, add `ISSUES` and name the candidates in the report, since `/improve` sweeps `ISSUES` sessions first.
On a user's `/done` or request to finish and close, when any candidate qualifies, load `retro` and run live mode now without asking first; its own confirmation exchange still applies.
Commit approved retro changes under the Commit rules above.
When nothing qualifies, say so in one line.

## Knowledge base

Mail COGIKYO SCRIBE any cogikyo decisions or facts this session has not already sent, following `cogikyo`; skip when there are none.

## Cleanup

For each worktree this session tree created or worked in, skip any another open session is working in and follow `../../git/worktrees/SKILL.md`'s finished-task guard.
Run `../../git/worktrees/landed.sh <worktree>`; on exit 0, remove the worktree and delete its branch under that skill; on exit 1, keep it and report the unique paths; on exit 2, keep it and report the refusal.

Run [cleanup.sql](cleanup.sql) unchanged through `sessions` with `?1` = root session ID and `?2` = now − 3 days in epoch milliseconds.
Select existing `/tmp/opencode/<name>` paths with `shared = 0`; keep `shared = 1` paths and any of unclear ownership without asking (`REFUSALS` hides sessions from the shared check).
Add `~/.local/state/opencode/scratch/<sessionID>/` for each session in the tree.
Keep both scratch roots and the shared `runs.jsonl`.
Keep any scratch or `/tmp/opencode` path that `grep -r` finds cited in `/home/cullyn/cogikyo/docs/`, including the containing directory selected for deletion.
Record count and size, then stop processes whose working directory or arguments point into the selected paths.
Delete with literal `rm -rf -- /tmp/opencode/<name> …` paths; send sandbox directories to `trash -- <path>`.

## Close

Close without asking; `/done` or the automatic finish point already approves closure.
On a user's `/done` or request to finish and close, ask once with the `question` tool only when closing looks like a mistake, such as own work left uncommitted, an open lane or pending decision, or a retro finding the user has not addressed; on no, stop without tagging.
The automatic finish point requires those blockers to be absent; deferred retro candidates become `ISSUES` instead.
Load `clear-lanes` and close finished lanes under its judgment.
Choose the state and flags below, then call `tools.opencode.session_tag` through `execute`, omitting `sessionID`.

## Tags

Choose one state and any flags without asking; pass the names below in lowercase as `state` and `flags`.
Pass all applicable flags, including existing ones that still apply, because the tool replaces them except for `REFUSALS`.

Pick the state by outcome:

- `CLOSED`: the session's work landed.
- `ARCHIVED`: the session was shelved and nothing landed.
- `RESEARCH`: the session's result is findings rather than changes.

Add each flag that applies:

- `FIX`: the work repaired a bug or regression.
- `ISSUES`: the session hit friction the retro did not resolve.
- `REVIEWME`: landed work deserves a later look, such as a risky or unreviewed change.
- `TODO`: follow-up work remains.
- `REFUSALS`: the session's content made a model stop or refuse; this excludes its history from agents' `sessions` queries.

## Report

Report commit OIDs, pushes sent and commits left unpushed for approval, retro outcome and deferred candidates, worktrees removed and kept with unique paths or refusal reasons, scratch paths removed (count and size), paths kept as shared or unclear, dirty state left, and the tag written.
