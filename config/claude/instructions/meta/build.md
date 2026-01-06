---
description: Execute meta project plans via sub-agents.
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task, AskUserQuestion, TodoWrite
---

# Meta Build

Execute implementation plans via sub-agents. Runs continuously until all batches complete.

## When to Use

- After `/meta init` and optionally `/meta review`
- To resume a partially completed project
- Plans should be reviewed and ready before building

## Workflow

### Step 1: Locate and Read Plans

```bash
ls ./meta/plan.md 2>/dev/null || ls ./.meta/plan.md 2>/dev/null
```

Read master plan to understand:
- Batch order and dependencies
- Current status of each batch

### Step 1.5: Cache Lint Output (TypeScript)

For TypeScript projects, run lint once and save output to temp file:

```bash
if [ -f "package.json" ]; then
  yarn lint 2>&1 > /tmp/meta-lint-output.txt || true
  echo "Lint output cached to /tmp/meta-lint-output.txt"
fi
```

**IMPORTANT:** Tell sub-agents to READ `/tmp/meta-lint-output.txt` instead of re-running `yarn lint`.

### Step 2: Find Next Executable Batch

A batch is executable when:
1. Status = `planned`
2. All dependencies have status = `complete`

Scan the Implementation Batches table in master plan.

If no executable batch found:
- All batches complete → Report success
- Batches remain but blocked → Report dependency issue

### Step 3: Execute Batch

Update the impl plan status to `in-progress`:
```markdown
**Status:** in-progress
```

Update master plan table status to `in-progress`.

Spawn execution agent:

```
Task(subagent_type="general-purpose", prompt="""
EXECUTION TASK: Implement batch {NNN} - {name}

Read the implementation plan at: {path}/impl-{NNN}-{name}.md

This plan contains:
- Purpose of this batch
- Files to modify/create
- Step-by-step instructions
- Verification checklist

LINT: If you need lint output, READ /tmp/meta-lint-output.txt using the Read tool.
DO NOT run yarn lint - it's slow. Use the cached output instead.

EXECUTE the plan:
1. Follow each step in order
2. Make the specified changes to files
3. Run verification checks
4. Report any issues encountered

CONSTRAINTS:
- Stay within scope of the implementation plan
- Do not modify files not listed in the plan
- If blocked, report the blocker rather than improvising
- Use cached lint output from /tmp/meta-lint-output.txt, not fresh lint runs

WHEN DONE:
Report which steps completed, any issues, and verification results.
""")
```

### Step 4: Update Status on Completion

After agent reports completion:

1. Verify reported changes match plan
2. Update impl plan status to `complete`:
   ```markdown
   **Status:** complete
   ```
3. Update master plan table status to `complete`
4. Check off relevant success criteria if applicable

### Step 5: Loop

Return to Step 2 to find next executable batch.

Continue until:
- All batches complete → Report final summary
- Critical error encountered → Report and stop
- User interrupts → Save state (statuses already in files)

### Step 6: Final Report

When all batches complete:

```markdown
## Build Complete

**Project:** {name}
**Batches executed:** {N}

### Execution Summary
| Batch | Status | Files Changed |
|-------|--------|---------------|
| 001 - {name} | complete | {N} |
| 002 - {name} | complete | {N} |
...

### Verification Results
- [x] {Success criterion 1}
- [x] {Success criterion 2}
...

### Issues Encountered
- {Issue 1}: {resolution}
...

### Follow-up Suggestions
- {Suggestion}
```

## Parallel Execution

When multiple batches are executable simultaneously (no dependency between them):

```
Task(run_in_background=true, ...) # Batch A
Task(run_in_background=true, ...) # Batch B
# Then use TaskOutput to collect results
```

This speeds up execution but increases complexity. Use judgment:
- Parallel: Independent batches, different file sets
- Sequential: Overlapping files, uncertain interactions

## Error Handling

**Agent reports blocker:**
1. Pause execution
2. Report blocker to user
3. Options: fix blocker, skip batch, abort build

**Agent makes changes outside plan scope:**
1. Note the deviation
2. Continue if harmless, report to user
3. May need to update other impl plans if affected

**Verification fails:**
1. Report failure
2. Options: retry, manual fix, skip verification

**Agent times out or crashes:**
1. Check partial progress
2. Resume from last completed step if possible
3. Or restart batch

## Resuming Partial Builds

`/meta build` can be called multiple times:
- Reads status from files
- Picks up where it left off
- No duplicate work (completed batches stay complete)

If a batch is stuck `in-progress`:
- Check if agent crashed mid-execution
- Review partial changes
- Either complete manually or reset to `planned`
