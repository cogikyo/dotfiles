# hyprd

Hyprland daemon and CLI.
Connects to Hyprland's IPC sockets to manage windows, workspaces, and sessions, then exposes commands over a Unix socket.
CLI-only tools (screenshot and VPN) run directly without the daemon.

## Structure

The entrypoint, daemon, and event dispatch live in `cmds/cmd/hyprd/`.
Domain packages live in `cmds/internal/hyprd/`, with direct screenshot and VPN commands under `cli/`.
The user unit is `config/systemd/user/hyprd.service` at the repo root.
In the table below, domain paths are relative to `cmds/internal/hyprd/`, entrypoint files to `cmds/cmd/hyprd/`, and config paths to `cmds/`.

## Where to find things

| Task                                               | Start here                                                                     |
| -------------------------------------------------- | ------------------------------------------------------------------------------ |
| Startup sequence / "what happens when hyprd boots" | `session/init.go` → `Init.Execute`                                             |
| Session definitions (dotfiles, leadpier, cogikyo)  | `config/hyprd.yaml` → `sessions.*`                                             |
| How a session maps to windows                      | `session/layout.go` → `Layout.openSession`                                     |
| Window types that make up a session                | `cmds/internal/config/hyprd.go` → compiled `ThreeBody` map                     |
| Which session opens on which workspace at boot     | `config/hyprd.yaml` → `sessions` entries with `init: true`                     |
| Command routing (CLI → daemon)                     | `main.go` → `daemon.go` dispatch table                                         |
| CLI-only tools (no daemon needed)                  | `cli/` — screenshot, VPN                                                       |
| Hyprland event → state update                      | `events.go`                                                                    |
| Adding a new daemon command                        | add file in `wm/`, register in `daemon.go`                                     |
| Adding a new CLI-only tool                         | add file in `cli/`, register in `main.go`                                      |
| Notification styling and sounds                    | `config/hyprd.yaml` → `notify.*`, logic in `notify/handler.go`                 |
| Notification activation (click or Alt+C)           | `notify/actions.go` — pending app routes + D-Bus ActionInvoked listener        |
| Kitty tab profiles (editor/agents/leadpier)        | `config/hyprd.yaml` → `tabs.*`, logic in `kitty/select.go` + `kitty/manage.go` |
| Interactive session picker                         | `session/picker.go` → `Picker.Execute`                                         |
| Firefox session snapshots                          | `browser/` — snapshot, restore, profile discovery                              |

## Startup flow

`hyprd init` imports the Wayland environment into systemd and D-Bus, starts the session target and user services, waits for the daemon socket, and sends `init`.
The daemon ensures the wallpaper, optionally waits for the network, restores eww and the initial browser layouts, opens configured sessions, and selects the initial workspace.
It also starts glava and Spotify and connects the configured Bluetooth device.
SDDM handles login authentication; hyprd does not lock the session at startup.

## Commands

```bash
hyprd                    # start daemon (foreground)
hyprd init               # import env, start services, run boot sequence
hyprd status             # check if running
hyprd status --json      # full state dump
hyprd rebuild            # rebuild binary and hot-restart (preserves state)
```

### Rebuild

`hyprd rebuild` builds from `~/dotfiles/cmds` or `$DOTFILES/cmds`, installs `~/.local/bin/hyprd`, saves runtime state, and restarts in place.
It uses `internal/gobuild` settings shared with `dctl update cmd` and ISO builds: `-trimpath -buildvcs=false`, `CGO_ENABLED=0`, empty `GOFLAGS`, and `GOWORK=off`.
It refuses during a full lock or an active OpenCode refresh job.
Use a scratch build for build-only checks; `hyprd rebuild` changes the running daemon.

### Window management

```bash
hyprd monocle                # float focused window in place and park tiled siblings
hyprd float                  # toggle floating, centered at monocle size
hyprd split wide             # toggle between the wide and default ratios
hyprd split narrow           # toggle between the narrow and default ratios
hyprd split default          # select the default ratio
hyprd hide                   # move slave to special workspace
hyprd swap                   # exchange master/slave positions
hyprd ws <n>                 # switch workspace with its transition animation
hyprd ws up|down             # move active window between workspaces 1..6, skipping music (6)
hyprd focus <class> [title]  # focus window by class, unhide if needed
hyprd bg ensure|kill         # ensure the wallpaper process or stop it
```

