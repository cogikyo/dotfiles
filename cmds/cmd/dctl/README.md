# dctl

`dctl` installs and maintains this single-user Arch Linux system.
It builds the offline ISO, applies configuration, and runs attended updates.

- Built for the Framework Desktop with AMD Strix Halo and Ethernet only.
- Installs the `cullyn` account and `costello` hostname.

> _See [Installation](../../../README.md#installation) for the overview._

## Usage

Run user commands from your own session; they elevate only where needed.

```sh
dctl [--json] [--plain] [--yes] <command> [args]
dctl <command> --help
```

- `--json` emits structured results where supported; child output uses stderr.
- `--plain` disables colors and animation.
- `--yes` (`-y`) accepts yes/no prompts, including release publication.
  - It never grants disk-erase consent or setup's lock confirmation.

## Install

Installation uses the ISO's package payload without a network.
The live ISO starts the installer on tty1.

> [!CAUTION]
>
> `dctl install` erases the whole **target drive**.
> Back up its data before confirming the erase.

```text
prepare ──▶ disk consent ──▶ install steps ──▶ unmount ──▶ summary
```

- Requires root, UEFI, and the dctl live ISO; stock Arch media are refused.
- Prompts for the login password, timezone, and LUKS passphrase.
- After success, remove the USB, reboot, and unlock with the disk passphrase.
- Failed install steps can leave a partly modified disk; there is no rollback.
  - Unmount and LUKS-close cleanup is attempted after success or failure.

### Disk selection and consent

The installer chooses the only eligible disk, or the only eligible NVMe disk.
An ambiguous selection stops before writing.

- Eligible disks must be unmounted, writable, non-removable, and not USB-connected.
- Minimum size: 32 GiB; identity: serial or WWN; sector size: 512 or 4096 bytes.
- The boot medium must be identified and excluded.
- Occupied `/mnt` or open `/dev/mapper/root` stops installation.
- Consent shows disk identity and asks **Erase `<disk>` and install?**, default **No**.
  - `--yes` is refused; identity and eligibility are rechecked after consent.

### Installed files and layout

The encrypted btrfs root is separate from the EFI system partition (ESP).
Root snapshots cannot restore the boot images or the other subvolumes.

| Storage      | Mount                   |
| ------------ | ----------------------- |
| 4 GiB FAT32  | `/boot`                 |
| `@`          | `/`                     |
| `@home`      | `/home`                 |
| `@log`       | `/var/log`              |
| `@pkg`       | `/var/cache/pacman/pkg` |
| `@snapshots` | `/.snapshots`           |

- Creates a shallow `~/dotfiles` checkout from the bundled `master` tip.
- Installs prebuilt commands in `~/.local/bin/` and disables root password login.
- Applies `system`, `packages`, and `home`; rebuilds and checks Limine boot images.
- Snapper retains five hourly and seven daily root snapshots.
- Secure Boot keys are enrolled later; installation does not require Setup Mode.

> _See [Repos](#repos) for automatic history fetching after installation._

### Snapshot recovery

Changing btrfs's default subvolume does not select a restored boot root.
This installation pins `@`, so a plain Snapper rollback is insufficient.

1. Boot the USB and unlock LUKS with the passphrase or recovery key.
2. Mount the btrfs top level with `subvolid=5`.
3. Move `@` aside and create a writable snapshot as `@`.
   - Source: `@snapshots/N/snapshot`; choose the intended snapshot number `N`.
4. Mount restored `@`, the other subvolumes, and the ESP at their fstab paths.
5. Chroot into the restored system and rebuild its boot files:

   ```sh
   limine-update
   ```

6. Reboot and keep the old `@` until the restored system boots successfully.

## Setup

Setup checks configuration, applies pending work, then offers security enrollment.
Run as the normal user for dependency planning and the separate lock gate.

```sh
dctl setup
dctl setup home firefox
dctl setup --from firefox
dctl --json setup --status
```

```text
system ─▶ network ─▶ packages ─▶ home ─▶ extra (background)
  ┌──────────────────────────────────────────┘
  ▼
secrets ─▶ ssh ─▶ repos ─▶ firefox ─▶ certs
  ┌─────────────────────────────────────┘
  ▼
All required phase-1 stages OK
  │
  ▼
Separate lock confirmation
  │
  ▼
luks ──▶ secureboot ──▶ reboot ──▶ totp ──▶ reboot and compare
```

### Stage selection

Stages run in catalog order, regardless of the order you name them.
Applied items are checked again and the summary gives the next action.

- Omit stage names to show the full plan and apply pending work.
- Name stages to redo them and pull in unfinished dependencies.
- Use `--from <name>` to redo that stage and later ones; optional stages need names.
- Use `--yes` to accept the plan; rerun without it for lock work.
- Use `--status` for a read-only check; pending, failed, or blocked work fails.
  - Manual or deferred work can exit 0; inspect the rows, not only the exit code.

### Stages and prerequisites

Required phase-1 stages run in the table's order, from system through certs.
The optional stages do not gate security enrollment.

| Stage      | Needs                       | Work                      |
| ---------- | --------------------------- | ------------------------- |
| system     | —                           | Overlay and services      |
| network    | —                           | Ethernet and DNS          |
| packages   | system, network             | Base, AUR, local packages |
| home       | packages                    | Links and fonts           |
| extra      | home, network               | Background apps           |
| secrets    | system                      | Restore private files     |
| ssh        | secrets, network            | GitHub access             |
| repos      | ssh                         | Clone and fetch history   |
| firefox    | packages                    | Profile and config        |
| certs      | firefox                     | Local CA and certificates |
| tailscale  | network                     | Optional login and SSH    |
| vpn        | secrets                     | Optional work VPN         |
| luks       | All required phase-1 stages | Two YubiKeys and recovery |
| secureboot | luks                        | Keys and boot signatures  |
| totp       | secureboot                  | Seal the boot TOTP        |

- Consecutive root stages share one sudo child; user stages stay in your session.
- Root runs `system`, `packages`, `tailscale`, `luks`, `secureboot`, and `totp`.
- `certs` uses sudo from the user session; user stages refuse root execution.

### Status and output

Status rows distinguish work dctl can apply from work that needs your attention.
The summary and shell hint are the next-action reference.

| Pill  | Meaning                 |
| ----- | ----------------------- |
| OK    | Done                    |
| TODO  | Work to run             |
| FIX   | Manual action           |
| LATER | Reboot or firmware step |
| OPT   | Runs only when named    |
| WAIT  | Needs earlier work      |
| ERR   | Failed or unknown       |

- `ASK` marks a prompt answer; `RUN` marks a command.
- Cancellation exits 130.
- A full run records its next action under `$XDG_STATE_HOME/dctl/next`.
  - Default: `~/.local/state/dctl/next`; zsh prints it as `dctl · …`.

### First-login details

Network and authentication failures can leave dependent stages waiting.
Independent stages can still run.

- Failed network checks offer **Retry** or **Skip online steps**.
- Home linking backs up conflicting files but refuses to replace real directories.
- Secrets ask for the YubiKey's PIV PIN or the age recovery phrase.
- Enter the SSH key's passphrase when prompted.
  - Register rejected keys with GitHub; review changed host-key fingerprints manually.
- Quit Firefox if profile creation asks, then retry:

  ```sh
  dctl setup firefox certs
  ```

- Firefox CSS needs the Vagari checkout from `repos`; restart after customization.

> _See [Secrets](#secrets) and [Repos](#repos) for their inputs and recovery._

### Packages and background work

Setup installs missing official packages and starts extra applications in the background.
Missing AUR and local base packages need manual installation.

- `packages` reads the base, AUR, and local inputs under `packages/`.
  - Missing official packages trigger `pacman -Syu --noconfirm --needed`.
  - No official packages missing means no upgrade from this stage.
- Install missing AUR packages:

  ```sh
  yay -S <name>
  ```

- Build missing local recipes from their `packages/<name>` directory:

  ```sh
  makepkg -si
  ```

- `extra` upgrades pacman packages and installs missing `extra.lst` entries via yay.
  - Output: `~/.local/state/dctl/extra.log`; failures show recent log lines.
  - Setup waits before later root work, before `certs`, and at the end.
- First Ctrl+C stops new stages and waits for `extra`; a second sends SIGINT.
  - Setup never sends SIGTERM or SIGKILL to pacman.

> _See [Package reconciliation](#package-reconciliation) for interactive selection._

### Optional networking

Tailscale and work VPN setup run only when named.

```sh
dctl setup tailscale
dctl setup vpn
```

- Tailscale login shows a QR code; scan with a phone logged in to Tailscale.
  - No browser opens; an existing connection only needs SSH enabled.
- VPN profiles come from `cmds/config/hyprd.yaml` and staged secrets.
  - Imports use hyprd; newly staged plaintext is removed even after failure.

### Lock confirmation and root execution

Lock work waits for every required phase-1 stage to be OK and asks separately.
Naming a lock stage still pulls in unfinished phase-1 work and requires consent.

> [!CAUTION]
>
> Direct root setup bypasses the plan, dependency pull-in, and lock confirmation.
> Run first setup as the normal user.

- `--yes` never accepts the lock confirmation.
- Named root stages under sudo run directly; each checks its own conditions.

> _See [Secure Boot](#secure-boot) for the enrollment procedure._

### Secure Boot

Enrollment trusts only the owner's keys.
It refuses automatic option-ROM trust and does not add Microsoft keys.

1. Press F2 and choose **Erase all Secure Boot Settings** to enter Setup Mode.
   Leave the TPM enabled.
2. Save with F10; prepare both YubiKeys' PINs and age identities before locking down.
3. Run setup as the user and accept the lock confirmation once phase 1 is OK:

   ```sh
   dctl setup
   ```

4. Complete the `luks` prompts for both keys and confirm the paper recovery copy.
5. Let `secureboot` enroll keys and sign images, then reboot with a YubiKey inserted.
   - If Secure Boot is still off, enable it with F2 and save with F10.
6. Run setup again to seal `totp` once Secure Boot is enforced.
7. Reboot and compare the boot code with the authenticator before unlocking.

> _See [Keys](#keys) for YubiKey preparation and paper recovery._

#### Enrollment checks

Keys and signatures can be ready while enforcement still needs a reboot.
Do not force enrollment past a failed check.

- Stops on an unreadable TPM event log, measured option ROMs, or `db_additions`.
- Rejects Microsoft KEK/db certificates after the owner's platform key is enrolled.
- Verifies Limine and UKI signatures, boot entries, and the root LUKS mapping.
  - Embedded UKI command lines are refused so snapshot entries can select a root.
- Updates `/etc/default/limine` and removes `/boot/EFI/BOOT/BOOTX64.EFI`.
- Keeps signing keys under `/var/lib/sbctl/` and enrolls firmware variables.

#### TOTP enrollment and recovery

The secret is sealed to SHA256 PCRs 0 and 7: firmware and Secure Boot state.
The boot code is advisory tamper evidence; the hook does not block disk unlock.

1. Enroll from a plain TTY, or clear terminal scrollback afterward.
2. Scan the QR code shown once and enter its authenticator code.
3. Wait for the verified image rebuild before rebooting.
   - If image checks fail, do not reboot until setup's Secure Boot checks pass.

> [!CAUTION]
>
> A missing or wrong boot TOTP after enrollment means **do not unlock**.
> Investigate the TPM and boot state before resealing.

- Before enrollment, the screen reports **Boot TOTP not set up yet**.
- After enrollment, unsealing failure reports **NO BOOT TOTP**.
- Missing hook dependencies at image build time leave that image without a boot code.
- Inspect boot-hook errors:

  ```sh
  journalctl -b -u boot-totp
  ```

Changed PCR state is never resealed automatically.
Only replace the secret after establishing a known firmware or Secure Boot change.

1. Restore Secure Boot enforcement.
2. Replace the secret and verify the new authenticator entry:

   ```sh
   sudo dctl keys totp
   ```

3. Reboot and compare the new code before unlocking.

#### Lock state

Setup records enrollment and image-build state separately.
These files are under `/etc/dctl/`.

| File                        | Records                            |
| --------------------------- | ---------------------------------- |
| `boot-totp`                 | Verified authenticator enrollment  |
| `boot-totp.built`           | Marker used for a verified rebuild |
| `luks-yubikeys`             | Serials enrolled for this disk     |
| `luks-recovery.unconfirmed` | Paper copy not yet confirmed       |

- A recorded key with no disk token needs its `luks-yubikeys` line removed to reenroll.
- After changing mkinitcpio `HOOKS` or `sd-totp`, rebuild images:

  ```sh
  sudo limine-update
  ```

> _See [hardware acceptance](#manual-hardware-acceptance) for real-machine checks._

## Update

Update runs attended upgrades and rebuilds from the current checkout.
Failed steps are reported; later steps continue and the summary gives a retry command.

```sh
dctl update
dctl update repos cmd
dctl update --only dctl,hyprd
```

- Run as the normal user; root execution is refused.
- Update never pulls `~/dotfiles` or the `DOTFILES` checkout; manage it separately.
- Any failed step makes the final result nonzero.
- Ctrl+C can interrupt pacman; update has no setup-style protected wait.

### Selection and step order

With no names, each step asks for consent, default **Yes**.
Named steps omit that prompt and retain this order.

1. `pacman`: upgrade official packages.
2. `aur`: upgrade AUR packages, excluding local recipes.
3. `packages`: reconcile installed packages with the lists.
4. `repos`: fast-forward eligible catalog checkouts, excluding dotfiles.
5. `cmd`: rebuild dotfiles commands and installed local recipes whose versions differ.
6. `go`: update eligible module-release tools.
7. `rust`: run `rustup update`, or skip if rustup is absent.

- `--all` or global `--yes` skips step/recipe prompts and passes `--noconfirm`.
  - It never installs or removes package-list drift.
- `--only NAME,...` limits `cmd` to named commands or local recipes.
  - With no step names it selects `cmd`; explicit step names must include `cmd`.

### Command and tool rebuilds

Dotfiles binaries are replaced only when their bytes differ.
Ewwd, newtab, and keys restart only when replaced.

- **hyprd** replaces itself via `hyprd rebuild`.
  - Skips during full lock, an OpenCode refresh job, or when the daemon is stopped.
- **Uncommitted `cmds/`** requires extra consent, default **No**; `--all` skips the build.
- **Local recipes** with different installed versions ask separately, default **Yes**.
  - Rebuilds via `makepkg -sfiC`, cleaning `src/`; `--all` skips prompts.
- **Go tools** need module-proxy checksum metadata; locally built tools are skipped.
  - Scans `GOBIN`, or the first GOPATH's `bin`; installs eligible tools at `@latest`.

### Package reconciliation

Reconciliation treats the package lists and local recipes as the desired set.
It changes package reasons before prompting, so cancelling is not a complete undo.

1. Mark listed packages explicit and unlisted packages needed by others as dependencies.
2. Offer missing packages in an all-checked install checklist.
3. Install selected official/AUR packages via yay; give build instructions for locals.
4. Offer unlisted explicit packages and orphans in an all-checked removal checklist.
5. Save unchecked entries to `packages/extra.lst` and mark them explicit.
6. Send selected removals to pacman, which asks again.

- Esc skips the current checklist; earlier reason changes remain.
- `--all`, global `--yes`, or no terminal: change reasons and report drift only.
- `--json`: this step reports drift without changes; other steps can still mutate.

## Firmware

Firmware flashing is outside dctl.
Follow Framework's instructions for the chosen release and updater.

- There is no firmware command; `fwupd` is absent from the base list.
- Firmware and Secure Boot changes can invalidate the sealed boot TOTP.

> _See [Secure Boot](#secure-boot) before replacing a secret after changed boot state._

## ISO

The ISO commands build, VM-test, sign, and write offline installation media.
Test, release, and USB choose the newest built image unless given a path.

- Build writes `iso/out/dotfiles-<rev12>.iso`, using the revision's first 12 characters.
- Test/release accept a positional image path; USB uses `--iso <path>`.

### Build

Build bundles the committed master tip and an offline package payload.
Extra applications remain an online setup step.

- Requires an Arch host, network, Go, Git, `archiso`, `devtools`, and pacman tooling.
- Requires a clean, committed checkout on `master`.
- Run as the user; dctl elevates privileged work through sudo.

```sh
dctl iso build
dctl iso build --fresh
```

- Bundles a depth-1 clone; working-tree changes and full history are excluded.
- Resolves base, AUR, and local recipes; missing packages stop the build.
- `extra.lst` must exist but is not bundled; use `dctl setup extra` after installation.
- Declared recipe PGP keys must be provided under recipe `keys/pgp/*.asc`.
- Uses `/var/cache/dctl-iso/` for caching; concurrent builds are refused.
  - Recipe keys include recipe revisions and direct runtime dependency versions.
  - Upstream VCS commits and build-dependency versions are not separate inputs.
  - Use `--fresh` for updated `-git` sources or a broken cache; it wipes all cache state.
- Payload or ISO above 2 GiB fails the release-size check.
  - An oversized completed ISO is retained for local use, but build returns an error.

### Test

The offline VM harness installs, unlocks LUKS with a passphrase, and boots twice.
It checks basic setup and an active display manager, not Secure Boot enforcement.

- Requires a user account with KVM access, QEMU, dosfstools, and mtools.
- Requires `/usr/share/edk2/x64/OVMF_CODE.secboot.4m.fd` and `OVMF_VARS.4m.fd`.

```sh
dctl iso test [path/to/image.iso] [--keep]
dctl iso test --head
dctl iso test --dctl /path/to/dctl --bundle /path/to/dotfiles.bundle
```

- `--head` builds current dctl code, including uncommitted edits, and needs Go/Git.
  - The bundle contains committed HEAD only, presented as `master`.
  - Cannot combine with `--dctl` or `--bundle`.
- Overrides replace the binary/bundle only; package/profile changes need a new ISO.
- The VM has no network, a 32 GiB disk, and Setup Mode firmware variables.
- Results stay in the printed `/var/tmp/dctl-iso-test-*` directory.
  - Logs: `serial.log`, `timings.json`, `setup.json`; screenshot: `greeter.png`.
  - Inspect the screenshot yourself; appearance and sign-in are not verified.
- `--keep` retains VM disks and firmware variables; otherwise those are removed.

> _See [hardware acceptance](#manual-hardware-acceptance) for checks beyond the VM._

### Release

Release signs checksums before offering to publish.
Declining publication leaves those signed files available for USB writing.

> [!CAUTION]
>
> `dctl iso release` publishes a **public GitHub release** after confirmation.
> Global `--yes` accepts that confirmation.

```sh
dctl iso release [path/to/dotfiles-REV.iso] [--key /path/to/key]
```

- Requires the user session, authenticated `gh`, and a hardware SSH signing key.
- Image limit: 2 GiB; filename revision must be contained in local `origin/master`.
  - The revision can be older than HEAD; release does not fetch or push.
- Default key: `~/.ssh/id_ed25519_sk_<serial>` for the one inserted YubiKey.
  - `--key` selects another hardware key trusted by `share/allowed_signers`.
- Signing asks for the FIDO2 PIN and touch; signature verification precedes publishing.
- Assets: ISO, `<iso>.sha256`, `<iso>.sha256.sig`.
  - Tag: `iso-YYYY.MM.DD-<rev12>`.

### USB

Writing requires the ISO and both signed checksum files together.
Unsigned builds are refused.

> [!CAUTION]
>
> `dctl iso usb` erases the whole **USB drive**.
> Choose an unmounted removable or USB disk, not a partition.

```sh
dctl iso usb /dev/sdX
dctl iso usb --iso ~/Downloads/dotfiles-REV.iso /dev/sdX
```

1. Run as the user; dctl elevates through sudo.
   - Direct root execution requires `--iso`.
2. Review the target and type its exact device path; `--yes` cannot skip this.
3. Wait for writing and sync to finish before using the USB.

- Refuses read-only, mounted, internal non-removable, or undersized disks.
- Requires serial/WWN identity and rechecks it after consent.
- Verifies the signature and ISO hash, hashes bytes written, and syncs writes.
  - It does not read the disk back; do not boot it after a write failure.

> _See [Release](#release) for signing without publication._

## Secrets

Secrets commands manage age ciphertext and private files in the user's home.
Run them as the normal user.

```sh
dctl secrets list
dctl secrets decrypt [name ...]
dctl secrets sync
dctl secrets rekey
dctl secrets verify-phrase
```

| Command       | Work                              |
| ------------- | --------------------------------- |
| list          | Manifest and target status        |
| decrypt       | Write selected plaintext targets  |
| sync          | Encrypt changed plaintext targets |
| rekey         | Re-encrypt all manifest secrets   |
| verify-phrase | Check recovery identity in memory |

- Default decrypt excludes staged entries; naming one includes it.
- Unchanged ciphertext keeps its recipients during sync; rekey changes all entries.

### Files and target rules

The manifest connects ciphertext to a target and its exact permissions.
Decryption replaces files atomically and keeps no plaintext backup.

```text
name:~/target/path:0600[:staged]
```

- Inputs under `secrets/`: `manifest`, `<name>.age`, `recipients`, `identities`.
- `identity.age` holds the phrase-wrapped recovery identity.
- Targets must stay inside `$HOME`, outside the checkout, without symlink/mount crossings.
- Decrypt asks before overwriting different content and applies the manifest mode.

### Unlocking and failures

Age tries plugin identities first, then offers the recovery phrase on failure.
Cancelling a plugin prompt stops without offering the phrase.

- Mutations through sync, rekey, key enrollment/removal take an exclusive checkout lock.
- Decrypt/sync/rekey/verify-phrase refuse `AGEDEBUG`, which can expose PINs and keys.
- Rekey stages all ciphertext first; decryption failure leaves `secrets/` unchanged.

### Secret recovery

The age phrase unlocks the recovery identity, not the disk.
Secret recovery does not restore user data; keep a separate off-site backup.

1. Keep the age phrase and LUKS recovery key available outside the encrypted disk.
2. Verify the phrase and rehearse decryption with both YubiKeys removed:

   ```sh
   dctl secrets verify-phrase
   dctl secrets decrypt <name>
   ```

## Keys

Keys commands prepare YubiKeys, manage enrollment, and replace the boot TOTP.
Key preparation and disk enrollment are separate operations.

```sh
dctl keys enroll
dctl keys status
sudo dctl keys status
dctl keys remove <serial>
```

### Enroll identities

Enrollment prepares PINs, age access, and a release-signing identity.
Later failures do not undo earlier changes to the key or checkout.

1. Insert only the intended YubiKey and run `dctl keys enroll` as the user.
2. Follow PIN, age identity/rekey, and signing-key prompts.
3. Inspect status before retrying a failure; repeat with the other key.

- Age uses the PIV PIN without touch; release signing uses the FIDO2 PIN and touch.
- Changes `secrets/`, `share/allowed_signers`, and local SSH signing-key files.

### Enroll disk tokens and recovery

Disk enrollment keeps the passphrase and adds two PIN-required, no-touch tokens.
The recovery key is an independent disk-unlock method.

1. Use the user setup flow for its phase-1 gate and lock confirmation:

   ```sh
   dctl setup luks
   ```

2. Insert one key at a time; enter the disk passphrase and FIDO2 PIN as prompted.
3. Copy the recovery key onto paper, store it away from the machine, and confirm.
   - Deferring paper confirmation leaves work for the next run.

- Direct `sudo dctl keys luks` runs without the setup gate.
- Three or fewer FIDO2 PIN tries blocks enrollment, which can spend three tries.
  - Restore tries with one correct PIN, then retry:

    ```sh
    ykman --device <serial> fido access verify-pin
    ```

- Resetting FIDO2 erases its credentials, including signing and disk credentials.

> _See [Lock state](#lock-state) for enrollment records._

### Inspect, replace, and remove

Removing an enrolled identity does not revoke its disk access.
Retain a tested passphrase or recovery key before deleting disk slots.

- User status reports identities; LUKS-header inspection needs sudo.
- `keys remove` removes the age recipient and signer, then rekeys secrets.
  - LUKS tokens and credentials on the YubiKey remain.
  - Partial failures report which changes need a retry.
- The header does not identify the key that created a token; identify its slot manually:

  ```sh
  sudo dctl keys status
  sudo systemd-cryptenroll --wipe-slot=<slot> <device>
  ```

> _See [Secure Boot](#secure-boot) before using `sudo dctl keys totp` to reseal._

### LUKS unlock

Insert the YubiKey before boot; unlock does not wait for a late insertion.
Token errors fall back to the passphrase or recovery-key prompt.

- Dctl-enrolled tokens ask once for **LUKS2 token PIN**, without touch.
- The `luks` stage removes legacy `fido2-device=auto` boot options to restore fallback.

## Porkbun DNS

Porkbun commands change remote DNS records, not the machine's resolver.
They require an explicit domain and refuse root/sudo execution.

```sh
dctl porkbun check <domain>
dctl porkbun list <domain>
dctl porkbun create <domain> <name> <type> <content>
dctl porkbun edit <domain> <id> --content VALUE
dctl porkbun delete <domain> <id>
```

### Credentials

Credentials are read as data, never evaluated as shell code.
Keep values out of shell history and editor backups.

1. Create credentials in the [Porkbun dashboard](https://porkbun.com/account/api).
2. Enable API access for the domain.
3. Provision `~/.local/share/dotfiles/porkbun.env` as a user-owned regular file.
   - Mode must be `0600`; symlinks are refused.
   - Use exactly two unquoted assignments; no comments, blanks, or shell expressions:

     ```text
     PORKBUN_API_KEY=value
     PORKBUN_API_SECRET_KEY=value
     ```

- Dctl does not decrypt credentials automatically.
- Restore ciphertext-backed credentials with `dctl secrets decrypt porkbun.env`.

### Record fields

Use the domain's listed record IDs for edits and deletion.
Edit preserves name, type, notes, and omitted TTL/priority fields.

- Use lowercase ASCII/punycode domains, without scheme, path, or trailing dot.
- Create relative lowercase subdomains; `@` or empty means the root.
  - Quote wildcard names such as `'*'`.
- Use positive decimal IDs, without signs or leading zeros.
- Supported types are A, AAAA, CNAME, TXT, MX, SRV; content uses Porkbun's format.
- Set `--ttl N` in seconds; omitted create TTL or zero uses the account minimum.
- Set `--prio N` for MX/SRV only, 0–65535; create defaults to zero.
- Duplicate name/type/content/priority is refused even if TTL differs.

### Consent and uncertain outcomes

Writes preview the change, recheck state after consent, and verify API readback.
These checks do not prove public DNS propagation or prevent all concurrent changes.

- TTY writes ask default **No**; global `--yes` grants consent.
  - JSON/non-TTY writes require `--yes`; `--plain` still permits TTY consent.
- `--dry-run` needs no consent.
  - Create asks the provider to validate; edit/delete preview locally only.
- `check` tests authentication, authority evidence, and a create dry-run.
  - It does not prove live-write permission.
- Provider warnings, missing authority evidence, or changed targets block writes.
- Failed mutation/readback can mean the change succeeded despite a nonzero exit.
  - Inspect `list` before retrying; there is no automatic retry or rollback.

## Repos

Repository management runs through setup and update, with no standalone repos command.
Setup clones missing repositories; update only fast-forwards eligible checkouts.

```sh
dctl setup repos
dctl update repos
```

- Catalog: `packages/repos.lst`, in clone order, with one entry per line:

  ```text
  owner/name ~/destination
  ```

- Destinations start with `~/` or `/`; existing directories are never replaced.
- Setup checks checkout roots/origins and fetches full history for shallow dotfiles.
  - No manual unshallow step is needed after installation.
- Update skips both `~/dotfiles` and the `DOTFILES` checkout.
- Tracked changes, detached HEAD, no upstream, ahead/diverged branches prevent merging.
  - Untracked files are excluded from the dirty check.
- Missing repositories need setup; broken checkouts need manual repair.
  - Move a broken checkout aside, then rerun `dctl setup repos`.

## Environment

Root discovery locates the checkout without changing user-target ownership.
Run user commands as the owning user.

- `DOTFILES` overrides the checkout root, which must contain `AGENTS.md`.
- Default under sudo: `~SUDO_USER/dotfiles`; otherwise: `$HOME/dotfiles`.

## Manual hardware acceptance

These checks require the Framework Desktop and real YubiKeys.
The VM cannot prove hardware enrollment, boot-code stability, or recovery access.

1. Confirm Setup Mode and TPM enablement before enrollment.
2. Before TOTP sealing, confirm the boot screen reports **Boot TOTP not set up yet**.
3. After enrollment, confirm own-key enforcement and setup status:

   ```sh
   dctl setup --status luks secureboot totp
   ```

4. Compare boot TOTP with the authenticator across two reboots.
5. Check each YubiKey separately before boot: one PIN prompt, no touch.
6. With both keys removed, unlock using the paper recovery key.
7. Decrypt secrets with each key separately, then rehearse phrase recovery.
8. Check network and Bluetooth devices:

   ```sh
   nmcli device status
   bluetoothctl list
   ```

   - Expect no Wi-Fi interface; pair and use a Bluetooth device.

9. Inspect kernel logs for firmware-load errors:

   ```sh
   journalctl -k -b
   ```

- Confirm token errors fall back to passphrase/recovery without an emergency shell.
- Confirm the TOTP hook does not delay LUKS unlock.
- With Secure Boot off, expect **NO BOOT TOTP**; restore enforcement before proceeding.
  - Investigate any remaining mismatch before resealing.
- Avoid repeated wrong PINs that can block a key; cancel age-phrase fallback in key tests.

> _See [Secure Boot](#secure-boot) for investigation and explicit resealing._
