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

- Image Editing: [gimp](https://www.gimp.org/)
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

- Keyboard: [Corne (Helidox) 42 key](https://keebmaker.com/products/corne-low-profile), with Kailh gChoc Light Blue (20g)
  - ZMK firmware (for bluetooth version of keyboard): [cogikyo/zmk-config](https://github.com/cogikyo/zmk-config)

  <details>
  <summary>Custom Layout:</summary>
  <br>
  <img src="https://user-images.githubusercontent.com/59071534/232157490-bc96cdec-fa8c-4245-a9fe-76fd57a381af.png" alt="layer 1">
  <img src="https://user-images.githubusercontent.com/59071534/232157618-c49b549f-6acf-4343-96d0-9f9932196b36.png" alt="layer 2">
  <img src="https://user-images.githubusercontent.com/59071534/232157647-baabd17f-9cf7-43b1-9577-37eb7daa326d.png" alt="layer 3">
  <img src="https://user-images.githubusercontent.com/59071534/232157666-a6fa76f4-43a2-414b-879d-26a200101e18.png" alt="layer 4">
  </details>

##### Misc Hardware

- Monitor: [SAMSUNG UR59 Series 32-Inch 4K UHD (3840x2160)](https://a.co/d/bZtUse0)
- Computer: Framework Desktop, AMD Strix Halo, Ethernet only
- Microphone: [Shure SM57](https://www.amazon.com/gp/product/B0000AQRST)
  - Audio Interface: [Scaarlett Solo 3rd Gen](https://www.amazon.com/gp/product/B07QR6Z1JB)
- Camera: [Canon EOS M50 Mark II](https://www.amazon.com/gp/product/B08KSLW8N3)
  - Lens: [Sigma 16mm f/1.4](https://www.amazon.com/gp/product/B084KYHYKN)

</details>

## 🛠️ Installation

The dctl ISO installs Arch and these dotfiles offline onto the Framework Desktop, with LUKS2, btrfs, Snapper, Limine, and Secure Boot.
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

Back up the internal disk.
In the BIOS, install version 3.06, enable Pluton, and put Secure Boot in Setup Mode.

### 4. Install

Boot the USB in UEFI mode; `dctl install` starts on tty1.
Enter the login password for `cullyn`, the timezone, and the LUKS passphrase, then type the disk path to confirm.
Without Setup Mode, the install continues without Secure Boot.
Remove the USB when it asks to reboot.

### 5. First login

Unlock LUKS with the passphrase, log in through SDDM, and connect Ethernet.
Insert a YubiKey, then run:

```sh
dctl setup
git -C ~/dotfiles fetch --unshallow
```

Setup asks before each stage and asks for sudo once.
It installs `packages/extra.lst` online and asks for the YubiKey PIN to decrypt the SSH keys.
When an SSH key is first used, the keyring asks for its passphrase; choose the option to unlock it automatically at login.
If setup reports a missing Firefox profile, start Firefox once and run `dctl setup firefox certs`.

### 6. YubiKeys and Secure Boot

Enroll each YubiKey with only that key inserted, and record the LUKS recovery key when it is shown:

```sh
dctl keys enroll
sudo dctl keys luks
```

If the install skipped Secure Boot, put the firmware in Setup Mode, boot, and run `dctl setup secureboot`.
Enable Secure Boot in the BIOS, reboot, and check `dctl setup --status secureboot`.
Then work through the [hardware checklist](cmds/cmd/dctl/README.md#manual-hardware-acceptance).

For daily upgrades, run `update`; see [Update](cmds/cmd/dctl/README.md#update).
