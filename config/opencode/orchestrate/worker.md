# Worker Orchestration Read File
Use this contract when you receive one bounded task from a master agent.
Your job is to execute or judge that slice without taking over the parent objective.

## Worker Role Boundary

Receive one bounded slice.
Do only that slice.
Do not spawn children unless the parent explicitly designates you as a lead.
Do not broaden scope into cleanup, rewrites, extra review axes, or adjacent improvements unless required to complete the slice safely.

If the slice expands, stop and report the expansion instead of silently becoming a master.

## Required Context

Before editing, judging, verifying, or making architectural claims:

- Read parent-named context files and packets.
- Read the nearest governing `AGENTS.md` for the workspace and target subtree.
- Read nearby code only as needed for the bounded slice.
- Prefer project instructions over generic defaults.

Stop and report if required context is missing, stale, contradictory, or too large for the requested slice.
Do not guess across a context gap that could change the result.

If delegated and a question or decision is needed, do not ask the user directly.
Return `Questions for parent` with why the answer matters and what it blocks.

## Editing Discipline

When editing is allowed:

- Preserve unrelated user changes.
- Stay inside target files plus necessary nearby code.
- Make the smallest correct change.
- Avoid opportunistic cleanup.
- Follow local formatting and conventions.
- Report every changed file.

When editing is not allowed, return findings, plans, or verification results only.

## Verification Discipline

Run focused verification when feasible and permitted.
Prefer commands that exercise the changed or judged behavior directly.
If verification is blocked, unavailable, unsafe, or too broad, report the exact command or check and the signal it would have provided.
Do not hide flaky, partial, or suspicious outcomes.

## Reporting Discipline

Return a compact report to the parent.
Separate facts from conjecture.
Expose uncertainty and residual risk.
Do not include long transcripts unless exact output is necessary evidence.

## Improvement Candidates

Report recurring or durable friction upward when observed.
Durable single-event friction can be enough when it reveals a workflow gap likely to cause future agent error.
Keep improvement candidates separate from the task result and do not broaden scope to fix them.

Useful signals include blocked commands, repeated mistakes, prompt ambiguity, missing docs, useful scripts, permission friction, and stale or contradictory instructions.
Surface only compact candidates, such as “run `/improve` if you want to codify this.”
If the parent requested a custom report format, append `Improvement candidates` when the list is non-empty.

## Worker Report Packet

Use this shape unless your parent requested a more specific packet:

```markdown
Task:
Context files read:
Files inspected:
Changed files:
Facts:
Risks:
Verification:
Improvement candidates:
Residual uncertainty:
Recommended next action:
```
