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

## 🛠️ Installation

The dctl ISO installs Arch and these dotfiles offline onto the Framework Desktop, with LUKS2, btrfs, Snapper, and Limine.
After the first login, setup makes the system work, then asks separately before it locks it down.
The [dctl guide](cmds/cmd/dctl/README.md) explains each command in detail.

> [!CAUTION]
> `dctl iso usb` erases the whole USB disk, and `dctl install` erases the whole target disk.

### 1. Get the ISO

Download the ISO with its `.sha256` and `.sha256.sig` files from [GitHub Releases](https://github.com/cogikyo/dotfiles/releases/latest), or build and publish one from an existing Arch machine:

```sh
dctl iso build
dctl iso test
git push
dctl iso release
```

The build needs a clean, committed `master`, and the release signs the checksum with a YubiKey.
See [ISO](cmds/cmd/dctl/README.md#iso) for requirements and test overrides.

### 2. Write the USB

```sh
dctl iso usb /dev/sdX
```

Add `--iso /path/to/dotfiles-REV.iso` for a downloaded ISO.
The command checks the signed checksum against `share/allowed_signers`, then asks you to type the device path.

### 3. Prepare the Framework

Back up the internal disk if it has data.
Put the firmware in Setup Mode before installing: F2 → **Erase all Secure Boot Settings** → F10 to save.
Leave the TPM enabled.

### 4. Install

Press F12 and choose the USB in UEFI mode; `dctl install` starts on tty1.
Enter the login password for `cullyn`, the timezone, and the LUKS passphrase, then confirm `Erase <disk> and install?`.
Output runs through prepare, disk consent, install steps, unmount, and summary.
At the summary, remove the USB and reboot.

### 5. First login

Unlock LUKS with the passphrase, log in through SDDM, and connect Ethernet.
Insert a YubiKey, then run:

```sh
dctl setup
git -C ~/dotfiles fetch --unshallow
```

Phase 1, **make it work**, runs system → network → packages → home → extra → secrets → ssh → repos → firefox → certs.
Setup shows a plan, asks once to run pending work, and uses sudo for root stages.
Extra packages run in the background with output in `~/.local/state/dctl/extra.log`; secrets use the PIV PIN or recovery phrase.
The first Ctrl+C stops new stages and waits for the background package work to finish; a second press sends SIGINT.
Tailscale and VPN are optional and run only when named: `dctl setup tailscale` or `dctl setup vpn`.
Tailscale login shows a QR code; it does not open a browser.
When an SSH key is first used, the keyring asks for its passphrase; choose the option to unlock it automatically at login.
Setup creates the Firefox Developer Edition profile if needed; quit Firefox if it asks, then rerun `dctl setup firefox certs`.
Restart Firefox after customization.

Zsh prints the next setup action as `dctl · …` at shell start.
Follow that hint until setup is complete.

### 6. YubiKeys, Secure Boot, and boot TOTP

If a YubiKey still needs its PINs and age identity, run `dctl keys enroll` with only that key inserted.
Repeat for the other key, then continue with `dctl setup` as your normal user.

Phase 2, **lock it down**, waits until every required phase-1 stage is OK and asks **Everything works. Lock it down now? YubiKeys, then Secure Boot**.
`--yes` never gives this confirmation.
Direct `sudo dctl setup` runs of `luks`, `secureboot`, or `totp` bypass the plan and lock confirmation; use the user flow for first setup.

1. The `luks` stage enrolls both YubiKeys, one at a time, with a FIDO2 PIN and no touch; it keeps the disk passphrase.
2. Write the recovery key on paper, keep it away from the machine, and confirm the copy when asked.
3. The `secureboot` stage enrolls only your own keys and signs the boot images; stop if it reports option ROMs or a missing TPM event log.
4. Reboot with a YubiKey inserted; if Secure Boot is still off, enable it with F2 and save with F10.
5. Run `dctl setup` again for `totp`, scan the QR code into your authenticator, and enter its code to verify it before the boot images are rebuilt.
6. Reboot and compare the boot TOTP with the authenticator before entering the LUKS PIN or passphrase.

If Setup Mode was not prepared before installation, follow the `secureboot` firmware action and rerun `dctl setup`.
Use a plain TTY for TOTP enrollment, or clear terminal scrollback afterward.
Before enrollment, the boot screen says **Boot TOTP not set up yet**.
After enrollment, a missing or wrong code means **do not unlock**; investigate before [resealing](cmds/cmd/dctl/README.md#secure-boot).
Then work through the [hardware checklist](cmds/cmd/dctl/README.md#manual-hardware-acceptance).

For daily upgrades, run `update`; see [Update](cmds/cmd/dctl/README.md#update).
