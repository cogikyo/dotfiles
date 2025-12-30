---
name: polish
description: Refactor and polish code for readability, idiomaticity, efficiency, and DRY principles. Use when cleaning up a feature or module after initial development is complete. Reorganizes file structure, separates concerns, ensures naming consistency, and improves code locality.
allowed-tools: Read, Grep, Glob, Edit, Write, Bash, Task, AskUserQuestion, EnterPlanMode, TodoWrite
---

# Polish - Code Refactoring Skill

A multi-phase, multi-agent refactoring workflow.

## Modes

### Quick Mode (`/polish quick` or `/polish -q`)

Lightweight pass for small scopes (single directory, few files). Skips multi-agent phases.

**Focus areas:**

- Variable and function naming
- Function organization within files
- Locality of behavior (related code together)
- Idiomatic patterns
- Interface consistency between files

**Process:**

1. Read all files in scope
2. Identify improvements (no exploration agents needed)
3. Present changes to user for approval
4. Execute edits directly
5. Brief summary

**When to use:** Small directories, quick cleanup, naming passes, minor reorganization.

**When NOT to use:** File moves, directory restructuring, cross-module changes → use full mode.

---

### Full Mode (default)

The complete 4-phase workflow below. Use for larger refactors, file restructuring, or cross-module work.

---

## Core Philosophy

Polish is run after a feature/module is mostly complete. The goal is to make code:

- **Readable** - Self-documenting, clear intent
- **Idiomatic** - Follows language conventions
- **Efficient** - No obvious performance issues
- **DRY** - No unnecessary duplication

**Do not change functionality.** Question from first principles if code seems stale or dead, but preserve behavior.

**Scope:** This skill covers **Go** and **TypeScript/React** codebases only.

**Git operations** are handled by the user, not this skill.

---

# WORKFLOW PHASES

## Phase 1: Initial Planning (Single Agent)

**CRITICAL: Always start in plan mode.** Use `EnterPlanMode` immediately.

Create an initial plan that includes for EACH proposed change:

```markdown
### Change: [Description]

**Files involved:**

- `path/to/file1.ts` - [what needs to be read/modified]
- `path/to/file2.go` - [what needs to be read/modified]

**Dependencies:** [other changes this depends on, or "none"] **Can parallelize:** [yes/no] **Exploration needed:** [specific questions to answer before executing]
```

The plan must be **self-contained** - a sub-agent with no prior context should be able to read the plan and gather everything needed from the file references.

### Initial Plan Structure

1. **Scope Definition** - What files/modules are being polished
2. **File Structure Changes** - Splits, moves, new directories
3. **Code Changes** - Refactors within files
4. **Shared Utility Updates** - Changes to common code
5. **Execution Order** - What depends on what
6. **Parallel Opportunities** - What can run simultaneously

**Get user confirmation before proceeding to Phase 2.**

---

## Phase 2: Exploration (Parallel Sub-Agents)

After initial plan approval, spawn exploration sub-agents to gather context that might be missing.

### Spawning Exploration Agents

Use the `Task` tool with `subagent_type: "Explore"` for each exploration goal. Run these **in parallel**.

Example exploration goals:

- "Find all usages of X function across the codebase"
- "Check shared utilities in pkg/, lib/, utils/ for existing implementations of Y"
- "Find similar patterns to Z that should be consolidated"
- "Identify all imports of the files being moved"
- "Check for dead code paths in module A"

### Exploration Agent Prompt Template

```
CONTEXT: Polishing [module/feature name]

EXPLORATION GOAL: [specific question to answer]

FILES TO START FROM:
- path/to/relevant/file1.ts
- path/to/relevant/file2.go

LOOK FOR:
- [specific patterns]
- [specific usages]
- [specific dependencies]

REPORT BACK:
- Findings relevant to the refactor
- Any concerns or blockers discovered
- Suggested additions to the plan
```

### Synthesize Findings

After all exploration agents complete:

1. Collect all findings
2. Identify missed dependencies or concerns
3. Create **Plan v2** incorporating discoveries - this captures edge cases and dependencies that the initial context and exploration may have missed
4. Present to user for feedback

**Get user confirmation on Plan v2 before proceeding to Phase 3.**

---

## Phase 3: Execution (Parallel Sub-Agents)

Execute the plan using sub-agents for parallelizable work.

### Execution Order Rules

1. **Directory creation** - Must happen first (can parallelize across independent paths)
2. **File moves/renames** - Must happen before content edits to moved files
3. **Content edits** - Can parallelize across independent files
4. **Import updates** - Must happen after moves, can parallelize
5. **Shared utility updates** - Should happen last (other files may depend on them)

**Test files** should live near their source files and move with them.

### Spawning Execution Agents

For each parallelizable unit of work, spawn a `Task` agent with `subagent_type: "general-purpose"`:

