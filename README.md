<!-- ============================================================================ -->
<!-- ⚙️ heading {{{ -->
<h1 align="center">⚙️ dotfiles ⚙️</h1>

<p align="right">
    <a href="https://github.com/nosvagor/vagari/stargazers">
        <img
            src="https://img.shields.io/github/stars/nosvagor/vagari?color=ecc45d&logo=apachespark&labelColor=24283b&logoColor=ecc45d&style=for-the-badge"
            title="what is love, baby don't hurt me"
        >
    </a>
    &nbsp;
    <!-- <a href="https://cullyn.eth/"> -->
    <!--     <img -->
    <!--         src="https://img.shields.io/github/sponsors/nosvagor?color=dc60bf&logo=githubsponsors&labelColor=24283b&logoColor=dc60bf&style=for-the-badge" -->
    <!--         title="github sponsors not set up, pref ETH to cullyn.eth for now" -->
    <!--     > -->
    <!-- </a> -->
</p>

<p align="center">
    <kbd>
        <img
            alt="dotfiles neonlights banner"
            src="https://github.com/nosvagor/dotfiles/blob/main/resources/share/dotfiles-banner.gif?raw=true"/>
    </kbd>
</p>

<p align="center">
  <a href="https://github.com/nosvagor/dotfiles/issues">
      <img
          src="https://img.shields.io/github/issues/nosvagor/dotfiles?color=ed5f6f&logo=fireship&labelColor=24283b&logoColor=ed5f6f&style=for-the-badge"
          title="this is not fine!!"
      >
  </a>
  &nbsp;
  <a href="https://github.com/nosvagor/dotfiles/discussions">
      <img
          title="alright then, keep your secrets"
          src="https://img.shields.io/github/discussions/nosvagor/dotfiles?color=ec9055&logo=github&labelColor=24283b&logoColor=ec9055&style=for-the-badge"
      >
  </a>
  &nbsp;
  <a href="https://github.com/nosvagor/dotfiles/contributors">
      <img
          src="https://img.shields.io/github/contributors/nosvagor/dotfiles?color=86b96f&logo=gitea&labelColor=24283b&logoColor=86b96f&style=for-the-badge"
          title="gotta do the cooking by the book"
      >
  </a>
  &nbsp;
  <br>
  <a href="https://www.reddit.com/r/unixporn/">
      <img
          src="https://vbr.wocr.tk/badge?color=59b5e5&logoColor=59b5e5&page_id=nosvagor/dotfiles&logo=linux&style=for-the-badge&lcolor=24283b"
          alt="visitor count 💀"
          title="hello there (I use arch, neovim, and rust, btw)"
      >
  </a>
  &nbsp;
  <a href="https://github.com/nosvagor/dotfiles/network/members">
      <img
          src="https://img.shields.io/github/forks/nosvagor/dotfiles?color=6c88f4&logo=git&labelColor=24283b&logoColor=6c88f4&style=for-the-badge"
          title="Somewhere, something incredible is waiting to be known. &mdash; Carl Sagan"
      >
  </a>
  &nbsp;
  <a href="https://youtu.be/gxAaO2rsdIs">
      <img
          src="https://img.shields.io/github/license/nosvagor/dotfiles?color=9d71f0&logo=gnu&labelColor=24283b&logoColor=9d71f0&style=for-the-badge"
          title="R_0 > 1 ? share it : change it"
      >
  </a>
</p>
<!-- }}} -->
<!-- ============================================================================ -->

