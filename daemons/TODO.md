# Daemons Migration TODO

## hyprd — kitty integration ✓

### Editor focus → `hyprd tab` ✓

- [x] Kitty remote control: tab switching via `KITTY_TAB_ID` (kitty @ focus-tab)
- [x] Nvim commands: nvimtree toggle/focus (kitty @ send-text)
- [x] Toggle-back: if already focused on target tab, focus previous window
- [x] Binds: `binds.conf` updated to use `hyprd tab {term|nvim|nvimtree|git|xplr}`
- [x] `bin/editor-focus` deleted

### Kitty tab management → `hyprd tabs` ✓

- [x] New command: `hyprd tabs init [session-type]` — create tabs with KITTY_TAB_ID, titles, CWDs
- [x] Tab definitions (names, icons, commands, CWDs) in `daemons.yaml` under `hypr.tabs`
- [x] New command: `hyprd tabs refresh [tab|all]` — close + reopen tab in place
- [x] Kitty session confs become one-liners: `launch zsh -ic "hyprd tabs init; exit"`
- [x] Kitty keybinds (`ctrl+shift+r>*`) call `hyprd tabs refresh` instead of kitty-refresh
- [x] Delete `bin/kitty-refresh`

### Notifications → `hyprd notify` ✓

- [x] Scope decided: `hyprd` becomes the repo-owned notification control plane; Dunst stays the renderer
- [x] Current repo-owned entry points absorbed: `claude-notify`, `codex-notify`, Dunst script hooks, `kitty-notify`, `alert`
- [x] Treat `hyprd notify` as a Dunst wrapper with Hypr/kitty-aware routing, not just an agent notifier
- [x] Add notification config to `daemons.yaml`: sounds dir, ws icon mapping, app/event policies, color/style presets, focus rules
- [x] Add `hyprd notify` command that accepts stdin JSON for hook payloads and explicit subcommands for Dunst script / simple callers
- [x] Normalize incoming events into one internal model (`source`, `event`, `app`, `summary`, `body`, `urgency`, `persistent`, `sound`, `focus target`)
- [x] Resolve useful context in Go: active workspace, kitty window/tab, Hypr PID focus target, ws icon asset, current project path
- [x] Route all repo-owned notifications through `hyprd notify`, then emit through Dunst + sound playback
- [x] Move `bin/alert` sound policy into `hyprd notify` so Dunst script behavior is daemon-owned instead of shell-owned
- [x] Replace Dunst global `script=` / urgency hooks with `hyprd notify` entry points instead of shell scripts
- [x] Replace Dunst kitty approval interception with `hyprd notify`, not `codex-notify`
- [x] Replace Claude/Codex hooks to call `hyprd notify` directly
- [x] Replace kitty command-finish notifications to call `hyprd notify` directly
- [x] Delete `config/claude/claude-notify`, `config/codex/codex-notify`, and `bin/kitty-notify` after cutover; temporary shims only if needed for migration safety

### Startup → `hyprd init` ✓

- [x] `bin/hyprstart` deleted — `hyprd` auto-runs init on fresh session (no windows detected)
- [x] Init sequence in `commands/init.go`: wallpaper, network wait, layout sessions, apps, lock
- [x] `hyprland.conf` uses `exec-once = hyprd` instead of `exec-once = hyprstart`

### Deleted scripts (no dangling refs) ✓

- [x] `bin/active-cwd` — replaced by hyprd state
- [x] `bin/alert` — absorbed into `hyprd notify`
- [x] `bin/eww-open` — no longer needed
- [x] `bin/kitty-notify` — absorbed into `hyprd notify`
- [x] `config/claude/claude-notify` — hooks call `hyprd notify` directly
- [x] `config/codex/codex-notify` — hooks call `hyprd notify` directly
- [x] `config/eww/bin/workspaces` — hyprd subscription covers it

## hyprd — remaining work

### Dunstify caller audit

- [x] `bin/layout` — deleted, layout absorbed into `hyprd layout`
- [x] `bin/lock` — migrated to `hyprd notify send`
- [x] `bin/record` — migrated to `hyprd notify send`
- [x] `bin/screenshot` — migrated to `hyprd notify send`
- [x] `daemons/hyprd/commands/init.go` — uses internal notify callback via daemon
- [x] `config/nvim/lua/config/autocmds.lua` (1 call) — fine as-is, one-off dunst restart

### Bugs fixed ✓

