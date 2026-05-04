# cmds

Go command workspace for Hyprland, eww, Firefox, Claude Code, and dotfiles management.

## Why

**Event-driven.** State changes push to subscribers when they happen, without polling.

**In-memory state.** One process holds the full picture. Commands that depend on each other (e.g. hide needs to know about monocle) share state directly.

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
- **[hyprd](cmd/hyprd/)** — Window management: monocle, split ratios, hide/show, swap, workspace nav, session layouts
- **[ewwd](cmd/ewwd/)** — System utilities: audio, music, network, date, weather, timer
- **[newtab](cmd/newtab/)** — Firefox new tab page: local HTTP server with bookmarks, history, and suggestions
- **[statusline](cmd/statusline/)** — Claude Code statusline renderer

## Layout

The module keeps command entrypoints under `cmd/`, shared packages under `internal/`, and runtime YAML config under `config/`.

```text
cmds/
├── cmd/                # binary entrypoints
│   ├── dctl/
│   ├── ewwd/
│   ├── hyprd/
│   ├── newtab/
│   └── statusline/
├── config/             # runtime YAML config
└── internal/           # shared and command-private packages
    ├── config/         # typed config loader
    ├── daemon/         # Unix socket helpers
    ├── dctl/
    ├── ewwd/
    └── hyprd/
```

`hyprd` is further split into `browser/`, `notify/`, `session/`, `state/`, `windows/`, and `wm/` to keep concerns separated.

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

```bash
./install.sh go          # build all
```

Binaries go to `~/.local/bin/`.
If `hyprd` is already running, `install.sh go` uses `hyprd rebuild` for hot-restart.

Config files live in `cmds/config/` in the source tree.
Config-backed commands read their config at startup; see command-specific docs for details.

### Hyprland startup

```conf
# hyprland.conf
exec-once = hyprd init   # imports env, starts daemons, runs boot sequence
exec-once = ewwd
```
