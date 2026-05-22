---
name: commit
kind: command
description: Smart git commits. Use when user says /commit or asks to commit changes. Supports /commit quick for small in-session commits; handles messy states with stash-first safety, atomic staging, and conventional commit messages.
invocation: user
---

# commit

Create safe, atomic git commits without copying lazy history style.
Default to one logical change per commit.
Ask only when the staging or grouping decision is genuinely ambiguous.

Use `$ARGUMENTS` to detect requested mode and scope.
`quick` in `$ARGUMENTS` means quick mode only if the safety rules still hold.
If `$ARGUMENTS` names files, paths, or a scope, commit only that requested slice unless the user clearly asked for all changes.
If no extra arguments are provided, usually commit all obvious changes, split into atomic commits when needed.

## Mode selection

- Use `/commit quick` only when the user explicitly asks for quick mode or `$ARGUMENTS` includes `quick`.
- Use `/commit` for the full safety workflow when the worktree is messy, stale, broad, or ambiguous.
- If quick mode becomes unsafe while inspecting the diff, say why briefly and switch to the full workflow.

## Quick mode

Quick mode is for small obvious commits that belong to the current session, the context the user supplied, or one coherent domain.

Use quick mode when all of these are true:

- The changed files are related to this session, the user's supplied context, or one coherent domain.
- The diff is small enough to understand from `git status --short`, `git diff`, and `git diff --cached`.
- The commit grouping is obvious and atomic.
- There is no need to detangle unrelated WIP.

Do not use quick mode when any of these are true:

- There are multiple unrelated logical changes.
- The worktree contains substantial changes not made or discussed in this session.
- A file mixes unrelated concerns that require careful hunk selection.
- The requested commit needs a fresh agent to detangle lots of changes.
- Any staging decision is unclear.

Quick workflow:

1. Inspect only the essentials.

```bash
git status --short
git diff
git diff --cached
```

2. Stage only files or hunks that belong to the current-session atomic change or one coherent domain.
3. Commit directly with the message rules below.
4. Run `git status --short` after commit to confirm the result.

Quick mode intentionally skips the safety stash and sub-agent cycle.
Do not stash, pop, restore, or launch sub-agents unless quick mode proves inappropriate.

If changes are already staged, include them only when they are clearly part of the same current-session or single-domain atomic commit.
Otherwise stop and ask before changing the index.

## Full workflow

Use the full workflow for messy states, broad changes, stale changes, multiple groups, or anything that needs careful staging.

### Create a recovery point

Before any staging, stash everything and immediately pop it back.
This creates a recovery point without changing the final worktree.

```bash
git stash push -u -m "WIP: $(date +%Y%m%d-%H%M%S)"
git stash pop
```

If staging goes wrong, `git stash list` shows the backup.

### Analyze the diff

Inspect the worktree and separate the changes into logical groups.

```bash
git status
git diff --stat
git diff
git diff --cached
git log --oneline -10
```

Grouping rules:

- Files in the same directory are often one group.
- Related functionality across directories can be one group.
- Unrelated fixes, config tweaks, docs, or cleanup should be separate commits.
- If the summary line needs `and`, it is probably two commits.

### Choose direct commit or agents

Handle the commit directly when the change is small and single-purpose.

- Direct commit is preferred for 1-2 files and one logical commit.
- Use sequential sub-agents when there are 2+ distinct groups or changes span multiple areas.
- Use one agent per logical group, not one agent per file.
- If files are interdependent, one agent handles all of those files.
- Each agent stages only its assigned files and commits.

Sub-agent prompt template:

```text
Commit changes for: [brief description]

Changes:
- [file]: [what changed]
- [file]: [what changed]

Stage and commit only these files:
- [file]
- [file]

Use this message shape:
verb(scope): description

- bullet if needed
- another bullet if needed

Keep commit-body and commit-description bullets contiguous.
Never put blank lines between bullets.

Do NOT touch files outside this list.
```

## Atomicity rules

Ignore recent git log style.
History may contain lazy messages like `fix`, `fixes`, or `wip`.
Never reproduce that style.

Default to splitting, not grouping.
Only group changes when they are genuinely the same logical change.
Different bug fixes, different config tweaks, and different areas should become separate commits.

Avoid asking the user how to group changes when the split is obvious from file paths and diff content.
Ask only when a file or staged state mixes concerns in a way that could lose intent.

## Commit message

Use this summary format:

```text
verb(scope/context): short summary
```

Scope rules:

- Auto-detect scope from paths and affected feature.
- Use two-level scopes for feature-heavy areas.
- Prefer `nvim/lsp` over `lsp`.
- Prefer `opencode/agents` or `opencode/commands` over `opencode`.
- Prefer `creatives/video` or `creatives/permissions` over `creatives`.
- Use a top-level scope only when the change truly spans the whole area equally.

Body rules:

- Use summary-only commits for single-file, single-change commits.
- Add a bulleted body when a commit touches 2+ files or makes 2+ distinct changes.
- Keep each bullet short, one line, and phrase-like.
- Keep body bullets contiguous.
- Do not insert blank lines between body bullets.
- The only blank line in a commit with a body is between the summary and the first bullet.
- Apply the same contiguous-bullet rule to generated commit-description previews, reword forms, and approval prompts.
- A commit description field must be `- bullet\n- bullet\n- bullet`, not `- bullet\n\n- bullet`.
- If editing a generated description, remove spacer lines between bullets before presenting it.

Commit description preview format:

```text
- change one
- change two
- change three
```

Correct multi-change message:

```text
verb(scope): short summary

- change one
- change two
- change three
```

## Verb choice

