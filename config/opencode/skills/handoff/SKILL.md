---
name: handoff
description: Use when the user invokes /handoff or asks to continue work in a fresh OpenCode session; produces a minimal, paste-ready continuation prompt instead of a compaction summary.
---

# Fresh session handoff

## Purpose

Create the smallest prompt that lets a fresh session continue useful work without broad reconnaissance.
This is a continuity boundary, not a transcript summary or durable project artifact.
Return the handoff in the response unless the user explicitly requests a file.

## Gather

Use current session knowledge first.
Run only narrow checks needed to make volatile claims accurate.

Collect:

1. The next objective and first useful action.
2. Each active repository or worktree, its branch, and whether it is clean.
3. Relevant committed state, including unpushed commits when they matter.
4. Uncommitted work that the next session must preserve or understand.
5. Decisions, clarifications, blockers, and open questions that are not encoded in source or context files.
6. The few exact files that establish governing context or directly support the next action.

Prefer durable truth in this order:

1. Git and current tree state.
2. Governing `AGENTS.md`, active specs, and other named context files.
3. Current user direction and settled session decisions.
4. Raw conversation detail only when nothing more durable records it.

Do not dispatch an exploration agent merely to prepare the handoff.
Do not repeat broad history or diff exploration already completed by the current session.

## Clarify

Ask questions only when a missing answer would materially change the handoff.
Typical reasons are an unclear next objective, ambiguous repository target, or uncertain treatment of dirty work.

Ask all necessary questions in one turn, with at most three short questions.
Do not ask for facts recoverable from the session, repository, or context files.
Treat arguments supplied to `/handoff` as authoritative guidance and avoid questions they already answer.

## Distill

Carry knowledge that would otherwise require rediscovery.
Point to knowledge already encoded in files instead of copying it.

Include:

- Exact repository or worktree paths and branches.
- Clean or dirty state, relevant commit IDs, and push state when useful.
- The next objective and a concrete starting action.
- Only load-bearing decisions, exclusions, blockers, and unresolved questions.
- A short ordered read list containing governing context first, then task-specific files.
- A narrow continuity check such as `git status` when stale state could cause harm.

Omit:

- Session chronology, completed-step narration, and tool transcripts.
- Architecture or implementation detail visible in the named files.
- Rules already present in `AGENTS.md` or another context file.
- Exhaustive changed-file lists, verification logs, and speculative future work.
- Generic collaboration preferences unless the user explicitly changed them for this work.
- Explanations intended to justify the handoff itself.

Default to 300 words or fewer.
Exceed that only for multiple active repositories, dangerous dirty state, or several decisions that cannot be recovered elsewhere.

## Output

Return only a paste-ready prompt under `# Fresh session handoff`.
Use these sections when they contain useful information and omit empty sections:

- `## Continue`
- `## State`
- `## Carry forward`
- `## Read first`

Make `Continue` state the objective and first action.
Make `State` compact, preferably one bullet per repository or worktree.
Make `Carry forward` contain only knowledge absent from the read list.
Make `Read first` an ordered list of exact paths and state why each matters in a short phrase.

End with one direct instruction to verify volatile state and continue.
Do not add commentary before or after the prompt.
