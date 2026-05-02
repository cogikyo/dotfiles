# hyprd

Hyprland daemon and CLI.
Connects to Hyprland's IPC sockets to manage windows, workspaces, and sessions, then exposes commands over a Unix socket.
CLI-only tools (screenshot, SSH) run directly without the daemon.

## Structure

```
hyprd/
├── main.go                     # CLI entry, command routing to daemon socket
├── daemon.go                   # lifecycle, server setup, command dispatch table
├── events.go                   # Hyprland event subscription loop → state updates
├── hyprd.service               # systemd user unit
│
├── cli/                        # CLI-only commands (no daemon socket, run directly)
│   ├── screenshot.go           #   region screenshot: wayfreeze + grim + satty
│   └── ssh.go                  #   PAM-driven SSH key loading via ssh-agent
│
├── vpn/                        # VPN connection management via NetworkManager
│   └── vpn.go                  #   list, status, toggle, up, down (nmcli)
│
├── browser/                    # Firefox session snapshot and restore
│   ├── browser.go              #   subcommand dispatch (windows/snapshot/show/hypr/restore)
│   ├── firefox.go              #   profile discovery + sessionstore loading
│   ├── mozlz4.go               #   Mozilla LZ4 decompression
│   ├── profile.go              #   Firefox profile path resolution
│   ├── restore.go              #   snapshot restore (URL replay or exact session replacement)
│   ├── session_store.go        #   sessionstore JSON parsing
│   ├── snapshot.go             #   named snapshot creation from browser windows
│   ├── browser_test.go         #   tests
│   └── sessions/               #   saved session snapshots (json + yaml)
│
├── hypr/                       # Hyprland IPC socket client
│   └── socket.go               #   command socket + event socket primitives
│
├── session/                    # startup, layout spawning, kitty tabs
│   ├── init.go                 #   Init.Execute: startup orchestration (bg → net → layouts → execs → pseudo-lock)
│   ├── layout.go               #   Layout.openSession: spawns windows from sessions.<name>.body
│   ├── lock.go                 #   Lock.{Pseudo,Unlock,Full}: visual blackout, audio/notify pause, restore
│   ├── bg.go                   #   mpvpaper wallpaper lifecycle
│   ├── picker.go               #   interactive eww session picker overlay
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
    ├── actions.go              #   D-Bus listener for dunst notification click-to-focus
    ├── assets.go               #   sound/icon path constants
    ├── context.go              #   per-ws notification context
    ├── helpers.go              #   sound/icon resolution from config
    └── types.go                #   NotifyRequest, Notifier
```

## Where to find things

| Task | Start here |
|---|---|
| Startup sequence / "what happens when hyprd boots" | `session/init.go` → `Init.Execute` |
| Session definitions (dotfiles, leadpier, cogikyo) | `config/hyprd.yaml` → `sessions.*` |
| How a session maps to windows | `session/layout.go` → `Layout.openSession` |
| Window types that make up a session | `config/hyprd.yaml` → `three_body.*` |
| Which session opens on which workspace at boot | `config/hyprd.yaml` → `sessions` entries with `init: true` |
| Command routing (CLI → daemon) | `main.go` → `daemon.go` dispatch table |
| CLI-only tools (no daemon needed) | `cli/` — screenshot, SSH |
| Hyprland event → state update | `events.go` |
| Adding a new daemon command | add file in `wm/`, register in `daemon.go` |
| Adding a new CLI-only tool | add file in `cli/`, register in `main.go` |
| Notification styling and sounds | `config/hyprd.yaml` → `notify.*`, logic in `notify/handler.go` |
| Notification click-to-focus | `notify/actions.go` — D-Bus ActionInvoked listener |
| Kitty tab profiles (editor/agents/leadpier) | `config/hyprd.yaml` → `tabs.*`, logic in `session/tab.go` + `tabs.go` |
| Interactive session picker | `session/picker.go` → `Picker.Execute` |
| Firefox session snapshots | `browser/` — snapshot, restore, profile discovery |

## Startup flow

```
hyprland.conf: exec-once = hyprd init
  └─ cmdInit (main.go)
      ├─ import Wayland env into systemd
      ├─ systemctl start hyprd.service
      ├─ wait for daemon socket
      └─ sendCommand("init")
          └─ Daemon.handleCommand("init") (daemon.go)
              └─ Init.Execute (session/init.go)
                  ├─ EnsureBG                        # mpvpaper wallpaper
                  ├─ waitNetwork
                  ├─ Layout.Execute(name) per session in init.sessions
                  │   └─ openSession (session/layout.go)
                  │       ├─ workspace <n>
                  │       ├─ for each body entry → exec three_body.<name>.command
                  │       ├─ layoutmsg mfact exact <split.default>
                  │       └─ focuswindow <master>
                  ├─ dispatchStartup                 # glava, spotify, bluetooth
                  ├─ workspace init.workspace
                  └─ Lock.Pseudo (if init.lock)      # blackout + submap
```

