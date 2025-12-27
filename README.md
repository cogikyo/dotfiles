<!-- ======================================================================--->
<!-- ☄️  heading {{{ -->
<h1 align="center">⚙️ dotfiles ⚙️</h1>

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
<!-- }}} -->
<!-- ======================================================================--->

<!-- 👾 Overview {{{ -->

## 👾 Overview

### **[I use arch, btw](https://wiki.archlinux.org/title/Arch_Linux)**

---

<img
    src="https://github.com/cogikyo/dotfiles/blob/master/share/social-preview.jpg?raw=true"
    title="we do these things not because they are easy, but because they are hard"
    width=43%
    align=right
/>

> 🌟 Additional guiding principles for software selection:
>
> - 🧪 **Innovative** (what's a better _way_ of doing things?)
> - 📖 **Open** (growth oriented; _infinite_ games)
> - 🌍 **Collaborative** (what can _we_ do better?)
>
> ❌ I try to avoid communities and software that:
>
> - 🚩 follow _the status quo_ (even if it is _easier_ at first)
> - 🥧 have _zero-sum mentality_ (_winning_ is **not** the goal)

---

> In short (JFK definitely would've used arch, btw):
>
> _"...we {do these} things **not** because they are easy, but **because they are hard**_,"<br>
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
- Wallpaper: [mpvpaper](https://github.com/GhostNaN/mpvpaper) (vidoes found [here](https://github.com/cogikyo/dotfiles/tree/master/share/videos))

</details>

<details open>
<summary>🎯 <b>Core Applications</b></summary>

- Editor: [neovim](https://neovim.io/)
- Browser: [Firefox](https://www.mozilla.org/en-US/firefox/developer/)
  - with custom [firefox css](https://github.com/cogikyo/vagari.firefox).
- Terminal: [kitty](https://sw.kovidgoyal.net/kitty/)
- Shell: [zsh](https://wiki.archlinux.org/title/zsh)

</details>

### 🎥 Appearance

<details open>
<summary>🎨 <b>Design Framework</b></summary>

- Color Scheme: [vagari](https://github.com/cogikyo/vagari#palette) (work in progress)
- GTK: [catppuccin macchiato (peach)](https://github.com/catppuccin/gtk) (temporary)
- Cursors: [Nordzy-white](https://github.com/alvatip/Nordzy-cursors) (temporary)
- Icons: [Nordzy](https://github.com/alvatip/Nordzy-icon) (temporary)

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

### 🍎 Notable Applications

- File Explorer: [xplr](https://github.com/sayanarijit/xplr)
- Image Editing: [gimp](https://www.gimp.org/)
- Imagine Generation: [midjourney](https://www.midjourney.com/)
- Image Processing: [darktable](https://www.darktable.org/)
- Vector Graphics: [inkscape](https://inkscape.org/)
- Screen Recording: [wf-recorder](https://github.com/ammen99/wf-recorder)
- Audio Editing: [audacity](https://www.audacityteam.org/)
  - Edit: `/usr/share/applications/audacity.desktop` (currently broken under
    wayland)
  - Change `Exec` line to: `Exec=env GDK_BACKEND=x11 audacity %F`
- Video Editing: [blender](https://www.blender.org/features/video-editing/)
- Music: [spotify](www.spotify.com) with [playerctl](https://github.com/altdesktop/playerctl)
- Music Visualizer: [glava](https://github.com/jarcode-foss/glava)

</details>

<!-- }}} -->

<!-- 🛠️ Installation {{{ -->

## 🛠️ Installation

<details open>
<summary><h5> 🧰 My Hardware</h5></summary>

- Keyboard: [Corne (Helidox) 42 key](https://keebmaker.com/products/corne-low-profile), with Kailh gChoc Light Blue (20g), and **custom layout**:
  ![image](https://user-images.githubusercontent.com/59071534/232157490-bc96cdec-fa8c-4245-a9fe-76fd57a381af.png)
  ![image](https://user-images.githubusercontent.com/59071534/232157618-c49b549f-6acf-4343-96d0-9f9932196b36.png)
  ![image](https://user-images.githubusercontent.com/59071534/232157647-baabd17f-9cf7-43b1-9577-37eb7daa326d.png)
  ![image](https://user-images.githubusercontent.com/59071534/232157666-a6fa76f4-43a2-414b-879d-26a200101e18.png) - ZMK firmware (for bluetooth version of keyboard): [cogikyo/zmk-config](https://github.com/cogikyo/zmk-config)
- Monitor: [SAMSUNG UR59 Series 32-Inch 4K UHD (3840x2160)](https://a.co/d/bZtUse0)
- Mouse: [MX Master 3S](https://www.logitech.com/en-us/products/mice/mx-master-3s.910-006556.html)
- CPU: [AMD Ryzen 7 3700X (16) @ 3.600GHz](https://www.amd.com/en/products/cpu/amd-ryzen-7-3700x)
  - GPU: [AMD ATI Radeon RX 5600 OEM/5600 XT / 5700/5700 XT](https://www.amd.com/en/products/graphics/amd-radeon-rx-5600-xt)
- Microphone: [Shure SM57](https://www.amazon.com/gp/product/B0000AQRST)
  - Audio Interface: [Scaarlett Solo 3rd Gen](https://www.amazon.com/gp/product/B07QR6Z1JB)
- Camera: [Canon EOS M50 Mark II](https://www.amazon.com/gp/product/B08KSLW8N3)
  - Lens: [Sigma 16mm f/1.4](https://www.amazon.com/gp/product/B084KYHYKN)

</details>

<details closed>
<summary><h4> 🔩 Literal Installation</h4></summary>

**1. Get the installation image:**

- **[archlinux-version-x86_64.iso](https://archlinux.org/download/)** (https://archlinux.org/download/)

**2. Prepare an installation medium:**

- Find USB device partition (**sda** or **sdb**, probably):

      lsblk -f

- Write to USB using **[dd](https://wiki.archlinux.org/title/Dd)** (sd"x", do not use partition number):

      dd bs=4M if=path/to/archlinux-version-x86_64.iso of=/dev/sdx conv=fsync oflag=direct status=progress

<br>

**3. Use guided arch installation**

- Boot to USB and run command (does a great job for me and is easy to follow):

      archinstall

- Some relevant options that might not be clear:

      audio client: pipewire
      best effort partition: btrfs
      desktop client: minimal (arch does not support hyprland at this time)

- Best to install a few essential packages during this step:

      base base-devel linux-headers git rustup

</details>

<details open>
<summary><h3> ⚙️  Post-Installation </h3></summary>

> **[General Recommendations](https://wiki.archlinux.org/title/General_recommendations)**  worth the read, seriously
>
> **Note**: the following is not meant to be a generalized installation guide;
> it's essentially a collection of commands to take a minimal arch install to
> my current configuration.
>
> Many of steps are extremely tailored to my needs, or my best attempt to solve
> certain problems. Notably, many of my configs are based around my custom keyboard
> layout, so don't expect shortcuts to be intuitive if you are trying to copy them.
>
> Eventually I'd like to create a generalized script to streamline
> installation for different hardware, but that's not a problem needing to be
> solved at this time.

<br>

**1. Install the latest stable of version of rust using [rustup](https://github.com/rust-lang/rustup):**

    rustup default nightly

<br>

**2. Install [paru](https://github.com/Morganamilo/paru):**

```sh
cd ~
mkdir .cache && cd .cache
git clone https://aur.archlinux.org/paru.git
cd paru
makepkg -si
```

<br>

**3. Clone dotfiles and install packages:**

```sh
cd ~
git clone https://github.com/cogikyo/dotfiles
paru -S --needed - < ~/dotfiles/etc/packages.lst
# might want to review/edit this list ⮭ before installing.

# no good repos nordzy icons/cursors, do manually ⮯
git clone https://github.com/alvatip/Nordzy-icon
cd ~/.cache
cd Nordzy-icon
./install.sh
cd ~/.cache
git clone https://github.com/alvatip/Nordzy-cursors
cd Nordzy-cursors
./install.sh

# note: a location and openweathermap key is needed for weather widget.
# https://home.openweathermap.org/api_keys
echo "{api key here}" > ~/.local/.owm_api_key
echo "{lat} {long}" > ~/.local/.location

```

<br>

**4. Change default sh:**

```sh
chsh -s /usr/bin/zsh
```

<br>

**5. Set up [sddm](https://wiki.archlinux.org/title/SDDM):**

```sh
cd ~/dotfiles/etc/sddm.conf.d
mkdir /etc/sddm.conf.d
sudo cp autologin.conf /etc/sddm.conf.d/
systemctl enable sddm
```

<br>

**6. Symbolically link most config files:**

```sh
mkdir ~/.config # might already be made
ln -sfn ~/dotfiles/config/* ~/.config/
ln -sfn ~/dotfiles/config/zsh/zshrc .zshrc
```

<br>

**7. Setup some root configurations:**

```sh
cd ~/dotfiles/etc
sudo ln -sfn ~/dotfiles/bin/* /usr/bin/
sudo cp bluetooth/main.conf /etc/bluetooth/main.conf
sudo cp udev/* /etc/udev/rules.d/ # probably don't want the bluetooth rules
sudo cp loader.conf /boot/loader/loader.conf
sudo cp gifview.desktop /usr/share/applications/gifview.desktop
sudo cp security/faillock.conf /etc/security/faillock.conf
sudo cp logid.cfg /etc/logid.cfg
sudo systemctl enable bluetooth

# DNS: systemd-resolved with Cloudflare (DNS-over-TLS)
sudo ln -sf ~/dotfiles/etc/systemd/resolved.conf /etc/systemd/
sudo systemctl enable --now systemd-resolved
sudo mkdir -p /etc/NetworkManager/conf.d
echo -e "[main]\ndns=systemd-resolved" | sudo tee /etc/NetworkManager/conf.d/dns.conf
sudo ln -sf /run/systemd/resolve/stub-resolv.conf /etc/resolv.conf
sudo systemctl restart NetworkManager
```

<br>

**8. Configure swap and memory management:**

```sh
# Create btrfs swap subvolume with 16GB swapfile
sudo btrfs subvolume create /swap
sudo truncate -s 0 /swap/swapfile
sudo chattr +C /swap/swapfile        # disable COW (required for btrfs swap)
sudo fallocate -l 16G /swap/swapfile
sudo chmod 600 /swap/swapfile
sudo mkswap /swap/swapfile
sudo swapon /swap/swapfile

# Add to /etc/fstab (low priority so zram is used first)
echo '/swap/swapfile none swap defaults,pri=10 0 0' | sudo tee -a /etc/fstab

# Earlyoom: prevent the brutal kernel OOM killer from randomly killing processes when memory is exhausted.
sudo systemctl enable --now earlyoom
```

<br>

**9. Configure hibernation (suspend-then-hibernate):**

```sh
# Get swapfile resume offset (needed for btrfs swapfile hibernation)
RESUME_OFFSET=$(sudo btrfs inspect-internal map-swapfile -r /swap/swapfile)
RESUME_UUID=$(findmnt -no UUID -T /swap/swapfile)

echo "Add to bootloader options: resume=UUID=$RESUME_UUID resume_offset=$RESUME_OFFSET"

# Edit bootloader entry to add resume parameters
sudo nvim /boot/loader/entries/*_linux.conf
# Add to 'options' line: resume=UUID=<UUID> resume_offset=<OFFSET>

# Add resume hook to mkinitcpio (after filesystems, before fsck)
sudo nvim /etc/mkinitcpio.conf
# HOOKS=(... filesystems resume fsck)
sudo mkinitcpio -P

# Configure suspend-then-hibernate delay
sudo mkdir -p /etc/systemd/sleep.conf.d
sudo cp ~/dotfiles/etc/systemd/sleep.conf.d/hibernate.conf /etc/systemd/sleep.conf.d/
```

<br>

**10. Install fonts:**

```sh
# Extract bundled fonts (Albert Sans, Lora, Archivo, etc.)
mkdir -vp ~/.local/share
tar -xzvf ~/dotfiles/etc/fonts.tar.gz fonts
mv fonts ~/.local/share/

# Build custom Iosevka Vagari (monospace)
cd /tmp
git clone --depth 1 https://github.com/be5invis/Iosevka.git
cd Iosevka
cp ~/dotfiles/etc/iosevka/private-build-plans.toml .
npm install
npm run build -- ttf-unhinted::Vagari --jCmd=12
cp -r dist/Vagari/TTF/* ~/.local/share/fonts/
fc-cache -fv
```

> **TODO:** Switch to curated fonts from [Fontshare](https://www.fontshare.com/):
>
> - Satoshi (geometric sans for UI)
> - General Sans or Cabinet Grotesk (display)
> - Update `fonts.tar.gz` with new selections

<br>

**11. Configure newtab server:**

The custom newtab page (`share/newtab/`) requires updating the Firefox profile path:

```sh
# Find your Firefox profile directory
ls ~/.mozilla/firefox/ | grep -E '\.default|\.dev-edition'

# Update the profile path in main.go (replace YOUR_PROFILE with your actual profile name), e.g.:
sed -i "s/sdfm8kqz.dev-edition-default/YOUR_PROFILE/" ~/dotfiles/share/newtab/main.go
```

Then set Firefox to use the custom newtab:

1. Open `about:config`
2. Set `browser.newtabpage.enabled` = `false`
3. Install an extension like [New Tab Override](https://addons.mozilla.org/en-US/firefox/addon/new-tab-override/)
4. Set custom URL to `http://localhost:42069`

Start the server (or add to startup):

```sh
cd ~/dotfiles/share/newtab && go run . &
# Or use the bin script: newtab-server
```

<br>

**12. Update Firefox `about:config` options:**

- Increase scaling factor due to 4k screen (HiDPI environment):

      layout.css.devPixelsPerPx = 1.25

- Stop asking to restore session (killing the windows counts as a crash, I guess)

      browser.sessionstore.resume_from_crash = false

- Hide PDF sidebar by default

      pdfjs.sidebarViewOnLoad = 0

- Disable some bloat:

      extensions.pocket.enabled = false

- Review basic settings (fonts 'n such)
- Install [vagari.firefox](https://github.com/cogikyo/vagari.firefox) userChrome css

<br>

**13. Configure SSH key:**

```sh
ssh-keygen -t ed25519 -C "your_email@example.com"
eval "$(ssh-agent -s)"
ssh-add ~/.ssh/id_ed25519

# https://github.com/settings/keys
# view your key, which github needs for auth/singing
bat ~/.ssh/id_ed25519.pub
```

- e.g., using SSH key to manage dotfiles:

```sh
# example moving to ssh origin:
git remote set-url origin git@github.com:cogikyo/dotfiles.git
```

<br>

> **My preferred directory structure:** _(definitely optional)_

```sh
cd ~
mkdir -vp media/music media/images media/videos media/gifs media/screenshots media/recordings
mkdir -vp downloads docs/templates docs/books docs/papers docs/share
git clone git@github.com:cogikyo/vagari.git
git clone git@github.com:cogikyo/notes.git
```

<!-- }}} -->
