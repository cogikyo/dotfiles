# dctl

`dctl` builds the offline UEFI ISO, installs Arch, applies setup stages, and runs updates for a single-user Framework Desktop with AMD Strix Halo and Ethernet.
This page is the command reference; follow [Installation](../../../README.md#installation) for the installation and first-login sequence.

## Usage

```sh
dctl [--json] [--plain] [--yes] <command> [args]
dctl <command> --help
```

- `--json` emits structured results where supported.
- `--plain` disables colors and animation.
- `--yes` (`-y`) accepts yes/no confirmations, including release publishing, but never confirms erasing a disk.

Bare `dctl` prints help.
Streamed child output goes to stderr so JSON results can use stdout.

## Install

> [!CAUTION]
>
> `dctl install` permanently erases the selected whole disk and all its data.
> Back up the disk and follow [Installation](../../../README.md#installation) before running it.

The command requires root, UEFI, and the dctl live ISO with its payload; it refuses an ordinary installed system or stock Arch ISO.
The account is fixed to `cullyn` and the hostname to `costello`.

The target must be an unmounted, writable, non-removable, non-USB disk of at least 32 GiB, with a serial or WWN and 512- or 4096-byte logical sectors; the boot disk is refused.
The installer picks the only such disk, or the only NVMe disk among several, and aborts when no single disk qualifies.
It shows the path, model, size, and identity, asks a default-no yes/no question that `--yes` cannot answer, and rechecks the target before writing.

Installation uses bundled packages without network access and creates a 4 GiB ESP, LUKS2, and btrfs.
The subvolumes are `@` for `/`, `@home` for `/home`, `@log` for `/var/log`, `@pkg` for `/var/cache/pacman/pkg`, and `@snapshots` for `/.snapshots`.
It configures Snapper and Limine, installs prebuilt commands in `~/.local/bin/`, and applies `setup system packages` as root and `setup home` as `cullyn` in the chroot.

The ISO bundle contains only the `master` tip, and the installed `~/dotfiles` is shallow.
Restore history once online with `git -C ~/dotfiles fetch --unshallow`, as shown in [Installation](../../../README.md#installation).

The installer always installs without Secure Boot, never enrolls keys, and does not need Setup Mode.
Set up [Secure Boot](#secure-boot) after the first boot.

A failure during the write/install phase reports that the disk has been modified and installation is incomplete; there is no automatic rollback.

### Snapshot recovery

**`snapper rollback` does not switch this installation's boot root.**
Both fstab and the kernel command line pin `subvol=/@`, so changing btrfs's default subvolume does not select a restored root.
The ESP at `/boot`, including the UKI, is outside the btrfs snapshots; `/home`, logs, and the package cache also have separate subvolumes.

Manual rollback requires the USB and a working LUKS passphrase or recovery key:

1. Boot the USB, unlock the installed LUKS volume, and mount the btrfs top level with `subvolid=5`.
2. Move `@` aside and create a writable snapshot of `@snapshots/N/snapshot` as `@`, choosing the intended snapshot number `N`.
3. Mount the restored `@` as the chroot root, then mount its other subvolumes and the ESP at their fstab paths.
4. Chroot into the restored system and run `limine-update` to rebuild its boot files before rebooting.

Keep the old `@` until the restored system boots successfully.

## Setup

```sh
dctl setup
dctl setup home firefox
dctl setup --all
dctl --json setup --status
```

Run as your normal user; stages run in catalog order, regardless of the order of names on the command line.
Consecutive root stages share one sudo child; user stages run in your session.
User stages refuse root execution.

- No names: show one plan and ask once before applying pending stages.
- Named stages: reapply their items without a setup confirmation, including items that already look done.
- `--all` or global `--yes`: show the plan and apply pending stages without asking.
- `--status`: check without changing anything and return nonzero for pending or failed items.

Applied items are checked again.
A successful apply run reports `setup complete`, or `setup complete with deferred` when only manual work remains.
Failures report `setup failed`; unresolved pending or unknown items also fail an apply run.
Sudo refusal marks root stages failed with `sudo refused; nothing applied as root`; a status-only run leaves them `unknown`.
Manual and unknown items alone do not fail the status check, so inspect their details.
Cancellation exits 130.
JSON results are an array of stages with `stage`, `state`, and `items`; each item has `item`, `state`, and optional `detail`.

### Stages and prerequisites

Catalog order is execution order:

- `system` copies `system/`, enables preset-listed system units without starting them, and links the systemd-resolved stub (root).
- `tailscale` enables Tailscale SSH and logs in if needed (root, online).
- `packages` checks base, AUR, and local payload names and installs missing official packages (root, online).
- `home` links config, fonts, public SSH keys, desktop entries, and user units, creates directories, and seeds app settings.
- `extra` installs missing `packages/extra.lst` entries online in the background and enables Docker's socket without starting it.
- `secrets` restores missing non-staged targets and corrects their modes.
- `repos` clones missing catalog repositories over GitHub SSH (online).
- `firefox` links customization into the Developer Edition profile and needs the CSS repository from `repos`.
- `certs` provisions mkcert's CA and leaf certificate and checks system and Firefox trust.
- `vpn` defers missing connections unless named with `dctl setup vpn`, which decrypts and imports profiles and removes newly staged plaintext even after failure.
- `keys` adds a LUKS FIDO2 token and recovery key if missing, but does not provision YubiKey identities (root).
- `secureboot` enrolls keys in Setup Mode or repairs signatures using enrolled keys (root).

Setup checks Ethernet carrier and DNS for `archlinux.org`, `aur.archlinux.org`, and `github.com` once, just before the first pending online stage.
If the check fails, it prints one action line, marks remaining pending online stages `failed` with `needs network`, and exits 1 after running offline stages such as `secrets`.
Launch Firefox Developer Edition once if `firefox` or `certs` reports a missing profile or NSS database.
After applying Firefox customization, restart Firefox.

`setup packages` installs missing official packages with `pacman -Syu --noconfirm --needed`, so installation includes a full upgrade rather than a partial upgrade.
It has no package checklist and does no upgrade when no official packages are missing.
Install missing AUR names with `yay -S` and local recipes with `makepkg -si` in `packages/<name>`.
There is no offline flag; package selection belongs to [Update package reconciliation](#package-reconciliation).

When extra packages are missing, `extra` runs a full `pacman -Syu` before installing them through yay.
It validates sudo before starting, keeps the credential active, and uses non-interactive sudo (`-n`) in the background.
Output goes to `~/.local/state/dctl/extra.log`; failures show the last 20 log lines.
Setup waits for `extra` before later root work, before `certs`, and at the end.
The first Ctrl+C stops scheduling and waits for `extra` to finish safely; a second press sends SIGINT.
Setup never sends SIGTERM or SIGKILL to pacman.

The `home-fonts` item checks that `fc-list` finds Vagari and Symbols Nerd Font Mono and runs `fc-cache` if either is missing.

The mkinitcpio `HOOKS` live in the system overlay at `system/etc/mkinitcpio.conf.d/dotfiles.conf`, not in installer-generated files.
After changing the HOOKS file or the `sd-totp` hook, run `sudo limine-update`; kernel updates and `dctl setup secureboot` also rebuild the images.

### Secure Boot

The item IDs are `secureboot-keys`, `secureboot-signed`, `secureboot-enforced`, and `secureboot-totp`.
In Setup Mode, setup creates sbctl keys if needed, configures Limine, rebuilds and verifies boot files, and runs `sbctl enroll-keys` to enroll only your own keys, without Microsoft keys.
Leave the TPM enabled: sbctl refuses enrollment if its eventlog shows option ROMs or is missing.
Dctl reports that refusal and never forces enrollment; stop rather than bypassing the check.
If enrollment needs Setup Mode, follow the [installation steps](../../../README.md#6-yubikeys-secure-boot-and-boot-totp) and rerun `sudo dctl setup secureboot`.

The signed check verifies Limine and UKI signatures, rejects an embedded UKI command line, and requires a Limine entry for each UKI.
It checks the LUKS mapping named by `rd.luks.name` and `root`, requires config enrollment, and rejects fallback `BOOTX64.EFI`.
Keys and signatures can be done while enforcement still needs a reboot or BIOS change; use the [manual hardware checklist](#manual-hardware-acceptance) to verify completion.

`secureboot-totp` is status-only and reports `manual` until a secret unseals in the current boot state; run `sudo dctl keys totp` once Secure Boot is enforced.
The command replaces any previous secret and seals a new one to the TPM's SHA256 PCRs 0 and 7: PCR 0 measures firmware, and PCR 7 measures Secure Boot state and keys.
It prints a QR code and an `otpauth://` URL once; scan it or enter the URL's `secret=` value into the authenticator.
The authenticator entry is labelled with the machine's hostname.
The authenticator can be the VivoKey Apex Flex implant read by phone NFC, a YubiKey OATH slot, or a phone TOTP app.

At each boot, `sd-totp` prints `Boot TOTP <code> (Ns left)` on the console every 30 seconds until LUKS is unlocked.
No TPM, no sealed secret, or changed PCRs produce `!!! NO BOOT TOTP: … !!!`; boot continues and the hook never delays unlock.
Inspect `tpm2-totp` errors with `journalctl -b -u boot-totp`.
If image creation cannot find `tpm2-totp` or its device library, it warns and omits the hook, so no boot code or warning appears.
The code is advisory: compare it with the authenticator before entering the PIN or passphrase.

`NO BOOT TOTP` is expected only before the first `dctl keys totp` or immediately after a change you made yourself: a BIOS/firmware update, turning Secure Boot off or on, or re-enrolling keys.
In those cases, unlock, restore Secure Boot enforcement if needed, run `sudo dctl keys totp`, and replace the authenticator entry.
At any other time, a missing or wrong code means **do not unlock**; stop and investigate.

The cause of `Early TPM SRK Setup failed` is not known.
Dctl uses no systemd TPM feature, and `tpm2-totp` creates its own primary, so this unit failure does not block LUKS unlock.
Do not clear the TPM or mask the unit to hide the error; inspect it with:

```sh
journalctl -b -o cat -u systemd-tpm2-setup-early
systemd-analyze has-tpm2
tpm2_getcap properties-variable # check inLockout and lockoutCounter
```

## Update

```sh
dctl update
dctl update repos cmd
dctl update --only dctl,hyprd
```

Run as your normal user; with no step names, each step shows its plan and asks `[Y/n]`.
Enter runs it and `n` skips it.
Named steps omit the per-step prompt and retain the order below.
**Update never pulls `~/dotfiles` or the `DOTFILES` checkout**; manage that checkout yourself before rebuilding commands.

`--all` or global `--yes` omits step and recipe prompts and passes `--noconfirm` to pacman, yay, and makepkg.
It never installs or removes package-list drift.
`--only NAME,...` limits `cmd` to named dotfiles commands or local recipes and selects only `cmd` when no steps are named; explicit step names must include `cmd`.

Steps run in this order:

1. `pacman`: run `sudo pacman -Syu` for official repositories.
2. `aur`: run `yay -Sua --ignore <local recipe names>` for installed AUR packages, leaving local recipes to `cmd`.
3. `packages`: reconcile installed packages with the lists and local recipes.
4. `repos`: fast-forward eligible catalog checkouts, excluding dotfiles.
5. `cmd`: build dotfiles commands and rebuild installed local recipes with different PKGBUILD versions.
6. `go`: update module-proxy-installed Go tools with `go install <package>@latest`.
7. `rust`: run `rustup update`, or skip if rustup is absent.

Failures are reported and later steps continue, but the final result is nonzero if any step failed.
Cancellation stops the run.

### Command and tool rebuilds

The `cmd` step uses shared `internal/gobuild` settings and replaces binaries in `~/.local/bin/` only when their bytes differ.
Hyprd owns its replacement through `hyprd rebuild`; a full lock, active OpenCode refresh job, or stopped daemon is a reported skip.
Ewwd, newtab, and keys restart only when replaced.

Uncommitted changes under `cmds/` require an extra default-no confirmation; `--all` skips that dotfiles build.
Installed local recipes with different versions each require a separate default-yes rebuild prompt, even when `cmd` was named explicitly.
They use `makepkg -sfiC`, which cleans `src/` first; `--all` rebuilds without asking, and uninstalled recipes are skipped.

The `go` step scans `GOBIN`, or the first GOPATH's `bin` directory when GOBIN is unset, and updates only tools whose build metadata has a module-proxy checksum.
Locally built tools are skipped, and individual recipe or Go-tool failures do not stop later work.

### Package reconciliation

The `packages` step uses `packages/*.lst` and local recipe names as the desired package set.
It first marks listed packages explicit and demotes unlisted packages required by others to dependencies, without asking.
This marking precedes removal to protect listed packages from `pacman -Rns`.

Missing listed packages appear in an **all-checked install checklist**.
Selected official and AUR packages go through yay; selected local recipes produce a `makepkg -si` instruction.
Then an all-checked removal checklist offers unlisted explicit packages and unlisted orphans.
Selected removals go to `sudo pacman -Rns`, which asks again before removing them.

Unchecked removal entries are saved in sorted order under `# official` or `# aur` in `extra.lst` and marked explicit.
Esc skips the current checklist; earlier package-reason changes remain, and skipping installation does not skip the later removal checklist.
`--all`, global `--yes`, or no terminal only performs marking and reports remaining drift; `--json` emits drift without changing anything.

## Firmware

Firmware flashing is a manual operation outside dctl; update the BIOS only when needed.
Reports of Framework Desktop BIOS 3.06 bricking machines are a reason to hold off.
`fwupd` is not in the base package list.
Follow the [expected missing-code rule](#secure-boot) when unlocking during this procedure.

1. Install the tool with `sudo pacman -S fwupd`.
2. Press F2 on reboot, turn Secure Boot off, and save with F10.
3. Boot and run `fwupdmgr refresh` then `fwupdmgr update`, or use Framework's EFI USB updater.
4. After the update, press F2, turn Secure Boot on, save with F10, and boot.
5. Run `sudo dctl keys totp` to re-seal for the changed PCRs 0 and 7, and replace the old authenticator entry with the new secret.

## ISO

Build writes `iso/out/dotfiles-<rev12>.iso`, where `rev12` is the first 12 characters of the built revision.
Test, release, and USB default to the newest of those images and print the one they chose.
Test and release accept a positional image path; USB uses `--iso <path>`.

### Build

Use an Arch host with network access, Go, Git, `archiso`, `devtools`, and pacman tooling, with a clean committed checkout on `master`.
Run as your normal user; dctl re-runs itself through sudo for privileged work.

```sh
dctl iso build
```

The build uses a depth-1 clone of `master` and bundles only that tip, not the working tree or full history.
It builds the Go commands and resolves base, AUR, and local recipes into the offline package payload, failing on missing packages.
`extra.lst` must exist but is excluded from the payload; install its entries online with `dctl setup extra`.
AUR/local builds use a per-build PGP keyring populated from recipe `keys/pgp/*.asc`; any declared `.SRCINFO` `validpgpkeys` fingerprint absent from that keyring stops the build before compilation.

Builds reuse `/var/cache/dctl-iso/` for the build chroot, downloaded recipe sources, built recipe packages, and official package downloads.
The cached chroot is refreshed on each build.
Recipe package keys use the AUR repo commit or local recipe tree, plus the direct runtime dependency versions in the refreshed chroot's package databases.
A recipe whose runtime dependencies name another recipe stops the build.

Use `dctl iso build --fresh` to get new upstream code for `-git` AUR recipes such as `mpvpaper-git`, or to replace a broken cache.
Valid cache entries are reused until the AUR repo or runtime dependency inputs change; upstream commits alone do not trigger a rebuild.
`--fresh` wipes the cache, so the first build after a wipe is a full build.

Concurrent builds are refused.
The final timing summary shows stage and recipe times, including recipe cache hits and misses.

The ISO includes `/opt/dctl/payload`, `/opt/dctl/targets`, `/opt/dctl/dotfiles.bundle`, and prebuilt commands in `/usr/local/bin`; installation verifies the payload checksums.
A payload or image above 2 GiB fails the release-size check; an oversized completed image is retained for local use but the command returns an error.

### Test

Use a normal account with KVM access, QEMU, dosfstools, mtools, and both `/usr/share/edk2/x64/OVMF_CODE.secboot.4m.fd` and `/usr/share/edk2/x64/OVMF_VARS.4m.fd`.

```sh
dctl iso test [path/to/image.iso] [--keep]
```

Use `dctl iso test --head` for the fast installer/setup loop; it requires Go and Git.
`--head` builds dctl from the checkout, including uncommitted code changes, and bundles the committed HEAD as `master`.
Uncommitted changes do not enter the bundle.
`--head` cannot be combined with `--dctl` or `--bundle`.

Package-list or ISO-profile changes still require `dctl iso build`; the package payload and live environment come from the ISO.

The VM has no network interface and uses a 32 GiB disk, Setup Mode firmware variables, and a `DCTLTEST` answers drive.
The harness installs, unlocks LUKS with a passphrase, boots twice, and requires `system`, `packages`, and `home` to be done, plus an active display manager.
It does not check Secure Boot.

Results live in the printed `/var/tmp/dctl-iso-test-*` directory: `serial.log`, `timings.json`, `setup.json`, and, after a 20-second wait, `greeter.png`.
Inspect the screenshot yourself; the test does not verify appearance or sign-in behavior.
VM disks and firmware variables are removed by default; `--keep` retains them.
Use `--dctl /path/to/dctl` and `--bundle /path/to/dotfiles.bundle` to test replacements without rebuilding the ISO.
Run [manual hardware acceptance](#manual-hardware-acceptance) separately.

### Release

> [!WARNING]
>
> `dctl iso release` signs the checksum and publishes a public GitHub release.

Run as your normal user with authenticated `gh` and an enrolled hardware SSH signing key.
The ISO filename revision must be contained in `origin/master`, and the image must be at most 2 GiB.
The revision may be older than HEAD, so later commits do not require a rebuild.
Release checks the local `origin/master` ref and does not fetch or push it.

```sh
dctl iso release [path/to/dotfiles-REV.iso] [--key /path/to/key]
```

The default key is `~/.ssh/id_ed25519_sk_<serial>` for the one inserted YubiKey; `--key` selects another key whose public key is trusted in `share/allowed_signers`.
Signing bypasses the SSH agent and asks for the FIDO2 PIN, then touch.

The command writes `<iso>.sha256` and `<iso>.sha256.sig`, verifies the signature, and asks before publishing.
The release tag is `iso-YYYY.MM.DD-<rev12>`, with the ISO and both checksum files as assets.

### USB

> [!CAUTION]
>
> `dctl iso usb` erases the selected whole disk.
> Choose an unmounted removable or USB disk, not a partition.

Keep the ISO, `<iso>.sha256`, and `<iso>.sha256.sig` together; unsigned builds are refused.
The signing command is [Release](#release), which also publishes the ISO.

```sh
dctl iso usb /dev/sdX
dctl iso usb --iso ~/Downloads/dotfiles-REV.iso /dev/sdX
```

Run as your normal user; dctl re-runs itself through sudo.
It verifies the checksum signature against `share/allowed_signers`, requires the exact device path, and checks the ISO hash before writing.
It refuses partitions, read-only or mounted disks, internal non-removable disks, undersized disks, and targets without a serial or WWN.

The device identity is rechecked after confirmation, and writing hashes the ISO bytes sent to the disk and syncs writes.
It does **not read the disk back**.
`--yes` does not skip typed confirmation, and a write failure means the disk must not be booted.

## Secrets

Run as your normal user; `secrets/` holds ciphertext, `recipients` for age recipients, `identities` for plugin stubs, and `identity.age` for the phrase-wrapped X25519 recovery identity.
Manifest entries use `name:~/target/path:0600[:staged]`.

```sh
dctl secrets list
dctl secrets decrypt [name ...]
dctl secrets sync
dctl secrets rekey
dctl secrets verify-phrase
```

- `list` reports manifest entries, ciphertext presence, and target status.
- `decrypt` writes non-staged entries by default; named entries can include staged secrets.
- `sync` encrypts changed plaintext targets across the manifest; unchanged ciphertext keeps its previous recipients.
- `rekey` re-encrypts every manifest secret, including staged entries, for the current recipient list.
- `verify-phrase` checks the recovery identity and its listed recipient in memory and writes nothing.

Decryption asks before overwriting differing content and keeps no plaintext backup.
Targets use atomic replacement with the exact manifest mode and must stay inside `$HOME`, outside the checkout, without symlinks or mount-point crossings.
Age tries plugin identities first and offers the recovery phrase if they fail; cancelling a plugin prompt stops without offering the phrase.

`sync`, `rekey`, `keys enroll`, and `keys remove` hold an exclusive checkout lock and refuse concurrent locked operations.
`decrypt`, `sync`, `rekey`, and `verify-phrase` refuse `AGEDEBUG` because it can expose PINs and file keys; `list` does not.

### Secret recovery

Keep the **age phrase** and **LUKS recovery key** available independently of the encrypted disk.
The phrase unlocks `identity.age`, not LUKS; use `verify-phrase` and rehearse decryption without a YubiKey.
Secret recovery does not restore user data, so maintain a separate off-site backup.

To store account recovery codes or a recovery kit, add `recovery:~/.local/share/dotfiles/recovery.txt:0600` to the manifest and provision that private plaintext outside the checkout.
Run `dctl secrets sync` to create its ciphertext, then retrieve it with `dctl secrets decrypt recovery` using a working YubiKey or the phrase.

## Keys

Follow [Installation](../../../README.md#installation) for first enrollment with only the intended YubiKey inserted; there is no fixed key count.

```sh
dctl keys enroll
sudo dctl keys luks
sudo dctl keys totp
dctl keys status
sudo dctl keys status
dctl keys remove <serial>
```

`enroll` runs as your user and configures FIDO2 PIN/always-UV and PIV PIN/PUK, adds an age identity, rekeys secrets, and provisions `~/.ssh/id_ed25519_sk_<serial>` for release signing.
A forced FIDO2 PIN change is completed before always-UV is enabled; age uses the PIV PIN without touch.
Enrollment updates age metadata and `share/allowed_signers` in the checkout.

`keys luks` requires root and adds a FIDO2 token with PIN but no touch, adds a recovery key if absent, and leaves the passphrase slot unchanged.
It asks before adding another token when one already exists; record the recovery key when it prints.
Unprivileged `status` reports enrollment, while LUKS header inspection needs sudo.

`totp` seals the [boot TOTP secret](#secure-boot); it requires root and enforced Secure Boot.
Use a plain TTY or clear terminal scrollback afterward so the enrollment secret does not remain visible.

`remove` refuses root, removes the age recipient and release signer, and rekeys secrets, but leaves LUKS tokens intact.
The LUKS header does not identify which YubiKey created a token.
Inspect it with `sudo dctl keys status` and identify the slot manually before revoking it with `systemd-cryptenroll --wipe-slot`; retain a tested passphrase or recovery key.

### LUKS unlock

Insert the YubiKey before boot; unlock does not wait for a key inserted later.
Systemd-cryptsetup uses the LUKS2 token plugin and asks once for `LUKS2 token PIN`, with no touch for tokens enrolled by `dctl keys luks`.
Any token error falls back to the passphrase or recovery-key prompt.
New installs omit `rd.luks.options=<uuid>=fido2-device=auto`; on an existing install, remove that option from `/etc/default/limine` and run `sudo limine-update` to enable this fallback.

## Porkbun DNS

`dctl porkbun` is Linux-only and manages one explicit domain at a time through the [Porkbun v3 API](https://porkbun.com/llms/dns).
It is separate from the machine resolver configured by `setup system`.

### Credentials

Provision `~/.local/share/dotfiles/porkbun.env` as a regular, non-symlink file owned by your user with mode `0600`.
It must contain exactly two unquoted assignments: `PORKBUN_API_KEY=value` and `PORKBUN_API_SECRET_KEY=value`, using Porkbun-issued values.
No spaces, comments, blank lines, `export`, or shell expressions are accepted; one ending newline is allowed.

Create credentials in the [account dashboard](https://porkbun.com/account/api) and enable API access for the domain; keep key values out of shell history and command arguments.
Set restrictive permissions before editing and avoid plaintext editor backups.
The command reads the file without evaluating shell code, decrypting, or using a credential fallback, and refuses root/sudo execution.

If the `porkbun.env` manifest entry has ciphertext, provision it with `dctl secrets decrypt porkbun.env`.
Use `dctl secrets sync` for encrypted synchronization; sync covers the whole manifest, and adding an entry alone creates neither ciphertext nor an identity.

### Commands and record fields

```sh
dctl porkbun check <domain>
dctl porkbun list <domain>
dctl porkbun create <domain> <name> <type> <content> [--ttl N] [--prio N] [--dry-run]
dctl porkbun edit <domain> <id> --content VALUE [--ttl N] [--prio N] [--dry-run]
dctl porkbun delete <domain> <id> [--dry-run]
```

Domains are lowercase ASCII without a scheme, path, or trailing dot; use punycode for internationalized names.
Create names are relative lowercase subdomains; `@` or an empty name means the root, and wildcard names such as `'*'` need shell quoting.
List returns fully qualified names and record IDs; take edit/delete IDs from the list for that domain.
IDs are positive decimal numbers without signs or leading zeros.

Writes support A, AAAA, CNAME, TXT, MX, and SRV only, with content in Porkbun's format.
Omitted create TTL or `--ttl 0` uses the account minimum; `--prio` applies only to MX/SRV, ranges from 0 to 65535, and defaults to zero on create.
Edit preserves name, type, notes, and omitted TTL/priority fields.

Duplicate name/type/content/priority is refused even when TTL differs; distinct values can coexist, and an already-matching edit reports unchanged.

### Consent and uncertain outcomes

Writes show current and proposed state and require a default-no TTY confirmation unless global `--yes` is supplied.
JSON or non-TTY writes require `--yes`; `--plain` still permits TTY confirmation.

Dry runs need no consent: create sends `dryRun=true` and requires `wouldSucceed=true` without a record ID, while edit/delete only preview locally without proving mutation permission.
`check` tests authentication, DNS readability, authority evidence, and a create dry-run for `_dctl-check`, not live writes.

Preflight requires matching domain metadata with `notLocal=0` and warning-free API responses; missing evidence or provider warnings block writes.
This is API authority evidence, not independent DNS resolution.
After consent, it refreshes authority and records and refuses a changed target or new duplicate before sending one mutation.

Readback verifies the intended state or absence by ID, including preserved edit fields, but does not prove public DNS propagation.
Concurrent changes remain possible between requests, and exact content comparison can make provider-normalized results uncertain.
A failed mutation response or failed/mismatched readback returns nonzero; inspect `list` before retrying because there is no automatic mutation retry or rollback.

## Repos

```sh
dctl setup repos
dctl update repos
```

Run as your normal user; `setup repos` reads `packages/repos.lst` in clone order, with one `owner/name path` pair per line, and clones missing directories over GitHub SSH.
The repository must be a GitHub `owner/name`, and the path must start with `~/` or `/`.
Existing directories are not replaced, and setup checks their origin URLs.
Clones use a hidden sibling directory and move into place only after success; failed clones leave no checkout or temporary clone.
A broken checkout is manual work: move it aside, then rerun `dctl setup repos`.

`update repos` fast-forwards branches with configured upstreams and no tracked changes; untracked files are not included in the dirty check.
It skips both `~/dotfiles` and the `DOTFILES` checkout and reports dirty, detached, ahead, diverged, or upstream-less repositories without merging them.
Absent repositories produce one summary line directing you to `dctl setup repos`; broken checkouts are failures.

## Environment

`DOTFILES` overrides the checkout root, which must contain `AGENTS.md`.
Without it, root discovery uses `~SUDO_USER/dotfiles` under sudo and `$HOME/dotfiles` otherwise.
The override does not change the home directory used for user targets; run user commands as the owning user.

## Manual hardware acceptance

These checks require the Framework Desktop and real YubiKeys; the VM test does not prove them.

- [ ] Before installation, confirm Secure Boot is off, its keys have not been erased, and the TPM is enabled.
- [ ] After enrollment and TOTP sealing, confirm Secure Boot is enforced with only your own keys and all items are done in `dctl setup --status secureboot`.
- [ ] Confirm Boot TOTP matches the authenticator across two reboots to check PCR 0 stability.
- [ ] Confirm `NO BOOT TOTP` appears with Secure Boot off, then re-enable Secure Boot and follow the [re-sealing procedure](#secure-boot).
- [ ] Confirm the TOTP hook never delays LUKS unlock.
- [ ] Insert each YubiKey separately before boot and unlock LUKS with one `LUKS2 token PIN` prompt and no touch.
- [ ] Confirm a token error falls back to the passphrase or recovery-key prompt without a locked emergency shell.
- [ ] Decrypt secrets with each YubiKey separately, without the other key or the age phrase.
- [ ] Reject a wrong FIDO2 PIN and a wrong PIV PIN; cancel any age-phrase fallback and avoid repeated failures that can block the key.
- [ ] Unlock LUKS with the recorded recovery key while both YubiKeys are removed.
- [ ] Pass `dctl secrets verify-phrase` and rehearse secret recovery without a YubiKey.
- [ ] Confirm `nmcli device status` shows no Wi-Fi interface.
- [ ] Confirm `bluetoothctl list` shows a Bluetooth controller, then pair and use a device.
- [ ] Confirm `journalctl -k -b` has no firmware load errors with `linux-firmware-{amd,amdgpu,mediatek,realtek}` installed.