Unlock restores the saved workspace and calls `dispatchStartup` so the glava/spotify/bluetooth restore surface lives in one place.

## Commands

```bash
hyprd                    # start daemon (foreground)
hyprd init               # import env, start services, run boot sequence
hyprd status             # check if running
hyprd status --json      # full state dump
hyprd rebuild            # rebuild binary and hot-restart (preserves state)
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
hyprd bg <mode>              # background: code, music, kill, lock, ensure
```

### Three-body & shadow

```bash
hyprd three-body editor      # focus/launch editor window
hyprd three-body agents      # focus/launch agents (checks notifications first)
hyprd three-body browser     # focus/launch browser window
hyprd three-body shadow      # toggle active/shadow slave
hyprd shadow                 # toggle visibility of shadow workspace
hyprd shadow list            # list windows parked on shadow workspace
```

### Sessions & layouts

```bash
hyprd layout --list              # list sessions grouped by workspace
hyprd layout <name>              # spawn windows for a named session
hyprd layout <ws>                # open the active session for that workspace
hyprd layout set <ws> <name>     # set active session for a workspace
hyprd picker open                # open interactive layout picker overlay
hyprd picker close               # close picker without action
hyprd picker confirm             # confirm selection
hyprd project <args>             # project path management
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

### Browser

```bash
hyprd browser windows [--all] [--profile <name|path>]
hyprd browser snapshot <name> [active|largest|index] [--profile <name|path>]
hyprd browser show <name>
hyprd browser hypr <name>
hyprd browser restore <name> [--mode exact|urls] [--profile <name|path>] [--force] [--dry-run]
hyprd browser launch [--profile <name|path>]
```

Browser snapshots are the Firefox layout primitive for sessions. The normal flow is:

1. Arrange the Firefox window how you want it.
2. Save it with `hyprd browser snapshot <name>`; use `largest` or a numeric window index if the active window is not the one you want.
3. Reference it in `daemons/config/hyprd.yaml` as `browser: <name>`.
4. Open the session with `hyprd layout <session>` or let `hyprd init` restore init sessions at boot.

`browser: <name>` is shorthand for an exact forced restore of that snapshot. Use the expanded map only for profile overrides or non-snapshot URL launches:

```yaml
browser: leadpier

browser:
  snapshot: leadpier
  profile: dev-edition

browser:
  urls:
    - https://example.com
```

Command meanings:

- `windows` lists Firefox windows from the sessionstore; without `--all`, trivial/new-tab windows are filtered out.
- `snapshot` writes a named snapshot under `browser/sessions/` from the selected Firefox window. Selectors are `active`, `largest`, or a 1-based window index.
- `show` prints the saved snapshot YAML summary.
- `hypr` prints a launch config generated from the snapshot; mostly useful for inspection now that session config can use `browser: <name>`.
- `restore` manually restores a snapshot. It defaults to exact session replacement with force; pass `--mode urls` to replay visible tabs into a normal Firefox window instead.
- `launch` clears the profile sessionstore and opens a clean new-tab window; this is used internally by the three-body browser command for non-snapshot launches.

### Screenshot

```bash
hyprd screenshot              # region screenshot to clipboard (wayfreeze + grim)
hyprd screenshot annotate     # region screenshot → satty annotation → clipboard
```

### SSH

```bash
hyprd ssh pam-load            # load SSH keys via PAM auth token (called from hyprlock)
```

### Notifications

```bash
hyprd notify hook claude <event>      # read Claude hook JSON from stdin
hyprd notify hook opencode            # read OpenCode notify JSON from argv/stdin
hyprd notify dunst                    # handle Dunst script callbacks
hyprd notify kitty-finish <command>   # emit kitty command-finish notification
```

### VPN

```bash
hyprd vpn work             # toggle configured NetworkManager VPN alias
hyprd vpn work up|down     # connect/disconnect explicitly
hyprd vpn work status      # status for one alias/connection
hyprd vpn install work     # import staged .nmconnection into NetworkManager
hyprd vpn install work --replace
hyprd vpn export work      # export NetworkManager profile to staged file
hyprd vpn status           # active VPN summary
hyprd vpn list             # list NetworkManager VPN connections
```

VPN aliases live in `config/hyprd.yaml` under `vpn.connections`. Importable profiles are staged under `~/.local/share/dotfiles/vpn/` and should be encrypted through `etc/secrets`; live imported connections and keyring passwords stay in NetworkManager.

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

`daemons/config/hyprd.yaml` — overrides compiled defaults for:

- `background` — mpvpaper wallpaper
- `init` — boot sequence (sessions, execs, lock)
- `notify` — sounds, icons, per-style appearance
- `windows` — ignored classes, hidden/shadow workspace names, split presets, monocle sizing
- `tabs` — kitty tab profiles (editor, agents, leadpier)
- `three_body` — window building blocks (class, title, command) referenced by sessions
- `sessions` — layouts grouped by workspace, then keyed by session name; `init: true` launches on boot (at most one per workspace)