```
EXECUTION TASK: [specific task description]

BEFORE STARTING - Read conventions:
- .claude/skills/polish/go.md (if Go files)
- .claude/skills/polish/typescript.md (if TS/React files)

PLAN REFERENCE: [which part of the plan this executes]

FILES TO MODIFY:
- path/to/file1.ts - [exact changes to make]
- path/to/file2.go - [exact changes to make]

PREREQUISITES COMPLETED: [list what's already done]

CONSTRAINTS:
- Do not change functionality
- Follow naming conventions (see polish skill docs)
- Maintain happy path with guard clauses
- Errors as objects

WHEN DONE: Report files changed and any issues encountered
```

### Parallel Execution Strategy

```
1. mkdir operations (parallel)
   ↓
2. File moves (parallel where independent)
   ↓
3. Content edits - batch by independence:
   - Batch A: [files with no interdependencies] (parallel)
   - Batch B: [files depending on Batch A] (parallel after A)
   ↓
4. Import/reference updates (parallel)
   ↓
5. Shared utility updates (sequential - high impact)
```

### During Execution

- **LSP will freak out** - This is expected during file moves. Ignore transient errors.
- Track all changes made by each agent
- Note any deviations from plan
- Collect issues for review phase

---

## Phase 4: Review (Fresh Agent)

Spawn a **fresh review agent** (`subagent_type: "general-purpose"`) with no prior context to sweep all changes. Use `general-purpose` because the reviewer may need to make small fixes.

### Review Agent Prompt

```
REVIEW TASK: Polish refactor verification

BEFORE STARTING - Read conventions:
- .claude/skills/polish/go.md (if Go files)
- .claude/skills/polish/typescript.md (if TS/React files)

ORIGINAL SCOPE: [files/modules that were polished]

CHANGES MADE:
- [list of all files created]
- [list of all files modified]
- [list of all files moved/deleted]

PLAN SUMMARY: [brief description of intended changes]

VERIFY:
1. No new TypeScript/Go errors introduced
2. No broken imports or references
3. Functionality preserved (no behavioral changes)
4. Naming conventions followed
5. Code passes linters (gofmt, ESLint)

CHECK FOR:
- Missed references to moved files
- Stale imports
- Type errors
- Incomplete refactors

REPORT:
- Issues found (with file:line references)
- Suggested fixes
- Overall assessment: PASS / ISSUES FOUND / NEEDS NEW PLAN
```

### Handling Review Results

**If PASS:** Complete with detailed summary (see Output section)

**If ISSUES FOUND:**

- Report issues to user
- Propose fixes
- Execute fixes (may need mini execution phase)
- Re-run review

**If NEEDS NEW PLAN:**

- Use discretion - if fixing issues leads to rabbit holes that make things worse, step back
- Unexpected side effects discovered or fix complexity exceeds original scope
- Report what happened
- Return to Phase 2 (Exploration) with lessons learned - effectively starting fresh

---

# REFERENCE

## Naming Conventions

### Functions & Methods

- **Important functions**: Single word names (`save`, `load`, `parse`)
- **Helpers**: verb + noun pattern, max 2 words (`parseInput`, `validateUser`)
- **Long names = code smell**: If you need 3+ words, directory/package should provide context

### Acronyms

- Always capitalized unless at word start: `parseCSV`, `loadJSON`, `useCSV`
- At word start, follow language convention: `csvParser`, `jsonLoader`

### General

- Directory/package path provides context
- Consistency between frontend and backend naming

## Code Style

### Happy Path Pattern

```go
func process(input Input) (Result, error) {
    if input.ID == "" {
        return Result{}, ErrMissingID
    }
    if !input.Valid() {
        return Result{}, ErrInvalidInput
    }
    // Happy path continues clean
    return doProcess(input), nil
}
```

### Errors as Objects

Prefer structured error types over string errors.

### Locality of Behavior

Code used together should be organized together. Fire together, wire together.

## Directory Structure (Polish Phase)

During development, verbose structure is acceptable. Polish compresses to final form:

- **Minimum ~2 files per directory**: Single-file directories usually belong in parent
- **Maximum ~6 files per directory**: More suggests over-scoping - split by concern
- **One word names preferred**: `documents/`, `billing/` - compound words (sparingly) for specific nouns (`formFields/`)
- **Exceptions**: `models/`, `components/` roots, `hooks/` collections, or other collections. Generally, rule for leaf node directories.

These are soft guidelines - use judgment based on cohesion.

## Language-Specific Guidelines

- [typescript.md](typescript.md) - TypeScript/React patterns
- [go.md](go.md) - Go idioms and patterns

## Tooling

- **Go**: gofmt with modernize (format on save)
- **TypeScript**: ESLint (format on save)

## Output

After completing all phases, provide:

1. **Execution Summary**

   - Files created (with purpose)
   - Files modified (with change description)
   - Files moved/deleted
   - Directories created

2. **Patterns Applied**

   - What refactoring techniques were used
   - Why each was chosen

3. **Shared Utilities**

   - Any common code updated
   - New utilities created

4. **Review Results**

   - Issues found and fixed
   - Remaining concerns

5. **Follow-ups**
   - Suggestions for future improvements
   - Related areas that might benefit from polish

## Checklist

See [checklist.md](checklist.md) for full verification checklist.
