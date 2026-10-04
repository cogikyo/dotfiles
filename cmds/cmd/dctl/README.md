# dctl

`dctl` builds the offline UEFI ISO, installs the machine, applies setup stages, and runs stepped updates.
The target is a single-user Framework Desktop with AMD Strix Halo and Ethernet.

## Usage

```sh
dctl [--json|--plain] [--yes] <command> [args]
```

Global flags:

- `--json` emits one JSON document for commands that return structured data.
- `--plain` disables colors and animation.
- `--yes` accepts boolean confirmations; it does not bypass typed disk or release confirmations.

Bare `dctl` prints help.
`dctl --help` lists top-level commands; use `dctl <command> --help` for its subcommands and flags.
Child commands stream output to stderr so structured `--json` results keep stdout clean.

## Install

**`dctl install` erases a whole disk.**
It requires root, UEFI, and the dctl ISO payload; it refuses an ordinary installed system or stock Arch ISO.
The live environment starts it on tty1.
See the [root README](../../../README.md#boot-and-install) for BIOS preparation and the hardware checklist.

The user is fixed to `cullyn` and the hostname to `costello`.
The form asks for a login password, prefilled timezone, and LUKS passphrase while background preparation verifies the payload and surveys disks.
The installer shows the target model, size, and serial, then requires you to type the disk path before wiping it.
It refuses the boot disk, mounted disks, USB/removable targets, and disks without a serial or WWN.

Installation uses the bundled packages without network access and creates a 4 GiB ESP, LUKS2, and btrfs subvolumes.
It configures Snapper and Limine, clones the Git bundle into `~/dotfiles`, and installs prebuilt commands into `~/.local/bin/`.
It runs `setup system packages` as root in the chroot, then `setup home` as `cullyn`.
When firmware is in Setup Mode, the installer runs `setup secureboot` to create keys if needed, configure Limine, rebuild and verify signed boot files, and enroll the keys.
Otherwise, installation continues without Secure Boot and asks for a later `dctl setup secureboot` after the BIOS keys are cleared.
During enrollment, `secureboot-enforced` may be `manual`; missing items or key/signature items that are not `done` stop installation.
Reboot after enrollment; if Secure Boot is still off, enable it in the BIOS before checking `dctl setup --status secureboot`.
Secure Boot disables fallback `BOOTX64.EFI` because upstream `update_limine_fallback` only copies the binary and never signs it.
The `post.d` hooks run in lexical order, so `89-dotfiles-limine-pristine` re-copies packaged Limine before upstream `90-limine-enroll-config` signs it.
After installation succeeds, it reports the hostname, disk path, and elapsed time and offers to reboot.
An installation failure after disk writes warns that the target disk has already been modified and installation is incomplete.

## Setup

```sh
dctl setup
dctl setup home firefox
dctl setup --all
dctl --json setup --status
```

Run as your normal user; setup runs selected root stages first in one sudo child, then user stages.
Within each batch, stages keep catalog order even when named in another order.
User stages refuse root execution.
With no stage names, setup shows each stage and asks `[Y/n]` for each pending stage; Enter applies it.
Naming stages reapplies their items, including ones that look done; `--all` or global `--yes` applies pending stages without asking.
Setup checks each applied item again.

`--status` changes nothing and returns nonzero for pending or failed items.
Other modes return nonzero for failed items, but allow you to skip pending work.
`manual` needs outside action and does not fail the command; refused sudo leaves root items `unknown`.
JSON output is an array of stages with `stage`, `state`, and `items`; each item has `item`, `state`, and optional `detail`.
An exit code alone does not prove that manual or unknown items are complete.

The catalog is:

- `system` copies `system/`, enables preset-listed units without starting them, and links the resolver stub (root).
- `packages` checks base, AUR, and local payload names and installs missing official packages (root).
- `home` links config, fonts, public SSH keys, desktop entries, and user units, creates directories, and seeds app settings.
- `extra` installs `packages/extra.lst` through yay and enables Docker's socket without starting it.
- `secrets` decrypts missing non-staged targets.
- `repos` clones missing repositories over GitHub SSH.
- `firefox` links customization into the Developer Edition profile after `repos`.
- `certs` provisions the mkcert CA and leaf certificate and verifies system and Firefox trust.
- `vpn` decrypts missing connections' staged profiles and imports them through `hyprd vpn install`.
- `tailscale` enables Tailscale SSH, with login if needed (root).
- `keys` enrolls LUKS FIDO2 and a recovery key if missing; use `dctl keys luks` to add another token (root).
- `secureboot` enrolls keys in Setup Mode or repairs signatures with enrolled keys (root).

The mkcert CA private key is `rootCA-key.pem` in the CAROOT directory reported by `mkcert -CAROOT` and stays on disk after `mkcert -install`.

There is no offline flag; the installer selects only its offline stages.
Connect Ethernet for online work, and launch Firefox once before `dctl setup firefox certs` if its profile is missing.
User units and relative `.wants` links come from `config/systemd/user/`.

Missing AUR or local payload packages require `yay -S` or `makepkg -si` in `packages/<name>`; `setup packages` reports that manual action.
If `/var/lib/pacman/sync/core.db` is missing, run `dctl update pacman` before package setup.

The Secure Boot item IDs are `secureboot-keys`, `secureboot-signed`, and `secureboot-enforced`.
The signed check verifies Limine and UKI signatures, rejects an embedded UKI command line, requires a `limine.conf` entry for each UKI, and checks that `rd.luks.name` and `root` refer to the opened LUKS mapping.
It also checks the Limine settings and rejects a fallback `BOOTX64.EFI`.
Key enrollment and firmware enforcement are separate: enrolled keys can be done while enforcement needs a manual reboot or BIOS change.
Setup does not provision YubiKey identities or flash firmware; use the [keys commands](#keys) and [firmware update step](#update).

## Secrets

Run secrets commands as your normal user.
Encrypted files and metadata live under `secrets/`.
`recipients` lists age recipients, `identities` holds plugin identity stubs, and `identity.age` holds the phrase-wrapped X25519 recovery identity.

Manifest entries use this format:

```text
name:~/target/path:0600[:staged]
```

```sh
dctl secrets list
dctl secrets decrypt
dctl secrets decrypt trend-vpn.nmconnection
dctl secrets sync
dctl secrets rekey
dctl secrets verify-phrase
```

`decrypt` writes all non-staged entries by default; named entries can include staged secrets.
It asks before overwriting targets whose content differs and makes no plaintext backup.
Writes use atomic replacement with the exact manifest mode, stay inside `$HOME` but outside the checkout, and reject symlinks and mount-point crossings.

`sync` encrypts changed plaintext targets; unchanged ciphertext keeps its previous recipients.
Use `rekey` after recipient changes to re-encrypt every manifest secret, including staged ones, to the current recipient list.
Age decryption tries the enrolled plugin identities first, then offers the recovery phrase if those fail; cancelling a plugin prompt stops the operation without offering the phrase.
`sync`, `rekey`, `keys enroll`, and `keys remove` hold an exclusive checkout lock and refuse to run while another operation holds it.
`decrypt`, `sync`, `rekey`, and `verify-phrase` refuse `AGEDEBUG` because plugin debug output can expose PINs and file keys; `list` does not.

### Recovery

Keep the age phrase and the LUKS recovery key available independently of the encrypted disk.
The age phrase unlocks `identity.age`; it does not unlock LUKS directly.
`dctl secrets verify-phrase` reads hidden input, checks the identity and its recipient in memory, and writes nothing.
The tracked monthly user timer reminds you to rehearse it.

Maintain a `recovery` manifest entry if you want `secrets/recovery.age` to hold account recovery codes, a password-manager recovery kit, or YubiKey PINs and PUKs.
This file is not created by the installer.
For example, add `recovery:~/.local/share/dotfiles/recovery.txt:0600` to the manifest, provision that private plaintext file outside the checkout, and run `dctl secrets sync`.
Retrieve it with `dctl secrets decrypt recovery` using a working YubiKey or the age phrase.
Keep an off-site data backup separately; secret recovery does not restore user data.

## Keys

With only the intended YubiKey inserted, run:

```sh
dctl keys enroll
sudo dctl keys luks
dctl keys status
sudo dctl keys status
```

Repeat enrollment for each key; there is no fixed A/B key count.
`enroll` configures FIDO2 PIN/always-UV and PIV PIN/PUK, adds an age identity, rekeys all secrets, and creates a hardware release-signing key at `~/.ssh/id_ed25519_sk_<serial>`.
If `ykman fido info` reports a forced PIN change, enrollment changes the FIDO2 PIN before enabling Always Require UV.
It updates `share/allowed_signers` and the age metadata in the checkout.
`keys luks` adds a FIDO2 token with PIN and touch, adds a recovery key if absent, and keeps the existing passphrase slot.
Write down the recovery key when shown.
Unprivileged `status` can report enrollment, but reading LUKS header details requires sudo.

`dctl keys remove <serial>` removes the age recipient and release signer and rekeys the secrets, but leaves LUKS tokens intact.
`keys remove` refuses root execution; use `sudo dctl keys status` to inspect LUKS tokens, whose header does not record which YubiKey created each one.
Identify the lost key's LUKS slot manually before revoking it with `systemd-cryptenroll --wipe-slot`; keep a tested passphrase or recovery key.

## Porkbun DNS

`dctl porkbun` manages one explicit domain at a time through the [Porkbun v3 API](https://porkbun.com/llms/dns).
It is Linux-only and separate from the `system` setup stage, which configures the machine's resolver.

### Provision credentials first

The Porkbun command does not provision its credential file.
Decrypt the `porkbun.env` manifest entry if its ciphertext is available, or provision the file manually.
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
dctl secrets sync
```

Sync encrypts changed targets using the configured age recipients.
On another Linux machine with the encrypted file and a working age identity, run `dctl secrets decrypt porkbun.env`.
Sync operates on the whole manifest, including entries unrelated to Porkbun.
Adding a manifest entry alone does not create ciphertext or an age identity.

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
`--json` and non-TTY writes require `--yes`.
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

Run both commands as your normal user; they refuse root execution.

```sh
dctl setup repos
dctl update repos
```

`setup repos` reads `packages/repos.lst`, one `owner/name path` pair per line in clone order, and clones missing repositories over GitHub SSH.
`repo` must be a GitHub `owner/name`, and `path` must start with `~/` or `/`.
The system overlay supplies pinned GitHub host keys in `system/etc/ssh/ssh_known_hosts`; setup does not scan or add host keys.

`update repos` fast-forwards clean checkouts with configured upstreams, but never pulls `~/dotfiles` or the `DOTFILES` checkout.
It reports dirty, detached, ahead, diverged, absent, or upstream-less repositories without merging them.

## Update

```sh
dctl update
dctl update repos cli
dctl update --all
```

Run as your normal user; the zsh alias `update` runs `dctl update`.
With no step names, each step shows its plan and asks `[Y/n]`; Enter runs it and `n` skips it.
Named steps skip the per-step prompt and keep the order below.
`--all` or global `--yes` skips prompts and passes `--noconfirm` to pacman/yay, but never flashes firmware.

- `pacman` runs `sudo pacman -Syu`, then reports package-list drift without rewriting lists or removing packages.
- `aur` runs `yay -Sua`.
- `repos` fast-forwards the clean catalog checkouts, excluding dotfiles.
- `cli` rebuilds changed dotfiles commands, updates proxy-installed Go tools, and runs `rustup update` if installed.
- `firmware` refreshes fwupd metadata and lists updates, then asks default-no before flashing; `--all` only reports.

The CLI step uses shared `internal/gobuild` settings and replaces binaries in `~/.local/bin/` only when their bytes differ.
Hyprd owns its replacement through `hyprd rebuild`; a full lock, active OpenCode refresh job, or stopped daemon produces a reported skip.
Ewwd and newtab restart only when replaced.
Uncommitted changes under `cmds/` require an extra confirmation; `--all` skips the dotfiles build instead.
Go tools in `GOBIN` or the first GOPATH's `bin` directory use `go install <package>@latest` only when build metadata has a module-proxy checksum; locally built tools are skipped.
Individual tool or step failures do not stop later work, but the command returns nonzero for failures.

Update never installs newly listed packages; rerun `dctl setup packages extra` and follow any manual AUR/local build instructions.
The drift report lists unlisted explicit packages, listed-but-missing packages, and unlisted orphans across all package lists and local recipes.
On a fresh offline install, run `dctl update pacman` first to synchronize official repository databases.

## ISO

### Build

Build on an Arch host with network access, Go, Git, `archiso`, `devtools`, and pacman tooling.
Run through sudo from your normal account; makechrootpkg and mkarchiso need root.
The build requires a clean, committed `master` and bundles its history instead of copying the working tree.
SSH clients still use `ssh-agent.socket`; commit the switch to `gcr-ssh-agent` before an ISO build.

```sh
sudo dctl iso build
```

It builds the Go commands, resolves `packages/base.lst`, `aur.lst`, and local PKGBUILDs into an offline package repository, and fails on missing packages.
`packages/extra.lst` is installed online by `dctl setup extra` and is excluded from the payload.
The build still reads all three lists, so `extra.lst` must exist.
AUR and local builds import recipe-shipped `keys/pgp/*.asc` into a per-build keyring and fail before building if a `validpgpkeys` fingerprint in `.SRCINFO` is absent from that keyring.
Package archives are fetched with `ParallelDownloads = 5`.
The ISO contains `/opt/dctl/payload`, `/opt/dctl/targets`, `/opt/dctl/dotfiles.bundle`, and prebuilt commands under `/usr/local/bin`.
The payload contains `SHA256SUMS`; the installer verifies its listed files with at most four files read at once.

Output is `iso/out/dotfiles-<12-character-revision>.iso`.
The build rejects images above the 2 GiB release limit; an oversized completed ISO is kept for local use but returns an error.
If sudo cannot find dctl, invoke its absolute path, such as `sudo "$HOME/.local/bin/dctl" iso build`.

### Test

Run without sudo on a host with KVM access, QEMU, dosfstools, mtools, and these OVMF files:

- `/usr/share/edk2/x64/OVMF_CODE.secboot.4m.fd`
- `/usr/share/edk2/x64/OVMF_VARS.4m.fd`

```sh
dctl iso test /path/to/dotfiles-REV.iso
```

The VM has no network interface and uses a 32 GiB virtual disk, Setup Mode firmware variables, and a `DCTLTEST` answers drive.
The harness installs, unlocks LUKS, boots twice, and requires healthy system, packages, home, and all three Secure Boot checks, plus an active display manager.
It reads `setup --status --json` for `system packages secureboot` as root and `home` as `cullyn`.
After a 20-second wait, it saves `greeter.png` for manual inspection; it does not verify the greeter's appearance or sign-in behavior.
It writes `serial.log`, `timings.json`, `setup.json`, and `greeter.png` under the printed `/var/tmp/dctl-iso-test-*` run directory.
It removes the disk and firmware variables by default; `--keep` retains them.

Use `--dctl /path/to/dctl` and `--bundle /path/to/dotfiles.bundle` to test replacements without rebuilding the ISO.
The installer honors that answers drive only inside a detected VM and selects the wipe target by its serial.
The `--dctl` override replaces the running installer before it acquires the install lock.
Run the [manual hardware checklist](../../../README.md#manual-hardware-acceptance) separately.

### Release

This command signs the checksum and **publishes a public GitHub release**.
Run it as your normal user with authenticated `gh` and an enrolled hardware SSH signing key.
The checkout must be clean on `master`, HEAD must match `origin/master`, the ISO filename must match HEAD, and its size must be at most 2 GiB.

```sh
dctl iso release /path/to/dotfiles-REV.iso
```

The default signing key is the only `~/.ssh/id_ed25519_sk_*` private key present; use `--key /path/to/key` when several exist.
Its public key must be in `share/allowed_signers`.
The command writes `<iso>.sha256` and `<iso>.sha256.sig`, signs with the security key, verifies the signature, and asks you to type the release tag before publishing.
It does not push master; push the approved revision before running release.
`gh release create` publishes the ISO and both checksum files with a tag of the form `iso-YYYY.MM.DD-<revision>`.

### USB

**This erases the selected whole disk.**
Keep the ISO, `<iso>.sha256`, and `<iso>.sha256.sig` together; an unsigned local build is refused.
The current signing command is `iso release`, which also publishes the ISO.

```sh
sudo dctl iso usb /path/to/dotfiles-REV.iso /dev/sdX
```

USB writing needs permission to open the raw disk, normally through sudo.
The command verifies the signature against `share/allowed_signers`, checks the ISO hash, and requires you to type the device path.
It refuses partitions, read-only or mounted disks, internal non-removable disks, disks smaller than the image, and targets without a serial or WWN.
It rechecks the device identity after confirmation and hashes the bytes read from the ISO during writing; it does not read the disk back.
`--yes` does not skip the typed confirmation.

## Environment

- `DOTFILES` overrides the dotfiles root.

Without `DOTFILES`, root discovery uses `~SUDO_USER/dotfiles` under sudo and `$HOME/dotfiles` otherwise.
This changes the checkout path, not the owning home directory; run user commands as the owning user.
