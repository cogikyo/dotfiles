#!/usr/bin/env bash
# install - Unified post-installation script for dotfiles
#
# Usage:
#   ./install.sh             # Interactive menu to select steps
#   ./install.sh all         # Run all steps in order
#   ./install.sh link go     # Run specific steps by name
#   ./install.sh --list      # List available steps
#   ./install.sh --help      # Show usage

set -euo pipefail

INSTALL_VERSION="2026.02.09.1"
DOTFILES="${DOTFILES:-$HOME/dotfiles}"
DOTFILES_REPO="${DOTFILES_REPO:-https://github.com/cogikyo/dotfiles.git}"
DOTFILES_REF="${DOTFILES_REF:-master}"

# Colors
R='\033[0;31m'
G='\033[0;32m'
Y='\033[1;33m'
B='\033[1;34m'
M='\033[0;35m'
F='\033[90m'
BD='\033[1m'
N='\033[0m'

info()    { printf '%b(↓)%b %s\n' "$B" "$N" "$*"; }
step()    { printf '%b(→)%b %s\n' "$B" "$N" "$*"; }
success() { printf '%b(✓)%b %s\n' "$G" "$N" "$*"; }
finish()  { printf '\n%b(✓✓) %s%b\n' "$G" "$*" "$N"; }
warn()    { printf '\n%b(!) %s%b\n' "$Y" "$*" "$N"; }
error()   { printf '%b(✗)%b %s\n' "$R" "$N" "$*" >&2; }
ask()     { printf '%b(?)%b %s\n' "$Y" "$N" "$*"; }
header()  { printf '\n%b━━━ %s ━━━%b\n\n' "$M" "$*" "$N"; }
faint()   { printf '%b%s%b\n' "$F" "$*" "$N"; }
script_header() { printf '%b== dotfiles install v%s ==%b\n' "$B" "$INSTALL_VERSION" "$N"; }

# Step definitions: name|description|requires_sudo|depends
STEPS=(
    "packages|Install packages from saved lists|yes|"
    "link|Symlink configs and scripts|no|"
    "secrets|Decrypt age-encrypted secrets to target paths|no|"
    "repos|Clone repositories and create directories|no|secrets"
    "system|Install system configs and enable services|yes|"
    "swap|Set up btrfs swap subvolume and swapfile|yes|"
    "hibernate|Configure suspend-then-hibernate|yes|swap"
    "fonts|Extract fonts and optionally build Iosevka|no|"
    "go|Build Go binaries (hyprd, ewwd, statusline, newtab)|no|"
    "firefox|Configure Firefox profile, theme, and preferences|no|"
    "shell|Change default shell to zsh|yes|"
    "dns|Set up systemd-resolved with Cloudflare DNS-over-TLS|yes|system"
)

# Track results across steps
PASSED=()
FAILED=()
SKIPPED=()
declare -A STEP_ACTIVE=()

# ── Helpers ──────────────────────────────────────────────────────────────────

confirm() {
    local prompt="$1" default="${2:-y}"
    local yn
    if [[ "$default" == "y" ]]; then
        read -rp "$prompt [Y/n] " yn
        yn="${yn:-y}"
    else
        read -rp "$prompt [y/N] " yn
        yn="${yn:-n}"
    fi
    [[ "$yn" =~ ^[Yy] ]]
}

has() { command -v "$1" &>/dev/null; }

array_contains() {
    local needle="$1"
    shift
    local item
    for item in "$@"; do
        [[ "$item" == "$needle" ]] && return 0
    done
    return 1
}

canonpath() {
    if has realpath; then
        realpath -m -- "$1"
    elif has readlink; then
        readlink -f -- "$1" 2>/dev/null || printf '%s\n' "$1"
    else
        printf '%s\n' "$1"
    fi
}

ensure_dotfiles_checkout() {
    local script_path script_real target_real
    script_path="${BASH_SOURCE[0]:-$0}"
    script_real=$(canonpath "$script_path")
    target_real=$(canonpath "$DOTFILES/install.sh")

    if [[ "$script_real" == "$target_real" ]]; then
        return 0
    fi

    if [[ ! -d "$DOTFILES/.git" || ! -f "$DOTFILES/install.sh" ]]; then
        has git || { error "git not found. Install git first."; exit 1; }

        if [[ -e "$DOTFILES" && ! -d "$DOTFILES/.git" ]]; then
            error "DOTFILES path exists but is not a git checkout: $DOTFILES"
            exit 1
        fi

        info "Bootstrapping dotfiles into $DOTFILES..."
        git clone --depth 1 --branch "$DOTFILES_REF" "$DOTFILES_REPO" "$DOTFILES"
    fi

    info "Re-running installer from $DOTFILES/install.sh..."
    exec "$DOTFILES/install.sh" "$@"
}

ensure_rustup_stable() {
    if ! has rustup; then
        warn "rustup not found; skipping Rust toolchain initialization"
        return 0
    fi

    local active=""
    active=$(rustup show active-toolchain 2>/dev/null | awk 'NR==1 {print $1}')

    if [[ "$active" == stable* ]]; then
        success "Rust toolchain already set to stable ($active)"
        return 0
    fi

    info "Initializing rustup stable toolchain..."
    rustup toolchain install stable
    rustup default stable
    success "Rust toolchain set to stable"
}

