# Daemons Migration TODO

## hyprd — kitty integration (next)

### Editor focus → `hyprd tab` command

- [x] Kitty remote control: tab switching via `KITTY_TAB_ID` (kitty @ focus-tab)
- [x] Nvim commands: nvimtree toggle/focus (kitty @ send-text)
- [x] Toggle-back: if already focused on target tab, focus previous window
- [x] Binds: `binds.conf` updated to use `hyprd tab {term|nvim|nvimtree|git|xplr}`
- [x] `bin/editor-focus` deleted

### Kitty tab management → `hyprd tabs`

- [x] New command: `hyprd tabs init [session-type]` — create tabs with KITTY_TAB_ID, titles, CWDs
- [x] Tab definitions (names, icons, commands, CWDs) in `daemons.yaml` under `hypr.tabs`
- [x] New command: `hyprd tabs refresh [tab|all]` — close + reopen tab in place
- [x] Kitty session confs become one-liners: `launch zsh -ic "hyprd tabs init; exit"`
- [x] Kitty keybinds (`ctrl+shift+r>*`) call `hyprd tabs refresh` instead of kitty-refresh
- [x] Delete `bin/kitty-refresh`

### Notifications → `hyprd notify`

- [ ] Decide scope: agent/kitty notifications only, or all desktop notification routing
- [ ] Current sources: `claude-notify`, `codex-notify`, Dunst approval script hook, `kitty-notify`, `alert`
- [ ] `hyprd` already has the useful context: workspace state, Hypr window PIDs, kitty helpers, and basic Dunst action integration
- [ ] Add `hyprd notify` command that accepts stdin JSON and routes events through Dunst + sound playback
- [ ] First migration target: Claude/Codex hooks call `hyprd notify ...` instead of owning notification logic in shell
- [ ] Dunst kitty approval interception should forward into `hyprd notify`, not duplicate logic in shell
- [ ] Reduce `config/claude/claude-notify` and `config/codex/codex-notify` to thin shims or delete them
- [ ] Decide whether `bin/kitty-notify` gets absorbed now or left as a trivial caller
- [ ] If `hyprd` is becoming a true Dunst wrapper, fold `bin/alert` policy into the same system; otherwise leave general app notifications alone
- [ ] After the move, audit remaining `dunstify` callers in `bin/` and only migrate the ones that benefit from shared routing/state

## hyprd — layout (after kitty integration)

### Absorb remaining `bin/layout`

- [ ] Make `daemons.yaml` the single source of truth for layout sessions
- [ ] Move remaining `WS` + `APP` data from `bin/layout` into config
- [ ] Add sessions for ws2 (slack) and ws3 (tableplus) to config
- [ ] Reconcile existing drift between binds/startup and configured sessions (`leadpier`, `cogikyo`, old `layout -n`)

### Redesign session config around three-body

- [ ] Replace the current master/slave-oriented session shape with a three-body-oriented session shape
- [ ] Sessions should reference reusable launch targets instead of embedding `terminal` / `firefox` / `claude` assumptions
- [ ] Add per-session selection of kitty tab profiles for editor + agents windows
- [ ] Keep project path ownership in `hyprd` so kitty sessions and three-body launches resolve from the same source
- [ ] Preserve hot-reload ergonomics so session/layout tuning stays easy via `daemons.yaml`

### Move `hyprd layout` onto three-body

- [ ] Stop arranging sessions by swapping `terminal` / `firefox` / `claude` in master/slave order
- [ ] Spawn/focus the three-body windows (`editor`, `browser`, `agents`) from config and let `hyprd three-body` enroll them
- [ ] Make layout startup idempotent enough to no-op or focus when the workspace is already populated
- [ ] Decide whether layout should explicitly set the active slave on first open (`browser` vs `agents`)
- [ ] Update `hyprstart` to call `hyprd layout ...` instead of `layout -n ...`

### Firefox session orchestration

- [ ] Decide the browser model first: declarative layout reconcile vs restore an existing Firefox session
- [ ] Extend config so browser layouts can describe pinned tabs, tab groups, and URLs per group
- [ ] Decide how Firefox state is applied: WebExtension/API control preferred, direct session file mutation only if necessary
- [ ] Distinguish one-time boot tabs from persistent session state so layout open does not clobber important browsing context
- [ ] Make browser sessions first-class per layout, not just a flat `urls:` list
- [ ] Revisit Firefox session prefs once the model is chosen

### Cutover

- [ ] Update `binds.conf` to use `hyprd layout` for ws2/ws3
- [ ] Remove the last `layout -n` callers from startup/binds
- [ ] Delete `bin/layout`

### Done

- [x] `ws-music` removed — `hyprd ws 1` handles it now
- [x] `focus-active` removed — `hyprd focus` replaced it
- [x] `resize-slave` removed — three-body replaces master/slave resize

## ewwd — after hyprd migration settles

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
- [ ] Remove `config/eww/bin/workspaces` if hyprd subscription covers it
- [ ] Remove `config/eww/bin/create_temp_files` if daemons handle tmp files
- [ ] Remove `config/eww/bin/ping` if not used
