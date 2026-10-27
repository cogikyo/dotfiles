# dctl

`dctl` installs and maintains this single-user Arch Linux system.
It builds the offline ISO, applies configuration, and runs attended updates.
On a Mac, it sets up and syncs Homebrew packages and shell configs; see [macOS](#macos).

- Detects AMD/Intel hardware and records a machine profile, with a preset for a recognized Framework Desktop.
- Installs the `cullyn` account with the chosen hostname.

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

The ISO is not signed for Secure Boot, so disable Secure Boot in firmware before booting it (on by default on Dell and most OEM laptops).
dctl enrolls its own keys later during [setup](#secure-boot).

> [!CAUTION]
>
> `dctl install` erases the whole **target drive**.
> Back up its data before confirming the erase.

```text
prepare ──▶ disk consent ──▶ install steps ──▶ unmount ──▶ summary
```

- Requires root, UEFI, and the dctl live ISO; stock Arch media are refused.
- First shows detected hardware and asks you to confirm a recognized Framework preset or choose a Wi-Fi policy and 0, 1, or 2 YubiKeys (default 2).
  - Wi-Fi is offered when a wireless driver is detected; missing CPU/GPU detection adds vendor questions.
  - Wi-Fi defaults off on desktops and on when a battery is detected; the Framework preset forces it off.
  - Framework DMI identifiers are not configured yet, so all machines currently use the questions.
  - NVIDIA detection warns but does not stop installation; NVIDIA is unsupported.
- Then prompts for the hostname, login password, timezone, and LUKS passphrase.
  - The hostname defaults to `costello` with the Framework preset; custom machines have no default.
  - After installation, the hostname lives only in `/etc/hostname`, outside the machine record.
- After success, remove the USB, reboot, and unlock with the disk passphrase.
- Failed install steps can leave a partly modified disk; there is no rollback.
  - Unmount and LUKS-close cleanup is attempted after success or failure.
- Most install steps show one spinner row each and write their command output to `/var/log/dctl-install.log` on the live ISO.
  - After successful installation, the log is copied to the installed system's `/var/log/dctl-install.log`.
  - If a logged step fails, dctl shows the log path and up to 20 trailing lines.

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
- Writes `/etc/dctl/machine` and installs base packages plus the CPU/GPU vendor sets selected by that record.
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
machine ─▶ system ─▶ network ─▶ packages ─▶ home ─▶ extra (background)
  ┌─────────────────────────────────────────────────────┘
  ▼
secrets ─▶ bridge ─▶ ssh ─▶ repos ─▶ firefox ─▶ certs
  ┌───────────────────────────────────────────────┘
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

Without `--status`, named stages and `--from` stages always pull in and check their direct needs.
For non-lock stages, each need pulls in its own needs only when it is not OK, recursively.

- Omit stage names to show the full plan and apply pending work.
- Name stages to redo them.
- Use `--from <name>` to redo that stage and later ones; `vpn` must be named.
- Lock stages whose checks pass show OK and are not redone, even when named or reached by `--from`.
- Use `--yes` to accept the plan; rerun without it for lock work.
  - Without a terminal, the plan prompt requires `--yes`.
- Use `--status` for a read-only check; pending, failed, or blocked work exits 1.
  - Manual or deferred work can exit 0; inspect the rows, not only the exit code.

### Stages and prerequisites

Required phase-1 stages run in the table's order, from machine through certs.
The optional `vpn` stage does not gate security enrollment.

| Stage      | Needs                       | Work                              |
| ---------- | --------------------------- | --------------------------------- |
| machine    | —                           | Hardware profile                  |
| system     | machine                     | Overlay, rendered files, services |
| network    | —                           | Network and DNS                   |
| packages   | machine, system, network    | Base, vendor, AUR, local packages |
| home       | packages                    | Links and fonts                   |
| extra      | home, network               | Background apps                   |
| secrets    | system                      | Restore private files             |
| bridge     | network, extra, secrets     | Pinned Cursor bridge and service  |
| ssh        | secrets, network            | GitHub access                     |
| repos      | ssh                         | Clone and fetch history           |
| firefox    | packages                    | Profile and config                |
| certs      | firefox                     | Local CA and certificates         |
| vpn        | secrets                     | Optional work VPN                 |
| luks       | All required phase-1 stages | Profile's 0–2 keys and recovery   |
| secureboot | luks                        | Keys and boot signatures          |
| totp       | secureboot                  | Seal the boot TOTP                |

The `system` stage creates users declared in `system/etc/sysusers.d/*.conf` with `systemd-sysusers`.
It checks units in its `running` list, currently only `sshd.service`, and starts them if inactive.

- Root stages other than lock stages are checked as the user first; only stages that are not OK and lock stages need sudo.
  - `dctl setup home` checks `packages`; if it is OK, `system` and `network` are not checked and no sudo is needed.
- Consecutive root stages share one sudo child; user stages stay in your session.
  - When sudo needs authentication, setup explains which stages it will check or run as root and prompts with `dctl: sudo password for <user>:`.
  - If no terminal can ask for the sudo password and no cached authentication is available, the stage fails with a message to run the named stages in a terminal.
- Root runs `machine`, `system`, `packages`, `luks`, `secureboot`, and `totp`.
- `certs` uses sudo from the user session; user stages refuse root execution.

### Machine profile

On an existing install, run `dctl setup machine` before system, package, or lock work if `/etc/dctl/machine` is missing or invalid.
Installation writes the record before applying setup stages.
The `machine` stage detects hardware and records the Wi-Fi policy and YubiKey count without asking for or changing the hostname.

The record uses `key=value` lines; all keys are required: `preset`, `cpu`, `gpus`, `form`, `backlight`, `wifi`, `wlan`, `yubikeys`, and `lane`.
Unknown keys, including `hostname`, are rejected.
Detection reads DMI, CPU/GPU vendors, battery/backlight, wireless drivers, and CPU topology.
Confirm the Framework preset when the machine is recognized.
Wireless detection checks PCI class `0x0280` and module aliases from every installed kernel; unmatched devices add nothing to `wlan`.
Alias lookup blocks installation and `dctl setup machine` if no installed kernel has PCI aliases or an alias file cannot be read.

- `system` renders CPU-affinity files and the Wi-Fi module blacklist from the record; duplicate paths in `system/` stop the stage.
- The CPU lane contains the highest core's SMT siblings on machines with at least four cores; smaller machines have no lane.
  - System `CPUAffinity` excludes the lane; Bluetooth, rtkit, audio services, hyprd, ewwd, and dunst use it.
  - User-service drop-ins go under `/etc/systemd/user/<unit>.service.d/`, not the home config.
  - Hyprd rebuilds and mpvpaper exclude the lane, with no explicit pinning when the record or lane is absent.
- `wifi=off` blacklists the recorded wireless drivers; `wifi=on` removes that blacklist.
- Hyprland reads the record through `config/hypr/machine.lua`.
  - Brightness uses `brightnessctl` with `backlight=yes`; otherwise ddcutil uses each connected display's DDC bus found through sysfs.
  - Lua checks only `lane` and `backlight`; if either is missing or invalid, dunst runs unpinned and brightness uses DDC.

### Status and output

Status rows distinguish work dctl can apply from work that needs your attention.
The summary and shell hint are the next-action reference.
The summary also lists phase-1 stages that did not run.

| Pill  | Meaning                   |
| ----- | ------------------------- |
| OK    | Done                      |
| TODO  | Work to run               |
| FIX   | Manual action             |
| LATER | Reboot or firmware step   |
| OPT   | Runs only when named      |
| WAIT  | Needs earlier work        |
| WARN  | Incomplete or interrupted |
| ERR   | Failed or unknown         |

- `ASK` marks a prompt answer; `RUN` marks a command.
- With pending work and no failed stages, `--status` ends with `WARN` counts, such as `6 to run, 3 waiting, 1 later`; it still exits 1.
  - A failed stage makes the status summary `ERR`.
- Esc at a dctl prompt inside a stage skips the rest of that stage with `FIX` and `skipped; run dctl setup <stage>`.
  - Dependents show `WAIT`; independent stages continue.
- Ctrl+C stops the run and exits 130; canceled or interrupted stages show `WARN` in the summary.
- A full run records its next action under `$XDG_STATE_HOME/dctl/next`.
  - Default: `~/.local/state/dctl/next`; zsh prints it as `dctl · …`.
  - Hyprd shows **Finish setting up** at each login while setup is pending; click it to run `dctl setup` in kitty.

### First-login details

Network and authentication failures can leave dependent stages waiting.
Independent stages can still run.

- Failed network checks offer **Retry** or **Skip online steps**.
  - **Skip online steps** leaves `network` as `FIX` with `online steps skipped`, rather than a failure.
- For blocked phase-1 stages, summary `WAIT` rows list every unmet prerequisite.
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

- `packages` reads base, selected AMD/Intel vendor, AUR, and local inputs under `packages/`.
  - Intel adds `sof-firmware`; `linux-firmware-intel`, `linux-firmware-mediatek`, and `linux-firmware-realtek` stay in base.
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
  - Its `extra-docker` items enable `docker.socket` and add you to the `docker` group, which takes effect at your next login.
  - Its `extra-bun` item runs `bun install --frozen-lockfile --ignore-scripts` when a locked OpenCode dependency for this platform is missing or has the wrong version.
  - Output: `~/.local/state/dctl/extra.log`; failures show up to eight recent lines with progress output and terminal codes removed.
  - An interrupted install shows `WARN` with `interrupted`, without a log tail.
  - Setup waits before later root work, before `certs`, and at the end.
- First Ctrl+C stops new stages and waits for `extra`; a second sends SIGINT.
  - Setup never sends SIGTERM or SIGKILL to pacman.
- `bridge` installs the pinned, patched checkout at `~/.local/share/cursor-sdk-bridge` from `packages/cursor-sdk-bridge/` and runs `npm ci --omit=dev`.
  - It restarts `cursor-bridge.service` after checkout or dependency changes, or if inactive.

> _See [Package reconciliation](#package-reconciliation) for interactive selection._

### Work VPN

Work VPN setup runs only when named.

```sh
dctl setup vpn
```

VPN profiles come from `cmds/config/hyprd.yaml` and staged secrets.
Imports use hyprd; newly staged plaintext is removed even after failure.

### Lock confirmation and root execution

Lock work waits for every required phase-1 stage to be OK and asks separately.
Naming a lock stage pulls in its full dependency chain before checks and still requires consent.

> [!CAUTION]
>
> Direct root setup bypasses the plan, dependency pull-in, and lock confirmation.
> Run first setup as the normal user.

- `--yes` never accepts the lock confirmation.
- Esc at the lock confirmation means **No**; Ctrl+C stops the run.
- Named root stages under sudo run directly; each checks its own conditions.

> _See [Secure Boot](#secure-boot) for the enrollment procedure._

### Secure Boot

Enrollment trusts only the owner's keys.
It refuses automatic option-ROM trust and does not add Microsoft keys.

1. Enter firmware setup and clear the Secure Boot keys to enter Setup Mode.
   On the Framework Desktop, press F2 and choose **Erase all Secure Boot Settings**.
   Leave the TPM enabled.
2. Save (F10 on Framework); prepare PINs and age identities for the profile's YubiKeys before locking down.
3. Run setup as the user and accept the lock confirmation once phase 1 is OK:

   ```sh
   dctl setup
   ```

4. Complete the `luks` prompts for the profile's 0–2 keys and confirm the paper recovery copy.
5. Let `secureboot` enroll keys and sign images, then reboot with a configured YubiKey inserted or unlock with the passphrase.
   - If Secure Boot is still off, enable it in firmware setup and save (F2/F10 on Framework).
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

The hook prints the first code or **Boot TOTP not set up yet** on the console before the passphrase prompt.
The unlock prompt waits up to 15 seconds for this output.
The code refreshes every 30 seconds on a new line and can appear while you type.

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
These files and the machine profile are under `/etc/dctl/`.

| File                        | Records                            |
| --------------------------- | ---------------------------------- |
| `machine`                   | Hardware and setup policy          |
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

Update runs attended upgrades, configuration checks, and rebuilds from the current checkout.
Failed steps are reported; later steps continue and the summary gives a retry command.

```sh
dctl update
dctl update repos cmd
dctl update --only dctl,hyprd
dctl update --dirty --only dctl
```

- Run as the normal user; root execution is refused.
- Update never pulls `~/dotfiles` or the `DOTFILES` checkout; manage it separately.
  - macOS update does pull it; see [macOS](#macos).
- Any failed step makes the final result nonzero.
- Ctrl+C can interrupt pacman; update has no setup-style protected wait.

### Selection and step order

With no names, each step asks for consent, default **Yes**.
Named steps omit that prompt and retain this order.

1. `pacman`: upgrade official packages.
2. `aur`: upgrade AUR packages, excluding local recipes.
3. `packages`: reconcile installed packages with the lists.
4. `setup`: reapply drifted `system`, `home`, and `firefox` setup stages.
5. `repos`: fast-forward eligible catalog checkouts, excluding dotfiles.
6. `cmd`: rebuild dotfiles commands and installed local recipes whose versions differ.
7. `bun`: install OpenCode plugin dependencies from `config/opencode/bun.lock`.
8. `bridge`: repair the checkout and service as in `setup bridge`.
9. `go`: update eligible module-release tools.
10. `rust`: run `rustup update`, or `rustup default stable` when no default is set, then update crates; skip if rustup is absent.

- `--all` or global `--yes` skips step/recipe prompts, accepts the setup plan, and passes `--noconfirm`.
  - It never installs or removes package-list drift.
  - Building uncommitted `cmds/` still needs `--dirty`.
- `--only NAME,...` limits `cmd` to named commands or local recipes.
  - With no step names it selects `cmd`; explicit step names must include `cmd`.

`setup` uses live checks and fixes only pending items, without pulling in other setup stages.
It checks `system` as the user first and uses sudo when needed to check or reapply changed `system/` files, including Firefox policies.
Restart Firefox after policy or profile configuration changes; update does not restart it.

### Command and tool rebuilds

Dotfiles binaries are replaced only when their bytes differ.
Ewwd, newtab, and keys restart only when replaced.

- **hyprd** replaces itself via `hyprd rebuild`.
  - Skips during full lock, an OpenCode refresh job, or when the daemon is stopped.
- **Uncommitted `cmds/`**: `--dirty` builds dotfiles commands without extra consent, even with `--all` or `--yes`.
  - Without `--dirty`, `--all` or `--yes` warn and skip the build.
  - With none of these flags, a terminal asks for consent, default **No**; without a terminal, the build fails and names `--dirty`.
- **Local recipes** with different installed versions ask separately, default **Yes**.
  - Rebuilds via `makepkg -sfiC`, cleaning `src/`; `--all` skips prompts.
- **Go tools** need module-proxy checksum metadata; locally built tools are skipped.
  - Scans `GOBIN`, or the first GOPATH's `bin`; installs eligible tools at `@latest`.
- **Cargo crates** from `cargo install --list` are reinstalled with `cargo install --locked`; path, git, and other-registry installs are skipped.
  - Cargo skips a crate already at its latest version, but rebuilds one installed with custom features or profile using the defaults.

### Package reconciliation

Reconciliation treats base, selected vendor lists, AUR, extra, and local recipes as the desired set.
It needs `/etc/dctl/machine`; if missing, run `dctl setup machine` first.
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
Follow the hardware vendor's instructions for the chosen release and updater.

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
- Resolves base, both AMD/Intel vendor sets, AUR, and local recipes; missing packages stop the build.
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
Dctl asks for each YubiKey's PIV PIN once per run, reusing it only after a successful decrypt; a failed decrypt clears the cached PIN.

- Mutations through sync, rekey, key enrollment/removal take an exclusive checkout lock.
- Decrypt/sync/rekey/verify-phrase refuse `AGEDEBUG`, which can expose PINs and keys.
- Rekey stages all ciphertext first; decryption failure leaves `secrets/` unchanged.

### Secret recovery

The age phrase unlocks the recovery identity, not the disk.
Secret recovery does not restore user data; keep a separate off-site backup.

1. Keep the age phrase and LUKS recovery key available outside the encrypted disk.
2. Verify the phrase and rehearse decryption with all YubiKeys removed:

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
3. Inspect status before retrying a failure; repeat for each key you plan to use.

- Age uses the PIV PIN without touch; release signing uses the FIDO2 PIN and touch.
- Changes `secrets/`, `share/allowed_signers`, and local SSH signing-key files.

### Enroll disk tokens and recovery

Disk enrollment keeps the passphrase and adds the profile's 0–2 PIN-required tokens, which also require touch when the key does.
The recovery key is an independent disk-unlock method.
With `yubikeys=0`, `luks` still requires confirmed paper recovery and permits Secure Boot and boot-TOTP setup.

1. Use the user setup flow for its phase-1 gate and lock confirmation:

   ```sh
   dctl setup luks
   ```

2. If the profile requires YubiKeys, insert one at a time and enter the disk passphrase and FIDO2 PIN as prompted.
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

For token unlock, insert the YubiKey before boot; unlock does not wait for a late insertion.
Token errors fall back to the passphrase or recovery-key prompt.

- Dctl-enrolled tokens ask once for **LUKS2 token PIN**, plus touch when the key requires it.
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

## Cloudflare sites

`dctl cloudflare` previews changes that make Cloudflare match `~/cogikyo/cogikyo/vagarios/sites.toml`.
`apply` writes the tunnel ingress, proxied DNS CNAMEs and WAF block rules, then verifies API readback.
Run as your normal user, without sudo.

```sh
dctl cloudflare
dctl cloudflare plan --sites /path/to/sites.toml
dctl cloudflare apply
dctl --yes --json cloudflare apply
```

- **`plan` is the default** and sends only GET requests.
- `--sites PATH` selects a different sites file for either command.
- `apply` asks for consent on a TTY; JSON/non-TTY use requires global `--yes`.

### Sites file and ownership

The [sites loader](../../internal/dctl/cloudflare/sites_linux.go) owns the format: a tunnel name, named `[allow]` lists, `[zone."name"]` defaults and ordered `[[site]]` entries with origins and optional `[[site.route]]` path rules.
IDs are resolved through the API; the account ID comes from the zones list.
Origins use `http`, `https` or `ssh` with an IPv4 host and port; `noTLSVerify` applies only to HTTPS.

A site's `allow` overrides its zone default; `public` means no restriction.
A declared apex site defaults to public, but an undeclared apex keeps the zone default.
Missing access policy is a load error; a non-public site needs a `[zone]` table so dctl can write its WAF rule.

- **Tunnel:** Replaces the whole ingress in file order: each site's routes, its origin, then one final 404.
  - The PUT can drop other tunnel config keys; the preview lists them.
- **DNS:** Creates one proxied CNAME per site pointing at this tunnel.
  - Deletes only CNAMEs targeting this tunnel that are absent from the file.
  - A conflicting record stops the plan; dctl overwrites nothing.
- **WAF:** Writes one block rule per non-public effective allowlist in each declared zone.
  - Owns refs prefixed `dctl-allow-` and replaces the legacy `vagarios-allowlist` rule.
  - Keeps other rules unchanged and in order; exceeding Free's five-rule limit stops the plan.

Nix reads the same file only to derive cloudflared's origin egress holes on edge.
It does not publish the sites' ingress.

### Token and age recovery

The token is data in `~/.local/share/dotfiles/cloudflare.env`, a user-owned regular file with mode `0600`; symlinks are refused.
Use one unquoted assignment, with no blanks, comments or shell expressions:

```text
CLOUDFLARE_API_TOKEN=value
```

The manifest entry is `cloudflare.env:~/.local/share/dotfiles/cloudflare.env:600`.
`dctl secrets sync` seals it as an age secret; `dctl secrets decrypt cloudflare.env` restores it.
It is non-staged, so setup can restore it when missing, as with [Porkbun credentials](#credentials).
Agent read/edit deny rules protect the plaintext file as they do `porkbun.env`.

The account-owned token `costello-dctl` has no expiry and is IP-filtered to home (`50.173.27.178`) and Trend VPN (`50.232.1.98`) as of 2026-10-27.
For this slice, the relevant grants are Cloudflare Tunnel Write, Zone Read, DNS Write and Zone WAF Write.
The command reads the plaintext token; it does not unlock age itself.

### Apply safety and edge checks

Apply rechecks the preview after consent, then sends WAF additions, tunnel ingress, DNS additions, DNS removals and WAF removals.
Changed block expressions cover the old or new policy first, then narrow after DNS changes, so a hostname is not published without its block rule.
On a failed mutation or readback, run `dctl cloudflare plan` before retrying; there is no automatic retry or rollback.
Readback proves API state, not that edge loaded the ingress or that a site responds.

After an ingress change, check edge's cloudflared unit journal for `Updated to new configuration`; restart that unit if the message is absent.
The 2026-10-27 cutover needed one restart; later live pushes remain untested.
An edge deploy that restarted cloudflared cut the tunnel for about 31 s because cloudflared waits a 30 s stop grace period.

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

## macOS

On macOS, dctl has only `setup` and `update`, and they work on a smaller catalog.
The darwin build selects it at compile time, and the Linux commands are not compiled in.
The Mac clones the public repository read-only and pulls from it; pushes happen elsewhere.

### Bootstrap

Install Homebrew, which also installs the Xcode Command Line Tools, then build dctl from a fresh clone:

```sh
NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
eval "$(/opt/homebrew/bin/brew shellenv)"
brew install go
git clone https://github.com/cogikyo/dotfiles ~/dotfiles
(cd ~/dotfiles/cmds && go build -o ~/.local/bin/dctl ./cmd/dctl)
~/.local/bin/dctl setup
```

- The Homebrew installer uses sudo; without passwordless sudo, drop `NONINTERACTIVE=1` and answer its prompts.
- Open a new shell after setup; the linked shell config puts Homebrew and `~/.local/bin` on `PATH`.

### Stages and steps

| Stage    | Needs    | Work                                        |
| -------- | -------- | ------------------------------------------- |
| packages | —        | Install missing `packages/brew.lst` entries |
| home     | packages | Link the shell and terminal-tool configs    |

- `home` links `~/.zshrc`, `~/.zshenv`, and the `config/` entries listed in `cmds/internal/dctl/home/mac.go`.
  - lazygit links into `~/Library/Application Support/lazygit`, where it looks on macOS.
- `packages` installs missing formulae and casks; it never removes unlisted ones.

Update steps run in this order, with the same consent and retry behavior as on Linux:

1. `dotfiles`: fast-forward `~/dotfiles` with `git pull --ff-only --no-rebase`.
2. `brew`: run `brew update` and `brew upgrade`.
3. `setup`: reapply drifted `packages` and `home` stages.
4. `cmd`: rebuild `~/.local/bin/dctl` when its bytes differ.

- Link changes in `mac.go` take effect on the run after `cmd` rebuilds dctl.
- The linked `zshenv` loads Homebrew's environment, so `ssh <mac> 'dctl update --all'` works without an interactive shell.
- `zshrc` loads it again because macOS's `/etc/zprofile` moves system paths ahead of Homebrew in login shells.

## Environment

Root discovery locates the checkout without changing user-target ownership.
Run user commands as the owning user.

- `DOTFILES` overrides the checkout root, which must contain `AGENTS.md`.
- Default under sudo: `~SUDO_USER/dotfiles`; otherwise: `$HOME/dotfiles`.

## Manual hardware acceptance

Run these checks on each target machine, with real YubiKeys when the profile requires them.
The VM cannot prove hardware enrollment, boot-code stability, or recovery access.

1. Check `/etc/dctl/machine` against the hardware and chosen Wi-Fi/YubiKey policy, then confirm Setup Mode and TPM enablement before enrollment.
2. Before TOTP sealing, confirm the boot screen reports **Boot TOTP not set up yet**.
3. After enrollment, confirm own-key enforcement and setup status:

   ```sh
   dctl setup --status luks secureboot totp
   ```

4. Compare boot TOTP with the authenticator across two reboots.
5. For each configured YubiKey, insert it before boot and check for one PIN prompt, plus touch when the key requires it.
6. With all YubiKeys removed, test both passphrase and paper recovery unlock.
7. Decrypt secrets with each configured age identity, then rehearse phrase recovery with all keys removed.
8. Check network and Bluetooth devices:

   ```sh
   nmcli device status
   bluetoothctl list
   ```

   - With Wi-Fi off, expect no Wi-Fi interface; with it on, verify the detected interface and a connection.
   - Pair and use a Bluetooth device.

9. Inspect kernel logs for firmware-load errors:

   ```sh
   journalctl -k -b
   ```

- Confirm token errors fall back to passphrase/recovery without an emergency shell.
- Confirm the first TOTP message appears before the unlock prompt, with a wait of at most 15 seconds.
- With Secure Boot off, expect **NO BOOT TOTP**; restore enforcement before proceeding.
  - Investigate any remaining mismatch before resealing.
- Avoid repeated wrong PINs that can block a key; cancel age-phrase fallback in key tests.
- Check rendered CPU-affinity files and `/etc/systemd/user/` lane drop-ins against the record, including no lane below four cores.
- Exercise brightness keys on backlight hardware and every connected DDC-capable display.
- With `yubikeys=0`, verify that recovery confirmation completes `luks` and unlocks the Secure Boot and boot-TOTP stages.

> _See [Secure Boot](#secure-boot) for investigation and explicit resealing._
