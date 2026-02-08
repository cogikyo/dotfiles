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

DOTFILES="${DOTFILES:-$HOME/dotfiles}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
BOLD='\033[1m'
NC='\033[0m'

info()    { printf '%b==>%b %s\n' "$BLUE" "$NC" "$*"; }
success() { printf '%b==>%b %s\n' "$GREEN" "$NC" "$*"; }
warn()    { printf '%b==>%b %s\n' "$YELLOW" "$NC" "$*"; }
error()   { printf '%b==>%b %s\n' "$RED" "$NC" "$*" >&2; }
header()  { printf '\n%b━━━ %s ━━━%b\n\n' "${BOLD}${BLUE}" "$*" "$NC"; }

# Step definitions: name|description|requires_sudo|depends
STEPS=(
    "packages|Install packages from saved lists|yes|"
    "link|Symlink configs and scripts|no|"
    "secrets|Decrypt age-encrypted secrets to target paths|no|"
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
}

# ── Step: link ───────────────────────────────────────────────────────────────

step_link() {
    header "Linking configs and scripts"

    # Config directories → ~/.config/
    info "Linking config directories to ~/.config/..."
    mkdir -p "$HOME/.config"

    for item in "$DOTFILES"/config/*; do
        local name
        name=$(basename "$item")

        case "$name" in
            claude|Cursor|firefox) continue ;;
        esac

        ln -sfnv "$item" "$HOME/.config/"
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
    mkdir -p "$HOME/.local/bin"

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
        info "Add secrets with: secrets add <name> <file> <target> [mode]"
        return 0
    fi

    "$DOTFILES/bin/secrets" decrypt
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

    local SWAP_SIZE="16G"
    local SWAP_DIR="/swap"
    local SWAP_FILE="$SWAP_DIR/swapfile"

    # Check if swap subvolume exists
    if sudo btrfs subvolume show "$SWAP_DIR" &>/dev/null; then
        success "Swap subvolume already exists at $SWAP_DIR"
    else
        info "Creating btrfs swap subvolume at $SWAP_DIR..."
        sudo btrfs subvolume create "$SWAP_DIR"
    fi

    # Check if swapfile is active
    if swapon --show=NAME --noheadings | grep -q "$SWAP_FILE"; then
        success "Swapfile already active: $SWAP_FILE"
    else
        if [[ -f "$SWAP_FILE" ]]; then
            warn "Swapfile exists but is not active, re-enabling..."
        else
            info "Creating ${SWAP_SIZE} swapfile..."
            sudo truncate -s 0 "$SWAP_FILE"
            sudo chattr +C "$SWAP_FILE"
            sudo fallocate -l "$SWAP_SIZE" "$SWAP_FILE"
            sudo chmod 600 "$SWAP_FILE"
            sudo mkswap "$SWAP_FILE"
        fi
        info "Activating swap..."
        sudo swapon "$SWAP_FILE"
    fi

    # Add to fstab if not present
    if grep -q "$SWAP_FILE" /etc/fstab; then
        success "Swapfile already in /etc/fstab"
    else
        info "Adding swapfile to /etc/fstab..."
        echo "$SWAP_FILE none swap defaults,pri=10 0 0" | sudo tee -a /etc/fstab
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

    local daemons=()
    for name in "${!GO_BINARIES[@]}"; do
        IFS='|' read -r module_dir _ _ is_daemon <<< "${GO_BINARIES[$name]}"
        [[ "$is_daemon" == "yes" ]] || continue
        daemons+=("$name")

        local src="$DOTFILES/$module_dir/$name/$name.service"
        # newtab has its own module dir
        [[ -f "$src" ]] || src="$DOTFILES/$module_dir/$name.service"

        if [[ ! -f "$src" ]]; then
            warn "Service file not found for $name (skipping)"
            continue
        fi

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
        git clone https://github.com/nosvagor/vagari.firefox.git "$vagari_dir"
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

    # Verify resolved.conf is installed (owned by step_system)
    if ! diff -q "$DOTFILES/etc/systemd/resolved.conf" /etc/systemd/resolved.conf &>/dev/null 2>&1; then
        warn "resolved.conf is missing or outdated — run the 'system' step first"
    fi

    # Enable systemd-resolved
    if systemctl is-active systemd-resolved &>/dev/null; then
        success "systemd-resolved already active"
    else
        info "Enabling systemd-resolved..."
        sudo systemctl enable --now systemd-resolved
    fi

    # Configure NetworkManager
    local NM_CONF="/etc/NetworkManager/conf.d/dns.conf"
    if [[ -f "$NM_CONF" ]] && grep -q "dns=systemd-resolved" "$NM_CONF"; then
        success "NetworkManager already configured for systemd-resolved"
    else
        info "Configuring NetworkManager to use systemd-resolved..."
        sudo mkdir -p /etc/NetworkManager/conf.d
        printf "[main]\ndns=systemd-resolved\n" | sudo tee "$NM_CONF" > /dev/null
    fi

    # Link resolv.conf
    local STUB="/run/systemd/resolve/stub-resolv.conf"
    if [[ -L /etc/resolv.conf ]] && [[ "$(readlink /etc/resolv.conf)" == "$STUB" ]]; then
        success "resolv.conf already linked to stub"
    else
        info "Linking resolv.conf to systemd-resolved stub..."
        sudo ln -sf "$STUB" /etc/resolv.conf
    fi

    info "Restarting NetworkManager..."
    sudo systemctl restart NetworkManager
    success "DNS configured with Cloudflare DNS-over-TLS"
}

# ── Menu and dispatch ────────────────────────────────────────────────────────

list_steps() {
    echo "Available steps:"
    echo
    local i=1
    for entry in "${STEPS[@]}"; do
        IFS='|' read -r name desc needs_root deps <<< "$entry"
        local badges=""
        [[ "$needs_root" == "yes" ]] && badges+=" ${YELLOW}(sudo)${NC}"
        [[ -n "$deps" ]] && badges+=" ${BLUE}(after: $deps)${NC}"
        printf '  %b%d%b. %-12s %s%b\n' "$BOLD" "$i" "$NC" "$name" "$desc" "$badges"
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

    # Check dependencies (only when tracking results, i.e. multi-step runs)
    local deps
    deps=$(get_step_deps "$name")
    if [[ -n "$deps" ]]; then
        IFS=',' read -ra dep_list <<< "$deps"
        for dep in "${dep_list[@]}"; do
            for f in "${FAILED[@]}"; do
                if [[ "$f" == "$dep" ]]; then
                    warn "Skipping '$name' — dependency '$dep' failed"
                    SKIPPED+=("$name")
                    return 0
                fi
            done
        done
    fi

    # Run in subshell to isolate set -e failures
    if ( "$func" ); then
        PASSED+=("$name")
    else
        FAILED+=("$name")
        warn "Step '$name' failed (continuing)"
    fi
}

print_summary() {
    [[ ${#PASSED[@]} -eq 0 && ${#FAILED[@]} -eq 0 && ${#SKIPPED[@]} -eq 0 ]] && return

    header "Summary"
    for name in "${PASSED[@]}"; do
        printf '  %b✓%b %s\n' "$GREEN" "$NC" "$name"
    done
    for name in "${FAILED[@]}"; do
        printf '  %b✗%b %s\n' "$RED" "$NC" "$name"
    done
    for name in "${SKIPPED[@]}"; do
        printf '  %b-%b %s (skipped)\n' "$YELLOW" "$NC" "$name"
    done
    echo

    if [[ ${#FAILED[@]} -gt 0 ]]; then
        warn "${#FAILED[@]} step(s) failed"
    else
        success "All steps completed successfully"
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
    printf '%bDotfiles Installer%b\n' "$BOLD" "$NC"
    echo
    list_steps
    echo
    printf '  %ba%b. Run all steps\n' "$BOLD" "$NC"
    printf '  %bq%b. Quit\n' "$BOLD" "$NC"
    echo
    read -rp "Select steps (space-separated numbers, 'a' for all, 'q' to quit): " selection

    [[ "$selection" == "q" ]] && exit 0

    if [[ "$selection" == "a" ]]; then
        run_all
        return
    fi

    for sel in $selection; do
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
    if [[ $# -eq 0 ]]; then
        interactive_menu
        exit 0
    fi

    case "$1" in
        all)        run_all ;;
        --list|-l)  list_steps ;;
        --help|-h)  usage ;;
        *)
            for step in "$@"; do
                run_step "$step"
            done
            print_summary
            ;;
    esac
}

main "$@"
