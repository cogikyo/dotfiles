# packages

Hand-curated package lists, local build recipes, and the repository clone catalog.
See the [dctl guide](../cmds/cmd/dctl/README.md) for package setup and ISO builds.

## Inputs

| Path                   | Role                              |
| ---------------------- | --------------------------------- |
| `base.lst`             | Official offline payload packages |
| `amd.lst`, `intel.lst` | Vendor-specific offline packages  |
| `aur.lst`              | AUR offline payload recipes       |
| `extra.lst`            | Online `dctl setup extra`         |
| `cogikyo.lst`          | Cogikyo-only online packages      |
| `repos.lst`            | Repository clone catalog          |
| `brew.lst`             | macOS Homebrew formulae and casks |
| `<pkg>/PKGBUILD`       | Local package recipe              |
| `makepkg.conf`         | Local build sudo overlay          |
| `cursor-sdk-bridge/`   | Bridge pin and patch for dctl     |

- Generate `cursor-sdk-bridge/fast-suffix.patch` with plain `git diff` defaults (3 context lines, myers) so dctl's patch-ID check matches.

## Rules

- Package lists use one package name per line; `#` starts a comment and blank lines are ignored.
- `repos.lst` uses `owner/name path` per line, with a `~/` or absolute destination and the same comment rules.
- `brew.lst` lists canonical Homebrew formula names and `cask:<token>` casks; macOS `dctl setup` installs missing entries and never removes unlisted ones.
- Name local recipe directories after the package; the loader uses directory names as package names.
- Ship keys for `validpgpkeys` under `<pkg>/keys/pgp/*.asc`; ISO builds fail before building if a required fingerprint is absent from the build keyring.
- The package loader requires base, both vendor, AUR, extra, and cogikyo lists; only base, both vendor sets, AUR, and local recipes not in `cogikyo.lst` enter the ISO targets and offline payload.
- Installation, package setup, and reconciliation select vendor lists from `/etc/dctl/machine`; `linux-firmware-intel`, `linux-firmware-mediatek`, and `linux-firmware-realtek` remain in base.
- Put cogikyo-only packages, including local recipes, in `cogikyo.lst`.
  `dctl setup extra`, reconciliation, and local recipe updates select them only with `cogikyo=on` in `/etc/dctl/machine` (absent means off).
- Keep every local recipe in yay's `--ignore`; setup builds local entries with the shared `makepkg.conf` overlay.
- Opt local recipes into `dctl update cmd`'s warn-only upstream release check with `<pkg>/.nvchecker.toml`.
  - Use a section named for `pkgbase`, `source = "git"`, the upstream `git` URL, and the tag `prefix`.
  - `librepods-max2` and `cursor-sdk-bridge` keep unwatched commit pins; the bridge has no PKGBUILD.
- `dctl update packages` reconciles the system with the lists; packages kept from its removal checklist are inserted into `extra.lst` under `# official` or `# aur`.
- A binary at `~/.local/bin/eww` shadows the packaged eww from `eww/`.
