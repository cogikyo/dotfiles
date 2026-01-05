---
description: Orchestrate large projects through hierarchical planning with init, review, and build phases.
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task, AskUserQuestion, TodoWrite, EnterPlanMode
---

# Meta - Project Orchestration Skill

A hierarchical system for managing complex, multi-part projects across fresh context boundaries.

## Usage

```
/meta init [path]    # Create master plan + implementation plans
/meta review         # Review and refine existing plans
/meta build          # Execute plans via sub-agents
```

## How It Works

```
init                    review                  build
  │                        │                      │
  ├─ Clarify scope         ├─ Read master plan    ├─ Read master plan
  ├─ Explore codebase      ├─ Read impl plans     ├─ Find next batch
  ├─ Create master plan    ├─ Check gaps/conflicts├─ Spawn sub-agent
  ├─ Spawn impl agents     ├─ Report findings     ├─ Update status
  └─ Auto-review           └─ Update if approved  └─ Repeat until done
```

Each mode runs from fresh context. The master plan is the handoff mechanism.

## File Structure

Default location: `./meta/` in current working directory (overridable during init)

```
./meta/
├── plan.md              # Master plan (source of truth)
├── impl-001-{name}.md   # Implementation plan (one per sub-agent)
├── impl-002-{name}.md
└── ...
```

## Mode Execution

Based on the argument provided, read the corresponding file:
- `init` → Read and execute `~/.claude/instructions/meta/init.md`
- `review` → Read and execute `~/.claude/instructions/meta/review.md`
- `build` → Read and execute `~/.claude/instructions/meta/build.md`

If no argument provided, ask the user which mode they want.

## Principles

- **Self-contained plans**: Each plan has all context needed for execution
- **One impl plan = one sub-agent**: Clear ownership, predictable execution
- **Status tracking in files**: Plans are self-documenting, survives context resets
- **Dependency ordering**: Build phase respects declared dependencies
