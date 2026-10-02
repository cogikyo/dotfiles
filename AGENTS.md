# Dotfiles

Arch + Hyprland (Wayland) dotfiles for a single-user Framework Desktop (AMD Strix Halo, Ethernet only).
Root of repo = `~/dotfiles`.

## Layout

- `config/` → linked into `~/.config/` by `dctl doctor --fix home`, with separate Firefox and OBS handling
- `cmds/` → Go command workspace; the ISO installs prebuilt commands into `~/.local/bin/`; see `cmds/README.md`
- `system/` → rootfs overlay mirroring `/`; **copied** into place (not symlinked)
- `packages/` → package lists (`base.lst`, `aur.lst`, `extra.lst`) and local PKGBUILDs
- `secrets/` → age-encrypted secrets and their manifest; `repos.json` → repos cloned under `~/`
- `config/opencode/` → OpenCode harness: `opencode.json`, always-loaded caps files (`AGENTS.md`, `COLLAB.md`, `ROUTING.md`), agents, skills, and plugins (see `config/opencode/plugins/README.md`)
- `iso/` → archiso profile; `iso/work/` and `iso/out/` are gitignored build artifacts
- `share/` → static assets

Editing linked config changes the live system.
User units and their `.wants` links live only in `config/systemd/user/`.
The system group enables only units named in `system/etc/systemd/system-preset/10-dotfiles.preset`, without starting them.

## Harness

OpenCode is the primary agent harness.
Edits under `config/opencode/` reach running sessions after an OpenCode restart, except instruction files, which OpenCode re-reads on every model step.

## Install

ISO builds require a clean, committed `master` and root for makechrootpkg/mkarchiso.
`dctl install` runs only as root on the dctl UEFI ISO and erases the selected whole disk after typed confirmation.
Run user doctor fixes as the user and use sudo only for root groups.
See the [`dctl` guide](cmds/cmd/dctl/README.md) for procedures and the required [SSH cutover](cmds/cmd/dctl/README.md#build).

## Commands

One Go module contains multiple binaries, with daemon sockets at `/tmp/{hyprd,ewwd}.sock`.

- `hyprd` — Hyprland window management
- `ewwd` — system signals for eww widgets
- `newtab` — Firefox new-tab backend
- [`dctl`](cmds/cmd/dctl/README.md) — ISO builder, installer, doctor, and maintenance commands

Use `hyprd rebuild` only for an attended live update and never during a full lock; see the [hyprd guide](cmds/cmd/hyprd/README.md#rebuild).
For build-only work, use targeted builds into `/tmp/opencode/bin/` and do not restart daemons or install binaries into the live user path.

## Conventions

- Prefer Go for new work. Bash only for genuinely shell-shaped helpers. If bash logic grows, move to `cmds/`.
- Bash: `#!/usr/bin/env bash` + `set -euo pipefail`.
- Interactive zsh enables `EXTENDED_GLOB`; use extended glob features when useful, but quote literal `#`, `^`, and `~` values in sourced zsh files, especially hex colors like `'fg=#824141'`.
- Logging: `info()` (blue), `success()`/`ok()` (green), `warn()` (yellow), `error()`/`err()` (red).
- Python one-offs: use `uv run --with <package>... python <script>` or `uv run --with <package>... python - <<'PY'`; do not install packages into system Python or leave activated venvs behind.
- Commit note: always include `config/nvim/lua/plugins/editor/harpoon.json` when it appears changed; it often changes incidentally and can be included in any commit without mention.

## Go (`go 1.26.2`)

Bias toward modern Go. Stdlib-first — prefer `log/slog`, `errors.Is`/`As`/`Join`, `slices`, `maps`, `iter`, `cmp`, `sync.WaitGroup.Go`, `testing/synctest`, `os.Root`/`os.OpenInRoot` before reaching for custom helpers or deps.

Modern idioms: `for range n`, iterator helpers via `iter`/`maps`/`slices`, `new(expr)` for optional pointer fields.

Workflow after non-trivial edits: `gofmt`/`goimports`, `go fix`, `go vet`, targeted `go test`. Build only affected binaries.

Concrete types and package-level functions by default. Interfaces only at consumer boundaries when actually needed.
