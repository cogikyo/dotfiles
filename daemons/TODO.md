# Daemons Migration TODO

## hyprd — kitty integration (next)

### Editor focus → `hyprd focus` upgrade
- [ ] Add kitty remote control: tab switching via `KITTY_TAB_ID` (kitty @ focus-tab)
- [ ] Add nvim commands: nvimtree toggle/focus (kitty @ send-text)
- [ ] Add toggle-back: if already focused on target tab, focus previous window
- [ ] Binds: update `binds.conf` editor-focus section to use `hyprd focus`
- [ ] Delete `bin/editor-focus`

### Kitty tab management → `hyprd tabs`
- [ ] New command: `hyprd tabs init [session-type]` — create tabs with KITTY_TAB_ID, titles, CWDs
- [ ] Tab definitions (names, icons, commands, CWDs) in `daemons.yaml` under sessions
- [ ] New command: `hyprd tabs refresh [tab|all]` — close + reopen tab in place
- [ ] Kitty session confs become one-liners: `launch zsh -ic "hyprd tabs init; exit"`
- [ ] Kitty keybinds (`ctrl+shift+r>*`) call `hyprd tabs refresh` instead of kitty-refresh
- [ ] Delete `bin/kitty-refresh`

### Notifications → `hyprd notify`
- [ ] Move `claude-notify` dispatch logic (dunstify + paplay) into hyprd
- [ ] hyprd already has workspace state + window PIDs — replaces ws_icon()/tab_icon() lookups
- [ ] Claude hooks call `hyprd notify {start|complete|idle|permission}` via stdin JSON
- [ ] Delete or reduce `config/claude/claude-notify` to a shim that pipes to hyprd
- [ ] `bin/kitty-notify` (10 lines) — absorb or keep, low priority

## hyprd — layout (after kitty integration)

### Absorb remaining `bin/layout`
- [ ] Add sessions for ws2 (slack) and ws3 (tableplus) to config
- [ ] Layout arrange should use three-body, not master/slave
- [ ] Update `binds.conf` to use `hyprd layout` for ws2/ws3
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
