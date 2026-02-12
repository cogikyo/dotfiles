---
description: Impact tracking system - summarize work at day/week/quarter/year boundaries. Rolls up hierarchically.
allowed-tools: Read, Glob, Grep, Edit, Write, Bash, Task, AskUserQuestion, TodoWrite
---

# End Of - Impact Tracking System

A hierarchical system for tracking and summarizing work impact over time.

## Usage

```
/endof day           # Analyze today's commits, add to weekly file
/endof week          # Summarize week's entries into Review section
/endof quarter       # Aggregate weekly Reviews into quarterly summary
/endof year          # Aggregate quarterly Reviews into yearly summary
```

Aliases: `d`, `w`, `q`, `y` work too (e.g., `/endof d`).

## How It Works

```
Daily commits  →  Weekly file  →  Quarterly file  →  Yearly file
    (day)           (week)          (quarter)          (year)
```

Each level summarizes the previous:
- **Day**: Captures git commits into daily sections of weekly file
- **Week**: Synthesizes daily entries into a Review section
- **Quarter**: Aggregates weekly Reviews into themes and accomplishments
- **Year**: Creates comprehensive annual summary from quarters

## File Structure

```
~/Documents/life/work/impact/
└── 2026/
    ├── 01/
    │   ├── 03.md    # Week ending Jan 3
    │   ├── 10.md    # Week ending Jan 10
    │   └── ...
    ├── 02/
    │   └── ...
    ├── Q1.md        # Q1 summary
    ├── Q2.md
    ├── Q3.md
    ├── Q4.md
    └── EOY.md       # End of year summary
```

Weekly files are named by the last workday of that week (usually Friday).

## Execution

Based on the argument provided, read the corresponding file in `~/.claude/instructions/endof/`:
- `day` or `d` → Read and execute `endof/day.md`
- `week` or `w` → Read and execute `endof/week.md`
- `quarter` or `q` → Read and execute `endof/quarter.md`
- `year` or `y` → Read and execute `endof/year.md`

If no argument provided, ask the user which time period they want to summarize.

## Principles

- Keep summaries concise but meaningful
- Focus on impact and outcomes, not just activities
- Group related work together
- Skip trivial housekeeping commits
- Preserve context for future reference
