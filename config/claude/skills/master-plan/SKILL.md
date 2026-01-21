---
name: master-plan
description: Edit master plan files. Never implement - only refine the plan document.
allowed-tools: Read, Edit, Glob, Grep, AskUserQuestion, Task, Write
---

# Master Plan

Edit and refine master plan files without implementing them.

**First, read full instructions:** `~/.claude/instructions/master-plan.md`

## Usage

```
/master-plan                    # Auto-detect PLANS/ dir in cwd
/master-plan path/to/PLANS      # Specific PLANS directory
/master-plan path/to/MASTER.md  # Specific master plan file
/master-plan questions          # Research & resolve open questions
/master-plan finish             # Transition to implementation mode (discovers CLAUDE.md files)
```

## Finish Command

`/master-plan finish` prepares the plan for implementation:
- Discovers and links all CLAUDE.md files in the project
- Creates IMPLEMENTATION.yaml for state tracking
- Adds instructions that enforce full-stack implementation
- Prevents "not in scope" excuses - agents must implement ALL described work

## PLANS/ Structure

```
PLANS/
├── MASTER.md      # Read first - overview, links
├── LOG.md         # Read second - append-only history
├── QUESTIONS.md   # Open questions needing answers
├── DISCOVERIES.md # Research, context, blockers
└── {SCOPE}.md     # Detailed scope plans
```
