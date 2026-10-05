# keys

Read-only Svalboard keymap viewer with app bind overlays.

`keys` is a Go HTTP server at `127.0.0.1:42070` that serves the page from `cmds/cmd/keys/`.

## Inputs

- Board definition: `share/keyboards/svalboard.json`, the Vial definition with matrix positions and custom keycodes.
- Keymap: `share/keyboards/svalboard.vil`, the Vial export with layers, layout options, and tap dances.
- Hyprland: live compositor binds from IPC, excluding submaps.
- kitty: its config loader, including defaults, `kitty.conf`, and included files.
- Neovim: headless Neovim loading the user config; global normal-mode maps only, without buffer-local maps.
- OpenCode: defaults and scopes from the installed binary, with overrides from the user config's `opencode/tui.json`.

## Open

Press `Super+M` to toggle the floating Chromium app, or open `http://127.0.0.1:42070/` in a browser.
The shortcut is in `config/hypr/binds.lua`; the `keymap` window rule is in `config/hypr/hyprland.lua`.
The user unit is `config/systemd/user/keys.service`.

## Controls

Select an app, a layer, and a modifier combination with the buttons.
The **Keys** view shows the keymap without app overlays; **scoped** adds context-specific app binds when available.

- `Tab` / `Shift+Tab`: cycle apps forward / backward.
- `1`–`9`: select layers by their visible order, not their layer number.
- `[` / `]`: cycle modifier combinations backward / forward in an app view.
- Hold modifiers: preview that combination if it is available; release them to restore the selection.
- Click a prefix key marked `›`, or type its unmodified key, to view the next keys in a sequence.
- `Esc` / `Backspace`: go up one prefix level; the breadcrumb buttons return to an earlier prefix or the root.

## Update the keymap and names

After changing the keymap in Vial, re-export it over `share/keyboards/svalboard.vil`.
The server reads the definition and export on each state request, and the page polls for changes without a restart or reload.
App extraction is cached until watched config files or binaries change; Hyprland binds are read on each request.

`cmds/cmd/keys/names.js` holds curated bind names under each app ID, keyed by the original extracted label.
Each entry's `name` supplies the short label, and its optional `about` supplies hover text.
Reload the page after changing this file.

## Rebuild and restart

Run `dctl update --only keys` as your normal user for an attended rebuild.
This builds from the working tree, installs `~/.local/bin/keys`, and restarts `keys.service` only when the binary is replaced.
To restart it separately, run `systemctl --user restart keys.service`.
