# Dotfiles

Arch Linux + Hyprland environment, Go commands, and an offline UEFI installer with AMD/Intel machine profiles and a Framework Desktop preset.
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
| `config/opencode/` | [Agents, skills, plugins](config/opencode/plugins/README.md) | Linked config                   |

User units and their `.wants` links live in `config/systemd/user/`; machine-specific CPU-lane drop-ins are rendered into `/etc/systemd/user/` by `dctl setup system`.
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
| `cage`   | Read-only command sandbox   | [Guide](cmds/cmd/cage/README.md)   |

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
> Never run `hyprd rebuild` during a full lock.

## Working here

- Run `dctl setup` and `dctl update` as the user; setup checks non-lock root stages as the user before batching needed root work through sudo.
- When a change adds something that needs updates, make sure `dctl update` covers it; extend dctl if needed.
- Install and restart routine changes yourself once they build and pass checks; the user gives standing approval for these.
  - Routine: `hyprd rebuild`, `dctl update --dirty --only <cmd>`, and user stages such as `dctl --yes setup home`.
  - Ask first for package upgrades, root or lock stages, secrets, and startup, lock, or boot changes that could leave the session unusable.
  - Use `dctl update --dirty` to build uncommitted `cmds/` without asking; it overrides the `--all`/`--yes` build skip.
- Use Bash only for shell-shaped helpers with `#!/usr/bin/env bash` and `set -euo pipefail`; quote literal `#`, `^`, and `~` in sourced zsh files with `EXTENDED_GLOB`.
- Run Python one-offs with `uv run --with <package> python <script>`.
- OpenCode applies saved plugin edits, new plugin directories, and `cli.json` live; `/reload` reloads `opencode.json`.
  - Claim a restart is needed only after a reload failed, and quote the log line.
  - Judge what a plugin can do from the installed host source in the `src` cache, not from `@opencode/plugin` types.
- Commit `config/opencode/cli.json` setting toggles, such as thinking or verbosity, without asking; fold them into a nearby opencode commit or commit them alone.
