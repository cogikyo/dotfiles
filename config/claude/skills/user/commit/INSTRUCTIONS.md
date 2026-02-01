# Commit

Smart git commits that handle messy states safely. Parallelizes when changes span multiple unrelated areas.

## Workflow

### 1. Safety Stash

Before any staging, stash everything:

```bash
git stash push -u -m "WIP: $(date +%Y%m%d-%H%M%S)"
git stash pop
```

This creates a recovery point. If staging goes wrong, `git stash list` shows the backup.

### 2. Analyze Changes

```bash
git status
git diff --stat
git diff          # unstaged
git diff --cached # staged
```

Group changes by logical scope. Look for:

- Files in same directory → likely same feature
- Related functionality across directories → group together
- Unrelated cleanup/fixes → separate commits

### 3. Decide: Direct or Parallel

**Handle directly when:**

- 1-2 files total
- All changes are related (same feature/fix)
- Simple cleanup or single commit

**Spawn sub-agents when:**

- 3+ unrelated change groups
- Changes span multiple distinct areas (e.g., nvim/, claude/, zsh/)
- Each group could be its own commit

If parallel: proceed to step 4. If direct: skip to step 5.

### 4. Parallel Commit (Sub-agents)

Launch Task agents with `subagent_type: "Bash"` for each group. Each agent:

- Gets assigned specific files/directories
- Follows steps 5-8 independently
- Commits only its assigned scope

**Agent assignment rules:**

- One agent per logical group (not per file)
- If files are interdependent, same agent handles all
- Agents work sequentially on staging (no race conditions)

**Prompt template for sub-agents:**

```
Commit changes for: [list of files/paths]

Follow the commit skill workflow:
1. Stage only these files using `git add -p` or `git add <file>`
2. Use commit format: verb(scope): description
3. Scope from paths (max 2 levels): nvim/lsp, claude/skills, etc.
4. Commit types: feat|add|extend|edit|fix|refactor|style|docs|test|chore|ci
5. Only commit staged changes, leave other files untouched

Files to commit:
[explicit file list]

Do NOT touch files outside this list.
```

Run agents **sequentially** (not parallel) to avoid staging conflicts.

### 5. Interactive Staging

For each logical group, use `git add -p` or stage specific files:

```bash
git add -p <file>  # hunk-by-hunk
git add <file>     # whole file
```

When reviewing hunks:

- `y` = stage this hunk
- `n` = skip this hunk
- `s` = split into smaller hunks
- `e` = edit hunk manually

Guide user through each decision. Explain what each hunk does.

### 6. Commit Message Format

```
verb(scope/context): description
```

**Scope**: Auto-detect from paths, max 2 levels

- `nvim/lsp` not just `lsp`
- `claude/skills` not just `skills`

### 7. Commit Types

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
| `chore`    | Build, deps, config              |
| `ci`       | CI/CD                            |

**NEVER use `update`** - it's too generic. Pick the specific verb:

- Adding a word to spell dictionary? → `add`
- Changing config values? → `edit`
- Adding new config options? → `extend`

**Breaking change**: Add `!` → `edit(api)!: rename endpoints`

### 8. Commit

```bash
git commit -m "$(cat <<'EOF'
verb(scope): description

Optional body explaining why, not what.
EOF
)"
```

Body only when non-obvious. Skip for trivial changes.

### 9. Repeat

If more changes remain (and not using parallel approach), repeat steps 5-8 for next logical group.

## Examples

### Mixed changes across features

```
# Status shows:
#   modified: nvim/lua/plugins/lsp.lua
#   modified: nvim/lua/plugins/telescope.lua
#   modified: zsh/.zshrc

# Group 1: LSP changes
git add -p nvim/lua/plugins/lsp.lua
git commit -m "fix(nvim/lsp): correct handler registration"

# Group 2: Telescope
git add nvim/lua/plugins/telescope.lua
git commit -m "extend(nvim/telescope): add file preview options"

# Group 3: Shell
git add zsh/.zshrc
git commit -m "add(zsh): fzf key bindings"
```

### Single file, multiple concerns

```
# lsp.lua has both a bug fix AND a new feature
git add -p nvim/lua/plugins/lsp.lua
# Stage only the fix hunks
git commit -m "fix(nvim/lsp): null check on handler"

git add -p nvim/lua/plugins/lsp.lua
# Stage the feature hunks
git commit -m "add(nvim/lsp): diagnostic virtual text toggle"
```
