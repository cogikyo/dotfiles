# hyprd

Window management daemon for Hyprland. Connects to Hyprland's IPC sockets to track workspace changes and window events, then exposes commands and state over a Unix socket.

## Structure

```
hyprd/
├── main.go                     # CLI entry, command routing to daemon socket
├── daemon.go                   # lifecycle, server setup, command dispatch table
├── events.go                   # Hyprland event subscription loop → state updates
├── hyprd.service               # systemd user unit
│
├── hypr/                       # Hyprland IPC socket client
│   └── socket.go               #   command socket + event socket primitives
│
├── session/                    # startup, layout spawning, kitty tabs
│   ├── init.go                 #   Init.Execute: startup orchestration (bg → net → layouts → execs → pseudo-lock)
│   ├── layout.go               #   Layout.openSession: spawns windows from sessions.<name>.body
│   ├── lock.go                 #   Lock.{Pseudo,Unlock,Full}: visual blackout, audio/notify pause, restore
│   ├── bg.go                   #   mpvpaper wallpaper lifecycle
│   ├── kitty.go                #   kitty remote-control client (list/focus/send-text)
│   ├── tab.go                  #   `hyprd tab <name>` - switch tab in focused kitty
│   ├── tabs.go                 #   `hyprd tabs init/refresh` - hydrate from config + titles
│   └── profile.go              #   detect which tab profile (editor/agents/leadpier) owns a kitty window
│
├── state/                      # thread-safe daemon state + derived views
│   ├── state.go                #   State struct, JSON dump, config accessor
│   ├── workspace.go            #   current ws + displaced-master tracking
│   ├── sessions.go             #   active session per workspace, project paths
│   ├── hidden.go               #   type defs: HiddenState, ThreeBodyState, MonocleState
│   ├── monocle.go              #   per-ws monocle state getters/setters
│   ├── threebody.go            #   per-ws three-body state getters/setters
│   └── tab_memory.go           #   per-ws per-profile tab history
│
├── wm/                         # window/workspace actions (each file = one command)
│   ├── ws.go                   #   `hyprd ws <n|up|down>` - switch + focus master
│   ├── split.go                #   `hyprd split [-x|-d|-l]` - cycle/set master ratio
│   ├── hide.go                 #   `hyprd hide` - toggle slave → special:hiddenSlaves
│   ├── swap.go                 #   `hyprd swap` - exchange master/slave
│   ├── monocle.go              #   `hyprd monocle` - float focused to dedicated ws
│   ├── focus.go                #   `hyprd focus <class> [title]` - focus + unhide
│   └── threebody.go            #   three-window layout with shadow-ws swapping
│
├── windows/                    # window-level helpers used across wm/
│   ├── match.go                #   class/title matching
│   └── tiled.go                #   sorted tiled window list, cursor centering
│
└── notify/                     # notification formatting + delivery (dunst bridge)
    ├── handler.go              #   dispatch by source (claude/opencode/kitty/dunst/send)
    ├── cli.go                  #   `hyprd notify ...` CLI parsing
    ├── context.go              #   per-ws notification context
    ├── helpers.go              #   sound/icon resolution from config
    └── types.go                #   NotifyRequest, Notifier
```

## Where to find things

| Task | Start here |
|---|---|
| Startup sequence / "what happens when hyprd boots" | `session/init.go` → `Init.Execute` |
| Session definitions (dotfiles, leadpier, cogikyo) | `configs/hyprd.yaml` → `sessions.*` |
| How a session maps to windows | `session/layout.go` → `Layout.openSession` |
| Window types that make up a session | `configs/hyprd.yaml` → `three_body.*` |
| Which session opens on which workspace at boot | `configs/hyprd.yaml` → `active_sessions` |
| Command routing (CLI → daemon) | `main.go` → `daemon.go` dispatch table |
| Hyprland event → state update | `events.go` |
| Adding a new `hyprd <cmd>` action | add file in `wm/`, register in `daemon.go` |
| Notification styling and sounds | `configs/hyprd.yaml` → `notify.*`, logic in `notify/handler.go` |
| Kitty tab profiles (editor/agents/leadpier) | `configs/hyprd.yaml` → `tabs.*`, logic in `session/tab.go` + `tabs.go` |

## Startup flow

```
systemd → hyprd (main.go)
  └─ Daemon.Run (daemon.go)
      ├─ EventLoop.Run (events.go)           # subscribes to Hyprland events
      └─ Init.Execute (session/init.go)
          ├─ EnsureBG                        # mpvpaper wallpaper
          ├─ waitNetwork
          ├─ Layout.Execute(name) per session in init.sessions
          │   └─ openSession (session/layout.go)
          │       ├─ workspace <n>
          │       ├─ for each body entry → exec three_body.<name>.command
          │       ├─ layoutmsg mfact exact <split.default>
          │       └─ focuswindow <master>
          ├─ exec init.execs                 # glava, spotify, bluetooth
          ├─ workspace init.workspace
          └─ Lock.Pseudo (if init.lock)      # blackout + submap
```

Unlock restores the saved workspace and re-runs `init.execs` so the glava/bluetooth restore surface
lives in one place.

## Commands

```bash
hyprd                    # start daemon (foreground)
hyprd status             # check if running
hyprd status --json      # full state dump
```

### Window management

```bash
hyprd monocle                # float focused window to dedicated workspace
hyprd split                  # cycle split ratio: xs → default → lg
hyprd split -x|-d|-l         # set specific ratio
hyprd hide                   # move slave to special workspace
hyprd swap                   # exchange master/slave positions
hyprd ws <n>                 # switch workspace, focus master
hyprd ws up|down             # move active window between workspaces 2..5
hyprd focus <class> [title]  # focus window by class, unhide if needed
```

### Sessions & layouts

```bash
hyprd layout --list              # list sessions grouped by workspace
hyprd layout <name>              # spawn windows for a named session
hyprd layout <ws>                # open the active session for that workspace
hyprd layout set <ws> <name>     # set active session for a workspace
```

### Tabs (kitty)

```bash
hyprd tab <name|alias>           # switch tab in focused kitty window
hyprd tabs init <profile> <pid>  # hydrate tab titles on kitty spawn
hyprd tabs refresh <name> <pid>  # re-apply titles
```

### Lock

```bash
hyprd lock             # pseudo-lock: workspace blackout + dunst pause + music pause + submap
hyprd lock unlock      # exit pseudo-lock (alias: hyprd lock -u)
hyprd lock full        # wraps hyprlock --grace 2 with the pseudo-lock pre/post hooks
```

### Query and subscribe

Used by eww widgets for real-time state.

```bash
hyprd query [topic]      # get state as JSON (workspace|hidden|split|three-body|all)
hyprd subscribe [...]    # stream events (workspace split)
```

eww integration:

```yuck
(deflisten hyprd `hyprd subscribe workspace split`)
(label :text {hyprd?.workspace?.current ?: "?"})
```

## Configuration

`../configs/hyprd.yaml` — overrides compiled defaults for:

- `background` — mpvpaper wallpaper
- `init` — boot sequence (sessions, execs, lock)
- `notify` — sounds, icons, per-style appearance
- `windows` — ignored classes, hidden/shadow workspace names, split presets, monocle sizing
- `tabs` — kitty tab profiles (editor, agents, leadpier)
- `three_body` — window building blocks (class, title, command) referenced by sessions
- `sessions` — named layouts composed of `three_body` bodies + project paths + browser URLs
- `active_sessions` — default session per workspace
