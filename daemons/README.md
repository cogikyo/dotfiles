# daemons

Go daemons and services for Hyprland, eww, and Firefox.

## Why

**Event-driven.** State changes push to subscribers the moment they happen — no polling, no latency.

**In-memory state.** One process holds the full picture. Commands that depend on each other (e.g. hide needs to know about monocle) share state directly.

**Single binary per domain.** Related features live in one process with shared context instead of scattered across independent scripts that can't coordinate.

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

## Daemons

- **[hyprd](hyprd/)** — Window management: monocle, split ratios, hide/show, swap, workspace nav, session layouts
- **[ewwd](ewwd/)** — System utilities: GPU stats, audio, brightness, music, network, date, weather, timer
- **[newtab](newtab/)** — Firefox new tab page: local HTTP server with bookmarks, history, and suggestions

## Shared infrastructure

The `daemon/` package provides the Unix socket server/client and subscription system used by hyprd and ewwd. It handles socket lifecycle, command routing, and event streaming. (newtab uses its own HTTP server and separate go.mod.)

```
daemon/
├── server.go      # Unix socket listener, command dispatch, signal handling
├── client.go      # Send commands, stream subscriptions, health check
└── subscribe.go   # Topic-based pub/sub with JSON event delivery
```

## Installation

```bash
install-go.sh          # build all
install-go.sh hyprd    # build one
install-go.sh --list   # see available
```

Binaries go to `~/.local/bin/`.

### Hyprland startup

```conf
# hyprland.conf
exec-once = hyprd
exec-once = ewwd
exec-once = newtab
```
