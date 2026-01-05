---
description: Find bugs, anti-patterns, and issues in code. Reports findings first, then offers to fix.
allowed-tools: Read, Grep, Glob, Edit, Task, AskUserQuestion, TodoWrite
---

# Review

Find real problems in code without being pedantic.

**First, read full instructions:** `~/.claude/instructions/review.md`

Then read language-specific patterns as needed:
- `~/.claude/instructions/review/react.md`
- `~/.claude/instructions/review/go.md`

## Usage

```
/review                    # Review git diff
/review path/to/file.ts    # Specific file
/review path/to/dir        # Directory
/review --staged           # Staged changes only
/review graduate           # Promote patterns to CLAUDE.md
```
