---
description: Scaffold new skills with proper structure and templates.
allowed-tools: Read, Write, Glob, AskUserQuestion, Bash
---

# New Skill - Instructions

Create new skills in a uniform way by scaffolding the required files.

## Workflow

### Step 1: Gather Information

Use `AskUserQuestion` to collect:

1. **Skill name** - lowercase, single word (e.g., `review`, `polish`, `endof`)
2. **Description** - one-line summary for skill listings
3. **Allowed tools** - select from: Read, Edit, Write, Bash, Glob, Grep, Task, WebFetch, WebSearch, AskUserQuestion, EnterPlanMode, TodoWrite
4. **Sub-instructions needed?** - whether to create a subdirectory for multiple instruction files

### Step 2: Create Files

Create the following files using `Write`:

#### Skill file: `~/.claude/skills/{name}.md`

Template:
```yaml
---
description: {description}
allowed-tools: {tools}
---

# {Name (Title Case)}

{Brief purpose description}

**Read full instructions:** `~/.claude/instructions/{name}.md`
```

#### Main instruction file: `~/.claude/instructions/{name}.md`

Template:
```yaml
---
description: {description}
allowed-tools: {tools}
---

# {Name (Title Case)} - Instructions

## Overview

{Detailed purpose and when to use}

## Workflow

1. ...
2. ...

## Output

{What to deliver when complete}
```

#### (Optional) Sub-instruction directory: `~/.claude/instructions/{name}/`

If sub-instructions requested, create the directory and a starter file.

### Step 3: Report

Output what was created:
- Skill file path
- Instruction file path(s)
- Next steps (edit instructions to add workflow details)

## Reference: Existing Pattern

Skills follow this structure:
- `skills/` - Entry point files (brief, references instructions)
- `instructions/` - Detailed workflow files
- `instructions/{name}/` - Sub-instructions for modes/variants

## Output

After scaffolding, remind user to:
1. Edit `~/.claude/instructions/{name}.md` to add detailed workflow
2. Add sub-instruction files if needed
3. Test with `/{name}`
