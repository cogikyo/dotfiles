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
> _"...we _{do these}_ things **not** because they are easy, but **because they are hard**_,"<br>
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
- GTK: [catppuccin macchiato (peach)](https://github.com/catppuccin/gtk)
- Cursors: [catppuccin-macchiato-dark](https://github.com/catppuccin/cursors)
- Icons: [Papirus-Dark](https://github.com/PapirusDevelopmentTeam/papirus-icon-theme)

</details>

<details open>
<summary>💬 <b>Fonts</b></summary>

- Sans Serif: [Albert Sans](https://fonts.google.com/specimen/Albert+Sans?query=Albert+Sans)
- Monospace: [Iosevka Vagari](https://typeof.net/Iosevka/)
- Other:
  - [Nerd Font Symbols](https://github.com/ryanoasis/nerd-fonts)
  - [Noto Color Emoji](https://fonts.google.com/noto/specimen/Noto+Color+Emoji)
  - [Lora (serif)](https://fonts.google.com/specimen/Lora),
  - [Archivo (display)](https://fonts.google.com/specimen/Archivo),
  - [Architects Daughter (handwritten)](https://fonts.google.com/specimen/Architects+Daughter)

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
- Mouse: [MX Master 3S](https://www.logitech.com/en-us/products/mice/mx-master-3s.910-006556.html)
- Computer: Framework Desktop, AMD Strix Halo, Ethernet only
- Microphone: [Shure SM57](https://www.amazon.com/gp/product/B0000AQRST)
  - Audio Interface: [Scaarlett Solo 3rd Gen](https://www.amazon.com/gp/product/B07QR6Z1JB)
- Camera: [Canon EOS M50 Mark II](https://www.amazon.com/gp/product/B08KSLW8N3)
  - Lens: [Sigma 16mm f/1.4](https://www.amazon.com/gp/product/B084KYHYKN)

</details>

## 🛠️ Installation

The dctl UEFI ISO installs Arch and these dotfiles offline onto a whole disk.
It uses LUKS2, btrfs, Snapper, Limine, and an SDDM video greeter before the Hyprland session.
There is no stock-ISO installer, dual-boot flow, or hibernation setup.

### Get the ISO

Download the ISO, its `.sha256` file, and its `.sha256.sig` file from [GitHub Releases](https://github.com/cogikyo/dotfiles/releases/latest).
Keep all three files together and use a checkout with the trusted release keys in `share/allowed_signers`.
On an existing Arch host, use an installed dctl or build it as described in [`cmds/README.md`](cmds/README.md).
The build needs a clean, committed `master`, network access, Go, `archiso`, `devtools`, Git, and pacman tooling on an Arch host.
Root is required for makechrootpkg and mkarchiso.
Complete the [SSH cutover](cmds/cmd/dctl/README.md#build) before building.

```sh
sudo dctl iso build
```

The result is `iso/out/dotfiles-<12-character-revision>.iso`, built from a Git bundle of the committed revision rather than the working tree.
If sudo cannot find dctl, use its absolute path, such as `sudo "$HOME/.local/bin/dctl" iso build`.
Set `ISO` to the downloaded or built image path:

```sh
ISO=/path/to/dotfiles-REV.iso
dctl iso test "$ISO"
```

The test runs without sudo and needs QEMU, KVM access, OVMF Secure Boot firmware, dosfstools, and mtools.
It installs without network access, unlocks and boots twice, checks doctor results, and saves timings and serial logs under `/var/tmp/dctl-iso-test-*`.
After a 20-second wait, it saves `greeter.png` for manual inspection; it does not verify the greeter.
See the [dctl guide](cmds/cmd/dctl/README.md#iso) for test overrides and release signing.
A local build needs signed checksum files before the USB command accepts it.
`dctl iso release "$ISO"` creates those files and publishes a public release; there is no signing-only dctl command.

### Write the USB

**This erases the whole USB disk.**
Replace `/dev/sdX` with an unmounted removable disk, not a partition.

```sh
sudo dctl iso usb "$ISO" /dev/sdX
```

The command verifies the signed checksum against `share/allowed_signers` and requires you to type the device path.
It rejects mounted disks, internal non-removable disks, and disks without a serial or WWN.

### Boot and install

Back up the target disk before installation.
For manual hardware preparation, set the Framework BIOS to version 3.06 with Pluton enabled and Secure Boot in Setup Mode.
The installer checks UEFI and Secure Boot state, but does not check the BIOS version or Pluton setting.
Boot the USB in UEFI mode.
The live environment starts this command as root on tty1:

```sh
dctl install
```

Enter the username, login password, hostname, timezone, and LUKS passphrase, then select the target disk if prompted.
**Typing the hostname at the final confirmation erases the selected disk.**
The installer refuses the boot disk, mounted disks, USB/removable targets, and targets without a serial or WWN.
It installs the offline package payload, clones the bundled history into `~/dotfiles`, and installs the prebuilt commands.
If firmware is not in Setup Mode, installation continues without Secure Boot and reports the required follow-up.

### First login

Reboot, unlock LUKS with the passphrase, and log in through SDDM.
Connect Ethernet for the online setup work.
Run doctor as the normal user; it reports root-only fixes separately.

```sh
dctl doctor
dctl doctor --fix
```

To fix one group, use `dctl doctor --fix home` or another group from the [dctl guide](cmds/cmd/dctl/README.md#doctor).
Do not run all fixes with sudo, because user groups refuse root execution.
Enroll each YubiKey separately with only that key inserted:

```sh
dctl keys enroll
sudo dctl keys luks
```

Record the LUKS recovery key when it is shown; the original passphrase slot remains available.
If Secure Boot enrollment was skipped, clear the firmware keys into Setup Mode, boot, and run `sudo dctl doctor --fix secureboot`.
Enable Secure Boot in the BIOS after enrollment and reboot before checking `sudo dctl doctor secureboot`.

## Maintenance

Run these as the normal user:

```sh
dctl secrets list
dctl secrets decrypt
dctl secrets sync
dctl secrets verify-phrase
dctl repos sync
dctl repos update
dctl update
```

`dctl update` upgrades through yay and reports package-list drift without rewriting lists or removing orphans.
See the [dctl guide](cmds/cmd/dctl/README.md) for secret recovery, recipient changes, and release publishing.

## Manual hardware acceptance

These checks require the Framework Desktop and real YubiKeys; the VM test does not prove them.

- [ ] Manually confirm BIOS 3.06, Pluton enabled, and Setup Mode before installation.
- [ ] Enable Secure Boot in the BIOS after key enrollment, reboot, and pass `sudo dctl doctor secureboot`.
- [ ] Boot and unlock LUKS with each of the two YubiKeys separately, with PIN and touch.
- [ ] Decrypt secrets with each YubiKey separately, without the other key or the age phrase.
- [ ] Reject a wrong FIDO2 PIN and a wrong PIV PIN; cancel any age-phrase fallback and avoid repeated failures that can block the key.
- [ ] Unlock LUKS with the recorded recovery key while both YubiKeys are removed.
- [ ] Pass `dctl secrets verify-phrase` and rehearse secret recovery without a YubiKey.
- [ ] Boot a Limine snapshot entry, restore it, and confirm the restored system boots.
- [ ] Pair and use a Bluetooth device, then pass `sudo dctl doctor hardware` with no WLAN interface.
- [ ] Check the kernel log for firmware load failures and confirm the `linux-firmware-{amd,amdgpu,mediatek,realtek}` split supports the hardware without errors.
