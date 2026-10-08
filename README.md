<h1 align="center">🍙 dotfiles 🍙</h1>

<p align="right">
    <a href="https://github.com/cogikyo/dotfiles/stargazers">
        <img
            src="https://img.shields.io/github/stars/cogikyo/dotfiles?color=ecc45d&logo=apachespark&labelColor=24283b&logoColor=ecc45d&style=for-the-badge"
            title="what is love, baby don't hurt me"
        >
    </a>
</p>

<p align="center">
    <kbd>
        <img
            alt="dotfiles neonlights banner"
            src="https://github.com/cogikyo/dotfiles/blob/master/share/dotfiles-banner.gif?raw=true"/>
    </kbd>
</p>
<p align="center">

---

> **[I use arch, btw](https://wiki.archlinux.org/title/Arch_Linux)**
>
> _"...we *{do these}* things **not** because they are easy, but **because they are hard**_,"<br>
>
> &emsp;&emsp;_"because that goal will serve to **organize** and **measure** the best of our energies and skills_,"<br>
>
> &emsp;&emsp;&emsp;&emsp;_"because that challenge is one that we are **willing to accept**, one we are **unwilling to postpone**_..."

---

## 👨‍💻 Software

<details open>
<summary>🖥️ <b>Display</b></summary>

- Display Server: [Wayland](https://wiki.archlinux.org/title/Wayland)
- Compositor: [Hyprland](https://hyprland.org/)
- Widgets: [eww](https://github.com/elkowar/eww)
- Wallpaper: [mpvpaper](https://github.com/GhostNaN/mpvpaper)

</details>

<details open>
<summary>🎯 <b>Core Applications</b></summary>

- Editor: [neovim](https://neovim.io/)
- Browser: [Firefox](https://www.mozilla.org/en-US/firefox/developer/) (with custom [firefox css](https://github.com/cogikyo/vagari.firefox))
- File Explorer: [xplr](https://github.com/sayanarijit/xplr)
- Terminal: [kitty](https://sw.kovidgoyal.net/kitty/)
- Shell: [zsh](https://wiki.archlinux.org/title/zsh)

</details>

<details open>
<summary>🍎 <b>Notable Applications</b></summary>

- Vector Graphics: [inkscape](https://inkscape.org/)
- Music: [spotify](www.spotify.com) with [playerctl](https://github.com/altdesktop/playerctl)
- Music Visualizer: [glava](https://github.com/jarcode-foss/glava)

</details>

### 🎥 Appearance

<details open>
<summary>🎨 <b>Design</b></summary>

- Color Scheme: [vagari](https://github.com/cogikyo/vagari#palette) (work in progress)
- Cursors: [catppuccin-macchiato-light](https://github.com/catppuccin/cursors)
- Icons: [Papirus-Dark](https://github.com/PapirusDevelopmentTeam/papirus-icon-theme)

</details>

<details open>
<summary>💬 <b>Fonts</b></summary>

- Sans Serif: [Satoshi](https://www.fontshare.com/fonts/satoshi)
- Serif: [Sentient](https://www.fontshare.com/fonts/sentient)
- Display: [Chillax](https://www.fontshare.com/fonts/chillax)
- Monospace: [Iosevka Vagari](https://typeof.net/Iosevka/), hinted
- Other:
  - [Nerd Font Symbols](https://github.com/ryanoasis/nerd-fonts)
  - [Noto Color Emoji](https://fonts.google.com/noto/specimen/Noto+Color+Emoji)
  - [Archivo (lock screen)](https://fonts.google.com/specimen/Archivo)
  - [Albert Sans](https://fonts.google.com/specimen/Albert+Sans)
  - [Lora](https://fonts.google.com/specimen/Lora)
  - [Architects Daughter](https://fonts.google.com/specimen/Architects+Daughter)

</details>

<details open>
<summary>🧰 <b>Hardware</b></summary>

- Keyboard: [Svalboard](https://svalboard.com/), configured with [Vial](https://get.vial.today/)
  - Keymap: [`share/keyboards/svalboard.vil`](share/keyboards/svalboard.vil), browsable with the [`keys` viewer](cmds/cmd/keys/README.md)

##### Misc Hardware

- Monitor: [SAMSUNG UR59 Series 32-Inch 4K UHD (3840x2160)](https://a.co/d/bZtUse0)
- Computer: Framework Desktop, AMD Strix Halo, Ethernet only
- Microphone: [Shure SM57](https://www.amazon.com/gp/product/B0000AQRST)
  - Audio Interface: [Scaarlett Solo 3rd Gen](https://www.amazon.com/gp/product/B07QR6Z1JB)
- Camera: [Canon EOS M50 Mark II](https://www.amazon.com/gp/product/B08KSLW8N3)
  - Lens: [Sigma 16mm f/1.4](https://www.amazon.com/gp/product/B084KYHYKN)

</details>

<a id="installation"></a>

## 🛠️ Installation

This system is built specifically for the Framework Desktop (AMD Strix Halo, Ethernet only).

- Offline Arch ISO: installs the base system and dotfiles without a network.
- LUKS2 and YubiKey unlock: encrypts the disk; keeps passphrase and paper recovery options.
- btrfs with Snapper: keeps root snapshots for recovery and manual rollback.
- Limine and unified kernel images (UKIs): supports signed boot images and snapshot boot.
- Own-key Secure Boot and TPM-sealed TOTP: adds signatures and boot-state evidence.
- Two-phase setup: makes the desktop work before asking separately to lock it down.

> _See the [dctl guide](cmds/cmd/dctl/README.md) for commands and recovery._

There is no model check, but this system is only tested on that machine.
AMD packages, no Wi-Fi packages, NVMe selection, and firmware menus limit portability.

> [!CAUTION]
>
> `dctl iso usb` erases the whole **USB**.
> `dctl install` erases the whole **target drive**.

```text
ISO ─▶ USB ─▶ firmware prep ─▶ install ─▶ reboot
  ┌─────────────────────────────────────────┘
  ▼
phase 1 ─▶ lock confirmation ─▶ luks ─▶ secureboot ─▶ reboot
  ┌─────────────────────────────────────────────────────┘
  ▼
totp ─▶ reboot and compare
```

### 1. Get the ISO

Download the ISO with its `.sha256` and `.sha256.sig` files from [Releases](https://github.com/cogikyo/dotfiles/releases/latest).

Or build a new one on an existing Arch machine:

```sh
dctl iso build
dctl iso test
dctl iso release # optional
```

- Build from a clean, committed `master`.
- USB writing needs signed checksums; release signs them before offering to publish.

> _See [ISO](cmds/cmd/dctl/README.md#iso) for requirements and test overrides._

### 2. Write the USB

Writing verifies the signed checksum and requires typed device-path consent.
Add `--iso /path/to/dotfiles-REV.iso` for a downloaded ISO.

```sh
dctl iso usb /dev/sdX
```

### 3. Prepare the Framework

Prepare the firmware now to avoid another visit during security enrollment.

1. Back up any data on the internal disk.
2. Press F2 and choose **Erase all Secure Boot Settings** to enter Setup Mode.
3. Leave the TPM enabled and press F10 to save.

### 4. Install

The installer creates the encrypted system and desktop from bundled packages.

1. Press F12 and choose the USB in UEFI mode; the ISO starts the installer on tty1.
2. Enter the login password for `cullyn`, the timezone, and the disk passphrase.
3. Check the selected disk before confirming **Erase `<disk>` and install?**.
4. At the completed summary, remove the USB and reboot.

### 5. First login

Unlock with the passphrase, log in, connect Ethernet, and insert a YubiKey.

```sh
dctl setup
```

Phase 1, **make it work**, then runs:

1. Check system services and Ethernet/DNS (`system`, `network`).
2. Apply packages and home settings (`packages`, `home`).
3. Install extra applications in the background (`extra`).
4. Restore secrets, SSH access, and repositories (`secrets`, `ssh`, `repos`).
5. Set up Firefox and local certificates (`firefox`, `certs`).

Setup asks you to accept the plan; zsh prints the next action as `dctl · …`.

> _See [Setup](cmds/cmd/dctl/README.md#setup) for stages and first-login details._

### 6. YubiKeys, Secure Boot, and boot TOTP

Phase 2, **lock it down**, secures the system after a separate confirmation.
Run setup as the normal user; `--yes` never accepts the lock confirmation.

1. Enroll both YubiKeys for disk unlock and save the recovery key onto paper.
2. Enroll the Secure Boot keys, then reboot with a YubiKey inserted.
3. Once Secure Boot is enforced, run setup again to seal and verify the boot TOTP.
4. Reboot and compare the boot code with the authenticator before unlocking.

After enrollment, a missing or wrong boot code means **do not unlock**.
Investigate before resealing.

> _See [Secure Boot](cmds/cmd/dctl/README.md#secure-boot) for the lock procedure._
> _See [hardware acceptance](cmds/cmd/dctl/README.md#manual-hardware-acceptance) for real-machine checks._

### Day-to-day use

```sh
dctl update --only {scope}
```

> _See [Update](cmds/cmd/dctl/README.md#update) for upgrades and package reconciliation._