Monocle keeps the focused window on its workspace and parks the other tiled windows on `special:mono<n>`.
Run it again to restore the parked windows.

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
hyprd edit <file>                # focus workspace nvim and open file
hyprd tab <editor|agents>:<0..4> # focus profile window and switch physical tab
hyprd tabs init <profile> <pid>  # launch configured tabs and close the launcher tab
hyprd tabs refresh <name> <pid>  # close and recreate the selected tab
```

**Tab refresh closes the tab and its running processes before creating the replacement.**
Save work before running it.

### Lock

```bash
hyprd lock full        # supervise the Quickshell session lock
```

`hyprd lock full` starts supervision asynchronously for `qs -c lock` with `LOCK_MODE=lock`; returning does not confirm that the session is locked.
The UI lives in `config/quickshell/lock/` at the repo root, and Quickshell's stdout and stderr are forwarded to hyprd's stderr.
Before each new launch, hyprd enters `lockbarrier` and keeps that submap until the launch emits `lock-secure: acquired`; a client that misses the 5-second deadline is stopped and retried.

Failed launches that never acquire the lock or fail in less than 3 seconds count as strikes; an acquired launch that fails after at least 3 seconds resets the count.
Three consecutive strikes trigger an attempt to end the session; if that fails, hyprd reports the error and continues retrying.

After acquisition, normal release requires exit 0 and an unlocked compositor; an `Aborting lock.` line prevents release and triggers a retry.
A watchdog checks for unlock every second and stops a client still alive 2 seconds after unlock is observed, allowing release without exit 0 unless an abort was logged.
There is no manual unlock command.

`$XDG_RUNTIME_DIR/hyprd-lock-$HYPRLAND_INSTANCE_SIGNATURE` preserves lock intent across daemon restarts; recording errors are logged.
Startup resumes pending requests and supervises surviving Quickshell clients marked with `LOCK_MODE=lock`, releasing only when a prior acquisition was recorded or observed and the compositor is unlocked.
An adopted client is stopped if, after 5 seconds, the compositor confirms it is still unlocked without a prior acquisition; failures trigger a relaunch.
Startup also relaunches a lock when the compositor is locked without a client, and refuses to start if there is neither a client nor recorded intent and the lock state cannot be read.
`hyprd rebuild` refuses while supervision is active; a lock requested during its restart handoff is deferred to the restarted daemon.

On the first launch, the desktop cover runs alongside supervision, with 2-second timeouts on cover helper commands.
It saves the workspace, switches to the empty workspace 7, pauses the background, dunst, and Spotify, stops GLava, and closes eww widgets.
Release restores the saved workspace, background, dunst, and widgets, restarts GLava and Spotify, and reconnects configured Bluetooth; playback resumes if music was playing before the lock.

### Browser

```bash
hyprd browser windows [--all]
hyprd browser snapshot <name> [active|largest|index]
hyprd browser show <name>
hyprd browser hypr <name>
hyprd browser restore <name> [--force] [--dry-run]
hyprd browser launch
```

Browser snapshots are the Firefox layout primitive for sessions. The normal flow is:

1. Arrange the Firefox window how you want it.
2. Save it with `hyprd browser snapshot <name>`; use `largest` or a numeric window index if the active window is not the one you want.
3. Reference it in `cmds/config/hyprd.yaml` as `browser: <name>`.
4. Open the session with `hyprd layout <session>` or let `hyprd init` restore init sessions at boot.

`browser: <name>` is shorthand for an exact restore of that snapshot.
Session layouts require an explicit snapshot and exact restore; a URL-only browser map is invalid.

Command meanings:

- `windows` lists Firefox windows from the sessionstore; without `--all`, trivial/new-tab windows are filtered out.
- `snapshot` writes a named snapshot under `browser/sessions/` from the selected Firefox window. Selectors are `active`, `largest`, or a 1-based window index.
- `show` prints the saved snapshot YAML summary.
- `hypr` prints a launch config generated from the snapshot; mostly useful for inspection now that session config can use `browser: <name>`.
- `restore` adds the snapshot as a window in the main Firefox profile. Use `--force` when Firefox is running.
- `launch` clears the profile sessionstore and opens a clean new-tab window; this is used internally by the three-body browser command for non-snapshot launches.

Snapshots taken from the active Firefox window record its home workspace.
`browser restore` claims that window there; edit its `snapshot.yaml` `workspace` value to change the destination.

All snapshots share the main Firefox profile, including cookies, logins, extensions, and shortcuts.
Each snapshot remains isolated as a separate Firefox window with its own pinned tabs, groups, and tabs.
Opening a layout while Firefox runs stops and relaunches Firefox, preserving existing windows and adding the requested snapshot window.
Opening a layout reuses an existing Firefox window when its selected tab title already matches the snapshot.

### Screenshot

```bash
hyprd screenshot              # region screenshot to clipboard (wayfreeze + grim)
hyprd screenshot annotate     # region screenshot → satty annotation → clipboard
```

### Notifications

```bash
hyprd notify hook opencode            # read OpenCode notify JSON from argv/stdin
hyprd notify dunst                    # handle Dunst script callbacks
hyprd notify kitty-finish <command>   # emit kitty command-finish notification
```

### VPN

```bash
hyprd vpn Trend            # toggle configured NetworkManager VPN connection
hyprd vpn Trend up|down    # connect/disconnect explicitly
hyprd vpn Trend status     # status for one connection
hyprd vpn install Trend    # load staged .nmconnection into NetworkManager
hyprd vpn install          # load all configured staged VPN profiles
hyprd vpn install Trend --no-replace
hyprd vpn install Trend --reset-secrets
hyprd vpn export Trend     # export NetworkManager profile to staged file
hyprd vpn status           # active VPN summary
hyprd vpn list             # list NetworkManager VPN connections
```

Configured VPN profile paths live in `cmds/config/hyprd.yaml` under `vpn.connections`.
`hyprd vpn install` loads a complete staged profile into NetworkManager; an incomplete profile requires an existing connection.
It prompts for a missing VPN password or IPsec PSK, stores and verifies the secrets in NetworkManager, then deletes the staged copy after success.

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

`cmds/config/hyprd.yaml` — overrides compiled defaults for:

- `background` — mpvpaper wallpaper
- `init` — network wait and initial workspace
- `notify` — sounds, icons, per-style appearance
- `windows` — ignored classes, hidden/shadow workspace names, split presets, monocle sizing
- `tabs` — kitty tab profiles (editor, agents, leadpier)
- `sessions` — layouts grouped by workspace, then keyed by session name; `init: true` launches on boot (at most one per workspace)

The window roles referenced by session bodies come from the compiled `ThreeBody` map in `cmds/internal/config/hyprd.go`.
