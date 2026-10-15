---
name: done
description: Use when the user invokes /done or asks to finish and close the current session; Collab runs comments, commits, and a qualifying live retro, then closes finished lanes and marks the session closed.
---

# Done

Collab runs this procedure attended in the current session, in the order below.
Invoking `/done` approves the comment pass, commits of this session's own work, a qualifying live retro, and closing finished lanes and the session.
It does not approve committing other sessions' work, pushing, or deleting anything.

## Comments

When this session changed code, send `build/scribe` a comment-only audit of exactly those changed hunks.
Fix or delete stale comments; add one only where a reader would otherwise form a wrong belief.
Skip when no code changed or a scribe pass already ran after the last edit.

## Commit

Load `commit` and commit this session's own uncommitted work as atomic stories; `/done` approves those paths.
Ask one question with the `question` tool only when ownership of a path or hunk is unclear.
Leave other sessions' work dirty and skip when nothing is left.

## Retro

Judge in-session, without subagents, using the Qualify bar in `../../improve/retro/SKILL.md` over this session and its children.
Drop candidates this session already fixed.
When any candidate qualifies, load `retro` and run live mode now without asking first; its own confirmation exchange still applies.
Commit approved retro changes under the Commit rules above.
When nothing qualifies, say so in one line.

## Close

Close without asking; `/done` already says the user wants the session closed.
Ask once with the `question` tool only when closing looks like a mistake, such as own work left uncommitted, an open lane or pending decision, or a retro finding the user has not addressed; on no, stop without renaming.
Load `clear-lanes` and close finished lanes under its judgment.
Read `$OPENCODE_SESSION_ID` from the shell, then call `sessions` with `query: "sql"`, `sql: "SELECT title FROM session_v2 WHERE id = ?"`, and the ID bound in `params`.
Call `tools.opencode.session_rename({ title })` through `execute` with `[CLOSED] <current title>`, omitting `sessionID` for the current session.
Never add a second `[CLOSED] ` prefix.

`[CLOSED]` means this session's work is committed and its retro was judged or run.
Later `/improve` runs scan it but send it no retro leaf, and auto-retitle leaves its title alone.

## Report

Report commit OIDs, retro outcome, dirty state left, and whether the session closed.