has_user_bus() {
    [[ -n "${DBUS_SESSION_BUS_ADDRESS:-}" ]] && return 0
    [[ -n "${XDG_RUNTIME_DIR:-}" && -S "${XDG_RUNTIME_DIR}/bus" ]] && return 0
    has busctl && busctl --user status &>/dev/null
}

in_chroot() {
    if has systemd-detect-virt; then
        systemd-detect-virt --quiet --chroot && return 0
    fi

    local root_id init_root_id
    root_id=$(stat -Lc '%d:%i' / 2>/dev/null || true)
    init_root_id=$(stat -Lc '%d:%i' /proc/1/root/. 2>/dev/null || true)
    [[ -n "$root_id" && -n "$init_root_id" && "$root_id" != "$init_root_id" ]]
}

require_desktop_environment() {
    if [[ "${DOTFILES_INSTALL_ALLOW_HEADLESS:-0}" == "1" ]]; then
        warn "Headless override enabled (DOTFILES_INSTALL_ALLOW_HEADLESS=1)"
        return 0
    fi

    if [[ $EUID -eq 0 ]]; then
        error "Run install.sh as your regular user (not root)"
        return 1
    fi

    if [[ -n "${SSH_CONNECTION:-}" || -n "${SSH_TTY:-}" ]]; then
        error "Refusing to run from SSH session"
        return 1
    fi

    if in_chroot; then
        error "Refusing to run from chroot/containerized install environment"
        return 1
    fi

    if [[ -z "${XDG_SESSION_TYPE:-}" ]]; then
        error "No desktop session detected (XDG_SESSION_TYPE is unset)"
        return 1
    fi

    if [[ "$XDG_SESSION_TYPE" != "wayland" && "$XDG_SESSION_TYPE" != "x11" ]]; then
        error "Unsupported session type '$XDG_SESSION_TYPE' (expected wayland or x11)"
        return 1
    fi

    if ! has_user_bus; then
        error "No user DBus/systemd session bus detected"
        return 1
    fi
}

needs_sudo() {
    if [[ $EUID -ne 0 ]]; then
        if ! sudo -n true 2>/dev/null; then
            info "Some steps require sudo access"
            sudo -v
        fi
    fi
}

# ── Step: packages ───────────────────────────────────────────────────────────

step_packages() {
    header "Installing packages"

    if ! has paru; then
        error "paru not found. Install it first:"
        echo "  cd ~/.cache && git clone https://aur.archlinux.org/paru.git && cd paru && makepkg -si"
        return 1
    fi

    "$DOTFILES/bin/update" --install
    ensure_rustup_stable
}

# ── Step: link ───────────────────────────────────────────────────────────────

