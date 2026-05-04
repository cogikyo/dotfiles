# dctl

`dctl` is the dotfiles control plane.

It replaces shell-shaped install/update helpers with typed Go commands, structured output, and explicit safety gates.

## Usage

```sh
dctl [--json|--plain] [--yes] [--defaults] <command> [args]
```

Global flags:

- `--json` emits one JSON document for commands that return structured data.
- `--plain` disables colors and terminal affordances.
- `--yes` accepts safe confirmations and required risk acknowledgements.
- `--defaults` avoids interactive prompts where a command has a default path.

Bare `dctl` opens an interactive command picker when stdin and stdout are real TTYs.

Use `dctl --help` or `dctl <command> --help` for the full Kong-generated command tree.

Running bare `dctl` in an interactive terminal opens a command picker.
Piped, scripted, `--plain`, `--json`, and explicit command invocations stay deterministic.

Non-TTY invocations, `--json`, `--plain`, `--defaults`, and explicit commands keep the normal parser path.

## Command Groups

Actions:

- `dctl check` runs install healthchecks.

Lifecycle:

- `dctl update run` updates packages, removes non-optional orphans, and saves package lists.
- `dctl update install` installs repo and AUR packages from saved lists.
- `dctl update check` reports replaceable `-git` packages.
- `dctl secrets ...` manages age-encrypted secrets.
- `dctl install ...` runs dotfiles install steps.
- `dctl repos ...` clones and fast-forwards configured repositories.
- `dctl iso ...` builds, verifies, writes, and releases custom Arch ISOs.

## Install Steps

`dctl install` opens the same interactive command picker as bare `dctl` when stdin/stdout are TTYs.
Choose an install subcommand such as `link`, `go`, `all`, `list`, or `check`.

`dctl install all` runs each step in order without the picker.

Individual steps:

- `packages` installs saved package lists.
- `link` symlinks repo-managed config and scripts into `$HOME`.
- `secrets` decrypts age secrets.
- `repos` clones configured repositories.
- `system` copies system configs and enables services.
- `hibernate` configures Btrfs swap and resume.
- `fonts` installs bundled fonts.
- `go` builds configured Go binaries and user services.
- `eww` builds the eww widget binary.
- `firefox` links Firefox profile CSS and `user.js`.
- `shell` switches the login shell to zsh.
- `dns` configures systemd-resolved and NetworkManager DNS.

Most install steps support `--dry-run`.
Dry-run prints planned changes and must not write files or run package/install side effects.
`secrets` and `repos` use their own commands for safe previews and do not advertise install-step dry-run support.

`system`, `hibernate`, and `dns` are root-affecting operations.
They require `--yes` or `--dry-run`.
That prevents non-interactive invocations from silently mutating `/etc`, boot config, swap, or DNS.

## Output

Default output is human-oriented and may use color.

`--plain` keeps the same human content without color or richer terminal formatting.

`--json` suppresses progress chatter.
It prints only command result objects where the command has structured output.
Errors are emitted to stderr as JSON messages when possible.

Data-listing commands use the structured printer.
Examples include `install list`, `install check`, and `secrets list`.

## Secrets

Secrets live under `etc/secrets`.

Manifest entries use this format:

```text
name:~/target/path:0600
```

Contracts:

- Secret names may contain letters, digits, `.`, `_`, and `-`.
- Targets must resolve inside `$HOME`.
- Existing target parents must also resolve inside `$HOME`.
- Decrypt writes atomically and backs up changed existing files as `.bak.<timestamp>`.
- `dctl secrets decrypt --dry-run` verifies decryptability without writing target files.

New or changed manifest entries must be trusted per machine before sync or decrypt.
Use `dctl secrets trust` to approve the current manifest without decrypting.

## Repos

`dctl repos sync` reads `etc/repos.toml`, creates standard user directories, and clones missing repos.
It also switches the dotfiles remote from HTTPS to SSH when appropriate and adds GitHub to `known_hosts`.

`dctl repos update` only fast-forwards cloned repos with configured upstreams.
Dirty or detached repos are reported and skipped instead of merged manually.

## ISO

ISO commands are intentionally sharp tools.

- `dctl iso verify` checks build inputs and cached outputs.
- `dctl iso build` must run as root and calls `mkarchiso`.
- `dctl iso usb /dev/sdX` erases the whole USB disk after validation and confirmation.
- `dctl iso release` tags, pushes `master`, pushes the tag, and creates a GitHub release.

Safety boundaries:

- USB targets must be whole `/dev/*` disks with USB transport and no mounted partitions.
- Releases require `master`, a clean worktree, an unused tag, and authenticated `gh`.
- `--yes` skips release and USB confirmations but not validation.

## Environment

- `DOTFILES` overrides the dotfiles root.
- `XDG_STATE_HOME` controls state storage; dctl uses `$XDG_STATE_HOME/dotfiles`.
- `NO_COLOR` forces plain output.
- `DOTFILES_INSTALL_NONINTERACTIVE=1` makes package install avoid prompts.
