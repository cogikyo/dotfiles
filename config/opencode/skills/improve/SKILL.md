---
name: improve
description: Use when the user invokes /improve or asks for an improvement sweep over recent OpenCode sessions. Collab owns it.
---

# Improve

## Authority

Collab owns triage, synthesis, user questions, and any approved changes.
Run papercuts and epistemology triage in-session; do not dispatch either procedure.
Find uses bounded past-mode retros in `scout/session` leaves; Solve uses the crowd lane.

## Find

### Scope

Treat `$ARGUMENTS` as extra instructions that override window, project, focus, or session count defaults, or request “find only” to skip Solve and Apply.
Accept ordinary time bounds such as “last week” or “since monday”; resolve them to explicit dates before querying.
Default to the last 3 days by `time_updated`.
Resolve the default project with `git rev-parse --show-toplevel` (read-only), or use the current directory outside a repository.
Select sessions whose `session_v2.directory` equals that path or is under it.
When the project is the dotfiles checkout (`$DOTFILES`, or `~/dotfiles` when unset), default to all projects because dotfiles owns the global config.
Select top-level sessions only (`parent_id IS NULL`) and exclude the current session using `$OPENCODE_SESSION_ID`.

### Store

Use `list` for exact-directory selection; prefix selection needs `sql` because `list.directory` is exact.
For prefix selection, match the project path itself or paths beginning with that path plus `/`.
Report the sampled top-level count and oldest-to-newest `time_updated` span once, disclosing any truncation without claiming full coverage.

### Triage

Load `papercuts` and `epistemology` and use their relevant methods with this skill's scope defaults.
Classify failed shell calls and subagent errors over the window with papercuts, carrying its diagnoses forward.
Scan user-role text for corrections and preferences with epistemology's candidate-reading method.
Do not write a full epistemology report; triage selects sessions and preserves their evidence and diagnosis.
Pick at most 5 sessions with the densest useful signal unless the user's instructions change the session count.
When evidence is spread thin across many sessions, such as one small correction in each, send it straight to synthesis.

### Fan out

Sessions whose title starts with `[CLOSED] ` stay in triage but never get a retro leaf because `/done` already judged or ran their retro.
Run one background `scout/session` leaf per selected session, in parallel.
Tell each leaf to load `retro` in past mode for its session and children, supplying the ID, focus, triage evidence, and papercuts diagnosis.
Require read-only findings marked unconfirmed, each with a question the user would need to answer.
Synthesize after the leaves return; report failed or incomplete leaves as evidence gaps.

### Synthesize

Deduplicate the triage and leaf findings, preserving distinct evidence and open questions.
Promote findings that recur across independent sessions using epistemology's evidence bar.
Batch the leaves' open questions to the user; discard single-session findings unless the user confirms the reusable lesson.
State problems and their evidence without proposing fixes.
End Find with a confirmed problem list.

## Solve

Load `crowd` for the confirmed problems, then combine local fixes with any crowd ideas into ranked proposals.
Use epistemology's smallest-owner list, plus `opencode.json` permissions and plugins or tools.
Prefer editing or deleting over adding, and rank the strongest evidence first.
Keep papercuts' permission protections and omit secrets from evidence.

## Apply

Apply only proposals the user specifically approves.
Global-config edits land in the dotfiles checkout; say when that is a separate repository from the current project before approval.
Keep edits inside approved targets and inspect the resulting changes.

## Report

Return confirmed problems and their evidence for find-only runs, or ranked proposals otherwise.
Report as epistemology does, plus:

- Scope and any coverage gaps.
- Any crowd source URL and date used.

Distinguish proposals from approved changes actually applied.