step_link() {
    header "Linking configs and scripts"

    link_tree_contents() {
        local src="$1"
        local dst="$2"
        mkdir -p "$dst"

        local item name target
        shopt -s dotglob nullglob
        for item in "$src"/*; do
            name=$(basename "$item")
            [[ "$name" == "." || "$name" == ".." ]] && continue
            target="$dst/$name"

            if [[ -d "$item" && ! -L "$item" ]]; then
                if [[ -e "$target" && ! -d "$target" ]]; then
                    rm -f "$target"
                fi
                mkdir -p "$target"
                link_tree_contents "$item" "$target"
            else
                ln -sfnv "$item" "$target"
            fi
        done
        shopt -u dotglob nullglob
    }

    # Config directories → ~/.config/<name> (content-level sync via symlinks)
    info "Linking config content into ~/.config/..."
    mkdir -p "$HOME/.config"
    mkdir -p "$HOME/.local/bin"

    for item in "$DOTFILES"/config/*; do
        local name
        name=$(basename "$item")

        case "$name" in
            claude|Cursor|firefox) continue ;;
        esac

        if [[ -d "$item" && ! -L "$item" ]]; then
            link_tree_contents "$item" "$HOME/.config/$name"
        else
            ln -sfnv "$item" "$HOME/.config/$name"
        fi
    done

    # Claude: partial linking
    info "Linking claude settings, skills, and scripts..."
    mkdir -p "$HOME/.config/claude"
    ln -sfnv "$DOTFILES/config/claude/settings.json" "$HOME/.config/claude/settings.json"
    ln -sfv "$DOTFILES/config/claude/claude-notify" "$HOME/.local/bin/claude-notify"
    "$DOTFILES/config/claude/skills/link.sh" user

    # .zshrc symlink
    info "Linking .zshrc..."
    ln -sfnv "$DOTFILES/config/zsh/zshrc" "$HOME/.zshrc"

    # Scripts → ~/.local/bin/
    info "Linking scripts to ~/.local/bin/..."

    for script in "$DOTFILES"/bin/*; do
        [[ -x "$script" ]] || continue
        ln -sfv "$script" "$HOME/.local/bin/"
    done

    success "Linking complete"
}

# ── Step: secrets ───────────────────────────────────────────────────────────

step_secrets() {
    header "Decrypting secrets"

    if ! has age; then
        error "age not found — install with: paru -S age"
        return 1
    fi

    local manifest="$DOTFILES/secrets/manifest"
    if [[ ! -f "$manifest" ]] || ! grep -qv '^#\|^$' "$manifest"; then
        warn "No secrets configured yet"
        info "Add entries in $DOTFILES/secrets/manifest, then run: secrets sync"
        return 0
    fi

    "$DOTFILES/bin/secrets" decrypt
}

# ── Step: repos ──────────────────────────────────────────────────────────────

step_repos() {
    header "Cloning repositories and creating directories"

    local REPOS_FILE="$DOTFILES/etc/repos.toml"

    if [[ ! -f "$REPOS_FILE" ]]; then
        error "Repos manifest not found: $REPOS_FILE"
        return 1
    fi

    # Create standard directories
    local DIRS=(
        "$HOME/downloads"
        "$HOME/documents"
        "$HOME/media/screenshots"
        "$HOME/media/recordings"
        "$HOME/media/images"
        "$HOME/media/gifs"
        "$HOME/agents"
    )

    for dir in "${DIRS[@]}"; do
        if [[ ! -d "$dir" ]]; then
            mkdir -p "$dir"
            success "Created $dir"
        fi
    done

    # Parse repos.toml and clone
    local repo="" path="" cloned=0 skipped=0

    while IFS= read -r line; do
        line="${line%%#*}"
        line="${line#"${line%%[![:space:]]*}"}"
        [[ -z "$line" ]] && continue

        if [[ "$line" =~ ^\[.*\] ]]; then
            # Process previous entry before starting new section
            if [[ -n "$repo" && -n "$path" ]]; then
                local expanded="${path/#\~/$HOME}"
                if [[ -d "$expanded" ]]; then
                    ((skipped++))
                else
                    mkdir -p "$(dirname "$expanded")"
                    info "Cloning $repo → $path"
                    git clone "git@github.com:$repo.git" "$expanded"
                    ((cloned++))
                fi
            fi
            repo="" path=""
            continue
        fi

        local key="${line%%=*}"
        local val="${line#*=}"
        key="${key#"${key%%[![:space:]]*}"}"
        key="${key%"${key##*[![:space:]]}"}"
        val="${val#"${val%%[![:space:]]*}"}"
        val="${val%"${val##*[![:space:]]}"}"
        val="${val#\"}" val="${val%\"}"

        case "$key" in
            repo) repo="$val" ;;
            path) path="$val" ;;
        esac
    done < "$REPOS_FILE"

    # Process last entry
    if [[ -n "$repo" && -n "$path" ]]; then
        local expanded="${path/#\~/$HOME}"
        if [[ -d "$expanded" ]]; then
            ((skipped++))
        else
            mkdir -p "$(dirname "$expanded")"
            info "Cloning $repo → $path"
            git clone "git@github.com:$repo.git" "$expanded"
            ((cloned++))
        fi
    fi

    echo
    success "Cloned $cloned repos ($skipped already exist)"
}

# ── Step: system ─────────────────────────────────────────────────────────────

step_system() {
    header "Installing system configs"
    needs_sudo

    # Source → destination mappings
    declare -A SYSTEM_FILES=(
        ["bluetooth/main.conf"]="/etc/bluetooth/main.conf"
        ["udev/81-bluetooth-hci.rules"]="/etc/udev/rules.d/81-bluetooth-hci.rules"
        ["udev/92-viia.rules"]="/etc/udev/rules.d/92-viia.rules"
        ["sddm.conf.d/autologin.conf"]="/etc/sddm.conf.d/autologin.conf"
        ["sddm.conf.d/hyprland.desktop"]="/etc/sddm.conf.d/hyprland.desktop"
        ["systemd/resolved.conf"]="/etc/systemd/resolved.conf"
        ["systemd/sleep.conf.d/hibernate.conf"]="/etc/systemd/sleep.conf.d/hibernate.conf"
        ["security/faillock.conf"]="/etc/security/faillock.conf"
        ["loader.conf"]="/boot/loader/loader.conf"
        ["logid.cfg"]="/etc/logid.cfg"
        ["gifview.desktop"]="/usr/share/applications/gifview.desktop"
    )

    local installed=0
    local skipped=0

    for src in "${!SYSTEM_FILES[@]}"; do
        local src_path="$DOTFILES/etc/$src"
        local dst_path="${SYSTEM_FILES[$src]}"

        if [[ ! -f "$src_path" ]]; then
            warn "Source not found: $src_path (skipping)"
            ((skipped++))
            continue
        fi

        local dst_dir
        dst_dir=$(dirname "$dst_path")
        [[ -d "$dst_dir" ]] || sudo mkdir -p "$dst_dir"

        if [[ -f "$dst_path" ]] && diff -q "$src_path" "$dst_path" &>/dev/null; then
            ((skipped++))
            continue
        fi

        info "Installing $src → $dst_path"
        sudo cp "$src_path" "$dst_path"
        ((installed++))
    done

    success "Installed $installed system configs ($skipped already up to date)"

    # Enable services
    echo
    info "Enabling services..."
    local SERVICES=(bluetooth sddm earlyoom)
    for svc in "${SERVICES[@]}"; do
        if systemctl is-enabled "$svc" &>/dev/null; then
            success "$svc already enabled"
        else
            info "Enabling $svc..."
            sudo systemctl enable "$svc"
            success "$svc enabled"
        fi
    done

    # Reload udev rules
    info "Reloading udev rules..."
    sudo udevadm control --reload-rules
    sudo udevadm trigger
}

# ── Step: swap ───────────────────────────────────────────────────────────────

step_swap() {
    header "Setting up swap"
    needs_sudo

    local SWAP_SIZE="${DOTFILES_SWAP_SIZE:-16G}"
    local SWAP_DIR="/swap"
    local SWAP_FILE="$SWAP_DIR/swapfile"

    if ! has btrfs; then
        error "btrfs command not found. Install btrfs-progs first."
        return 1
    fi

    # Ensure /swap is a dedicated btrfs subvolume
    if sudo btrfs subvolume show "$SWAP_DIR" &>/dev/null; then
        success "Swap subvolume already exists at $SWAP_DIR"
    else
        if [[ -e "$SWAP_DIR" ]]; then
            error "$SWAP_DIR exists but is not a btrfs subvolume. Refusing to modify it automatically."
            return 1
        fi
        info "Creating btrfs swap subvolume at $SWAP_DIR..."
        sudo btrfs subvolume create "$SWAP_DIR"
    fi

    # Prevent compression for swap extents.
    sudo btrfs property set "$SWAP_DIR" compression none &>/dev/null || true

    # Check if swapfile is active
    if swapon --show=NAME --noheadings | sed 's/^[[:space:]]*//' | grep -Fxq "$SWAP_FILE"; then
        success "Swapfile already active: $SWAP_FILE"
    else
        if [[ -e "$SWAP_FILE" ]]; then
            warn "Existing inactive swapfile found, recreating: $SWAP_FILE"
            sudo swapoff "$SWAP_FILE" 2>/dev/null || true
            sudo rm -f "$SWAP_FILE"
        fi

        if sudo btrfs filesystem mkswapfile --help &>/dev/null; then
            info "Creating ${SWAP_SIZE} swapfile with btrfs mkswapfile..."
            sudo btrfs filesystem mkswapfile --size "$SWAP_SIZE" --uuid clear "$SWAP_FILE"
        else
            info "Creating ${SWAP_SIZE} swapfile (manual btrfs-safe path)..."
            sudo truncate -s 0 "$SWAP_FILE"
            sudo chattr +C "$SWAP_FILE"
            sudo btrfs property set "$SWAP_FILE" compression none &>/dev/null || true
            sudo fallocate -l "$SWAP_SIZE" "$SWAP_FILE"
            sudo chmod 600 "$SWAP_FILE"
            sudo mkswap "$SWAP_FILE"
        fi

        info "Activating swap..."
        sudo swapon "$SWAP_FILE"
    fi

    # Add to fstab if not present
    if awk -v swap_path="$SWAP_FILE" '$0 !~ /^[[:space:]]*#/ && $1 == swap_path && $3 == "swap" {found=1} END {exit(found ? 0 : 1)}' /etc/fstab; then
        success "Swapfile already in /etc/fstab"
    else
        info "Adding swapfile to /etc/fstab..."
        printf '%s none swap defaults,pri=10 0 0\n' "$SWAP_FILE" | sudo tee -a /etc/fstab > /dev/null
        success "Added to /etc/fstab"
    fi

    success "Swap setup complete"
}

# ── Step: hibernate ──────────────────────────────────────────────────────────

step_hibernate() {
    header "Configuring hibernation"
    needs_sudo

    local SWAP_FILE="/swap/swapfile"

    if [[ ! -f "$SWAP_FILE" ]]; then
        error "Swapfile not found at $SWAP_FILE. Run the 'swap' step first."
        return 1
    fi

    # Get resume parameters
    local RESUME_OFFSET
    RESUME_OFFSET=$(sudo btrfs inspect-internal map-swapfile -r "$SWAP_FILE")
    local RESUME_UUID
    RESUME_UUID=$(findmnt -no UUID -T "$SWAP_FILE")

    info "Resume UUID:   $RESUME_UUID"
    info "Resume Offset: $RESUME_OFFSET"

    # Update bootloader entry
    local LOADER_ENTRY
    LOADER_ENTRY=$(find /boot/loader/entries/ -name '*_linux.conf' 2>/dev/null | head -1)

    if [[ -z "$LOADER_ENTRY" ]]; then
        error "No bootloader entry found in /boot/loader/entries/"
        error "Manually add to your bootloader options: resume=UUID=$RESUME_UUID resume_offset=$RESUME_OFFSET"
        return 1
    fi

    if grep -q "resume=UUID=" "$LOADER_ENTRY"; then
        warn "Resume parameters already present in $LOADER_ENTRY"
        info "Current options line:"
        grep "^options" "$LOADER_ENTRY"
        echo
        warn "Verify these match: resume=UUID=$RESUME_UUID resume_offset=$RESUME_OFFSET"
    else
        info "Adding resume parameters to $LOADER_ENTRY..."
        sudo sed -i "s|^options.*|& resume=UUID=$RESUME_UUID resume_offset=$RESUME_OFFSET|" "$LOADER_ENTRY"
        success "Updated bootloader entry"
    fi

    # Add resume hook to mkinitcpio
    if grep -q 'HOOKS=.*resume' /etc/mkinitcpio.conf; then
        success "resume hook already in mkinitcpio.conf"
    else
        info "Adding resume hook to mkinitcpio.conf..."
        sudo sed -i 's/\(filesystems\)/\1 resume/' /etc/mkinitcpio.conf
        info "Regenerating initramfs..."
        sudo mkinitcpio -P
        success "Initramfs rebuilt with resume hook"
    fi

    # Verify sleep config
    if [[ -f /etc/systemd/sleep.conf.d/hibernate.conf ]]; then
        success "sleep.conf.d/hibernate.conf already installed"
    else
        warn "sleep.conf.d/hibernate.conf not found — run the 'system' step"
    fi

    success "Hibernation configured"
}

# ── Step: fonts ──────────────────────────────────────────────────────────────

step_fonts() {
    header "Installing fonts"

    local FONT_DIR="$HOME/.local/share/fonts"
    local TAR_FILE="$DOTFILES/etc/fonts.tar.gz"

    if [[ ! -f "$TAR_FILE" ]]; then
        error "Font archive not found: $TAR_FILE"
        return 1
    fi

    # Extract bundled fonts
    if [[ -d "$FONT_DIR" ]] && [[ -n "$(ls -A "$FONT_DIR" 2>/dev/null)" ]]; then
        success "Font directory already populated: $FONT_DIR"
        if confirm "Re-extract fonts from fonts.tar.gz?" "n"; then
            mkdir -p "$FONT_DIR"
            info "Extracting fonts..."
            tar -xzf "$TAR_FILE" -C "$HOME/.local/share/"
            success "Fonts extracted"
        fi
    else
        mkdir -p "$HOME/.local/share"
        info "Extracting fonts from fonts.tar.gz..."
        tar -xzf "$TAR_FILE" -C "$HOME/.local/share/"
        success "Fonts extracted to $FONT_DIR"
    fi

    # Optionally build Iosevka Vagari
    echo
    if confirm "Build Iosevka Vagari custom font? (requires npm, takes a while)" "n"; then
        if [[ -x "$DOTFILES/etc/iosevka/build.sh" ]]; then
            "$DOTFILES/etc/iosevka/build.sh"
        else
            error "Iosevka build script not found at $DOTFILES/etc/iosevka/build.sh"
            return 1
        fi
    fi

    info "Refreshing font cache..."
    fc-cache -f
    success "Fonts installed"
}

# ── Step: go ─────────────────────────────────────────────────────────────────

# Binary definitions: name → module_dir|build_path|description|is_daemon|output_dir
declare -A GO_BINARIES=(
    ["hyprd"]="daemons|./hyprd|Hyprland window management daemon|yes|"
    ["ewwd"]="daemons|./ewwd|System utilities daemon for eww|yes|"
    ["statusline"]="config/claude/statusline|.|Claude Code statusline generator|no|config/claude/statusline"
    ["newtab"]="daemons/newtab|.|Firefox new tab HTTP server|yes|"
)

build_go_binary() {
    local name="$1"

    IFS='|' read -r module_dir build_path desc is_daemon output_dir <<< "${GO_BINARIES[$name]}"
    local full_path="$DOTFILES/$module_dir"
    local install_dir="$HOME/.local/bin"
    [[ -n "$output_dir" ]] && install_dir="$DOTFILES/$output_dir"

    if [[ ! -d "$full_path" ]] || [[ ! -f "$full_path/go.mod" ]]; then
        error "Module not found: $full_path"
        return 1
    fi

    info "Building $name from $module_dir/$build_path"
    (cd "$full_path" && go build -o "$install_dir/$name" "$build_path")
    success "Installed $name → $install_dir/$name"
}

install_daemon_services() {
    local service_dir="$HOME/.config/systemd/user"
    mkdir -p "$service_dir"

    if ! has_user_bus; then
        error "No user session bus available; cannot manage user services"
        return 1
    fi

    local daemons=()
    for name in "${!GO_BINARIES[@]}"; do
        IFS='|' read -r module_dir _ _ is_daemon <<< "${GO_BINARIES[$name]}"
        [[ "$is_daemon" == "yes" ]] || continue

        local src="$DOTFILES/$module_dir/$name/$name.service"
        # newtab has its own module dir
        [[ -f "$src" ]] || src="$DOTFILES/$module_dir/$name.service"

        if [[ ! -f "$src" ]]; then
            warn "Service file not found for $name (skipping)"
            continue
        fi

        daemons+=("$name")
        cp "$src" "$service_dir/$name.service"
    done

    systemctl --user daemon-reload

    for name in "${daemons[@]}"; do
        if systemctl --user is-active "$name" &>/dev/null; then
            info "Restarting $name..."
            systemctl --user restart "$name"
            success "$name restarted"
        else
            info "Enabling $name..."
            systemctl --user enable --now "$name"
            success "$name enabled and started"
        fi
    done
}

step_go() {
    header "Building Go binaries"

    if ! has go; then
        error "Go not found. Install it from packages first."
        return 1
    fi

    local install_dir="$HOME/.local/bin"
    mkdir -p "$install_dir"

    local failed=0
    local built=0

    for name in "${!GO_BINARIES[@]}"; do
        if build_go_binary "$name"; then
            ((++built))
        else
            ((++failed))
        fi
    done

    echo
    if [[ $failed -eq 0 ]]; then
        success "All $built binaries built successfully"
    else
        warn "$built succeeded, $failed failed"
        return 1
    fi

    install_daemon_services
}

# ── Step: firefox ───────────────────────────────────────────────────────────

FIREFOX_PROFILE_DIR=""
FIREFOX_PROFILE_REL=""

detect_firefox_profile() {
    local ff_root=""

    for candidate in "$HOME/.mozilla/firefox" "$HOME/.config/mozilla/firefox"; do
        if [[ -f "$candidate/profiles.ini" ]]; then
            ff_root="$candidate"
            break
        fi
    done

    if [[ -z "$ff_root" ]]; then
        error "No Firefox profiles.ini found"
        error "Start Firefox at least once to generate a profile, then re-run"
        return 1
    fi

    # Parse profiles.ini for dev-edition-default profile
    local profile_path="" section_name="" section_path=""

    while IFS='=' read -r key value; do
        key="${key%%[[:space:]]*}"
        value="${value##[[:space:]]}"
        case "$key" in
            \[*) section_name=""; section_path="" ;;
            Name) section_name="$value" ;;
            Path) section_path="$value" ;;
        esac
        if [[ "$section_name" == "dev-edition-default" && -n "$section_path" ]]; then
            profile_path="$section_path"
            break
        fi
    done < "$ff_root/profiles.ini"

    if [[ -z "$profile_path" ]]; then
        error "Firefox Developer Edition profile not found in $ff_root/profiles.ini"
        error "Install Firefox Developer Edition and launch it once to create a profile"
        return 1
    fi

    local full_path="$ff_root/$profile_path"
    if [[ ! -d "$full_path" ]]; then
        error "Profile directory does not exist: $full_path"
        error "Start Firefox to initialize the profile, then re-run"
        return 1
    fi

    FIREFOX_PROFILE_DIR="$full_path"
    FIREFOX_PROFILE_REL="${full_path#"$HOME"/}"
    info "Detected Firefox profile: $FIREFOX_PROFILE_REL"
}

