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
- `dctl porkbun ...` checks and manages personal Porkbun DNS records on Linux.

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
- `certs` trusts the mkcert CA and provisions a shared certificate for localhost, `local.leadpier.com`, and `local.cullyn.dev`.
- `shell` switches the login shell to zsh.
- `dns` configures systemd-resolved and NetworkManager DNS.

Most install steps support `--dry-run`.
Dry-run prints planned changes and must not write files or run package/install side effects.
`secrets` and `repos` use their own commands for safe previews and do not advertise install-step dry-run support.

`system`, `hibernate`, `certs`, and `dns` are root-affecting operations.
They require `--yes` or `--dry-run`.
That prevents non-interactive invocations from silently mutating system config, boot state, swap, trust stores, or DNS.

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

## Porkbun DNS

`dctl porkbun` manages one explicit domain at a time through the [Porkbun v3 API](https://porkbun.com/llms/dns).
It is Linux-only and separate from `dctl install dns`, which configures the machine's resolver.

### Provision credentials first

The Linux credential file is not provisioned by this implementation; the secrets manifest contains metadata only.
Create API keys in the [Porkbun account dashboard](https://porkbun.com/account/api), enable API access for the intended domain, and apply appropriate key restrictions.

Use a local editor to create `~/.local/share/dotfiles/porkbun.env` as a regular, non-symlink file owned by your normal Linux user with mode `0600`.
Its only contents must be two unquoted `KEY=value` lines, one for `PORKBUN_API_KEY` and one for `PORKBUN_API_SECRET_KEY`.
Use the values issued by Porkbun, with no spaces, comments, blank lines, `export`, or shell expressions.
An ending newline is allowed.
Set restrictive permissions before entering the keys and avoid editor backups or swap files that expose them.
Do not put key values in shell commands, command-line arguments, or shell history.

The command reads this already-provisioned file without evaluating shell code or prompting for decryption.
It refuses root/sudo use, unsafe files, and malformed credentials; it has no environment, Caddy, root-file, or other credential fallback.
This personal file does not replace Caddy's credential copy used for certificate renewal.

Optional encrypted synchronization uses the existing age workflow:

```sh
dctl secrets trust
dctl secrets sync
```

Sync encrypts the provisioned file using the configured age recipient.
On another Linux machine with the encrypted file and configured age identity, run `dctl secrets trust` and then `dctl secrets decrypt`.
These commands operate on the whole manifest, not only Porkbun; manifest approval and decryption passphrases belong to the secrets workflow.
No encrypted Porkbun file, identity, or recipient is created by adding the manifest entry.

### Commands and names

```sh
dctl porkbun check <domain>
dctl porkbun list <domain>
dctl porkbun create <domain> <name> <type> <content> [--ttl N] [--prio N] [--dry-run]
dctl porkbun edit <domain> <id> --content VALUE [--ttl N] [--prio N] [--dry-run]
dctl porkbun delete <domain> <id> [--dry-run]
```

Domains must be lowercase ASCII, without a URL scheme, path, or trailing dot; use punycode for internationalized domains.
For create, `@` or an empty name means the domain root.
Other names are relative lowercase subdomains such as `www`, `_acme-challenge`, or `_sip._tcp`; do not append the domain.
Quote wildcard names such as `'*'` so the shell does not expand them.
List returns fully qualified names, numeric record IDs, types, content, TTLs, and MX/SRV priorities.
Always take edit/delete IDs from the list for the explicit domain.
IDs must be positive decimal numbers without signs, leading zeros, or path characters.

Writes support A, AAAA, CNAME, TXT, MX, and SRV only.
Porkbun validates content and account limits; SRV content uses the provider's weight/port/target format, with priority in `--prio`.
Omitted create TTL, or `--ttl 0`, uses Porkbun's account minimum rather than a CLI-defined TTL.
MX/SRV priority defaults to zero on create.
Edit preserves the name, type, notes, and omitted TTL/priority fields.
There is no rename, upsert, bulk operation, delete-by-name, administrative DNS, or registration command.

Create allows multiple records with the same name/type when their content or priority differs.
The same name/type/content/priority is rejected as a duplicate even if TTL differs; no existing record is silently overwritten.
Edit also refuses to duplicate another record's content/priority, and an already-matching state produces an explicit unchanged result without a mutation.

### Consent, authority, and verification

Every write shows current and proposed state and requires a default-no TTY confirmation, unless global `--yes` is supplied.
`--json`, non-TTY, and `--defaults` writes require `--yes`; `--defaults` never grants consent.
`--plain` changes display formatting and still permits confirmation on a TTY.
JSON output contains one result with the preview, evidence, and warnings, without prompt or progress chatter; command errors also use dctl's stderr error output.

`--dry-run` needs no consent.
Create dry-run sends the documented `dryRun=true` request and requires `wouldSucceed=true` without a record ID.
Edit/delete dry-runs read current API state and preview locally; they never call the mutation endpoint or prove mutation permission.
`check` separately reports authentication, DNS readability, authority evidence, and a DNS-create dry-run for a probe TXT record named `_dctl-check`.
It does not claim that live create, edit, or delete has been tested.

Preflight requires matching `/domain/get/<domain>` metadata with documented `notLocal=0`, plus DNS responses without warnings.
Missing or unexpected authority evidence blocks writes.
Any provider warning blocks writes, including warnings that Porkbun holds an inactive copy after a move to the customer's own Cloudflare account.
The API's `cloudflare` proxy field and the dashboard's “DNS Powered by Cloudflare” label are not used as migration evidence.
No provider switch or independent DNS-resolution reconciliation is attempted.

After consent, the command refreshes API authority and records and refuses a changed target or a new duplicate before sending one mutation.
It then fetches records to verify the intended state or absence by ID, including preserved fields on edit.
Porkbun offers no atomic compare-and-swap here, so a concurrent change can still occur between requests.
Readback compares returned content exactly; provider normalization can produce an uncertain result that needs inspection.
A failed mutation response or failed/mismatched readback returns nonzero with identifying information and an uncertain outcome; inspect `list` before any manual retry.
The command never retries a mutation or rolls it back automatically, and API readback does not prove public DNS propagation.

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