Each verb should tell the reader what kind of change happened without reading the diff.
Never use `update`; it is too generic.

| Type       | Use when                                            |
| ---------- | --------------------------------------------------- |
| `feat`     | Major new functionality or an entirely new feature  |
| `add`      | New file, option, component, or small addition      |
| `extend`   | Existing feature gains a new capability             |
| `improve`  | General quality improvement not covered above       |
| `adjust`   | Small behavior, permission, ordering, or threshold  |
| `edit`     | Static content or values change                     |
| `fix`      | Bug fix                                             |
| `ui`       | Visual presentation, layout, styling, or components |
| `ux`       | User flow, interaction, copy, affordance, or feel   |
| `dx`       | Developer workflow, tooling, ergonomics, or clarity |
| `refactor` | Internal code restructure with same behavior        |
| `reorg`    | File, directory, module, or ownership reshuffle     |
| `style`    | Formatting or whitespace                            |
| `docs`     | Documentation                                       |
| `test`     | Tests                                               |
| `chore`    | Build, dependencies, or config                      |
| `ci`       | CI/CD                                               |

Verb distinctions:

- Do not default to `improve` or `adjust`; choose a more specific verb when one fits.
- Use `ui` for focused visual or component presentation work.
- Use `ux` for focused interaction, flow, wording, affordance, or user-facing feel.
- Use `dx` for focused developer workflow, tooling, naming clarity, or maintainer ergonomics.
- Use `improve` only when the change is a broad quality improvement that is not clearly UI, UX, DX, behavior, or bug fix.
- Use `adjust` for small behavior or logic tweaks, especially permissions, ordering, thresholds, defaults, or policy.
- Use `edit` for static content or value changes.
- Use `add` for small additions.
- Use `feat` for significant new workflows or features.
- Use `refactor` for code structure changes that preserve behavior.
- Use `reorg` for moving files, directories, modules, commands, docs, or ownership boundaries.
- Add `!` for breaking changes, e.g. `edit(api)!: rename endpoints`.

Examples:

- New status component: `add`.
- Restricting user permissions: `adjust`.
- New date filter field: `add`.
- Restyling a status pill: `ui`.
- Rewording empty-state copy: `ux`.
- Clarifying command help text for maintainers: `dx`.
- Whole download modal with ZIP bundling: `feat`.
- Multi-file picker auto-switches to bulk mode: `ux`.
- Moving code between files with same behavior: `refactor`.
- Moving command packages into a new workspace layout: `reorg`.

## Bad messages

Do not use verbose summaries, lazy one-word summaries, broad scopes, or grouped concerns.

Bad:

```text
fix(nvim): update the LSP configuration to handle the new diagnostic handler registration and also fix the null pointer issue that was causing crashes
```

Bad:

```text
fix
```

Bad:

```text
edit(config): various tweaks
```

Bad because `and` joins two concerns:

```text
feat(creatives): org owner collaborator visibility and approved-only downloads
```

Good split:

```text
adjust(creatives/permissions): org owner collaborator visibility
adjust(creatives/download): restrict downloads to approved-only
```

Bad because the scope is too broad:

```text
fix(creatives): prevent stale video preview during navigation
```

Good:

```text
fix(creatives/video): prevent stale preview
```

## Commit commands

Summary-only commit:

```bash
git commit -m "fix(nvim/lsp): correct handler registration"
```

Commit with body:

Use a single message string for the entire body.
Do not pass one `-m` per bullet, because Git inserts a blank paragraph between every `-m` flag.

```bash
git commit -m "$(cat <<'EOF'
verb(scope): short summary

- bullet if multiple changes
- another change
EOF
)"
```

## Mixed files

Use non-interactive file staging when whole files belong to one commit.
Use patch staging only when a single file contains separate concerns that must become separate commits.

```bash
git add -p <file>
git add <file>
```

## Hook failures

If a commit fails due to a pre-commit hook, do not amend.

1. Fix the errors on the staged files.
2. Re-stage the fixed files.
3. Retry the commit as a new commit attempt.

## Safety rules

- Preserve unrelated user changes.
- Stage only intended files and hunks.
- Never commit secrets.
- Do not update git config unless explicitly requested.
- Do not skip hooks unless explicitly requested.
- Do not use interactive `-i` unless explicitly requested.
- Do not force-push, push, amend, reset, restore, or create empty commits unless explicitly requested.
- If existing staged changes are not clearly part of the requested commit, stop and ask before changing the index.
- Before committing in full mode, inspect `git status`, `git diff`, `git diff --cached`, and `git log --oneline -10`.

## Examples

Separate unrelated changes:

```bash
git add nvim/lua/plugins/lsp.lua
git commit -m "fix(nvim/lsp): correct handler registration"

git add nvim/lua/plugins/telescope.lua
git commit -m "extend(nvim/telescope): add file preview options"

git add zsh/.zshrc
git commit -m "add(zsh): fzf key bindings"
```

Grouped related changes:

```bash
git add nvim/lua/plugins/lsp.lua nvim/lua/plugins/cmp.lua nvim/lua/plugins/treesitter.lua
git commit -m "$(cat <<'EOF'
edit(nvim): completion and diagnostic tweaks

- disable ghost text in cmp
- add null check on lsp handler
- pin treesitter parsers for lua and go
EOF
)"
```

Split mixed concerns in one file:

```bash
git add -p nvim/lua/plugins/lsp.lua
git commit -m "fix(nvim/lsp): null check on handler"

git add -p nvim/lua/plugins/lsp.lua
git commit -m "add(nvim/lsp): diagnostic virtual text toggle"
```
