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
- Monospace: [Iosevka Vagari](https://typeof.net/Iosevka/) (custom build, see `etc/iosevka/`)
- Symbols: [Nerd Font Symbols](https://github.com/ryanoasis/nerd-fonts)
- Emoji: [Noto Color Emoji](https://fonts.google.com/noto/specimen/Noto+Color+Emoji)
- Other: [Lora (serif)](https://fonts.google.com/specimen/Lora),
  [Archivo (display)](https://fonts.google.com/specimen/Archivo),
  [Architects Daughter (handwritten)](https://fonts.google.com/specimen/Architects+Daughter)

</details>

<details open>
<summary>🧰 <b>My Hardware</b></summary>

- Keyboard: [Corne (Helidox) 42 key](https://keebmaker.com/products/corne-low-profile), with Kailh gChoc Light Blue (20g), and **custom layout**:
  ![image](https://user-images.githubusercontent.com/59071534/232157490-bc96cdec-fa8c-4245-a9fe-76fd57a381af.png)
  ![image](https://user-images.githubusercontent.com/59071534/232157618-c49b549f-6acf-4343-96d0-9f9932196b36.png)
  ![image](https://user-images.githubusercontent.com/59071534/232157647-baabd17f-9cf7-43b1-9577-37eb7daa326d.png)
  ![image](https://user-images.githubusercontent.com/59071534/232157666-a6fa76f4-43a2-414b-879d-26a200101e18.png)
- ZMK firmware (for bluetooth version of keyboard): [cogikyo/zmk-config](https://github.com/cogikyo/zmk-config)
- Monitor: [SAMSUNG UR59 Series 32-Inch 4K UHD (3840x2160)](https://a.co/d/bZtUse0)
- Mouse: [MX Master 3S](https://www.logitech.com/en-us/products/mice/mx-master-3s.910-006556.html)
- CPU: [AMD Ryzen 7 3700X (16) @ 3.600GHz](https://www.amd.com/en/products/cpu/amd-ryzen-7-3700x)
  - GPU: [AMD ATI Radeon RX 5600 OEM/5600 XT / 5700/5700 XT](https://www.amd.com/en/products/graphics/amd-radeon-rx-5600-xt)
- Microphone: [Shure SM57](https://www.amazon.com/gp/product/B0000AQRST)
  - Audio Interface: [Scaarlett Solo 3rd Gen](https://www.amazon.com/gp/product/B07QR6Z1JB)
- Camera: [Canon EOS M50 Mark II](https://www.amazon.com/gp/product/B08KSLW8N3)
  - Lens: [Sigma 16mm f/1.4](https://www.amazon.com/gp/product/B084KYHYKN)

</details>

## 🛠️ Installation

### **1. Get the installation image:**

- **[archlinux-version-x86_64.iso](https://archlinux.org/download/)**

### **2. Load on USB**

<details>
<summary>🐧 <b>Linux</b> (terminal)</summary>

Find your USB device (`sda`, `sdb`, etc.):

    lsblk -f

Write to USB using [dd](https://wiki.archlinux.org/title/Dd) — use the **disk** (e.g. `/dev/sdx`), not a partition:

    dd bs=4M if=path/to/archlinux-version-x86_64.iso of=/dev/sdx conv=fsync oflag=direct status=progress

</details>

<details>
<summary>🍎 <b>macOS</b> (terminal)</summary>

Find your USB device (`disk2`, `disk3`, etc.):

    diskutil list

Unmount the disk, then write with [dd](https://wiki.archlinux.org/title/Dd):

    diskutil unmountDisk /dev/diskX
    sudo dd bs=4M if=path/to/archlinux-version-x86_64.iso of=/dev/rdiskX status=progress

> Use `/dev/rdiskX` (raw disk) for significantly faster writes.

</details>

<details>
<summary>🪟 <b>Windows</b> (GUI)</summary>

Use **[Rufus](https://rufus.ie/)** — select the ISO, pick your USB drive, and write in DD mode.

</details>

### 3. Arch Install

```sh
curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash
```

### 4. Post Install

```sh
curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash
```