- [x] **Nil deref in kitty Env access** — guarded `w.Env` with nil check in `kitty.go`
- [x] **Race in config reload** — `daemon.go` uses `atomic.Pointer[config.HyprConfig]`
- [x] **Persistent notification infinite loop** — `notify.go` capped at 600 retries (~10 min)
- [x] **Silent error drops** — `runCmd()` logs errors to stderr
- [x] **Zombie process risk** — replaced `Process.Release()` with `go cmd.Wait()`
- [x] **Dangling `resize-slave` call** — `bin/layout` deleted

### Simplification done ✓

- [x] Consolidated cmd handlers in `main.go` via `sendCommand()` + `requireArg()`
- [x] Extracted `buildDunstArgs()` helper from `sendDunst()`
- [x] Normalized sound/icon map keys to lowercase at config load
- [x] Replaced `stringPtr`/`intPtr`/`boolPtr` with generic `ptr[T]`
- [x] Go 1.22+ modernizations: `slices.Sorted(maps.Keys(...))`, `range N`, `SplitSeq`

## hyprd — layout ✓

### Absorb `bin/layout` ✓

- [x] Added `slack` (ws2), `tableplus` (ws3), `leadpier` (ws4) sessions to `daemons.yaml`
- [x] Updated Go defaults in `config.go` to match
- [x] `layout.go` skips arrangement for simple single-window sessions
- [x] `binds.conf` updated: all workspaces now use `hyprd layout`
- [x] `bin/layout` deleted — no dangling references remain

### Redesign session config around three-body ✓

- [x] Replace the current master/slave-oriented session shape with a three-body-oriented session shape
- [x] Sessions reference reusable launch targets (`body: [editor, browser, agents]`) from `three_body` config
- [x] Add per-session selection of kitty tab profiles (`tabs:` map on sessions)
- [x] Keep project path ownership in `hyprd` so kitty sessions and three-body launches resolve from the same source
- [x] Preserve hot-reload ergonomics so session/layout tuning stays easy via `daemons.yaml`
- [x] Simple sessions (slack, tableplus, etc.) use `command:` field, no three-body

### Move `hyprd layout` onto three-body ✓

- [x] Stop arranging sessions by swapping `terminal` / `firefox` / `claude` in master/slave order
- [x] Spawn/focus three-body windows from config and let auto-enrollment handle arrangement
- [x] Layout startup idempotent: no-ops when workspace already has windows
- [x] Removed `arrangeLayout()` and `WindowConfig` type entirely
- [x] First body member focused as master after spawn

### Firefox session orchestration (partial)

- [x] Config models pinned tabs, tab groups, and URLs per session (`BrowserConfig`)
- [x] Current implementation flattens all URLs into sequential `firefox` opens
- [ ] No CLI flag for pinned tabs — WebExtension needed for pinning and grouping
- [ ] Future: build lightweight WebExtension that consumes `BrowserConfig` via native messaging
- [ ] Distinguish one-time boot tabs from persistent session state so layout open does not clobber important browsing context

### Done

- [x] `ws-music` removed — `hyprd ws 1` handles it now
- [x] `focus-active` removed — `hyprd focus` replaced it
- [x] `resize-slave` removed — three-body replaces master/slave resize
- [x] `hyprstart` removed — `hyprd init` auto-detects fresh session
- [x] All workspaces (2-5) migrated to `hyprd layout`
- [x] `bin/layout` deleted

## ewwd — after hyprd migration settles

### Markup cleanup

- [x] Split live `eww.yuck` into includes under `config/eww/yuck/`
- [x] Archive retired widgets/windows in `config/eww/archive/legacy-widgets.yuck`
- [x] Move widget-ready derived state into daemons where it clearly belongs:
  `hyprd workspace.current_str/occupied_str/monocle_str`,
  `ewwd network.link_ramp`,
  `ewwd audio.sink_bluetooth`,
  `ewwd music.playing/volume_percent/*_short/single_track`

### Wire eww widgets to ewwd

- [ ] Replace `bin/brightness` calls in `eww.yuck` with `ewwd action brightness`
- [ ] Replace `bin/pulse_info` calls with `ewwd action audio`
- [ ] Replace `bin/music_info` calls with `ewwd action music`
- [ ] Replace `bin/timer_info` calls with `ewwd action timer`
- [ ] Replace `bin/network_info` poll with `ewwd subscribe network`
- [ ] Replace `bin/weather_info` poll with `ewwd subscribe weather`
- [ ] Replace `bin/date_info` poll with `ewwd subscribe date`
- [ ] Replace `bin/computer_info` poll with `ewwd subscribe gpu`

### Cleanup after widget rewire

- [ ] Delete `config/eww/bin/` scripts (brightness, pulse_info, music_info, etc.)
- [ ] Remove `config/eww/bin/create_temp_files` if daemons handle tmp files
- [ ] Remove `config/eww/bin/ping` if not used
