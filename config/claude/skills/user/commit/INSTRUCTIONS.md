# Commit

Smart git commits that handle messy states safely. Always groups by logical area and creates atomic commits — no user interaction needed unless grouping is genuinely ambiguous.

## Core Principle

**Do not ask the user how to group commits.** Analyze the diffs, determine logical groups by directory/feature, and proceed. The grouping is almost always obvious from file paths and change content. Only ask if changes are truly entangled in a way that could reasonably go multiple ways.

## Workflow

### 1. Safety Stash

Before any staging, stash everything:

```bash
git stash push -u -m "WIP: $(date +%Y%m%d-%H%M%S)"
git stash pop
```

This creates a recovery point. If staging goes wrong, `git stash list` shows the backup.

### 2. Analyze and Group

```bash
git status
git diff --stat
git diff          # unstaged
git diff --cached # staged
```

Determine logical groups by scope. Look for:

- Files in same directory → likely same group
- Related functionality across directories → group together
- Unrelated cleanup/fixes → separate commits

### 3. Execute via Sub-Agents

Launch one Task agent (`subagent_type: "Bash"`) per logical group, run **sequentially** to avoid staging conflicts.

**Handle directly (no sub-agent) only when:**

- 1-2 files total, single commit needed

**Use sub-agents when:**

- 2+ distinct groups exist
- Changes span multiple areas (e.g., nvim/, daemons/, xplr/)

**Agent assignment rules:**

- One agent per logical group (not per file)
- If files are interdependent, same agent handles all
- Each agent stages only its files and commits

**Prompt template for sub-agents:**

```
Commit changes for: [brief description]

Changes:
- [file]: [what changed]
- [file]: [what changed]

Stage and commit only these files:
```bash
git add [files] && git commit -m "$(cat <<'EOF'
verb(scope): description
EOF
)"
```

Do NOT touch files outside this list.
```

### 4. Commit Message Format

```
verb(scope/context): description
```

**Scope**: Auto-detect from paths, max 2 levels

- `nvim/lsp` not just `lsp`
- `claude/skills` not just `skills`

### 5. Commit Types

| Type       | When                             |
| ---------- | -------------------------------- |
| `feat`     | Major new functionality          |
| `add`      | New file, option, small addition |
| `extend`   | Expand existing feature          |
| `edit`     | Modify without breaking          |
| `fix`      | Bug fix                          |
| `refactor` | Restructure, same behavior       |
| `style`    | Formatting, whitespace           |
| `docs`     | Documentation                    |
| `test`     | Tests                            |
| `chore`    | Build, dependencies, config      |
| `ci`       | CI/CD                            |

**NEVER use `update`** - it's too generic. Pick the specific verb:

- Adding a word to spell dictionary? → `add`
- Changing config values? → `edit`
- Adding new config options? → `extend`

**Breaking change**: Add `!` → `edit(api)!: rename endpoints`

### 6. Commit

```bash
git commit -m "$(cat <<'EOF'
verb(scope): description

Optional body explaining why, not what.
EOF
)"
```

Body only when non-obvious. Skip for trivial changes.

### 7. Interactive Staging (when needed)

For files with mixed concerns, use `git add -p` to split hunks:

```bash
git add -p <file>  # hunk-by-hunk
git add <file>     # whole file
```

Only needed when a single file contains changes for different commits.

## Examples

### Mixed Changes Across Features

```
# Status shows:
#   modified: nvim/lua/plugins/lsp.lua
#   modified: nvim/lua/plugins/telescope.lua
#   modified: zsh/.zshrc

# No asking — just group and commit:

# Agent 1: LSP changes
git add nvim/lua/plugins/lsp.lua
git commit -m "fix(nvim/lsp): correct handler registration"

# Agent 2: Telescope
git add nvim/lua/plugins/telescope.lua
git commit -m "extend(nvim/telescope): add file preview options"

# Agent 3: Shell
git add zsh/.zshrc
git commit -m "add(zsh): fzf key bindings"
```

### Single File, Multiple Concerns

```
# lsp.lua has both a bug fix AND a new feature
git add -p nvim/lua/plugins/lsp.lua
# Stage only the fix hunks
git commit -m "fix(nvim/lsp): null check on handler"

git add -p nvim/lua/plugins/lsp.lua
# Stage the feature hunks
git commit -m "add(nvim/lsp): diagnostic virtual text toggle"
```
