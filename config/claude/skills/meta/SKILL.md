---
name: meta
description: Orchestrate large projects through hierarchical planning with init, review, and build phases.
allowed-tools: Read, Write, Edit, Glob, Grep, Bash, Task, AskUserQuestion, TodoWrite, EnterPlanMode
---

# Meta

Multi-phase project orchestration for complex, multi-part implementations.

**First, read full instructions:** `~/.claude/instructions/meta.md`

Then read mode-specific instructions:
- `~/.claude/instructions/meta/init.md`
- `~/.claude/instructions/meta/review.md`
- `~/.claude/instructions/meta/build.md`

## Modes

- `/meta init [path]` - Clarify scope, create master plan + implementation plans
- `/meta review` - Review and refine plans
- `/meta build` - Execute plans via sub-agents
