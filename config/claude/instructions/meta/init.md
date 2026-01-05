---
description: Initialize a meta project - clarify scope, create master plan, spawn implementation plan agents.
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task, AskUserQuestion, TodoWrite, EnterPlanMode
---

# Meta Init

Create the master plan and implementation plans for a complex project.

## Arguments

```
/meta init           # Use ./meta/ in current directory
/meta init ./plans   # Use custom path
```

## Workflow

### Step 1: Check for Existing Plans

```bash
ls {path}/plan.md 2>/dev/null
```

If exists, ask user:
- **Overwrite**: Start fresh, delete existing plans
- **Resume**: Keep existing plans, just re-run review

### Step 2: Clarify Scope

Use `AskUserQuestion` to understand:
1. What is the project goal? (1-2 sentences)
2. What's explicitly in scope?
3. What's explicitly out of scope?
4. Any architectural constraints or preferences?
5. Success criteria - how will we know it's done?

### Step 3: Explore Codebase

Spawn up to 3 Explore agents (parallel) to understand context:

```
Task(subagent_type="Explore", prompt="""
CONTEXT: Planning [project name]

EXPLORE: [specific area]
- Look for existing patterns
- Find relevant files
- Identify dependencies

REPORT: Files found, patterns observed, concerns
""")
```

Example exploration goals:
- "Find existing implementations of similar features"
- "Understand the current architecture in {area}"
- "Identify all touch points for {concept}"

### Step 4: Create Master Plan

Write `{path}/plan.md` with this structure:

```markdown
# [Project Name]

## Goal
[1-2 sentence objective from Step 2]

## Scope
**In scope:**
- [item 1]
- [item 2]

**Out of scope:**
- [item 1]
- [item 2]

## Architecture Decisions
- [Decision 1]: [Rationale]
- [Decision 2]: [Rationale]

## Implementation Batches
| # | Name | Dependencies | Status |
|---|------|--------------|--------|
| 1 | [name] | none | planned |
| 2 | [name] | 1 | planned |
| 3 | [name] | 1, 2 | planned |

## Success Criteria
- [ ] [Criterion 1]
- [ ] [Criterion 2]
```

**Batch guidelines:**
- Each batch = work for one sub-agent
- Order by dependencies (independent batches first)
- Aim for 3-8 batches typically
- Too few = batches too large, too many = coordination overhead

### Step 5: Create Implementation Plans

For each batch, spawn a sub-agent to create its implementation plan:

```
Task(subagent_type="Plan", prompt="""
CONTEXT: Creating implementation plan for batch {N} of [project]

MASTER PLAN SUMMARY:
[paste relevant parts of master plan]

YOUR BATCH: {batch name}
DEPENDENCIES: {list of prior batches that must complete first}

EXPLORE the codebase to understand:
- Files that need modification
- Existing patterns to follow
- Potential conflicts

CREATE an implementation plan at: {path}/impl-{NNN}-{name}.md

Use this format:
---
# Batch {NNN}: {Name}

**Status:** planned
**Dependencies:** {none | impl-001, impl-002}

## Purpose
[What this batch accomplishes]

## Files
- `path/to/file.ts` - [create/modify/delete]: [what changes]

## Steps
1. [Specific action]
2. [Specific action]
...

## Verification
- [ ] [How to verify step 1]
- [ ] [How to verify step 2]
---

Be specific. Another agent will execute this plan with no prior context.
""")
```

**Parallelization:** Spawn agents for independent batches in parallel. Sequential batches must wait.

### Step 6: Auto-Review

After all impl plans are created, review for:
- **Gaps**: Are all parts of the goal covered?
- **Conflicts**: Do any plans contradict each other?
- **Dependencies**: Are all cross-batch dependencies declared?
- **Specificity**: Can another agent execute each plan without guessing?

Report findings to user:
```
## Init Complete

Created master plan and {N} implementation plans at {path}/

### Summary
- Batch 1: {name} - {file count} files
- Batch 2: {name} - {file count} files
...

### Review Findings
- [Issue 1]: [suggestion]
- [Issue 2]: [suggestion]

### Next Steps
- Run `/meta review` to refine plans further
- Run `/meta build` when ready to execute
```

## Edge Cases

**User provides vague goal:**
Ask follow-up questions. Don't guess at scope.

**Codebase too large to explore:**
Focus exploration on areas mentioned in user's goal. Ask user to narrow scope if needed.

**Too many batches needed:**
Consider if the project should be split into multiple `/meta` projects.

**Dependencies form a cycle:**
This is a plan error. Break the cycle by splitting a batch or reordering.
