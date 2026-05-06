# Dotfiles

Arch + Hyprland (Wayland) dotfiles. Single-user. Root of repo = `~/dotfiles`.

## Layout

- `config/` → symlinked into `~/.config/` by `install.sh link`
- `bin/` → symlinked into `~/.local/bin/` (legacy; being replaced by `cmds/`)
- `cmds/` → Go command workspace; built into `~/.local/bin/` by `install.sh go`. See `cmds/README.md`.
- `etc/` → system configs **copied** to `/etc/` by `install.sh system` (not symlinked)
- `skills/` → agent skills, linked by `skills/link.sh`
- `iso/` → archiso profile; `iso/work/` and `iso/out/` are gitignored build artifacts
- `share/` → static assets

Everything in `config/` and `bin/` is symlinked wholesale except: `config/claude/settings.json` and `config/firefox/` are linked individually or handled specially.
Editing the repo IS editing the live system.

## Install

`./install.sh all` | `./install.sh <name>` | `--list` | `--check`

Steps: `packages`, `link`, `secrets`, `repos`, `system`, `hibernate`, `fonts`, `go`, `eww`, `firefox`, `shell`, `dns`.

## Commands

Go command workspace. One module, multiple binaries. Sockets at `/tmp/{hyprd,ewwd}.sock`.

- `hyprd` — Hyprland window management
- `ewwd` — system signals for eww widgets
- `newtab` — Firefox new-tab backend
- `statusline` — Claude Code statusline

After editing `hyprd`, run `hyprd rebuild` — it builds, preserves runtime state, and hot-restarts in place.
For other commands, use targeted builds from `cmds/` (`go build -o ~/.local/bin/<name> ./cmd/<name>`).

## Conventions

- Prefer Go for new work. Bash only for genuinely shell-shaped helpers. If bash logic grows, move to `cmds/`.
- Bash: `#!/usr/bin/env bash` + `set -euo pipefail`.
- Interactive zsh enables `EXTENDED_GLOB`; use extended glob features when useful, but quote literal `#`, `^`, and `~` values in sourced zsh files, especially hex colors like `'fg=#824141'`.
- Logging: `info()` (blue), `success()`/`ok()` (green), `warn()` (yellow), `error()`/`err()` (red).
- Nerd Font / multi-width UTF glyphs: use Python (`Write`/`Edit` corrupts them).
- Commit note: always include `config/nvim/lua/plugins/editor/harpoon.json` when it appears changed; it often changes incidentally and can be included in any commit without mention.

## Go (`go 1.26.2`)

Bias toward modern Go. Stdlib-first — prefer `log/slog`, `errors.Is`/`As`/`Join`, `slices`, `maps`, `iter`, `cmp`, `sync.WaitGroup.Go`, `testing/synctest`, `os.Root`/`os.OpenInRoot` before reaching for custom helpers or deps.

Modern idioms: `for range n`, iterator helpers via `iter`/`maps`/`slices`, `new(expr)` for optional pointer fields.

Workflow after non-trivial edits: `gofmt`/`goimports`, `go fix`, `go vet`, targeted `go test`. Build only affected binaries.

Concrete types and package-level functions by default. Interfaces only at consumer boundaries when actually needed.