step_firefox() {
    header "Configuring Firefox"

    detect_firefox_profile

    # Install vagari.firefox userChrome CSS
    local vagari_dir="$HOME/vagari.firefox"

    if [[ ! -d "$vagari_dir" ]]; then
        info "Cloning vagari.firefox..."
        git clone https://github.com/cogikyo/vagari.firefox.git "$vagari_dir"
    else
        success "vagari.firefox already cloned at $vagari_dir"
    fi

    local chrome_dir="$FIREFOX_PROFILE_DIR/chrome"
    mkdir -p "$chrome_dir"

    info "Linking userChrome CSS files..."
    for css_file in "$vagari_dir"/css/*; do
        ln -sfv "$css_file" "$chrome_dir/"
    done
    success "userChrome CSS linked"

    # Install user.js preferences
    local userjs_src="$DOTFILES/config/firefox/user.js"
    local userjs_dst="$FIREFOX_PROFILE_DIR/user.js"

    if [[ ! -f "$userjs_src" ]]; then
        error "user.js not found at $userjs_src"
        return 1
    fi

    ln -sfv "$userjs_src" "$userjs_dst"
    success "user.js linked"

    # Update newtab config with profile path
    local config_yaml="$DOTFILES/daemons/config.yaml"
    local new_db_path="$FIREFOX_PROFILE_REL/places.sqlite"

    if [[ ! -f "$config_yaml" ]]; then
        warn "daemons/config.yaml not found, skipping newtab config update"
    else
        local current_db
        current_db=$(grep 'firefox_db:' "$config_yaml" | sed 's/.*firefox_db:[[:space:]]*//' | tr -d '"')

        if [[ "$current_db" == "$new_db_path" ]]; then
            success "newtab config already points to correct profile"
        else
            info "Updating newtab firefox_db path..."
            sed -i "s|firefox_db:.*|firefox_db: \"$new_db_path\"|" "$config_yaml"
            success "Updated firefox_db to $new_db_path"
        fi
    fi

    echo
    success "Firefox configured"
    warn "Restart Firefox for changes to take effect"
}

