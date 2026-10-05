# cmds

Go command workspace for Hyprland, eww, Firefox, and dotfiles management.

## Why

**Event-driven.** State changes push to subscribers when they happen.

**In-memory state.** One process holds the full picture.
Commands that depend on each other (e.g. hide needs to know about monocle) share state directly.

**Single binary per domain.** Related features live in one process with shared context instead of scattered scripts that can't coordinate.

## Architecture

```
                  ┌─────────────────────────────────────────────────────────────┐
                  │                        eww widgets                          │
                  │         (deflisten hyprd-state `hyprd subscribe ...`)       │
                  │         (deflisten ewwd-state `ewwd subscribe ...`)         │
                  └─────────────────────────┬───────────────────────────────────┘
                                            │ Unix socket streams
                          ┌─────────────────┴─────────────────┐
                          ▼                                   ▼
                  ┌───────────────────┐               ┌───────────────────┐
                  │      hyprd        │               │       ewwd        │
                  │  /tmp/hyprd.sock  │               │  /tmp/ewwd.sock   │
                  └────────┬──────────┘               └───────────────────┘
                          │
                          ▼
                  ┌───────────────────┐
                  │  Hyprland IPC     │
                  │  .socket.sock     │ ← commands
                  │  .socket2.sock    │ ← events
                  └───────────────────┘
```

## Commands

- **[dctl](cmd/dctl/)** — Dotfiles control plane
- **[ewwd](cmd/ewwd/)** — System utilities: audio, clock, computer, cycles, music, network, date, weather, timer
- **[hyprd](cmd/hyprd/)** — Window management: monocle, split ratios, hide/show, swap, workspace nav, session layouts
- **[keys](cmd/keys/)** — Svalboard keymap viewer: local HTTP server with app bind overlays; unrelated to `dctl keys`
- **[newtab](cmd/newtab/)** — Firefox new tab page: local HTTP server with bookmarks, history, and suggestions
- **[src](cmd/src/)** — Source inspection cache for upstream repos, Go modules, npm package repos, and Arch package sources

## Layout

The module keeps command entrypoints under `cmd/`, shared packages under `internal/`, and runtime YAML config under `config/`.

## Shared infrastructure

The `internal/daemon` package provides the Unix socket server/client and subscription system used by `hyprd` and `ewwd`.
It handles socket lifecycle, command routing, and event streaming.

`newtab` is in the same Go module but uses its own HTTP server.

```
internal/daemon/
├── server.go      # Unix socket listener, command dispatch, signal handling
├── client.go      # Send commands, stream subscriptions, health check
└── subscribe.go   # Topic-based pub/sub with JSON event delivery
```

## Installation

The dctl ISO installs prebuilt commands into `~/.local/bin/`.
`dctl update cmd` builds from the working tree, replaces only changed commands, and restarts ewwd, newtab, and keys only when replaced.
It uses `hyprd rebuild` for hyprd so the daemon owns its state-preserving restart and lock refusal.

To build dctl on an existing machine, run from `cmds/` with Go 1.26.2 or later and an existing `~/.local/bin/` directory:

```sh
go build -o "$HOME/.local/bin/dctl" ./cmd/dctl
```

For build-only checks, use an existing scratch directory instead of the live binary path:

```sh
go build -o /tmp/opencode/bin/ ./cmd/dctl
```

See the [hyprd guide](cmd/hyprd/README.md#rebuild) for attended live rebuilds.
The [`dctl` guide](cmd/dctl/README.md) covers ISO builds, installation, setup stages, and updates.

Config files live in `cmds/config/` in the source tree.
Config-backed commands read their config at startup; see command-specific docs for details.
