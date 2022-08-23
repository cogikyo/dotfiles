<h1 align="center">⚙️ dotfiles ⚙️</h1>

<p align="center">
    <img src="https://img.shields.io/badge/OS-Arch Linux-984D55?style=flat-sqaure&labelColor=1a1b26"/>
    <img src="https://img.shields.io/badge/Display Server-Wayland-5454B9?style=flat-sqaure&labelColor=1a1b26"/>
    <img src="https://img.shields.io/badge/Window Manager-Hyprland-774989?style=flat-sqaure&labelColor=1a1b26"/>
    <img src="https://img.shields.io/badge/Theme-Tokyo Night Storm-DF6BC0?style=flat-sqaure&labelColor=1a1b26"/>
    <kbd>
        <img
            alt="dotfiles neonlights banner"
            src="https://github.com/nosvagor/dotfiles/blob/main/resources/share/dotfiles-banner.gif?raw=true"/>
    </kbd>
</p>

<p align="center">
    <img src="https://img.shields.io/github/stars/nosvagor/dotfiles?color=e0af68&logo=github&labelColor=1a1b26&logoColor=e0af68&style=for-the-badge">&nbsp;&nbsp;
    <img src="https://img.shields.io/github/issues/nosvagor/dotfiles?color=9ece6a&logo=gitbook&labelColor=1a1b26&logoColor=9ece6a&style=for-the-badge">&nbsp;&nbsp;
    <img src="https://visitor-badge-reloaded.herokuapp.com/badge?color=b4f9f8&logoColor=b4f9f8&page_id=nosvagor/dotfiles&logo=linux&style=for-the-badge&lcolor=1a1b26"/>&nbsp;&nbsp;
    <img src="https://img.shields.io/github/forks/nosvagor/dotfiles?color=7aa2f7&logo=git&labelColor=1a1b26&logoColor=7aa2f7&style=for-the-badge">&nbsp;&nbsp;
    <img src="https://img.shields.io/github/license/nosvagor/dotfiles?color=bb9af7&logo=gnu&labelColor=1a1b26&logoColor=bb9af7&style=for-the-badge">&nbsp;&nbsp;
</p>

---

&nbsp;

## Overview 👾

