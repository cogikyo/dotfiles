# Commit

Smart git commits that handle messy states safely. Default to atomic commits — one logical change per commit. No user interaction needed unless grouping is genuinely ambiguous.

## Modes

- `/commit quick`: fast in-session path for small, obvious changes from the current conversation.
- `/commit`: full safety path for messy, stale, or ambiguous worktrees.

Use quick mode only when the user explicitly asks for it or the invocation includes `quick`.

## Quick Mode

Fast path for a few-line or few-file change that is clearly related to the current session or context the user just supplied.

### Use Quick Mode When

- The changed files are related to this session's work or user-provided context.
- The diff is small enough to understand from `git status` and `git diff`.
- The commit grouping is obvious and atomic.
- There is no need to detangle unrelated WIP.

### Do Not Use Quick Mode When

- There are multiple unrelated logical changes.
- The worktree contains substantial changes not made or discussed in this session.
- A file mixes unrelated concerns that require careful hunk selection.
- The requested commit needs a fresh agent to detangle lots of changes.
- Any staging decision is unclear.

If quick mode is unsafe or ambiguous, say why briefly and fall back to the full workflow.

### Quick Workflow

1. Inspect only the essentials:

```bash
git status --short
git diff
git diff --cached
```

2. Stage only files or hunks that belong to the current session's atomic change.
3. Commit directly with the message rules below.
4. Run `git status --short` after commit to confirm the result.

Quick mode intentionally skips the safety stash and sub-agent cycle. Do not stash, pop, restore, or launch sub-agents unless quick mode proves inappropriate.

If there are already staged changes, include them only when they are clearly part of the same current-session atomic commit. Otherwise stop and ask before changing the index.

## Core Principles

**Ignore git log style.** Do NOT mimic recent commit messages from `git log`. The history may contain lazy one-word commits (`fix`, `fixes`, `wip`) — never reproduce that style. Always follow the format rules below, regardless of what the log looks like.

**Default to splitting, not grouping.** When in doubt, make separate commits. Only group changes into one commit when they are genuinely the same logical change (e.g., three files edited to implement one feature). Different bug fixes, different config tweaks, different areas = separate commits. Do not ask the user how to group — the split is almost always obvious from file paths and change content.

**If the summary line needs "and", it's two commits.** A summary like "org owner visibility and approved-only downloads" is two separate concerns — split them. Each commit should have one verb, one scope, one purpose. If you can't describe it without a conjunction, split it.

## Full Workflow

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
verb(scope/context): short summary
```

**Scope**: Auto-detect from paths, always use 2 levels for feature-heavy areas

- `nvim/lsp` not just `lsp`
- `claude/skills` not just `skills`
- `creatives/video` not just `creatives`
- `creatives/permissions` not just `creatives`

**Top-level scope alone is almost never specific enough.** If a scope contains sub-features (e.g., `creatives` has video, permissions, download, upload, status), always drill down to `scope/sub-feature`. A commit scoped to just `creatives` should be rare — only when the change truly spans the entire feature equally (e.g., renaming the feature itself).

**Style rules:**

- Summary line: terse, no filler words, no verbose explanations
- **Bulleted body is MANDATORY** when a commit touches 2+ files or makes 2+ distinct changes:
  ```
  verb(scope): short summary

  - change one
  - change two
  - change three
  ```
- Each bullet: short phrase, not a full sentence, one line, no wrapping
- Body bullets must be contiguous: no blank lines between bullet points
- Single-file single-change commits: summary line only, no body needed

**Never do this:**

```
# BAD: long verbose summary, no bullets
fix(nvim): update the LSP configuration to handle the new diagnostic handler registration and also fix the null pointer issue that was causing crashes

# BAD: one-word lazy commit
fix

# BAD: cramming unrelated changes into one commit
edit(config): various tweaks

# BAD: scope too broad, "and" = two commits
feat(creatives): org owner collaborator visibility and approved-only downloads
# GOOD: split into two commits with sub-scopes
adjust(creatives/permissions): org owner collaborator visibility
adjust(creatives/download): restrict downloads to approved-only

