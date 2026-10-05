# Dotfiles

Arch Linux + Hyprland environment, Go commands, and an offline UEFI installer targeting the Framework Desktop (AMD Strix Halo, Ethernet only).
Edits to linked `config/` files change what applications read; see [setup](./README.md) for how changes reach the system.

> [!WARNING]
> Keep this file at the repo root: dctl requires `$DOTFILES/AGENTS.md` to find the checkout.

## Layout

| Path               | Holds                                                        | Reaches the system by           |
| ------------------ | ------------------------------------------------------------ | ------------------------------- |
| `config/`          | Application settings                                         | Home/Firefox setup links        |
| `cmds/`            | [Go commands](cmds/README.md)                                | Installer or targeted builds    |
| `system/`          | Rootfs overlay                                               | `dctl setup system` copies      |
| `packages/`        | Curated lists, PKGBUILDs, repo catalog                       | ISO and dctl setup              |
| `secrets/`         | Age ciphertext and manifest                                  | `dctl secrets decrypt`          |
| `iso/`             | Archiso profile                                              | `sudo dctl iso build`           |
| `share/`           | Fonts and assets                                             | Setup or application references |
| `config/opencode/` | [Agents, skills, plugins](config/opencode/plugins/README.md) | Linked config; plugin restart   |

User units and their `.wants` links live only in `config/systemd/user/`.
Local `AGENTS.md` files cover `cmds/`, `config/hypr/`, `config/eww/`, and `packages/`.

## Commands

| Command  | Role                        | Docs                               |
| -------- | --------------------------- | ---------------------------------- |
| `dctl`   | Install, setup, update, ISO | [Guide](cmds/cmd/dctl/README.md)   |
| `hyprd`  | Hyprland daemon and CLI     | [Guide](cmds/cmd/hyprd/README.md)  |
| `ewwd`   | Widget signals and actions  | [Guide](cmds/cmd/ewwd/README.md)   |
| `newtab` | Firefox new-tab server      | [Guide](cmds/cmd/newtab/README.md) |
| `keys`   | Svalboard keymap viewer     | [Guide](cmds/cmd/keys/README.md)   |
| `src`    | Upstream source cache       | [Source](cmds/internal/src/)       |

Hyprd and ewwd talk over `/tmp/<name>.sock`.

## Where to look

| Task                              | Path                                               |
| --------------------------------- | -------------------------------------------------- |
| Keybinds / monitor / window rules | `config/hypr/{binds,hyprland}.lua`                 |
| Widgets / data                    | `config/eww/`, `cmds/internal/ewwd/providers/`     |
| Session catalog / tab profiles    | `cmds/config/hyprd.yaml`, `config/kitty/sessions/` |

## Terms

- Three-body uses editor/agents/browser roles with a master, active slave, and shadow on `special:shadow`.
- Monocle floats the focused window and parks tiled siblings on `special:mono<n>` until restored.
- Split selects master-ratio presets; share mode changes gaps/ratios and suppresses notifications and widgets.
- Full lock supervises the Quickshell lock with `qs -c lock` and has no manual unlock command.

## Hazards

> [!CAUTION]
> `dctl install` and `dctl iso usb` erase whole disks; `dctl iso release` publishes publicly.
> Use `hyprd rebuild` only for an attended update and never during a full lock.

## Working here

- Run `dctl setup` and `dctl update` as the user; setup batches root stages in one sudo child.
- Use Bash only for shell-shaped helpers with `#!/usr/bin/env bash` and `set -euo pipefail`; quote literal `#`, `^`, and `~` in sourced zsh files with `EXTENDED_GLOB`.
- Run Python one-offs with `uv run --with <package> python <script>`.
- Always include `config/nvim/lua/plugins/editor/harpoon.json` in a commit when it changed, without mention.
