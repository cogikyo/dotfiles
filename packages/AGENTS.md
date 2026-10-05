# packages

Hand-curated package lists, local build recipes, and the repository clone catalog.
See the [dctl guide](../cmds/cmd/dctl/README.md) for package setup and ISO builds.

## Inputs

| Path             | Role                              |
| ---------------- | --------------------------------- |
| `base.lst`       | Official offline payload packages |
| `aur.lst`        | AUR offline payload recipes       |
| `extra.lst`      | Online `dctl setup extra`         |
| `repos.lst`      | Repository clone catalog          |
| `<pkg>/PKGBUILD` | Local offline payload recipe      |

## Rules

- Package lists use one package name per line; `#` starts a comment and blank lines are ignored.
- `repos.lst` uses `owner/name path` per line, with a `~/` or absolute destination and the same comment rules.
- Name local recipe directories after the package; the loader uses directory names as package names.
- Ship keys for `validpgpkeys` under `<pkg>/keys/pgp/*.asc`; ISO builds fail before building if a required fingerprint is absent from the build keyring.
- ISO builds require the base, AUR, and extra lists, but only base, AUR, and local packages enter the offline payload.
- `dctl update packages` reconciles the system with the lists; packages kept from its removal checklist are inserted into `extra.lst` under `# official` or `# aur`.
- A binary at `~/.local/bin/eww` shadows the packaged eww from `eww/`.
