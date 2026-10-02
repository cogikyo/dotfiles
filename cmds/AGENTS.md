# cmds

Go module `dotfiles/cmds`; see `go.mod` for the language version and the [workspace guide](./README.md) for architecture.
Command details: [dctl](cmd/dctl/README.md), [hyprd](cmd/hyprd/README.md#rebuild), [ewwd](cmd/ewwd/README.md), and [newtab](cmd/newtab/README.md).

## Layout

| Path        | Holds                               |
| ----------- | ----------------------------------- |
| `cmd/`      | Binary entrypoints                  |
| `internal/` | Shared and command-private packages |
| `config/`   | Runtime YAML                        |

`internal/config/` loads daemon YAML from `~/dotfiles/cmds/config/`, independently of dctl's `DOTFILES` override.

## Working here

- Run `gofmt`/`goimports`, `go fix`, `go vet`, and targeted `go test` from `cmds/` after non-trivial edits.
- Build only affected binaries into `/tmp/opencode/bin/`, without installing or restarting daemons.
- Prefer modern standard-library APIs such as `log/slog`, `errors`, `slices`, `maps`, and `iter` before custom helpers or dependencies.

## Couplings

- Keep hyprd verbs aligned across `cmd/hyprd/{main,daemon}.go` and `../config/hypr/binds.lua`.
- Names in `config/ewwd.yaml` → `windows` must match `defwindow` names under `../config/eww/yuck/`.
- `internal/dctl/binaries/` → `Names` drives ISO builds, installer placement, and doctor checks.
- Register new doctor groups in `internal/dctl/cli/doctor.go`.
- Keep `internal/config/hyprd.go` → `ThreeBody` launch titles/session paths aligned with `../config/kitty/sessions/`.
- Keep `config/hyprd.yaml` → `windows.{split,gaps_out}` fitted to Hyprland's gaps/ratios and Kitty's cell size.
- `hyprd browser snapshot` writes tracked files under `internal/hyprd/browser/sessions/`.
