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
- `binds.lua` prefixes every Corne-block description with `Corne: `; `../../cmds/cmd/keys/hyprland.go` hides binds with that prefix from the Svalboard viewer.
- `hyprd tab` sends Alt+N and Alt+Shift+N; `../../system/usr/lib/firefox-developer-edition/firefox.cfg` replaces Firefox's built-in Alt+1–8 keys to handle them.
- `hyprland.lua` and the lid binds in `binds.lua` assume the internal panel is `eDP-1`.
- Lock supervision belongs to the [hyprd lock controller](../../cmds/internal/hyprd/session/lock.go); these configs control triggers, and the lock UI lives in `../quickshell/lock/`.

Gaps and split ratios are fitted to Kitty's cell grid.
Change `general.gaps_out`, `master.mfact`, and `floatSize` together with the fallback `tv` profile in `../../cmds/config/hyprd.yaml` → `displays` and cell-size settings in `../kitty/kitty.conf`.
hyprd overrides the gaps, mfact, and shadow `range`, `color`, and `color_inactive` with the active profile's values; `floatSize` stays fitted to the `tv` profile on every screen.

## Checking

Inspect loaded-config parsing errors with read-only `hyprctl configerrors`.
