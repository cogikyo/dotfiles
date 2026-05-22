---
name: orchestrate
description: Shared orchestration contracts for Drive, Plan, Build, Review, and their subagents. Use when coordinating subagents, preserving context, writing handoff packets, or enforcing context-file discipline.
invocation: user
---

# orchestrate

Use this skill for multi-agent work where context management matters more than raw execution speed.

## Role Boundaries

- Masters manage objective state, sequencing, delegation, synthesis, and user sync points.
- Scouts discover required context and return compact packets.
- Critics attack plans, assumptions, risks, and proposed changes.
- Builders edit code in bounded slices.
- Verifiers run or design the smallest useful verification.
- Leaf agents do not spawn more agents unless their prompt explicitly says they are a lead.

## Context Discipline

Masters should preserve their own context window.
Read durable context files, instructions, and child summaries directly.
Delegate broad code search, implementation inspection, and file editing to subagents.

Before a worker edits or judges code, it must know which context files govern the target subtree.
If a context packet names required files, the worker must read them before acting.
If required context is missing, stale, or contradictory, the worker reports the gap instead of guessing.

## Context Packet

Scouts return this shape:

```markdown
Objective:
Likely workspace/repo:
Target files:
Required context files:
Context files read:
Relevant conventions:
Verification commands:
Known traps:
Open unknowns:
Recommended next agent:
```

## Master State Packet

Masters maintain this shape internally and summarize it when handing off:

```markdown
Objective:
Current state:
Decisions:
Active plan:
Delegated work:
Open risks:
Next action:
```

## Handoff Packet

Plans and long-running masters produce this shape for fresh starts:

```markdown
Recommended path:
Evidence:
Rejected alternatives:
Execution slices:
Context required:
Risks:
Verification:
Questions before build:
```

## Worker Report

Workers return this shape:

```markdown
Task:
Files inspected:
Context files read:
Changed files:
Facts:
Risks:
Verification:
Residual uncertainty:
Recommended next action:
```

## LeadPier Context Router

When work is in a LeadPier workspace, context files are symlinked from `pier/context/` into target repos.
Do not copy those docs.
Use nearby linked files as routing instructions.

Common LeadPier context files:

- `AGENTS.md` for workspace orientation.
- `GO.md` for Go work.
- `backend/AGENTS.md` for backend work.
- `backend/SERVICES.md` for backend service shape.
- `backend/GO.md`, `backend/DATABASE.md`, `backend/ROUTES.md`, `backend/LOGGING.md`, and `backend/DOCS.md` when relevant.
- `backend/services/<service>/AGENTS.md` for service-specific backend work.
- `frontend/AGENTS.md` for frontend work.
- `frontend/TS.md`, `frontend/FORMS.md`, `frontend/DATA.md`, `frontend/UI.md`, and `frontend/ARCHITECTURE.md` when relevant.
- App or package `AGENTS.md` files under `frontend/apps/*` and `frontend/packages/*` when touching that subtree.

Use `pier context verify` only when the task requires checking or repairing context links.

## User Sync Points

Masters should pause and sync with the user when:

- The objective is ambiguous enough to change the implementation path.
- The next action is destructive, security-sensitive, production-impacting, or hard to undo.
- Multiple viable paths have meaningfully different long-term costs.
- A delegated result contradicts the plan or another agent's evidence.
- The work would expand beyond the user's requested scope.