# ── Step: shell ──────────────────────────────────────────────────────────────

step_shell() {
    header "Setting default shell to zsh"

    local current_shell
    current_shell=$(getent passwd "$USER" | cut -d: -f7)

    if [[ "$current_shell" == "/usr/bin/zsh" ]]; then
        success "Default shell is already zsh"
        return 0
    fi

    if ! has zsh; then
        error "zsh not found. Install it from packages first."
        return 1
    fi

    info "Changing default shell from $current_shell to /usr/bin/zsh..."
    chsh -s /usr/bin/zsh
    success "Default shell changed to zsh (takes effect on next login)"
}

# ── Step: dns ────────────────────────────────────────────────────────────────

step_dns() {
    header "Setting up DNS (systemd-resolved + Cloudflare DNS-over-TLS)"
    needs_sudo

    local RESOLVED_SRC="$DOTFILES/etc/systemd/resolved.conf"
    local RESOLVED_DST="/etc/systemd/resolved.conf"

    # Require a synced resolved.conf from step_system.
    if [[ ! -f "$RESOLVED_SRC" ]]; then
        error "Source resolved.conf not found at $RESOLVED_SRC"
        return 1
    fi
    if ! sudo test -f "$RESOLVED_DST"; then
        error "$RESOLVED_DST not found."
        error "Run the 'system' step first."
        return 1
    fi
    if ! sudo cmp -s "$RESOLVED_SRC" "$RESOLVED_DST"; then
        error "resolved.conf is missing or outdated."
        error "Run './install.sh system' before running 'dns'."
        return 1
    fi

    # Enable/start systemd-resolved
    if systemctl is-enabled systemd-resolved &>/dev/null; then
        success "systemd-resolved already enabled"
    else
        info "Enabling systemd-resolved..."
        sudo systemctl enable systemd-resolved
    fi
    sudo systemctl restart systemd-resolved

    # Configure NetworkManager via dedicated drop-in.
    local NM_DIR="/etc/NetworkManager/conf.d"
    local NM_CONF="$NM_DIR/10-dotfiles-dns.conf"
    local nm_tmp
    nm_tmp=$(mktemp)
    printf "[main]\ndns=systemd-resolved\n" > "$nm_tmp"
    if sudo test -f "$NM_CONF" && sudo cmp -s "$nm_tmp" "$NM_CONF"; then
        success "NetworkManager DNS drop-in already configured"
    else
        info "Configuring NetworkManager to use systemd-resolved..."
        sudo mkdir -p "$NM_DIR"
        sudo install -m 0644 "$nm_tmp" "$NM_CONF"
    fi
    rm -f "$nm_tmp"

    local nm_conflicts
    nm_conflicts=$(grep -Rns --include='*.conf' '^[[:space:]]*dns=' "$NM_DIR" 2>/dev/null | grep -Fv "$NM_CONF" | grep -Fv 'dns=systemd-resolved' || true)
    if [[ -n "$nm_conflicts" ]]; then
        warn "Other NetworkManager dns= entries were found and may override this setup:"
        printf '%s\n' "$nm_conflicts"
    fi

    if [[ "${DOTFILES_DNS_FORCE_CLOUDFLARE:-0}" == "1" ]]; then
        if ! has nmcli; then
            warn "DOTFILES_DNS_FORCE_CLOUDFLARE=1 requested, but nmcli is not available"
        else
            info "Forcing active NetworkManager connections to Cloudflare DNS..."
            local conn
            while IFS= read -r conn; do
                [[ -n "$conn" ]] || continue
                sudo nmcli connection modify "$conn" \
                    ipv4.ignore-auto-dns yes \
                    ipv4.dns "1.1.1.1 1.0.0.1" \
                    ipv6.ignore-auto-dns yes \
                    ipv6.dns "2606:4700:4700::1111 2606:4700:4700::1001"
            done < <(nmcli -t -f NAME connection show --active 2>/dev/null)
        fi
    fi

    # Link resolv.conf
    local STUB="/run/systemd/resolve/stub-resolv.conf"
    if [[ -L /etc/resolv.conf ]] && [[ "$(readlink /etc/resolv.conf)" == "$STUB" ]]; then
        success "resolv.conf already linked to stub"
    else
        if [[ -e /etc/resolv.conf && ! -L /etc/resolv.conf ]]; then
            info "Backing up existing /etc/resolv.conf to /etc/resolv.conf.pre-dotfiles.bak"
            sudo cp /etc/resolv.conf /etc/resolv.conf.pre-dotfiles.bak
        fi
        info "Linking resolv.conf to systemd-resolved stub..."
        sudo ln -sfn "$STUB" /etc/resolv.conf
    fi

    info "Restarting NetworkManager..."
    sudo systemctl restart NetworkManager

    if ! systemctl is-active systemd-resolved &>/dev/null; then
        error "systemd-resolved is not active after configuration"
        return 1
    fi
    if [[ "$(readlink /etc/resolv.conf 2>/dev/null || true)" != "$STUB" ]]; then
        error "/etc/resolv.conf is not linked to $STUB"
        return 1
    fi

    if has resolvectl; then
        local runtime_dns
        runtime_dns=$(resolvectl dns 2>/dev/null | awk 'NF > 2 {for (i = 3; i <= NF; i++) print $i}' | sort -u | tr '\n' ' ')
        [[ -n "$runtime_dns" ]] && info "Runtime DNS servers: $runtime_dns"
    fi

    success "DNS configured with Cloudflare DNS-over-TLS + strict DNSSEC"
}