- Operating System: [Arch Linux](https://wiki.archlinux.org/title/Arch_Linux)
- Display Server: [Wayland](https://wiki.archlinux.org/title/Wayland), with display manager: [SDDM](https://wiki.archlinux.org/title/SDDM)
- Window manager/compositor: [Hyprland](https://github.com/hyprwm/Hyprland)
- Terminal: [alacritty](https://github.com/alacritty/alacritty), with terminal multiplexer: [tmux](https://en.wikipedia.org/wiki/Tmux)
- Shell: [zsh](https://wiki.archlinux.org/title/zsh),
  - Plugins: [syntax highlighting](https://github.com/zsh-users/zsh-syntax-highlighting), [auto suggestions](https://github.com/zsh-users/zsh-autosuggestions), [auto complete](https://github.com/marlonrichert/zsh-autocomplete), [colored man pages](https://github.com/ael-code/zsh-colored-man-pages)
  - Prompt: custom [starship](https://starship.rs/) + [custom DNA greeter](cullyn/.local/bin/dna)
  - Quick navigation: [jump](https://github.com/gsamokovarov/jump)
- Editor: [neovim-nightly](https://github.com/neovim/neovim),
  - Theme: [Tokyo Night Storm](https://github.com/folke/tokyonight.nvim)
  - [config files](cullyn/.config/nvim/) _(work in progress)_ 🔨
- Browser: [firefox-developer-eddition](https://www.mozilla.org/en-US/firefox/developer/), with keyboard focused usage via:
  - [vimium](https://github.com/philc/vimium)
  - and a custom, minimal css theme: [tokyo night]() _(work in progress)_ 🔨
- File manager: [nnn](https://github.com/jarun/nnn)
  - [config files](cullyn/.config/nnn/) _(work in progress)_ 🔨
- Music: [YouTube Music](https://aur.archlinux.org/packages/ytmdesktop-git)
  - I might try to write a CLI app for YouTube Music (the electron web emulator is pretty bad).
  - However, [playerctl](https://github.com/altdesktop/playerctl) solves most usage issues, for now.
- Status bar (widgets): [eww](https://github.com/elkowar/eww)
- Launcher: [rofi](https://github.com/davatorium/rofi)
- PDF viewer: [zathura](https://pwmt.org/projects/zathura/)
- Notifications: [dunst](https://github.com/dunst-project/dunst)
- Music visualizer: [cava](https://github.com/karlstav/cava)
- Fonts: [google fonts](https://aur.archlinux.org/packages/ttf-google-fonts-git/) / [nerd fonts](https://github.com/ryanoasis/nerd-fonts)
  - [Albert Sans (serif)](https://fonts.google.com/specimen/Albert+Sans?query=Albert)
  - [Roboto Slab (serif)](https://fonts.google.com/specimen/Roboto+Slab?query=Roboto+S)
  - [Hack Nerd Font (monospace)](https://github.com/ryanoasis/nerd-fonts)
  - Noto Color Emoji and Noto Emoji
- GTK: [Tokyo Night GTK (storm)](https://github.com/Fausto-Korpsvart/Tokyo-Night-GTK-Theme)
  - TODO: maybe customize it further, see [.local/share/themes](https://github.com/nosvagor/dotfiles/tree/main/cullyn/.local/share/themes) 🔨
- Cursor: [Nordzy Cursors](https://github.com/alvatip/Nordzy-cursors)
- Icons: [Nordzy Icon Theme](https://github.com/alvatip/Nordzy-icon)

</details>
&nbsp;
<details>
<summary><b> My Hardware 🖥️ </b></summary>

- **Mouse**: MX Master 3S
- **CPU**: AMD Ryzen 7 3700X (16) @ 3.600GHz
- **GPU**: AMD ATI Radeon RX 5600 OEM/5600 XT / 5700/5700 XT
- **Monitor**: SAMSUNG UR59 Series 32-Inch 4K UHD (3840x2160)
- **Keybaord:** Corne (Helidox) 42 key, Kailh gChoc Light Blue (20g),

</details>

&nbsp;

## Installation 🛠️

<details>
<summary><b>1. Installing Linux 🐧 </b></summary>

##### [Official ArchWiki Installation Guide](https://wiki.archlinux.org/title/installation_guide)

##### [Please do read the FAQ](https://wiki.archlinux.org/title/Frequently_asked_questions)

**⚠️ Note:** _the following is essentially what I did if you want to start from scratch and
follow along. Installation does not have to be exactly like I describe; these
instructions are mostly just here for personal reference to debug what I might be
doing wrong, or did wrong. Some minor steps or, reason behind my choices, are not explicitly stated._

**1.1 Acquire an installation image**: https://archlinux.org/download/

**1.2 Prepare an installation medium:**

- find USB device partition (**sda** or **sdb**, probably):

      lsblk -f

- write to usb using **dd** (sd"x", do not use partition number):

      dd bs=4M if=path/to/archlinux-version-x86_64.iso of=/dev/sdx conv=fsync oflag=direct status=progress

**1.3 Use guided arch installation:**

- boot to usb and run command:

      archinstall

- nice to install some base packages needed for now:

      base base-devel linux-headers git stow coreutils bat btop go rustup python

---

---

</details>
&nbsp;
<details>
<summary><b>2. Post-Installation 🧰 </b></summary>

##### [ArchWiki General Recommendations](https://wiki.archlinux.org/title/General_recommendations)

2.1 Install AUR helper ([paru](https://github.com/Morganamilo/paru))

    git clone https://aur.archlinux.org/paru.git
    cd paru
    makepkg -si

&nbsp;

2.2 Install packages:

    cd $HOME && git clone https://github.com/nosvagor/dotfiles.git
    paru -S - < $HOME/dotfiles/resources/packages.txt

&nbsp;

2.3 Configure shell and link config files:

- Change default shell to zsh:

      chsh -s $(which zsh)

- Install shell related packages:

      go install github.com/gsamokovarov/jump@latest
      curl -sS https://starship.rs/install.sh | sh
      $HOME/dotfiles/cullyn/.local/bin/zsh-install

- Link config files using [GNU stow](https://brandon.invergo.net/news/2012-05-26-using-gnu-stow-to-manage-your-dotfiles.html):

      cd $HOME/dotfiles
      mv cullyn $USER
      stow $USER
      stow home

**⚠️ Note:** _you probably don't want my zsh config, or many of my personal configs, and should update them now._

&nbsp;

2.5 Configure SSH for GitHub _(here for personal reference)_:

    ssh-keygen -t ed25519 -C "your_email@example.com"
    eval "$(ssh-agent -s)"
    ssh-add ~/.ssh/id_ed25519
    bat ~/.ssh/id_ed25519.pub
    # Add ssh key to github, then update remote origin:
    git remote set-url origin git@github.com:USERNAME/REPOSITORY.git

&nbsp;

2.6 Update [bluetooth](https://wiki.archlinux.org/title/bluetooth):

    systemctl enable bluetooth.service
    systemctl start bluetooth.service #(if before reboot)

- pair device

      bluetoothctl
      power on
      scan on
      pair DEVICE
      trust DEVICE
      connect DEVICE

- auto power-on:

      /etc/bluetooth/main.conf
      ---
      [Policy]
      AutoEnable=true

- disable shitty built in bluetooth:

      lsusb | grep "Bluetooth"

      # yields
      Bus 001 Device 004: ID 0b05:18ea ASUSTek Computer, Inc. Bluetooth Radio

      touch /etc/udev/rules.d/81-bluetooth-hci.rules
      ---
      SUBSYSTEM=="usb", ATTRS{idVendor}=="0b05", ATTRS{idProduct}=="18ea", ATTR{authorized}="0"

- verify by checking available agents (should just be one, preferably):

      bluetoothctl list

      # yields
      Controller 3C:7C:3F:A2:38:10 costello [default]

&nbsp;

2.7 Create default directories (personal preference, see [user-dirs.dir](https://github.com/nosvagor/dotfiles/blob/main/cullyn/.config/user-dirs.dirs)):

    #simple script to edit if you want something different
    dir-setup

    # yields
    ├── documents
    │   ├── papers
    │   ├── share
    │   ├── templates
    │   └── textbooks
    ├── downloads
    ├── media
    │   ├── gifs
    │   ├── images
    │   ├── music
    │   ├── recordings
    │   └── videos

&nbsp;

2.8 Edit boot config (skip boot menu):

    ──────┬────────────────────────────────────────────────────────────────────
          │ File: /boot/loader/loader.conf
    ──────┼────────────────────────────────────────────────────────────────────
      1   │ timeout 0
    ──────┴────────────────────────────────────────────────────────────────────

&nbsp;

2.9 Add extra figlet fonts (I sometimes use for visual headers in files):

    cp -va  "$HOME/dotfiles/resources/ascii-fonts/." "/usr/share/figlet/fonts/"

&nbsp;

2.10 Update some environment variables (might be a better way to do this, working as of now)

    ────┬──────────────────────────────────────────────────────────────────────
        │ File: /etc/environment
    ────┼──────────────────────────────────────────────────────────────────────
    1   │ #
    2   │ # This file is parsed by pam_env module
    3   │ #
    4   │ # Syntax: simple "KEY=VAL" pairs on separate lines
    5   │
    6   │ # X11 vars (for QT applications)
    7   │ XCURSOR_SIZE=14
    8   │
    9   │ # misc (for tmux colors)
    10  │ TERM=xterm-256color
    ────┴──────────────────────────────────────────────────────────────────────

&nbsp;

2.11 Symlink some important GTK settings

    ln -sfn ~/.icons/default/index.theme /usr/share/icons/default/index.theme
    ln -sfn $HOME/.local/share/themes/Tokyonight /usr/share/themes

---

---

</details>
&nbsp;
<details>

<summary><b>3. Customization 🖌️</b></summary>

#### [Firefox](https://www.mozilla.org/en-US/firefox/developer/) 🦊

🦊.1 update various `about:config` options:

- Update scaling factor if in HiDPI environment:

      layout.css.devPixelsPerPx = 1.25

- Stop asking to restore session (I often just kill the window and don't want the prompt later)

      browser.sessionstore.resume_from_crash = false

&nbsp;

🦊.2 Firefox basic options options:

- Disable hardware acceleration (firefox keeps seizing)

---

---

</details>

&nbsp;

## Gallery 🎨

&nbsp;

## Support 🤝

- Personal: [cullyn.eth](https://cullyn.eth/)
  - ETH `0xe377b77E7177c236EDE5abD317960Fc023efD841`
- Open source: [gitcoin.co](https://gitcoin.co/)
