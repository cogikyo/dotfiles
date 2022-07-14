<h1 align="center">⚙️ dotfiles | tokyo night 🌆</h1>

<p align="center">
<kbd>
<img
    alt="dotfiles neonlights banner" width="690"
    src="https://github.com/nosvagor/dotfiles/blob/main/resources/share/dotfiles-banner.gif?raw=true" />
</kbd>
</p>

<p align="center">
    <img src="https://img.shields.io/github/stars/nosvagor/dotfiles?color=%231a1b26&label=STARGAZERS&labelColor=%231a1b26&style=for-the-badge"/>&nbsp;&nbsp;
    <img src="https://visitor-badge-reloaded.herokuapp.com/badge?color=1a1b26&page_id=nosvagor/dotfiles&style=for-the-badge&lcolor=1a1b26"/>&nbsp;&nbsp;
    <img src="https://img.shields.io/github/issues/nosvagor/dotfiles?color=1a1b26&labelColor=1a1b26&style=for-the-badge"/>&nbsp;&nbsp;
    <img src="https://img.shields.io/github/license/nosvagor/dotfiles?color=1a1b26&labelColor=1a1b26&style=for-the-badge"/>
</p>

---

&nbsp;

# Overview 👾

<details>
<font size="+2"><summary><b> Essentials 📦 </b></font></summary>

- Operating System: [Arch Linux](https://wiki.archlinux.org/title/Arch_Linux)
- Display Server: [Wayland](https://wiki.archlinux.org/title/Wayland), with display manager: [SDDM](https://wiki.archlinux.org/title/SDDM)
- Window manager: [Hyprland](https://github.com/hyprwm/Hyprland), with hotkey daemon: [swhkd](https://github.com/waycrate/swhkd)
- Terminal: [alacritty](https://github.com/alacritty/alacritty), with terminal multiplexer: [tmux](https://en.wikipedia.org/wiki/Tmux)
- Shell: [zsh](https://wiki.archlinux.org/title/zsh),
  - Plugins: [syntax highlighting](https://github.com/zsh-users/zsh-syntax-highlighting), [auto suggestions](https://github.com/zsh-users/zsh-autosuggestions), [auto complete](https://github.com/marlonrichert/zsh-autocomplete), [colored man pages](https://github.com/ael-code/zsh-colored-man-pages)
  - Prompt: custom [starship](https://starship.rs/) + [custom DNA greeter](cullyn/.local/bin/dna)
  - Quick navigation: [jump](https://github.com/gsamokovarov/jump)
- Editor: [neovim-nightly](https://github.com/neovim/neovim),
  - [config files and instructions](cullyn/.config/nvim/) _(work in progress)_ 🔨
- Browser: [firefox-developer-eddition](https://www.mozilla.org/en-US/firefox/developer/), with keyboard focused usage via:
  - [vimium](https://github.com/philc/vimium)
  - and a custom, minimal css theme: [errant]() _(work in progress)_ 🔨
- File manager: [nnn](https://github.com/jarun/nnn)
  - [config files and instructions](cullyn/.config/nnn/) _(work in progress)_ 🔨

</details>
&nbsp;
<details>
<font size="+2"><summary><b> Aesthetics 👓 </b></font></summary>

- Status bar: [waybar](https://github.com/Alexays/Waybar)
- Launcher: [rofi](https://github.com/davatorium/rofi)
- PDF viewer: [zathura](https://pwmt.org/projects/zathura/)
- Notifications: [dunst](https://github.com/dunst-project/dunst)
- Music:
- Music visualizer:
- Fonts: [google fonts](https://aur.archlinux.org/packages/ttf-google-fonts-git/) / [nerd fonts](https://github.com/ryanoasis/nerd-fonts)
  - Open Sans (serif)
  - Roboto Slab (serif)
  - Hack Nerd Font (monospace)
  - Noto Color Emoji and Noto Emoji
- GTK:
- Cursor:
- Icons:

</details>
&nbsp;
<details>
<font size="+2"><summary><b> Hardware 🖥️ </b></font></summary>

- **Mouse**: MX Master 3S
- **CPU**: AMD Ryzen 7 3700X (16) @ 3.600GHz
- **GPU**: AMD ATI Radeon RX 5600 OEM/5600 XT / 5700/5700 XT
- **Monitor**: SAMSUNG UR59 Series 32-Inch 4K UHD (3840x2160)
- **Keybaord:** Corne (Helidox) 42 key, Kailh gChoc Light Blue (20g),

</details>

&nbsp;

# Installation 🛠️

<details>
<font size="+2"><summary><b>1. Installing Linux 🐧 </b></font></summary>

---

---

##### [Official ArchWiki Installation Guide](https://wiki.archlinux.org/title/installation_guide)

##### [Please do read the FAQ](https://wiki.archlinux.org/title/Frequently_asked_questions)

#### Quick Start ☕

<font size="-1">Note: the following is essentially what I did if you want to start from scratch and
follow along. Installation does not have to be exactly like I describe; these
instructions are mostly just here for personal reference to debug what I might be
doing wrong, or did wrong. Some minor steps or, reason behind my choices, are not explicitly stated. </font>

**1.1 Acquire an installation image**: https://archlinux.org/download/

**1.2 Prepare an installation medium:**

- find USB device partition (**sdb** probably):

      lsblk -f

- write to usb using **dd** (sd"x", do not use partition number):

      dd bs=4M if=path/to/archlinux-version-x86_64.iso of=/dev/sdx conv=fsync oflag=direct status=progress

**1.3 Use guided arch installation:**

- boot to usb and run command:

      archinstall

- some base packages needed for now:

      base base-devel linux-headers git stow dkms coreutils vim bat btop go rust python

---

---

</details>
&nbsp;
<details>
<font size="+2"><summary><b>2. Post-Installation 🧰 </b></font></summary>

---

---

##### [ArchWiki General Recommendations](https://wiki.archlinux.org/title/General_recommendations)

2.1 Install AUR helper ([paru](https://github.com/Morganamilo/paru) is nice too)

    git clone https://aur.archlinux.org/yay-bin.git
    cd yay-bin
    makepkg -si

2.2 Install packages:

    cd $HOME && git clone https://github.com/nosvagor/dotfiles.git
    yay -S - < $HOME/dotfiles/resources/packages.txt

2.3 Configure shell and link config files:

- Change default shell to zsh:

      chsh -s $(which zsh)

- Install shell related packages:

        go install github.com/gsamokovarov/jump@latest
        curl -sS https://starship.rs/install.sh | sh
        $HOME/dotfiles/cullyn/.local/bin/zsh-install

- Link config files:

      cd $HOME/dotfiles && stow cullyn
      stow zsh

**⚠️ Note:** _you probably don't want my zsh config, or many of my personal configs, and should update them now._

2.5 Configure SSH for GitHub _(here for personal reference)_:

      ssh-keygen -t ed25519 -C "your_email@example.com"
      eval "$(ssh-agent -s)"
      ssh-add ~/.ssh/id_ed25519
      bat ~/.ssh/id_ed25519.pub

- add ssh key to github, then update remote origin:

      git remote set-url origin git@github.com:USERNAME/REPOSITORY.git

---

---

</details>
&nbsp;
<details>

---

---

<font size="+2"><summary><b>3. Customization 🖌️</b></font></summary>

3.1 Firefox 🦊 `about:config`

- Update scaling factor if in HiDPI environment:

      layout.css.devPixelsPerPx = 1.25

---

---

</details>

&nbsp;

# Gallery 🎨

&nbsp;

# Support 🤝

- Personal: `cullyn.eth`
- Open source: [gitcoin.co](https://gitcoin.co/)