## 👾 Overview
- **Operating System**: [Arch
  Linux](https://wiki.archlinux.org/title/Arch_Linux)
- **Display Server**: [Wayland](https://wiki.archlinux.org/title/Wayland)
- **Color Scheme**: [vagari](https://github.com/nosvagor/vagari) (also a design philosophy)
- **Compositor**: [Hyprland](https://hyprland.org/) (pseudo window manager)
- **Widgets**: [eww](https://github.com/elkowar/eww) (status bar and other user interface controls)
- **Core Applications**:
    - **Terminal**: [kitty](https://sw.kovidgoyal.net/kitty/)
    - **Editor**: [neovim](https://neovim.io/)
    - **Shell**: [Nushell](https://www.nushell.sh/)
    - **Browser**: [Firefox](https://www.mozilla.org/en-US/firefox/developer/) (with custom UI/UX)

> ⚠️ Refactor Work in Progress, full overview will be updated once completed

<!--   - Plugins: [syntax highlighting](https://github.com/zsh-users/zsh-syntax-highlighting), [auto suggestions](https://github.com/zsh-users/zsh-autosuggestions), [auto complete](https://github.com/marlonrichert/zsh-autocomplete), [colored man pages](https://github.com/ael-code/zsh-colored-man-pages) -->
<!--   - Prompt: custom [starship](https://starship.rs/) + [custom DNA greeter](cullyn/.local/bin/dna) -->
<!--   - Quick navigation: [jump](https://github.com/gsamokovarov/jump) -->
<!--   - Theme: [Tokyo Night (storm)](https://github.com/folke/tokyonight.nvim) -->
<!--   - [config files](cullyn/.config/nvim/) _(work in progress)_ 🔨 -->
<!--   - [vimium](https://github.com/philc/vimium) -->
<!--   - and a custom, minimal css theme: [tokyo night]() _(work in progress)_ 🔨 -->
<!--   - [config files](cullyn/.config/nnn/) _(work in progress)_ 🔨 -->
<!-- - **Music**: [YouTube Music](https://aur.archlinux.org/packages/ytmdesktop-git) -->
<!--   - I might try to write a CLI app for YouTube Music (the electron web emulator is pretty bad). -->
<!--   - However, [playerctl](https://github.com/altdesktop/playerctl) solves most usage issues, for now. -->
<!-- - **Launcher**: [rofi](https://github.com/davatorium/rofi) -->
<!-- - **PDF viewer**: [zathura](https://pwmt.org/projects/zathura/) -->
<!-- - **Notifications**: [dunst](https://github.com/dunst-project/dunst) -->
<!-- - **Music visualizer**: [cava](https://github.com/karlstav/cava) -->
<!-- - **Fonts**: [google fonts](https://aur.archlinux.org/packages/ttf-google-fonts-git/) / [nerd fonts](https://github.com/ryanoasis/nerd-fonts) -->
<!--   - [Albert Sans (serif)](https://fonts.google.com/specimen/Albert+Sans?query=Albert) -->
<!--   - [Roboto Slab (serif)](https://fonts.google.com/specimen/Roboto+Slab?query=Roboto+S) -->
<!--   - [Hack Nerd Font (monospace)](https://github.com/ryanoasis/nerd-fonts) -->
<!--   - Noto Color Emoji and Noto Emoji -->
<!-- - **GTK**: [Tokyo Night GTK (storm)](https://github.com/Fausto-Korpsvart/Tokyo-Night-GTK-Theme) -->
<!--   - TODO: maybe customize it further, see [.local/share/themes](https://github.com/nosvagor/dotfiles/tree/main/cullyn/.local/share/themes) 🔨 -->
<!-- - **Cursor**: [Nordzy Cursors](https://github.com/alvatip/Nordzy-cursors) -->
<!-- - **Icons**: [Nordzy Icon Theme](https://github.com/alvatip/Nordzy-icon) -->

</details>
&nbsp;
<details>
<summary><b> 🧰 My Hardware</b></summary>

- **Mouse**: MX Master 3S
- **CPU**: AMD Ryzen 7 3700X (16) @ 3.600GHz
- **GPU**: AMD ATI Radeon RX 5600 OEM/5600 XT / 5700/5700 XT
- **Monitor**: SAMSUNG UR59 Series 32-Inch 4K UHD (3840x2160)
- **Keybaord:** Corne (Helidox) 42 key, Kailh gChoc Light Blue (20g),

</details>

&nbsp;

## 🛠️ Installation

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

      base base-devel linux-headers git openssh stow lua go rustup bat btop sd fd procs ripgrep fzf nushell python firefox-developer-edition

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

2.10 Symlink some important GTK settings

    ln -sfn ~/.icons/default/index.theme /usr/share/icons/default/index.theme
    ln -sfn $HOME/.local/share/themes/Tokyonight /usr/share/themes

&nbsp;

2.12 Nice tree replacement (found when configuring nnn)

    go install github.com/kitagry/gtree@latest

&nbsp;

2.13 SDDM setup (No theme setup, I prefer autologin). See [SDDM](https://wiki.archlinux.org/title/SDDM) for more instructions, if you wish to customize theme.

    sudo cp your-user-icon.png "/usr/share/sddm/faces/$USER.face.icon"
    cp ~/dotfiles/resources/sddm.conf.d/*.conf /etc/sddm.conf.d/

---

---

</details>
&nbsp;
<details>

<summary><b>3. Customization 🖌️</b></summary>

#### [Firefox](https://www.mozilla.org/en-US/firefox/developer/) 🦊

🦊.1 update various `about:config` options:

- Update scaling factor if in HiDPI environment:

      layout.css.devPixelsPerPx = 1.3

- Stop asking to restore session (I often just kill the window and don't want the prompt later)

      browser.sessionstore.resume_from_crash = false

- Hide PDF sidebar by default

      pdfjs.sidebarViewOnLoad = 0

- Enable ability to use custom css (TODO: create script to symlink css files)

      toolkit.legacyUserProfileCustomizations.stylesheets = true

  - Enable remote debugging and live editing of css (Ctrl + Alt + Shift + I):

        F12 -> settings -> Adavnce Settings ->
        -> Enable remote debugging
        -> Enable browser chrome and add-on debuggins toolboxes

&nbsp;

🦊.2 Firefox basic options options:

- Disable hardware acceleration (firefox keeps seizing)

---

---

</details>
