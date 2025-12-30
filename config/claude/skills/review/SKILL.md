---
name: review
description: Find bugs, anti-patterns, and issues in code. Reports findings first, then offers to fix. Focuses on real problems, not style nitpicks. Use on changed files, a directory, or specific files.
allowed-tools: Read, Grep, Glob, Edit, Task, AskUserQuestion, TodoWrite
---

# Review - Code Issue Detection

Find real problems in code without being pedantic or overly cautious.

## Philosophy

**Find issues that matter:**
- Bugs and logic errors
- Security vulnerabilities
- Clear anti-patterns with real consequences
- Modernization opportunities (deprecated APIs, better alternatives)
- Performance issues that actually impact users

**Ignore:**
- Style preferences (formatting, naming opinions)
- "Could be cleaner" without concrete benefit
- Hypothetical edge cases that won't happen
- Missing comments/docs on working code
- Defensive coding for impossible scenarios

**Never suggest:**
- Adding excessive null checks
- Wrapping everything in try/catch
- Creating abstractions for single-use code
- Feature flags for simple changes
- Backwards-compatibility shims when code can just change

## Usage

```
/review                    # Review recent changes (git diff)
/review path/to/file.ts    # Review specific file
/review path/to/dir        # Review directory
/review --staged           # Review staged changes only
/review graduate           # Promote patterns to CLAUDE.md
```

---

## Subcommand: graduate

Promotes proven patterns from this skill into CLAUDE.md for project-wide enforcement.

### When to Graduate

- Pattern has caught real bugs 3+ times
- Team agrees it's a project convention (not just preference)
- Pattern is specific enough to be actionable

### Process

1. Review `react.md` and `go.md` for candidates
2. Ask user which patterns to promote
3. Add concise summary to CLAUDE.md Code Style section
4. Keep detailed examples in skill files (CLAUDE.md stays high-level)

### Graduate Workflow

```
/review graduate
```

This will:
1. Show patterns currently in skill files
2. Highlight any that have been frequently flagged
3. Ask which to add to CLAUDE.md
4. Update CLAUDE.md with one-line summaries
5. Add "See `/review` for details" reference

### Format for CLAUDE.md

Keep graduated patterns brief:

```markdown
### TypeScript/React

- **Avoid useEffect** - derive state directly, use useMemo, or call in event handlers
- **[NEW] Check cleanup in effects** - subscriptions, timers, listeners need cleanup
```

The skill files keep the full examples and rationale.

---

## Workflow

### 1. Gather Context

Before flagging anything, understand the code:
- Read the files in scope
- Check how functions are used (callers matter)
- Understand the domain/feature purpose
- Look at related tests if they exist

### 2. Identify Issues

For each issue found, determine:
- **Severity**: bug | security | anti-pattern | modernize | perf
- **Confidence**: certain | likely | possible
- **Impact**: What breaks or degrades if unfixed

Only report issues where confidence is "certain" or "likely".
"Possible" issues should be mentioned briefly, not emphasized.

### 3. Report Findings

Present issues grouped by file:

```markdown
## path/to/file.ts

### [bug] Null reference on line 45
The `user` object can be undefined when `fetchUser` fails silently.
This will crash when accessing `user.name`.

**Fix:** Add null check or fix `fetchUser` to throw on failure.

### [anti-pattern] useEffect for derived state (line 23-27)
The filtered list can be computed directly from props.
useEffect here causes unnecessary re-renders.

**Fix:** Replace with direct computation or useMemo.
```

### 4. Offer Fixes

After presenting all findings:
- Ask user which issues to fix
- Apply fixes for approved issues
- Show diff summary of changes made

## Severity Guide

| Severity | Description | Examples |
|----------|-------------|----------|
| bug | Code doesn't work as intended | Null refs, wrong logic, race conditions |
| security | Exploitable vulnerability | XSS, injection, auth bypass, data exposure |
| anti-pattern | Works but causes real problems | useEffect abuse, N+1 queries, memory leaks |
| modernize | Deprecated or has better alternative | Old APIs, superseded patterns |
| perf | Measurable performance impact | Unnecessary re-renders, O(n^2) in hot path |

## Language-Specific Checks

- [react.md](react.md) - React/TypeScript patterns
- [go.md](go.md) - Go patterns

## What NOT to Flag

Resist the urge to flag:
- Working code that could be "more elegant"
- Missing error handling for internal code (trust your own functions)
- Type assertions when the type is actually known
- Short variable names in small scopes
- Inline logic that doesn't need extraction
- Missing JSDoc/godoc on clear functions

## Evolution

This skill improves over time. When you encounter recurring issues:
1. Note them in review output
2. Consider adding to language-specific files
3. Patterns that recur 3+ times deserve documentation

## Output Format

```markdown
# Review: [scope description]

## Summary
- X bugs found
- X anti-patterns found
- X modernization opportunities

## Issues

### path/to/file.ts

[issues for this file]

### path/to/other.go

[issues for this file]

## Recommendations

[Any cross-cutting observations]

---

Would you like me to fix any of these issues? (list numbers or "all")
```
