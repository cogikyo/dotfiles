---
name: papercuts
description: Use when the user invokes /papercuts or asks to diagnose failed commands, permission denials, or agent misuse in OpenCode sessions. Collab owns it. Read-only unless the user asks to apply a fix.
---

# Papercuts

## Authority

Collab owns this procedure.
Do not dispatch a child unless the evidence window is too large to classify here.
Do not edit permissions, skills, or agents unless the user asks after the report.

## Scope

Default to today and the current workspace.
Treat `$ARGUMENTS` as the window: a session id, directory, agent, or time bound.

Start with the `sessions` tool and any log or `/drive-log` evidence supplied by the user.
Select sessions by directory and time, then inspect `tools` with `tool: "shell"` for failed calls.
Use `part` with the returned `<message ID>#<index>` address to read the command, error, and output.
Include `subagent` failures when the question concerns agent use or dispatch.
Compare later calls from the same command family to see whether a legal form succeeded.
Tool history does not prove every permission ask or reviewer decision; report missing permission evidence instead of inferring it from a failed command.

Ignore read-only git that already has an allowed twin unless the agent never switched to it.

## Classify

Cluster by command family, not by every line.

For each cluster, pick one verdict:

| Verdict     | Meaning                                                                                |
| ----------- | -------------------------------------------------------------------------------------- |
| Recovered   | Denied, then the allowed form ran and work continued. Keep the rule.                   |
| Agent miss  | A legal equivalent already exists. Hygiene, not a config bug.                          |
| Hole        | Denied form repeats; no recovery or legal equivalent exists. The allow list is narrow. |
| Proper deny | Destructive or out-of-contract. The agent moved on or should have. Keep deny.          |

`ask` is only a candidate when Collab, attended, sometimes needs the command and discard or history rewrite is not the default.

`git add` without `--` is Recovered when `git add --` follows.
Worktree `git checkout --` and `git restore --` are Proper deny unless the user later needed that discard.
`git -C` on an already-allowed read is an Agent miss when the agent never switches to `workdir=`.

## Report

Return a short diagnosis, not a log dump.

- Name the sessions and agents that produced the clusters.
- Give one example command per cluster, a count, and the verdict.
- For Agent miss, name the legal form.
- For Hole, name the missing allow pattern.
- Propose a fix only when a Hole is real. Do not apply it here.

Omit empty verdict sections.
If nothing failed in the window, say that and stop.

## Epistemology handoff

For each reported command family, include the relevant session IDs, time range, diagnosis, and whether the evidence suggests a skill or instruction-content issue.
Make that judgment from the command classification and do not widen into general transcript mining.
When content improvement is indicated, append a paste-ready scope in this form:

`/epistemology sessions <ids>; time <range>; focus on <command family>; papercuts diagnosis: <diagnosis>`

Epistemology owns any later transcript analysis and content proposal.
