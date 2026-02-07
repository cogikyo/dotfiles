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

`~/.config/hyprd/config.yaml` — monitor geometry, split ratios, monocle sizing, colors

`~/.config/hyprd/sessions.yaml` — session definitions for `hyprd layout`

## Structure

```
hyprd/
├── main.go              # CLI entry, command routing to daemon socket
├── daemon.go            # lifecycle, server setup, command handler
├── state.go             # thread-safe workspace and window state
├── events.go            # Hyprland event subscription loop
├── config/              # YAML config loading
├── commands/            # monocle, split, hide, swap, ws, focus, layout
└── hypr/                # Hyprland IPC socket client
```
