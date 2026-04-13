# hyprd

Window management daemon for Hyprland. Connects to Hyprland's IPC sockets to track workspace changes and window events, then exposes commands and state over a Unix socket.

## Commands

```bash
hyprd                    # start daemon (foreground)
hyprd status             # check if running
hyprd status --json      # full state dump
```

### Window management

```bash
hyprd monocle            # float focused window to dedicated workspace
hyprd split              # cycle split ratio: xs → default → lg
hyprd split -x|-d|-l     # set specific ratio
hyprd hide               # move slave to special workspace
hyprd swap               # exchange master/slave positions
hyprd ws <n>             # switch workspace, focus master
hyprd ws up|down         # move active window between workspaces 2..5
hyprd focus <class> [title]  # focus window by class, unhide if needed
```

### Sessions

```bash
hyprd layout --list      # list available sessions
hyprd layout <name>      # spawn windows for session
```

### Query and subscribe

Used by eww widgets for real-time state.

```bash
hyprd query [topic]      # get state as JSON (workspace|monocle|hidden|split|all)
hyprd subscribe [...]    # stream events (workspace monocle split)
```

eww integration:

```yuck
(deflisten hyprd `hyprd subscribe workspace monocle split`)
(label :text {hyprd?.workspace?.current ?: "?"})
```

## Configuration

`../configs/hyprd.yaml` — monitor geometry, split ratios, monocle sizing, colors

Session definitions live in the same `hyprd.yaml` file so layout names and startup behavior stay in one place.

`hyprd` reads its config from the repo-level daemon config directory and uses that to drive startup, session layout spawning, and workspace behavior.

## Structure

```
hyprd/
├── daemon.go            # lifecycle, server setup, command handler
├── events.go            # Hyprland event subscription loop
├── hypr/                # Hyprland IPC socket client
├── main.go              # CLI entry, command routing to daemon socket
├── notify/              # notification formatting and delivery
├── session/             # startup, layout spawning, session orchestration
├── state/               # shared daemon state and derived view state
├── windows/             # window matching and window-centric helpers
└── wm/                  # window/workspace actions: monocle, split, hide, swap, ws, focus
```
