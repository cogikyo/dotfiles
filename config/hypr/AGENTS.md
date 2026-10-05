# hypr

Hyprland config uses Lua `hl.*`, not `hyprland.conf`.
`.luarc.json` points LuaLS at API stubs in `/usr/share/hypr/stubs`.

## Layout

| Path            | Owns                                      |
| --------------- | ----------------------------------------- |
| `hyprland.lua`  | Monitor, rules, autostart, layout options |
| `binds.lua`     | Keybinds and submaps                      |
| `hypridle.conf` | Idle escalation and sleep hooks           |

## Couplings

- Binds call hyprd verbs; keep them aligned with `../../cmds/cmd/hyprd/{main,daemon}.go`.
- Lock supervision belongs to the [hyprd lock controller](../../cmds/internal/hyprd/session/lock.go); these configs control triggers, and the lock UI lives in `../quickshell/lock/`.

Gaps and split ratios are fitted to Kitty's cell grid.
Change `general.gaps_out`, `master.mfact`, and `floatSize` together with `windows.{split,gaps_out,monocle}` in `../../cmds/config/hyprd.yaml` and cell-size settings in `../kitty/kitty.conf`.

## Checking

Inspect loaded-config parsing errors with read-only `hyprctl configerrors`.
