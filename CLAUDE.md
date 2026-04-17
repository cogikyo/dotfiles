# CLAUDE.md

Agent orientation for this repo. Terse by design — skim, then dig.

Arch + Hyprland (Wayland) dotfiles. Single-user. Root of repo = `~/dotfiles`.

## Layout

- `config/` → symlinked wholesale into `~/.config/` by `install.sh link`, **except** `claude/`, `codex/`, `firefox/` (handled per-file below)
- `bin/` → symlinked into `~/.local/bin/` (on PATH via `config/shell/env.sh`)
- `daemons/` → Go sources for `hyprd`, `ewwd`, `newtab`, `statusline`; built into `~/.local/bin/` by `install.sh go`
- `etc/` → system configs copied into `/etc/…` by `install.sh system` (needs sudo)
- `share/` → static assets (sounds, videos, wallpapers, banners)
- `skills/` → agent skills, linked by `skills/link.sh` (see Skills below)
- `iso/` → archiso profile for custom live ISO; `iso/work/` and `iso/out/` are build artifacts (gitignored)
- `install.sh` → post-install step runner (see Install steps)
- `build.sh` → builds/releases the custom Arch ISO (sudo; root only)
- `README.md` → human-facing showcase; **this file is the agent contract**

## Symlink rules

Editing source of truth lives in this repo. `~/.config/*`, `~/.local/bin/*`, and `~/.codex/*` are symlinks **into this repo** — edit here, not there.

Exceptions (NOT wholesale-linked from `config/`):
- `config/claude/settings.json` → individual link to `~/.config/claude/settings.json`
- `config/codex/config.toml` → individual link to `~/.codex/config.toml`
- `config/firefox/` → handled by `install.sh firefox` (profile merge, not a plain symlink)

`etc/` is **copied** (not symlinked) on `install.sh system`. Edits there won't propagate until re-run.

## Install steps

`./install.sh all` runs everything. Individual steps: `./install.sh <name>`. `--list` for current list, `--check` for healthchecks.

```
01 packages    — paru/pacman from etc/packages*.lst (sudo)
02 link        — symlink config/ and bin/ (also `bin/relink`)
03 secrets     — decrypt age-encrypted etc/secrets/ to targets
04 repos       — clone repos, mkdir user dirs (needs 03)
05 system      — copy etc/ → /etc/, enable services (sudo)
06 hibernate   — swapfile + suspend-then-hibernate (sudo)
07 fonts       — extract etc/fonts.tar.gz; optional Iosevka build
08 go          — build daemons/ → ~/.local/bin/
09 eww         — install prebuilt bin/eww (EWW_BUILD=1 to rebuild from source)
10 firefox     — profile, theme, user.js (needs 04)
11 shell       — chsh to zsh (sudo)
12 dns         — systemd-resolved + Cloudflare DoT (sudo, needs 05)
```

## bin/ scripts

All are `set -euo pipefail` bash (except `bin/eww` which is a committed prebuilt binary).

- `relink`    → shim for `install.sh link`
- `update`    → paru update, orphan cleanup, writes `etc/packages*.lst`
- `secrets`   → age encrypt/decrypt for `etc/secrets/` (passphrase-protected identity)
- `vpn`       → NetworkManager L2TP helper (`TrendCapitalVPN` by default)
- `screenshot`→ Hyprland region shot with freeze
- `record`    → screen record (VAAPI h264)
- `hunk-commit`→ interactive staged-hunk commit splitter
- `eww`       → **prebuilt** eww binary (patched); do not treat as text

## Daemons (`daemons/`)

Go workspace. One module, multiple `cmd/`-style entry dirs. See `daemons/README.md` for architecture + sockets.

- `hyprd` — Hyprland window management: monocle, split ratios, hide/show, swap, workspaces, session layouts. Socket: `/tmp/hyprd.sock`.
- `ewwd`  — System signals for eww: GPU, audio, music, network, date, weather, timer. Socket: `/tmp/ewwd.sock`.
- `newtab`— Firefox new-tab page backend.
- `statusline` — Claude Code statusline.

