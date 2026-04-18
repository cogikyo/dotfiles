# CLAUDE.md

Arch + Hyprland (Wayland) dotfiles. Single-user. Root of repo = `~/dotfiles`.

## Layout

- `config/` → symlinked into `~/.config/` by `install.sh link`
- `bin/` → symlinked into `~/.local/bin/` (legacy; being replaced by `daemons/`)
- `daemons/` → Go workspace, multiple `cmd/`-style binaries; built into `~/.local/bin/` by `install.sh go`. See `daemons/README.md`.
- `etc/` → system configs **copied** to `/etc/` by `install.sh system` (not symlinked)
- `skills/` → agent skills, linked by `skills/link.sh`
- `iso/` → archiso profile; `iso/work/` and `iso/out/` are gitignored build artifacts
- `share/` → static assets

Everything in `config/` and `bin/` is symlinked wholesale except: `config/claude/settings.json`, `config/codex/config.toml`, and `config/firefox/` are linked individually or handled specially. Editing the repo IS editing the live system.

## Install

`./install.sh all` | `./install.sh <name>` | `--list` | `--check`

Steps: `packages`, `link`, `secrets`, `repos`, `system`, `hibernate`, `fonts`, `go`, `eww`, `firefox`, `shell`, `dns`.

## Daemons

Go workspace. One module, multiple binaries. Sockets at `/tmp/{hyprd,ewwd}.sock`.

- `hyprd` — Hyprland window management
- `ewwd` — system signals for eww widgets
- `newtab` — Firefox new-tab backend
- `statusline` — Claude Code statusline

After editing `hyprd`, run `hyprd rebuild` — it builds, preserves runtime state, and hot-restarts in place. For other daemons, use targeted builds (`go build -o ~/.local/bin/<name> ./<name>`).

## Secrets

`etc/secrets/` uses age with passphrase-protected identity. CLI: `bin/secrets`.

## Conventions

- Prefer Go for new work. Bash only for genuinely shell-shaped helpers. If bash logic grows, move to `daemons/`.
- Bash: `#!/usr/bin/env bash` + `set -euo pipefail`.
- Logging: `info()` (blue), `success()`/`ok()` (green), `warn()` (yellow), `error()`/`err()` (red).
- Nerd Font / multi-width UTF glyphs: use Python (`Write`/`Edit` corrupts them).

## Go (`go 1.26.2`)

Bias toward modern Go. Stdlib-first — prefer `log/slog`, `errors.Is`/`As`/`Join`, `slices`, `maps`, `iter`, `cmp`, `sync.WaitGroup.Go`, `testing/synctest`, `os.Root`/`os.OpenInRoot` before reaching for custom helpers or deps.

Modern idioms: `for range n`, iterator helpers via `iter`/`maps`/`slices`, `new(expr)` for optional pointer fields.

Workflow after non-trivial edits: `gofmt`/`goimports`, `go fix`, `go vet`, targeted `go test`. Build only affected binaries.

Concrete types and package-level functions by default. Interfaces only at consumer boundaries when actually needed.