# ── Menu and dispatch ────────────────────────────────────────────────────────

list_steps() {
    echo "Available steps:"
    echo
    local i=1
    for entry in "${STEPS[@]}"; do
        IFS='|' read -r name desc needs_root deps <<< "$entry"
        local badges=""
        [[ "$needs_root" == "yes" ]] && badges+=" ${Y}(sudo)${N}"
        [[ -n "$deps" ]] && badges+=" ${B}(after: $deps)${N}"
        printf '  %b%02d%b. %-12s %s%b\n' "$BD" "$i" "$N" "$name" "$desc" "$badges"
        ((i++))
    done
}

get_step_deps() {
    local target="$1"
    for entry in "${STEPS[@]}"; do
        IFS='|' read -r name _ _ deps <<< "$entry"
        if [[ "$name" == "$target" ]]; then
            echo "$deps"
            return
        fi
    done
}

run_step() {
    local name="$1"
    local func="step_$name"

    if ! declare -f "$func" &>/dev/null; then
        error "Unknown step: $name"
        list_steps
        return 1
    fi

    if array_contains "$name" "${PASSED[@]}" || array_contains "$name" "${FAILED[@]}" || array_contains "$name" "${SKIPPED[@]}"; then
        return 0
    fi

    if [[ "${STEP_ACTIVE[$name]:-0}" -eq 1 ]]; then
        error "Dependency cycle detected at step '$name'"
        FAILED+=("$name")
        return 1
    fi
    STEP_ACTIVE["$name"]=1

    # Resolve dependencies eagerly so direct runs still enforce deps.
    local deps
    deps=$(get_step_deps "$name")
    if [[ -n "$deps" ]]; then
        IFS=',' read -ra dep_list <<< "$deps"
        for dep in "${dep_list[@]}"; do
            dep="${dep#"${dep%%[![:space:]]*}"}"
            dep="${dep%"${dep##*[![:space:]]}"}"
            [[ -n "$dep" ]] || continue

            if ! run_step "$dep"; then
                warn "Skipping '$name' — dependency '$dep' failed"
                SKIPPED+=("$name")
                STEP_ACTIVE["$name"]=0
                return 0
            fi

            if array_contains "$dep" "${FAILED[@]}" || array_contains "$dep" "${SKIPPED[@]}"; then
                warn "Skipping '$name' — dependency '$dep' did not complete successfully"
                SKIPPED+=("$name")
                STEP_ACTIVE["$name"]=0
                return 0
            fi
        done
    fi

    # Run in subshell to isolate set -e failures
    if ( "$func" ); then
        PASSED+=("$name")
        STEP_ACTIVE["$name"]=0
    else
        FAILED+=("$name")
        STEP_ACTIVE["$name"]=0
        warn "Step '$name' failed (continuing)"
        return 1
    fi
}

print_summary() {
    [[ ${#PASSED[@]} -eq 0 && ${#FAILED[@]} -eq 0 && ${#SKIPPED[@]} -eq 0 ]] && return

    header "Summary"
    for name in "${PASSED[@]}"; do
        printf '  %b✓%b %s\n' "$G" "$N" "$name"
    done
    for name in "${FAILED[@]}"; do
        printf '  %b✗%b %s\n' "$R" "$N" "$name"
    done
    for name in "${SKIPPED[@]}"; do
        printf '  %b-%b %s (skipped)\n' "$Y" "$N" "$name"
    done
    echo

    if [[ ${#FAILED[@]} -gt 0 ]]; then
        warn "${#FAILED[@]} step(s) failed"
    else
        finish "All steps completed successfully"
    fi
}

run_all() {
    for entry in "${STEPS[@]}"; do
        IFS='|' read -r name _ _ _ <<< "$entry"
        run_step "$name"
    done
    print_summary
}

interactive_menu() {
    echo
    printf '%bDotfiles Installer%b\n' "$BD" "$N"
    echo
    list_steps
    echo
    printf '  %ba%b. Run all steps\n' "$BD" "$N"
    printf '  %bq%b. Quit\n' "$BD" "$N"
    echo
    read -rp "Select steps (space-separated numbers, 'a' for all, 'q' to quit): " selection

    [[ "$selection" == "q" ]] && exit 0

    if [[ "$selection" == "a" ]]; then
        run_all
        return
    fi

    for sel in "${selection[@]}"; do
        if [[ "$sel" =~ ^[0-9]+$ ]] && (( sel >= 1 && sel <= ${#STEPS[@]} )); then
            local entry="${STEPS[$((sel-1))]}"
            IFS='|' read -r name _ _ _ <<< "$entry"
            run_step "$name"
        else
            error "Invalid selection: $sel"
        fi
    done
    print_summary
}

usage() {
    echo "Usage: ./install.sh [OPTIONS] [STEP...]"
    echo
    echo "Unified post-installation script for dotfiles."
    echo
    echo "Modes:"
    echo "  (no args)     Interactive menu"
    echo "  all           Run all steps in order"
    echo "  STEP [STEP..] Run specific steps by name"
    echo
    echo "Options:"
    echo "  --list, -l    List available steps"
    echo "  --help, -h    Show this help"
    echo
    list_steps
}

main() {
    ensure_dotfiles_checkout "$@"
    script_header

    if [[ $# -eq 0 ]]; then
        if ! require_desktop_environment; then
            error "Aborting. Use DOTFILES_INSTALL_ALLOW_HEADLESS=1 to bypass this check."
            exit 1
        fi
        interactive_menu
        exit 0
    fi

    case "$1" in
        --list|-l)  list_steps ;;
        --help|-h)  usage ;;
        all)
            if ! require_desktop_environment; then
                error "Aborting. Use DOTFILES_INSTALL_ALLOW_HEADLESS=1 to bypass this check."
                exit 1
            fi
            run_all
            ;;
        *)
            if ! require_desktop_environment; then
                error "Aborting. Use DOTFILES_INSTALL_ALLOW_HEADLESS=1 to bypass this check."
                exit 1
            fi
            for step in "$@"; do
                run_step "$step"
            done
            print_summary
            ;;
    esac
}

main "$@"