Editing any of these requires `install.sh go` to take effect (running binaries are in `~/.local/bin/`). Configs live in `daemons/configs/` (and `*.local.yaml` files there are gitignored for machine overrides).

## Secrets

`etc/secrets/` uses age with a passphrase-protected identity. `bin/secrets` is the CLI. `etc/secrets/identity` (plaintext) is gitignored as a safety net — never commit plaintext keys.

## Skills

- `skills/user/` — user-level skills (commit, learn, scribe). Linked to both `~/.codex/skills/` and `~/.claude/skills/` by `skills/link.sh user`.
- `skills/project/` — per-project skills (e.g. `endof`). Linked to `./.codex/skills/` and `./.claude/skills/` by `skills/link.sh project [name]`.
- `skills/link.sh` is called from `install.sh link` for user skills; project skills opt-in.

## Conventions

- Prefer Go for new implementation work outside repo-root install/bootstrap shell (`install.sh`, `build.sh`) and genuinely shell-shaped helpers in `bin/`. If logic grows beyond straightforward command orchestration, move it into `daemons/` or a dedicated Go command/package instead of growing Bash.
- Bash: `#!/usr/bin/env bash` + `set -euo pipefail`.
- Logging: `info()` (blue), `success()`/`ok()` (green), `warn()` (yellow), `error()`/`err()` (red). Match existing style in neighboring scripts.
- Commits: Conventional Commits. Split unrelated changes into separate commits — do not bundle.
- Files are mostly symlinks on the live system; editing the repo IS the edit.
- When writing files with Nerd Font / multi-width UTF glyphs, use Python (`Write`/`Edit` corrupts them).

## Go (`go 1.26.2`)

- Stdlib-first. Prefer `context`, `log/slog`, `errors.Is`/`As`/`Join`, `slices`, `maps`, `iter`, `cmp`, `min`/`max`/`clear`, `sync.WaitGroup.Go`, `testing/synctest`, and `os.Root` before custom helpers or third-party deps.
- Run `gofmt`/`goimports`, `go fix`, `go vet`, and targeted `go test` in every touched Go module after meaningful edits.
- Prefer concrete types and package-level functions. Define interfaces at the consumer boundary when multiple implementations or tests actually need them; do not start with interface-first design.
- Pass `context.Context` as the first parameter for request/lifecycle-bound work. Do not store contexts in structs.
- Keep error flow left-aligned. Return early, wrap with `%w`, use `errors.Is`/`As` instead of string matching, and keep error strings lowercase with no trailing punctuation.
- Keep goroutine lifetimes obvious. Prefer synchronous APIs. Use `WaitGroup.Go` over `Add`/`Done` unless the older pattern is genuinely clearer, and use `testing/synctest` for time/concurrency-heavy tests.
- Make zero values useful. Avoid pointer params just to save copies. Use pointer receivers for mutation or mutex-bearing structs, and do not mix receiver types on the same type.
- Prefer modern language/library idioms when they improve clarity: `for range n` for counted loops, iterator-based helpers via `iter`/`maps`/`slices`, and `new(expr)` for optional pointer fields when it reads better.
- For filesystem work that joins a trusted base with untrusted relative names, prefer `os.OpenRoot`/`os.OpenInRoot` over `filepath.Join` + `os.Open`.
- Use generics to remove real duplication, not to build abstraction towers. If a concrete function is clearer, keep it concrete.
- In tests, prefer plain `testing`, table-driven cases, subtests, and clear `got`/`want` failures over assert DSLs.

## Gotchas

- `config/claude/` and `config/codex/` are NOT linked wholesale — only `settings.json` / `config.toml` are. Don't assume adding a file under `config/claude/` will appear in `~/.config/claude/`.
- `etc/` changes need `install.sh system` to land on the live system.
- Daemon edits need `install.sh go` to take effect.
- `iso/work/` and `iso/out/` are build output of `build.sh`; don't hand-edit.
- `AGENTS.md` at repo root is a symlink to this file (Codex reads `AGENTS.md`, Claude reads `CLAUDE.md`).
