# hypr

Hyprland config uses Lua `hl.*`, not `hyprland.conf`.
`.luarc.json` points LuaLS at API stubs in `/usr/share/hypr/stubs`.

## Layout

| Path            | Owns                                      |
| --------------- | ----------------------------------------- |
| `hyprland.lua`  | Monitor, rules, autostart, layout options |
| `binds.lua`     | Keybinds and submaps                      |
| `hyprlock.conf` | Lock-screen appearance                    |
| `hypridle.conf` | Idle escalation and sleep hooks           |

## Couplings

- Binds call hyprd verbs; keep them aligned with `../../cmds/cmd/hyprd/{main,daemon}.go`.
- Lock/idle behavior belongs to the [hyprd lock controller](../../cmds/internal/hyprd/session/lock.go); these configs control appearance and triggers.

Gaps and split ratios are fitted to Kitty's cell grid.
Change `general.gaps_out` and `master.mfact` together with `windows.{split,gaps_out}` in `../../cmds/config/hyprd.yaml` and cell-size settings in `../kitty/kitty.conf`.

## Checking

Inspect loaded-config parsing errors with read-only `hyprctl configerrors`.
