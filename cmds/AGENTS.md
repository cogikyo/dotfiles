# cmds

Go module `dotfiles/cmds`; see `go.mod` for the language version and the [workspace guide](./README.md) for architecture.
Command details: [dctl](cmd/dctl/README.md), [hyprd](cmd/hyprd/README.md#rebuild), [ewwd](cmd/ewwd/README.md), [newtab](cmd/newtab/README.md), and [keys](cmd/keys/README.md).

## Layout

| Path        | Holds                               |
| ----------- | ----------------------------------- |
| `cmd/`      | Binary entrypoints                  |
| `internal/` | Shared and command-private packages |
| `config/`   | Runtime YAML                        |

`internal/config/` loads daemon YAML from `~/dotfiles/cmds/config/`, independently of dctl's `DOTFILES` override.

## Working here

- Run `gofmt`/`goimports`, `go fix`, `go vet`, and targeted `go test` from `cmds/` after non-trivial edits.
- Verify with a scratch build into `/tmp/opencode/bin/`, then install per the root [Working here](../AGENTS.md#working-here) rules; never copy a `/tmp` binary into place.
- Prefer modern standard-library APIs such as `log/slog`, `errors`, `slices`, `maps`, and `iter` before custom helpers or dependencies.

## Couplings

- Keep hyprd verbs aligned across `cmd/hyprd/{main,daemon}.go` and `../config/hypr/binds.lua`.
- Keep `../system/etc/greetd/config.toml` → `--locked-cmd "hyprd lock client"` aligned with the local `hyprd lock client` verb in `cmd/hyprd/main.go`.
- Names in `config/ewwd.yaml` → `windows` must match `defwindow` names under `../config/eww/yuck/`.
- `internal/dctl/binaries/` → `Names` drives ISO builds, installer placement, and CLI updates.
- Register setup stages in execution order in `internal/dctl/cli/setup.go`; consecutive root stages share one sudo child, and background work must finish before later root work or stages that need sudo.
- `luks.Needs` is computed from the non-optional phase-1 catalog; keep lock stages after phase 1 in `internal/dctl/cli/setup.go`.
- `internal/machine` writes `/etc/dctl/machine` as `key=value` data with required keys `preset`, `cpu`, `gpus`, `form`, `backlight`, `wifi`, `wlan`, `yubikeys`, and `lane`; `../config/hypr/machine.lua` consumes only `lane` and `backlight`.
  - Keep `internal/machine` standard-library-only; prompts belong to `internal/dctl/setup.AskMachine`.
  - The hostname is an installer answer written to `/etc/hostname`; it is not a record key.
- `internal/dctl/packages/targets.go` owns the ISO groups under `/opt/dctl/targets/`: `common` (base, AUR, local), `amd`, and `intel`.
- `internal/machine.Profile.Files` owns rendered CPU-affinity and Wi-Fi blacklist paths; never duplicate them in `../system/` or put static lane drop-ins in `../config/systemd/user/`.
- `luks` uses the record's `yubikeys` count (0–2) and always requires confirmed paper recovery; zero keys must still permit Secure Boot and boot-TOTP enrollment.
- `../system/etc/initcpio/install/sd-totp` and `internal/dctl/secureboot/totp.go` share the boot-TOTP marker path and tpm2-totp error strings; keep them aligned.
- `internal/gobuild/` owns build flags and environment for CLI updates, hyprd rebuilds, and ISO builds.
- Keep `internal/config/hyprd.go` → `ThreeBody` launch titles/session paths aligned with `../config/kitty/sessions/`.
- Keep `config/hyprd.yaml` → `windows.{split,gaps_out,monocle}` fitted to Hyprland's gaps/ratios and Kitty's cell size.
- `hyprd browser snapshot` writes tracked files under `internal/hyprd/browser/sessions/`.
- `internal/ui` sets `DOTFILES_TREE=open` to continue a parent's tree or `closed` to start separate sections after a blank line; `dctl setup` passes it through sudo.
- `dctl update cmd` parses raw `hyprd rebuild` output, so hyprd socket verbs stay plain text; only `hyprd vpn` and `hyprd opencode now|recycle` render trees.
- Keep cage's CLI aligned with the call in `../config/opencode/plugins/guard/server.ts` and the `cage` wrapper spec in `../config/opencode/plugins/guard/shell.ts`.
