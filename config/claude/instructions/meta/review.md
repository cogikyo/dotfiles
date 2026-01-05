---
description: Review and refine meta project plans.
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task, AskUserQuestion, TodoWrite
---

# Meta Review

Review and refine existing master plan and implementation plans.

## When to Use

- After `/meta init` for additional refinement passes
- When returning to a project with fresh context
- Before `/meta build` to validate plans are ready

## Workflow

### Step 1: Locate Plans

```bash
ls ./meta/plan.md 2>/dev/null || ls ./.meta/plan.md 2>/dev/null
```

If no plans found, inform user and suggest `/meta init`.

### Step 2: Read Master Plan

Read `{path}/plan.md` to understand:
- Project goal
- Scope boundaries
- Architecture decisions
- Batch structure and dependencies
- Success criteria

### Step 3: Read All Implementation Plans

```bash
ls {path}/impl-*.md
```

For each impl plan, read and note:
- Status (planned/in-progress/complete)
- Dependencies
- Files involved
- Step count and specificity

### Step 4: Check for Issues

**Gaps in Coverage:**
- Does every part of the goal have a batch?
- Are all success criteria addressed by at least one batch?
- Any files mentioned in master plan missing from impl plans?

**Conflicts Between Plans:**
- Do any batches modify the same files in incompatible ways?
- Are there contradicting approaches?

**Dependency Issues:**
- Are all cross-batch dependencies declared?
- Any cycles in the dependency graph?
- Any batch depending on something that doesn't exist?

**Alignment with Goal:**
- Do the batches, when combined, achieve the stated goal?
- Any scope creep in impl plans?
- Any out-of-scope work snuck in?

**Specificity:**
- Can an agent execute each plan without guessing?
- Are file paths concrete or vague?
- Are steps actionable or hand-wavy?

### Step 5: Report Findings

```markdown
## Review Complete

**Plans reviewed:** {path}/
- Master plan: {status}
- Implementation plans: {N} total ({complete} complete, {in_progress} in progress, {planned} planned)

### Issues Found

#### Critical (must fix before build)
- [ ] {Issue}: {suggestion}

#### Warnings (should fix)
- [ ] {Issue}: {suggestion}

#### Suggestions (optional improvements)
- {Suggestion}

### Ready for Build?
{Yes / No - fix critical issues first}
```

### Step 6: Update Plans (if approved)

If user approves suggested changes:
1. Edit the relevant plan files
2. Update status table in master plan if batch structure changed
3. Re-run Step 4-5 to verify fixes

## Common Review Patterns

**"This batch is too big":**
Split into multiple batches. Create new impl plan, update master plan table.

**"Missing dependency":**
Add to the `Dependencies:` line in impl plan. Update master plan table.

**"Vague steps":**
Spawn an Explore agent to gather specifics, then update the impl plan.

**"Conflicting changes":**
Either merge batches, reorder them, or add explicit coordination steps.

**"Scope creep":**
Check if addition is necessary. If not, remove. If yes, update master plan scope.
