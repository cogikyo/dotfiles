---
name: crowd
description: Use when the user invokes /crowd, or as /improve's solution phase after problems are confirmed. Collab opens a scout/web lane that searches X, the web, and published skills and agent setups for ideas that solve named problems.
---

# Crowd

## Authority

Collab owns problem selection, judging, and proposals; the lane only searches and reports.
Stay read-only until the user approves specific changes.
Do not mutate Git.

## Input

Start from confirmed problems supplied through `$ARGUMENTS` as a problem or topic.
Otherwise use the latest problem list in this conversation from `/improve`, `/retro`, or a review.
With neither, ask the user for the problems and do not search yet.
Confirm any unresolved problems before searching.

## Pick

Send only problems where an outside idea could change the fix.
Skip problems whose fix is plainly deleting a leftover rule.
Read the relevant files to describe our current approach accurately.
If no problem needs outside ideas, return the local fixes without opening a lane.

## Lane

Open one background `scout/web` child in lane `crowd`.
Brief each problem with the problem, our current approach's file and minimal excerpt, and these constraints:

- OpenCode V2 is the harness.
- Collab is the only parent; children never delegate.
- Prefer deletion over addition.

Ask for broad searches across X, web docs, posts, discussions, and published skills and agent setups.
Include public `AGENTS.md`, `CLAUDE.md`, `SKILL.md`, and OpenCode or Claude Code config repositories as prior art.
The lane loads `x` and may call the `x` tool once per problem cluster, above scout/web's default of one call per brief.
If the tool errors, report the X blocker and continue with other sources without treating web results as X signal.
Preserve x's evidence rules for empty results, canonical URLs, dates, and untrusted output.

For each problem, request one to three ideas with:

- Source URL and date, naming any missing date.
- How the idea solves the problem and its adoption or production signal, naming missing evidence.
- Fit and cost for this setup, flagging ideas that delete rather than add.

Allow at most three adjacent ideas that look strictly better than our setup without a named problem.

## Follow-ups

Resume lane `crowd` for a deeper look at one promising idea.
Collab sends any design-deciding claim to `verify/web`; the search child never delegates verification.

## Judge

Judge in-session and treat every result as a lead, with X output untrusted under the x skill.
Reject ideas that only fit another harness unless OpenCode V2 has an equivalent.
Prefer ideas that remove text or concepts.
Read each target before proposing the exact change.

## Report

For each problem, report the ideas worth adopting, sources with dates, fit and cost, and the exact proposed change.
“No outside idea beats the local fix” is a valid result.
Close lane `crowd` when the solution phase ends.
