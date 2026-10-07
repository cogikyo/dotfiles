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
- Build only affected binaries into `/tmp/opencode/bin/`, without installing or restarting daemons.
- Prefer modern standard-library APIs such as `log/slog`, `errors`, `slices`, `maps`, and `iter` before custom helpers or dependencies.

## Couplings

- Keep hyprd verbs aligned across `cmd/hyprd/{main,daemon}.go` and `../config/hypr/binds.lua`.
- Names in `config/ewwd.yaml` → `windows` must match `defwindow` names under `../config/eww/yuck/`.
- `internal/dctl/binaries/` → `Names` drives ISO builds, installer placement, and CLI updates.
- Register setup stages in execution order in `internal/dctl/cli/setup.go`; consecutive root stages share one sudo child, and background work must finish before later root work or stages that need sudo.
- `internal/gobuild/` owns build flags and environment for CLI updates, hyprd rebuilds, and ISO builds.
- Keep `internal/config/hyprd.go` → `ThreeBody` launch titles/session paths aligned with `../config/kitty/sessions/`.
- Keep `config/hyprd.yaml` → `windows.{split,gaps_out,monocle}` fitted to Hyprland's gaps/ratios and Kitty's cell size.
- `hyprd browser snapshot` writes tracked files under `internal/hyprd/browser/sessions/`.
- `internal/ui` marks an open output tree with `DOTFILES_TREE`; dotfiles commands started under it continue that tree, and `dctl setup` passes it through sudo.
- `dctl update cmd` parses raw `hyprd rebuild` output, so hyprd socket verbs stay plain text; only `hyprd vpn` and `hyprd opencode now|recycle` render trees.