# BAD: top-level scope when sub-feature is obvious
fix(creatives): prevent stale video preview during navigation
# GOOD: drill into the sub-feature
fix(creatives/video): prevent stale preview during navigation
```

### 5. Commit Types

| Type       | When                                              |
| ---------- | ------------------------------------------------- |
| `feat`     | Major new functionality, entirely new feature      |
| `add`      | New file, option, component, small addition        |
| `extend`   | Expand existing feature with new capability        |
| `improve`  | Better UX/DX, smoother flow, no new functionality  |
| `adjust`   | Tweak behavior, permissions, ordering, thresholds  |
| `edit`     | Modify content/values without changing behavior    |
| `fix`      | Bug fix                                            |
| `refactor` | Restructure code, same behavior (internal only)    |
| `style`    | Formatting, whitespace                             |
| `docs`     | Documentation                                      |
| `test`     | Tests                                              |
| `chore`    | Build, dependencies, config                        |
| `ci`       | CI/CD                                              |

**Verb choice matters.** Each verb should tell the reader *what kind of change* happened without reading the diff. Common mistakes:

- **Don't default to `refactor`** — it means "restructure internals, same external behavior." If you're adding a component, changing permissions, or improving a flow, that's not a refactor.
- **NEVER use `update`** — it's too generic. Pick the specific verb.
- **`improve` vs `refactor`**: `improve` changes the user/developer experience (better flow, smoother UX). `refactor` changes only internal structure with zero behavior change.
- **`adjust` vs `edit`**: `adjust` tweaks behavior/logic (permissions, ordering). `edit` modifies static content/values.
- **`add` vs `feat`**: `add` is a small addition (component, filter, option). `feat` is a significant new feature (entire modal, new workflow).

Examples of correct verb choice:

- Adding a new status component? → `add`
- Restricting what users can do? → `adjust`
- Adding a date filter field? → `add`
- Building a whole download modal with ZIP bundling? → `feat`
- Making a multi-file picker auto-switch to bulk mode? → `improve`
- Moving code between files, same behavior? → `refactor`

**Breaking change**: Add `!` → `edit(api)!: rename endpoints`

### 6. Commit

```bash
git commit -m "$(cat <<'EOF'
verb(scope): short summary

- bullet if multiple changes
- another change
EOF
)"
```

**Default: atomic commits** — one logical change per commit. Separate commits for separate concerns, even if they're small. Only group when changes are genuinely part of the same logical unit (not just "edited in the same session"). When grouped, bulleted body is mandatory.

### 7. Pre-commit Hook Failures

If a commit fails due to a pre-commit hook (lint, format, types):

1. Fix the errors on the staged files
2. Re-stage the fixed files
3. Retry the commit (new commit, NOT `--amend`)

### 8. Interactive Staging (when needed)

For files with mixed concerns, use `git add -p` to split hunks:

```bash
git add -p <file>  # hunk-by-hunk
git add <file>     # whole file
```

Only needed when a single file contains changes for different commits.

## Examples

### Atomic Commits (preferred)

```
# Status shows:
#   modified: nvim/lua/plugins/lsp.lua
#   modified: nvim/lua/plugins/telescope.lua
#   modified: zsh/.zshrc

# Separate commits for unrelated changes:

git add nvim/lua/plugins/lsp.lua
git commit -m "fix(nvim/lsp): correct handler registration"

git add nvim/lua/plugins/telescope.lua
git commit -m "extend(nvim/telescope): add file preview options"

git add zsh/.zshrc
git commit -m "add(zsh): fzf key bindings"
```

### Grouped Commit with Bullets (closely related changes)

```
# Several nvim plugin tweaks that belong together:

git add nvim/lua/plugins/lsp.lua nvim/lua/plugins/cmp.lua nvim/lua/plugins/treesitter.lua
git commit -m "$(cat <<'EOF'
edit(nvim): completion and diagnostic tweaks

- disable ghost text in cmp
- add null check on lsp handler
- pin treesitter parsers for lua and go
EOF
)"
```

### Single File, Multiple Concerns

```
# lsp.lua has both a bug fix AND a new feature — split into atomic commits
git add -p nvim/lua/plugins/lsp.lua
git commit -m "fix(nvim/lsp): null check on handler"

git add -p nvim/lua/plugins/lsp.lua
git commit -m "add(nvim/lsp): diagnostic virtual text toggle"
```

### App/Feature Commits (larger codebases)

For monorepos or feature-heavy projects, use `scope/context` to namespace:

```
add(creatives/status): CreativeStatus component and rename Legal to Compliance

- Introduce CreativeStatus component with context-aware display labels
- Users see "IN REVIEW" instead of internal ADMIN_REVIEW/LEGAL_REVIEW statuses
- Add label prop to base Status component for custom display text
- Rename "Legal Review" to "Compliance Review" in sections, reviewers, and filter options
```

```
adjust(creatives/permissions): restrict user view to read-only comments and show all admin actions

- Hide comment input for non-admin in detail and preview views
- Admin detail page always shows actions and edit regardless of reviewer assignment
- Reorder approve before reject in preview action footer
- Rename "Collaborators" label to "Users"
```

```
feat(creatives/download): BulkDownload modal with JSZip for user creative downloads

- Add BulkDownloadModal that resolves asset URLs concurrently and bundles into ZIP
- Add useCreativesDownloadList hook with period filter support for user schema
- Replace bulk upload button in filters toolbar with download button
- Export creativesListSchema and DownloadConfig for reuse
```

```
improve(creatives/upload): bulk upload flow with multi-file auto-switch

- Add multiple file selection support to FilePicker with onMultipleFiles callback
- Auto-switch from NewCreative to BulkUpload when multiple files are dropped
- Accept initialFiles prop on BulkUpload to pre-populate the queue
- Remove separate bulk upload button from admin ListPage
```

Note the verb choices: `add` for a new component, `adjust` for permission changes, `feat` for a whole new feature, `improve` for a smoother existing flow. Never `refactor` unless it's purely internal restructuring.
