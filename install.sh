#!/usr/bin/env bash
# vim: set foldmethod=marker foldlevel=0:

set -euo pipefail

VERSION="0.2.2"

usage() {
    cat <<'EOF'
Usage: ./install.sh [OPTIONS] [STEP...]

Post-installation script for dotfiles.

Commands:
  all              Run all steps in order
  {STEP}           Run specific steps by name

Options:
  -l, --list       List available steps
  -c, --check      Run healthchecks for all (or specified) steps
  -h, --help       Show this help

Environment:
  ARCH=1           Archinstall bootstrap (run as root from live ISO)
  AUTO=1           Unattended mode (default: 0)
  PAUSE=N          Seconds before each step (default: 3)
  SKIP=1           Skip chroot post-install in ARCH mode
EOF
    echo
    list_steps
}

# =================================================================================================
#  Runtime Config  {{{

# Internal constants
readonly STEP_SKIPPED_RC=42

# Core
DOTFILES="$HOME/dotfiles"

# Behavior
ARCH="${ARCH:-0}"
AUTO="${AUTO:-0}"
PAUSE="${PAUSE:-3}"
SKIP="${SKIP:-0}"

# }}}
# =================================================================================================

# =================================================================================================
#  Logging  {{{
# =================================================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[1;34m'
MAGENTA='\033[0;35m'
FAINT='\033[90m'
BOLD='\033[1m'
RESET='\033[0m'

if [[ ! -t 1 || -n "${NO_COLOR:-}" ]]; then
    RED='' GREEN='' YELLOW='' BLUE='' MAGENTA='' FAINT='' BOLD='' RESET=''
fi

# -- internal helpers ----------------------------------------------------------

_log() {
    local color="$1" level="$2"; shift 2
    printf '%b[%-5s]%b %s\n' "$color" "$level" "$RESET" "$*"
}

_banner_kv() {
    printf '  %b%-18s%b %s\n' "$FAINT" "$1" "$RESET" "$2"
}

_art() {
    local color="$1"; shift
    local line; for line in "$@"; do printf '%b%s%b\n' "$color" "$line" "$RESET"; done
}

# -- status --------------------------------------------------------------------

info()   { _log "$BLUE"   "INFO"  "$*"; }
ok()     { _log "$GREEN"  "OK"    "$*"; }
warn()   { _log "$YELLOW" "WARN"  "$*"; }
err()    { _log "$RED"    "ERROR" "$*" >&2; }
die()    { err "$*"; exit 1; }

# -- structure -----------------------------------------------------------------

header() { printf '\n%b--- %s ---%b\n\n' "$MAGENTA" "$*" "$RESET"; }
step()   { printf '%b  ==>%b %s\n' "$BLUE" "$RESET" "$*"; }
dim()    { printf '%b%s%b\n' "$FAINT" "$*" "$RESET"; }
bold()   { printf '%b%s%b\n' "$BOLD" "$*" "$RESET"; }

# -- interaction ---------------------------------------------------------------

confirm() {
    local prompt="$1" default="${2:-y}" yn
    if is_auto; then
        dim "$prompt -> $default (auto)"
        [[ "$default" == "y" ]]
        return
    fi
    if [[ "$default" == "y" ]]; then
        printf '%b  ?  %b %s %b[Y/n]%b ' "$MAGENTA" "$RESET" "$prompt" "$FAINT" "$RESET"
    else
        printf '%b  ?  %b %s %b[y/N]%b ' "$MAGENTA" "$RESET" "$prompt" "$FAINT" "$RESET"
    fi
    read -r yn
    yn="${yn:-$default}"
    [[ "$yn" =~ ^[Yy] ]]
}

# -- banners ------------------------------------------------------------------

banner() {
    local mode="$1" selection="$2"
    echo
    _art "$YELLOW" \
        '██╗███╗   ██╗███████╗████████╗ █████╗ ██╗     ██╗     ' \
        '██║████╗  ██║██╔════╝╚══██╔══╝██╔══██╗██║     ██║     ' \
        '██║██╔██╗ ██║███████╗   ██║   ███████║██║     ██║     ' \
        '██║██║╚██╗██║╚════██║   ██║   ██╔══██║██║     ██║     ' \
        '██║██║ ╚████║███████║   ██║   ██║  ██║███████╗███████╗' \
        '╚═╝╚═╝  ╚═══╝╚══════╝   ╚═╝   ╚═╝  ╚═╝╚══════╝╚══════╝'
    dim "  dotfiles v$VERSION"
    echo
    _banner_kv "mode" "$mode"
    _banner_kv "selection" "$selection"
    _banner_kv "dotfiles" "$DOTFILES"
    _banner_kv "auto" "$AUTO"
    _banner_kv "pause" "$PAUSE"
    echo
}

banner_arch() {
    echo
    _art "$BLUE" \
        ' █████╗ ██████╗  ██████╗██╗  ██╗'\
        '██╔══██╗██╔══██╗██╔════╝██║  ██║'\
        '███████║██████╔╝██║     ███████║'\
        '██╔══██║██╔══██╗██║     ██╔══██║'\
        '██║  ██║██║  ██║╚██████╗██║  ██║'\
        '╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝'
    dim "  bootstrap v$VERSION"
    echo
    _banner_kv "skip" "$SKIP"
    _banner_kv "auto" "$AUTO"
    _banner_kv "pause" "$PAUSE"
    echo
}

banner_end() {
    local mode="$1" status="$2" details="${3:-}"
    echo
    if [[ "$status" == "FAILED" ]]; then
        _art "$RED" \
            '███████╗ █████╗ ██╗██╗     ███████╗██████╗ ' \
            '██╔════╝██╔══██╗██║██║     ██╔════╝██╔══██╗' \
            '█████╗  ███████║██║██║     █████╗  ██║  ██║' \
            '██╔══╝  ██╔══██║██║██║     ██╔══╝  ██║  ██║' \
            '██║     ██║  ██║██║███████╗███████╗██████╔╝' \
            '╚═╝     ╚═╝  ╚═╝╚═╝╚══════╝╚══════╝╚═════╝'
    else
        _art "$GREEN" \
            '███████╗██╗   ██╗ ██████╗███████╗███████╗███████╗' \
            '██╔════╝██║   ██║██╔════╝██╔════╝██╔════╝██╔════╝' \
            '███████╗██║   ██║██║     █████╗  ███████╗███████╗' \
            '╚════██║██║   ██║██║     ██╔══╝  ╚════██║╚════██║' \
            '███████║╚██████╔╝╚██████╗███████╗███████║███████║' \
            '╚══════╝ ╚═════╝  ╚═════╝╚══════╝╚══════╝╚══════╝'
    fi
    dim "  $mode"
    [[ -n "$details" ]] && dim "  $details"
    echo
}

# }}}
# =================================================================================================

# =================================================================================================
#  Step Registry  {{{

# name|description|requires_sudo|depends
STEP_DEFS=(
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

# }}}
# =================================================================================================

# =================================================================================================
#  Core Helpers  {{{

has() { command -v "$1" &>/dev/null; }

is_auto() {
    [[ "$AUTO" == "1" || ! -t 0 ]]
}

array_contains() {
    local needle="$1"
    shift
    local item
    for item in "$@"; do
        [[ "$item" == "$needle" ]] && return 0
    done
    return 1
}

canonpath() { realpath -m -- "$1"; }

ensure_dotfiles_checkout() {
    local script_path script_real target_real
    script_path="${BASH_SOURCE[0]:-$0}"
    script_real=$(canonpath "$script_path")
    target_real=$(canonpath "$DOTFILES/install.sh")

    if [[ "$script_real" == "$target_real" ]]; then
        return 0
    fi

    if [[ ! -d "$DOTFILES/.git" || ! -f "$DOTFILES/install.sh" ]]; then
        has git || { err "git not found. Install git first."; exit 1; }

        if [[ -e "$DOTFILES" && ! -d "$DOTFILES/.git" ]]; then
            err "DOTFILES path exists but is not a git checkout: $DOTFILES"
            exit 1
        fi

        info "Bootstrapping dotfiles into $DOTFILES..."
        git clone --depth 1 --branch master https://github.com/cogikyo/dotfiles.git "$DOTFILES"
    fi

    info "Re-running installer from $DOTFILES/install.sh..."
    exec "$DOTFILES/install.sh" "$@"
}

# }}}
# =================================================================================================

# =================================================================================================
#  Arch Bootstrap Mode  {{{

prepare_arch_partial_config() {
    local config_path="$1"
    local firmware_mode="$2"

    python3 - "$config_path" "$firmware_mode" <<'PYEOF'
import json
import sys

config_path, firmware_mode = sys.argv[1:3]

with open(config_path, encoding="utf-8") as f:
    config = json.load(f)

# Let archinstall handle these interactively.
config.pop("disk_config", None)
config.pop("auth_config", None)

# Avoid pre-seeding keymap.
locale_cfg = config.get("locale_config")
if isinstance(locale_cfg, dict):
    locale_cfg.pop("kb_layout", None)

# Systemd-boot is UEFI-only.
bootloader_cfg = config.setdefault("bootloader_config", {})
bootloader_cfg["bootloader"] = "Grub" if firmware_mode == "bios" else "Systemd-boot"

# Minimal — full desktop setup is handled post-install.
config["profile_config"] = None

with open(config_path, "w", encoding="utf-8") as f:
    json.dump(config, f, indent=4)
PYEOF
}

run_arch_post_install() {
    local target_root="/mnt"
    local target_user

    command -v arch-chroot >/dev/null || die "arch-chroot is required for post-install"
    [[ -d "$target_root/etc" ]] || die "Target root not found: $target_root"
    [[ -f "$target_root/etc/passwd" ]] || die "Target root is missing /etc/passwd: $target_root"

    target_user=$(
        awk -F: '
            $3 >= 1000 && $1 != "nobody" && $7 !~ /(nologin|false)$/ {
                print $1
                exit
            }
        ' "$target_root/etc/passwd"
    )
    [[ -n "$target_user" ]] || die "Could not detect target user in $target_root/etc/passwd"

    local user_home
    user_home=$(awk -F: -v user="$target_user" '$1 == user { print $6 }' "$target_root/etc/passwd")
    [[ -n "$user_home" ]] || user_home="/home/$target_user"

    info "Running post-install in chroot for user '$target_user'..."
    arch-chroot "$target_root" /bin/bash -s -- \
        "$target_user" \
        "$user_home" \
        "$AUTO" \
        "$PAUSE" <<'CHROOT_EOF'
set -euo pipefail

target_user="$1"
user_home="$2"
install_auto="$3"
install_pause="$4"
dotfiles_dir="$user_home/dotfiles"
sudoers_file="/etc/sudoers.d/99-dotfiles-install"

if ! id "$target_user" &>/dev/null; then
    echo "Target user does not exist in chroot: $target_user" >&2
    exit 1
fi

pacman -Sy --noconfirm archlinux-keyring
pacman -S --needed --noconfirm sudo git base-devel age

install -d -m 0755 /etc/sudoers.d
printf '%s ALL=(ALL) NOPASSWD:ALL\n' "$target_user" > "$sudoers_file"
chmod 0440 "$sudoers_file"
visudo -cf "$sudoers_file" >/dev/null
trap 'rm -f "$sudoers_file"' EXIT

if [[ ! -d "$user_home" ]]; then
    echo "Home directory missing for user '$target_user': $user_home" >&2
    exit 1
fi

if [[ -d "$dotfiles_dir/.git" ]]; then
    sudo -H -u "$target_user" git -C "$dotfiles_dir" fetch --depth 1 origin master
    sudo -H -u "$target_user" git -C "$dotfiles_dir" checkout -f FETCH_HEAD
elif [[ -e "$dotfiles_dir" ]]; then
    echo "dotfiles path exists but is not a git checkout: $dotfiles_dir" >&2
    exit 1
else
    sudo -H -u "$target_user" git clone --depth 1 --branch master \
        https://github.com/cogikyo/dotfiles.git "$dotfiles_dir"
fi
chown -R "$target_user:$target_user" "$dotfiles_dir"

sudo -H -u "$target_user" env \
    ARCH=0 \
    AUTO="$install_auto" \
    DOTFILES_INSTALL_CHROOT=1 \
    PAUSE="$install_pause" \
    "$dotfiles_dir/install.sh" all
CHROOT_EOF
}

run_arch_mode() {
    local script_path script_dir source_config tmp_source_config config firmware_mode raw_base
    script_path="${BASH_SOURCE[0]:-$0}"
    script_dir="$(cd "$(dirname "$script_path")" && pwd)"
    source_config="$script_dir/etc/arch.json"
    tmp_source_config="/tmp/arch_source_config.json"
    config="/tmp/arch_config.json"

    banner_arch

    [[ $EUID -eq 0 ]] || die "Run ARCH=1 mode as root from the live ISO"
    command -v python3 >/dev/null || die "python3 is required"
    command -v archinstall >/dev/null || die "archinstall is required"
    command -v curl >/dev/null || die "curl is required"

    trap 'rm -f "$config" "$tmp_source_config"' EXIT

    firmware_mode="bios"
    if [[ -d /sys/firmware/efi ]]; then
        firmware_mode="uefi"
    fi
    info "Detected firmware mode: $firmware_mode"

    if [[ -f "$source_config" ]]; then
        info "Loading local config: $source_config"
        cp -f "$source_config" "$config"
    else
        raw_base="https://raw.githubusercontent.com/cogikyo/dotfiles/master"
        info "Local config not found at $source_config"
        info "Downloading config: $raw_base/etc/arch.json"
        curl -fsSL "$raw_base/etc/arch.json" -o "$tmp_source_config" || die "Failed to download etc/arch.json"
        cp -f "$tmp_source_config" "$config"
    fi

    info "Preparing partial config..."
    prepare_arch_partial_config "$config" "$firmware_mode"
    ok "Partial config ready: $config"

    info "Starting archinstall..."
    warn "Set disk partitioning and authentication in the archinstall UI"
    if ! archinstall --config "$config"; then
        err "archinstall failed"
        warn "Recent archinstall log output:"
        if [[ -f /var/log/archinstall/install.log ]]; then
            tail -n 120 /var/log/archinstall/install.log || true
        else
            warn "No /var/log/archinstall/install.log found"
        fi
        banner_end "ARCH BOOTSTRAP" "FAILED" "archinstall returned non-zero"
        exit 1
    fi

    if [[ "$SKIP" == "1" ]]; then
        warn "Skipping post-install automation (SKIP=1)"
    else
        if ! run_arch_post_install; then
            err "Post-install automation failed"
            warn "You can reboot and run ~/dotfiles/install.sh all manually"
            banner_end "ARCH BOOTSTRAP" "FAILED" "chroot post-install failed"
            exit 1
        fi
    fi

    ok "Installation complete"
    banner_end "ARCH BOOTSTRAP" "SUCCESS" "reboot then run deferred desktop steps"
    info "Next steps:"
    if [[ "$SKIP" == "1" ]]; then
        step "1. Reboot into the new system"
        step "2. Log in as your configured user"
        step "3. Clone dotfiles and run: ./install.sh all"
    else
        step "1. Reboot into the new system"
        step "2. Log in as your configured user"
        step "3. Run deferred steps if needed (e.g. ./install.sh firefox after first Firefox launch)"
    fi
    echo
}

# }}}
# =================================================================================================

# =================================================================================================
#  Step Prerequisites  {{{

ensure_rustup_stable() {
    if ! has rustup; then
        warn "rustup not found; skipping Rust toolchain initialization"
        return 0
    fi

    local active=""
    active=$(rustup show active-toolchain 2>/dev/null | awk 'NR==1 {print $1}')

    if [[ "$active" == stable* ]]; then
        ok "Rust toolchain already set to stable ($active)"
        return 0
    fi

    info "Initializing rustup stable toolchain..."
    rustup toolchain install stable
    rustup default stable
    ok "Rust toolchain set to stable"
}

has_user_bus() {
    [[ -n "${DBUS_SESSION_BUS_ADDRESS:-}" ]] && return 0
    [[ -n "${XDG_RUNTIME_DIR:-}" && -S "${XDG_RUNTIME_DIR}/bus" ]] && return 0
    has busctl && busctl --user status &>/dev/null
}

in_chroot() {
    [[ "${DOTFILES_INSTALL_CHROOT:-0}" == "1" ]] && return 0
    has systemd-detect-virt && systemd-detect-virt --quiet --chroot &>/dev/null && return 0
    return 1
}

is_paru_usable() {
    has paru || return 1
    paru --version &>/dev/null
}

needs_sudo() {
    if [[ $EUID -eq 0 ]]; then
        return 0
    fi

    if ! has sudo; then
        err "sudo not found. Install sudo first."
        return 1
    fi

    if ! sudo -n true 2>/dev/null; then
        info "Some steps require sudo access"
        sudo -v
    fi
}

bootstrap_paru() {
    if is_paru_usable; then
        ok "paru already installed"
        return 0
    fi

    info "Bootstrapping paru from AUR prebuilt package (paru-bin)"
    has git || { err "git not found. Install git first."; return 1; }
    has makepkg || { err "makepkg not found. Install base-devel first."; return 1; }

    local cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}"
    local build_root="$cache_dir/paru-bootstrap"

    rm -rf "$build_root"
    mkdir -p "$cache_dir"
    if ! git clone --depth 1 "https://aur.archlinux.org/paru-bin.git" "$build_root"; then
        err "Failed to clone paru-bin AUR repo"
        return 1
    fi
    if ! (cd "$build_root" && makepkg -si --noconfirm --needed); then
        err "makepkg failed while bootstrapping paru-bin"
        rm -rf "$build_root"
        return 1
    fi
    rm -rf "$build_root"

    if ! is_paru_usable; then
        err "paru bootstrap failed"
        return 1
    fi

    ok "paru bootstrapped from AUR"
}

# }}}
# =================================================================================================

# =================================================================================================
#  STEP {PACKAGES}: Install package sets and Rust stable toolchain  {{{

step_packages() {
    header "Installing packages"

    needs_sudo
    bootstrap_paru

    "$DOTFILES/bin/update" --install
    ensure_rustup_stable
}

healthcheck_packages() {
    if ! is_paru_usable; then
        err "Healthcheck failed: paru is not runnable"
        return 1
    fi
    return 0
}

# }}}
# =================================================================================================

# =================================================================================================
#  STEP {LINK}: Symlink dotfiles config, scripts, and shell profile  {{{

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

    # Config directories -> ~/.config/<name> (content-level sync via symlinks)
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

    # Scripts -> ~/.local/bin/
    info "Linking scripts to ~/.local/bin/..."

    for script in "$DOTFILES"/bin/*; do
        [[ -x "$script" ]] || continue
        ln -sfv "$script" "$HOME/.local/bin/"
    done

    ok "Linking complete"
}

healthcheck_link() {
    local zsh_src zsh_dst
    zsh_src=$(canonpath "$DOTFILES/config/zsh/zshrc")
    zsh_dst=$(canonpath "$HOME/.zshrc")

    [[ -L "$HOME/.zshrc" ]] || { err "Healthcheck failed: ~/.zshrc is not a symlink"; return 1; }
    [[ "$zsh_src" == "$zsh_dst" ]] || { err "Healthcheck failed: ~/.zshrc does not point to $DOTFILES/config/zsh/zshrc"; return 1; }
    [[ -d "$HOME/.config" ]] || { err "Healthcheck failed: ~/.config does not exist"; return 1; }
    [[ -d "$HOME/.local/bin" ]] || { err "Healthcheck failed: ~/.local/bin does not exist"; return 1; }
    return 0
}

# }}}
# =================================================================================================

# =================================================================================================
#  STEP {SECRETS}: Decrypt age-managed secrets into local target paths  {{{

step_secrets() {
    header "Decrypting secrets"

    if ! has age; then
        err "age not found - install with: pacman -S age"
        return 1
    fi

    local manifest="$DOTFILES/etc/secrets/manifest"
    if [[ ! -f "$manifest" ]] || ! grep -qv '^#\|^$' "$manifest"; then
        warn "No secrets configured yet"
        info "Add entries in $DOTFILES/etc/secrets/manifest, then run: secrets sync"
        return 0
    fi

    if [[ ! -t 0 && ! -r /dev/tty ]]; then
        warn "No interactive TTY available for age passphrase prompt; skipping secrets"
        return "$STEP_SKIPPED_RC"
    fi

    "$DOTFILES/bin/secrets" decrypt
}

healthcheck_secrets() {
    local manifest="$DOTFILES/etc/secrets/manifest"
    local has_entries=0
    local line name target mode expanded target_mode

    [[ -f "$manifest" ]] || return "$STEP_SKIPPED_RC"

    while IFS= read -r line; do
        line="${line%%#*}"
        line="${line#"${line%%[![:space:]]*}"}"
        line="${line%"${line##*[![:space:]]}"}"
        [[ -z "$line" ]] && continue
        has_entries=1

        IFS=':' read -r name target mode <<< "$line"
        [[ -n "$target" ]] || continue

        expanded="${target/#\~/$HOME}"
        if [[ ! -e "$expanded" ]]; then
            err "Healthcheck failed: missing secret target '$expanded' for '$name'"
            return 1
        fi

        if [[ "$mode" =~ ^[0-7]{3,4}$ ]]; then
            target_mode=$(stat -c '%a' "$expanded" 2>/dev/null || true)
            if [[ -n "$target_mode" && "$target_mode" != "$mode" ]]; then
                warn "Secret mode mismatch for $expanded (expected $mode, got $target_mode)"
            fi
        fi
    done < "$manifest"

    [[ $has_entries -eq 1 ]] || return "$STEP_SKIPPED_RC"
    return 0
}

# }}}
# =================================================================================================

# =================================================================================================
#  STEP {REPOS}: Create base directories and clone repos from manifest  {{{

step_repos() {
    header "Cloning repositories and creating directories"

    local REPOS_FILE="$DOTFILES/etc/repos.toml"

    if ! has git; then
        err "git not found. Install it from packages first."
        return 1
    fi

    if [[ ! -f "$REPOS_FILE" ]]; then
        err "Repos manifest not found: $REPOS_FILE"
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
            ok "Created $dir"
        fi
    done

    # Ensure github.com host key exists to avoid interactive SSH prompts.
    if has ssh-keyscan; then
        mkdir -p "$HOME/.ssh"
        chmod 700 "$HOME/.ssh"
        if ! ssh-keygen -F github.com -f "$HOME/.ssh/known_hosts" &>/dev/null; then
            info "Adding github.com SSH host key..."
            ssh-keyscan -H github.com >> "$HOME/.ssh/known_hosts" 2>/dev/null || true
            chmod 600 "$HOME/.ssh/known_hosts"
        fi
    fi

    # Parse repos.toml and clone
    local repo="" path="" cloned=0 skipped=0 failed=0

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
                    info "Cloning $repo -> $path"
                    if git clone "git@github.com:$repo.git" "$expanded"; then
                        ((cloned++))
                    elif git clone "https://github.com/$repo.git" "$expanded"; then
                        warn "SSH clone failed, used HTTPS fallback for $repo"
                        ((cloned++))
                    else
                        err "Failed to clone $repo"
                        ((failed++))
                    fi
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
            info "Cloning $repo -> $path"
            if git clone "git@github.com:$repo.git" "$expanded"; then
                ((cloned++))
            elif git clone "https://github.com/$repo.git" "$expanded"; then
                warn "SSH clone failed, used HTTPS fallback for $repo"
                ((cloned++))
            else
                err "Failed to clone $repo"
                ((failed++))
            fi
        fi
    fi

    echo
    ok "Cloned $cloned repos ($skipped already exist)"
    if [[ $failed -gt 0 ]]; then
        warn "$failed repo(s) failed to clone"
        return 1
    fi
}

healthcheck_repos() {
    local repos_file="$DOTFILES/etc/repos.toml"
    local line key val path expanded
    local expected=0
    local missing=0

    [[ -f "$repos_file" ]] || { err "Healthcheck failed: missing repos manifest"; return 1; }

    while IFS= read -r line; do
        line="${line%%#*}"
        line="${line#"${line%%[![:space:]]*}"}"
        line="${line%"${line##*[![:space:]]}"}"
        [[ -z "$line" ]] && continue

        key="${line%%=*}"
        val="${line#*=}"
        key="${key#"${key%%[![:space:]]*}"}"
        key="${key%"${key##*[![:space:]]}"}"
        val="${val#"${val%%[![:space:]]*}"}"
        val="${val%"${val##*[![:space:]]}"}"
        val="${val#\"}" val="${val%\"}"

        if [[ "$key" == "path" && -n "$val" ]]; then
            ((++expected))
            expanded="${val/#\~/$HOME}"
            if [[ ! -d "$expanded" ]]; then
                err "Healthcheck failed: repo path missing '$expanded'"
                ((++missing))
            fi
        fi
    done < "$repos_file"

    if (( expected == 0 )); then
        warn "No repo paths found in manifest during healthcheck"
        return 0
    fi

    (( missing == 0 )) || return 1
    return 0
}

# }}}
# =================================================================================================

# =================================================================================================
#  STEP {SYSTEM}: Install system-level config files and enable core services  {{{

step_system() {
    header "Installing system configs"
    needs_sudo

    local chroot_mode=0
    if in_chroot; then
        chroot_mode=1
        warn "Chroot environment detected; runtime service actions will be limited"
    fi

    # Source -> destination mappings
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

        info "Installing $src -> $dst_path"
        sudo cp "$src_path" "$dst_path"
        ((installed++))
    done

    ok "Installed $installed system configs ($skipped already up to date)"

    # Enable services
    echo
    info "Enabling services..."
    local SERVICES=(bluetooth sddm earlyoom)
    for svc in "${SERVICES[@]}"; do
        if systemctl is-enabled "$svc" &>/dev/null; then
            ok "$svc already enabled"
        else
            info "Enabling $svc..."
            if sudo systemctl enable "$svc" &>/dev/null; then
                ok "$svc enabled"
            else
                warn "Could not enable $svc in current environment"
            fi
        fi
    done

    # Reload udev rules
    if [[ $chroot_mode -eq 1 ]]; then
        warn "Skipping udev reload in chroot"
    else
        info "Reloading udev rules..."
        sudo udevadm control --reload-rules
        sudo udevadm trigger
    fi
}

healthcheck_system() {
    local checks=(
        "/etc/bluetooth/main.conf"
        "/etc/udev/rules.d/81-bluetooth-hci.rules"
        "/etc/udev/rules.d/92-viia.rules"
        "/etc/sddm.conf.d/autologin.conf"
        "/etc/sddm.conf.d/hyprland.desktop"
        "/etc/systemd/resolved.conf"
        "/etc/systemd/sleep.conf.d/hibernate.conf"
    )
    local path

    for path in "${checks[@]}"; do
        if ! sudo test -f "$path"; then
            err "Healthcheck failed: missing system file '$path'"
            return 1
        fi
    done

    return 0
}

# }}}
# =================================================================================================

# =================================================================================================
#  STEP {SWAP}: Provision btrfs swap subvolume/file and fstab entry  {{{

step_swap() {
    header "Setting up swap"
    needs_sudo

    local SWAP_SIZE="16G"
    local SWAP_DIR="/swap"
    local SWAP_FILE="$SWAP_DIR/swapfile"

    # Ensure /swap is a dedicated btrfs subvolume
    if sudo btrfs subvolume show "$SWAP_DIR" &>/dev/null; then
        ok "Swap subvolume already exists at $SWAP_DIR"
    else
        if [[ -e "$SWAP_DIR" ]]; then
            err "$SWAP_DIR exists but is not a btrfs subvolume. Refusing to modify it automatically."
            return 1
        fi
        info "Creating btrfs swap subvolume at $SWAP_DIR..."
        sudo btrfs subvolume create "$SWAP_DIR"
    fi

    # Prevent compression for swap extents.
    sudo btrfs property set "$SWAP_DIR" compression none &>/dev/null || true

    # Check if swapfile is active
    if swapon --show=NAME --noheadings | sed 's/^[[:space:]]*//' | grep -Fxq "$SWAP_FILE"; then
        ok "Swapfile already active: $SWAP_FILE"
    else
        if [[ -e "$SWAP_FILE" ]]; then
            warn "Existing inactive swapfile found, recreating: $SWAP_FILE"
            sudo swapoff "$SWAP_FILE" 2>/dev/null || true
            sudo rm -f "$SWAP_FILE"
        fi

        info "Creating ${SWAP_SIZE} swapfile with btrfs mkswapfile..."
        sudo btrfs filesystem mkswapfile --size "$SWAP_SIZE" --uuid clear "$SWAP_FILE"

        info "Activating swap..."
        sudo swapon "$SWAP_FILE"
    fi

    # Add to fstab if not present
    if awk -v swap_path="$SWAP_FILE" '$0 !~ /^[[:space:]]*#/ && $1 == swap_path && $3 == "swap" {found=1} END {exit(found ? 0 : 1)}' /etc/fstab; then
        ok "Swapfile already in /etc/fstab"
    else
        info "Adding swapfile to /etc/fstab..."
        printf '%s none swap defaults,pri=10 0 0\n' "$SWAP_FILE" | sudo tee -a /etc/fstab > /dev/null
        ok "Added to /etc/fstab"
    fi

    ok "Swap setup complete"
}

healthcheck_swap() {
    local root_fs
    root_fs=$(findmnt -no FSTYPE / 2>/dev/null || true)
    [[ "$root_fs" == "btrfs" ]] || return "$STEP_SKIPPED_RC"

    [[ -f /swap/swapfile ]] || { err "Healthcheck failed: /swap/swapfile missing"; return 1; }
    if ! awk '$0 !~ /^[[:space:]]*#/ && $1 == "/swap/swapfile" && $3 == "swap" {found=1} END {exit(found ? 0 : 1)}' /etc/fstab; then
        err "Healthcheck failed: /swap/swapfile is missing from /etc/fstab"
        return 1
    fi
    return 0
}

# }}}
# =================================================================================================

# =================================================================================================
#  STEP {HIBERNATE}: Configure resume args, hooks, and initramfs for hibernate  {{{

step_hibernate() {
    header "Configuring hibernation"
    needs_sudo

    local SWAP_FILE="/swap/swapfile"

    if [[ ! -f "$SWAP_FILE" ]]; then
        err "Swapfile not found at $SWAP_FILE. Run the 'swap' step first."
        return 1
    fi

    # Get resume parameters
    local RESUME_OFFSET=""
    RESUME_OFFSET=$(sudo btrfs inspect-internal map-swapfile -r "$SWAP_FILE" 2>/dev/null || true)
    RESUME_OFFSET=$(printf '%s\n' "$RESUME_OFFSET" | awk '/^[[:space:]]*[0-9]+[[:space:]]*$/ {gsub(/[[:space:]]/, "", $0); print; exit}')
    if [[ -z "$RESUME_OFFSET" ]]; then
        # Older btrfs-progs may not support `-r`; parse the standard output format.
        RESUME_OFFSET=$(sudo btrfs inspect-internal map-swapfile "$SWAP_FILE" 2>/dev/null || true)
        RESUME_OFFSET=$(printf '%s\n' "$RESUME_OFFSET" | awk '/Resume offset/ {print $3; exit}')
    fi
    if [[ ! "$RESUME_OFFSET" =~ ^[0-9]+$ ]]; then
        err "Failed to determine a valid numeric resume_offset for $SWAP_FILE"
        err "Check output of: sudo btrfs inspect-internal map-swapfile -r $SWAP_FILE"
        return 1
    fi
    local RESUME_UUID
    RESUME_UUID=$(findmnt -no UUID -T "$SWAP_FILE" 2>/dev/null || true)
    if [[ -z "$RESUME_UUID" ]]; then
        err "Failed to determine resume UUID for $SWAP_FILE"
        return 1
    fi

    info "Resume UUID:   $RESUME_UUID"
    info "Resume Offset: $RESUME_OFFSET"

    # Update systemd-boot loader entry
    local LOADER_ENTRY
    LOADER_ENTRY=$(find /boot/loader/entries/ -name '*_linux.conf' 2>/dev/null | head -1)

    if [[ -z "$LOADER_ENTRY" ]]; then
        err "No systemd-boot loader entry found in /boot/loader/entries/"
        return 1
    fi

    if ! grep -q '^options' "$LOADER_ENTRY"; then
        err "No 'options' line found in $LOADER_ENTRY"
        return 1
    fi

    info "Updating resume parameters in $LOADER_ENTRY..."
    sudo sed -i -E \
        -e 's/[[:space:]]resume=UUID=[^[:space:]]+//g' \
        -e 's/[[:space:]]resume_offset=[^[:space:]]+//g' \
        -e "s|^options.*|& resume=UUID=$RESUME_UUID resume_offset=$RESUME_OFFSET|" \
        "$LOADER_ENTRY"
    ok "Updated systemd-boot entry"

    # Add resume hook to mkinitcpio
    if grep -q 'HOOKS=.*resume' /etc/mkinitcpio.conf; then
        ok "resume hook already in mkinitcpio.conf"
    else
        info "Adding resume hook to mkinitcpio.conf..."
        sudo sed -i 's/\(filesystems\)/\1 resume/' /etc/mkinitcpio.conf
        info "Regenerating initramfs..."
        sudo mkinitcpio -P
        ok "Initramfs rebuilt with resume hook"
    fi

    # Verify sleep config
    if [[ -f /etc/systemd/sleep.conf.d/hibernate.conf ]]; then
        ok "sleep.conf.d/hibernate.conf already installed"
    else
        warn "sleep.conf.d/hibernate.conf not found - run the 'system' step"
    fi

    ok "Hibernation configured"
}

healthcheck_hibernate() {
    if [[ ! -f /swap/swapfile ]]; then
        err "Healthcheck failed: /swap/swapfile missing"
        return 1
    fi
    if ! grep -q 'HOOKS=.*resume' /etc/mkinitcpio.conf; then
        err "Healthcheck failed: mkinitcpio resume hook is missing"
        return 1
    fi
    return 0
}

# }}}
# =================================================================================================

# =================================================================================================
#  STEP {FONTS}: Install bundled fonts and refresh font cache  {{{

step_fonts() {
    header "Installing fonts"

    local FONT_DIR="$HOME/.local/share/fonts"
    local TAR_FILE="$DOTFILES/etc/fonts.tar.gz"

    if [[ ! -f "$TAR_FILE" ]]; then
        err "Font archive not found: $TAR_FILE"
        return 1
    fi

    # Extract bundled fonts
    if [[ -d "$FONT_DIR" ]] && [[ -n "$(ls -A "$FONT_DIR" 2>/dev/null)" ]]; then
        ok "Font directory already populated: $FONT_DIR"
        if confirm "Re-extract fonts from fonts.tar.gz?" "n"; then
            mkdir -p "$FONT_DIR"
            info "Extracting fonts..."
            tar -xzf "$TAR_FILE" -C "$HOME/.local/share/"
            ok "Fonts extracted"
        fi
    else
        mkdir -p "$HOME/.local/share"
        info "Extracting fonts from fonts.tar.gz..."
        tar -xzf "$TAR_FILE" -C "$HOME/.local/share/"
        ok "Fonts extracted to $FONT_DIR"
    fi

    # Optionally build Iosevka Vagari
    echo
    if confirm "Build Iosevka Vagari custom font? (requires npm, takes a while)" "n"; then
        if [[ -x "$DOTFILES/etc/iosevka/build.sh" ]]; then
            "$DOTFILES/etc/iosevka/build.sh"
        else
            err "Iosevka build script not found at $DOTFILES/etc/iosevka/build.sh"
            return 1
        fi
    else
        info "Skipping Iosevka build"
    fi

    info "Refreshing font cache..."
    fc-cache -f
    ok "Fonts installed"
}

healthcheck_fonts() {
    local font_dir="$HOME/.local/share/fonts"
    [[ -d "$font_dir" ]] || { err "Healthcheck failed: font directory missing"; return 1; }
    [[ -n "$(ls -A "$font_dir" 2>/dev/null)" ]] || { err "Healthcheck failed: font directory is empty"; return 1; }
    return 0
}

# }}}
# =================================================================================================

# =================================================================================================
#  STEP {GO}: Build Go tools and install/enable user services  {{{

# Binary definitions: name -> module_dir|build_path|description|is_daemon|output_dir
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
        err "Module not found: $full_path"
        return 1
    fi

    info "Building $name from $module_dir/$build_path"
    (cd "$full_path" && go build -o "$install_dir/$name" "$build_path")
    ok "Installed $name -> $install_dir/$name"
}

install_daemon_services() {
    local service_dir="$HOME/.config/systemd/user"
    mkdir -p "$service_dir"

    if ! has_user_bus; then
        warn "No user session bus available; skipping daemon service enable/start"
        return "$STEP_SKIPPED_RC"
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
            ok "$name restarted"
        else
            info "Enabling $name..."
            systemctl --user enable --now "$name"
            ok "$name enabled and started"
        fi
    done
}

step_go() {
    header "Building Go binaries"

    if ! has go; then
        err "Go not found. Install it from packages first."
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
        ok "All $built binaries built successfully"
    else
        warn "$built succeeded, $failed failed"
        return 1
    fi

    local service_rc=0
    install_daemon_services || service_rc=$?
    if [[ $service_rc -eq "$STEP_SKIPPED_RC" ]]; then
        return 0
    fi
    return "$service_rc"
}

healthcheck_go() {
    local name module_dir build_path desc is_daemon output_dir install_dir
    for name in "${!GO_BINARIES[@]}"; do
        IFS='|' read -r module_dir build_path desc is_daemon output_dir <<< "${GO_BINARIES[$name]}"
        install_dir="$HOME/.local/bin"
        [[ -n "$output_dir" ]] && install_dir="$DOTFILES/$output_dir"
        if [[ ! -x "$install_dir/$name" ]]; then
            err "Healthcheck failed: missing Go binary '$install_dir/$name'"
            return 1
        fi
    done
    return 0
}

# }}}
# =================================================================================================

# =================================================================================================
#  STEP {FIREFOX}: Link Firefox theme/prefs and update newtab DB path  {{{

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
        err "No Firefox profiles.ini found"
        err "Start Firefox at least once to generate a profile, then re-run"
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
        err "Firefox Developer Edition profile not found in $ff_root/profiles.ini"
        err "Install Firefox Developer Edition and launch it once to create a profile"
        return 1
    fi

    local full_path="$ff_root/$profile_path"
    if [[ ! -d "$full_path" ]]; then
        err "Profile directory does not exist: $full_path"
        err "Start Firefox to initialize the profile, then re-run"
        return 1
    fi

    FIREFOX_PROFILE_DIR="$full_path"
    FIREFOX_PROFILE_REL="${full_path#"$HOME"/}"
    info "Detected Firefox profile: $FIREFOX_PROFILE_REL"
}

step_firefox() {
    header "Configuring Firefox"

    if ! detect_firefox_profile; then
        if in_chroot; then
            warn "Skipping Firefox setup until first user login/profile creation"
            return "$STEP_SKIPPED_RC"
        fi
        return 1
    fi

    # Install vagari.firefox userChrome CSS
    local vagari_dir="$HOME/vagari.firefox"

    if [[ ! -d "$vagari_dir" ]]; then
        info "Cloning vagari.firefox..."
        git clone https://github.com/cogikyo/vagari.firefox.git "$vagari_dir"
    else
        ok "vagari.firefox already cloned at $vagari_dir"
    fi

    local chrome_dir="$FIREFOX_PROFILE_DIR/chrome"
    mkdir -p "$chrome_dir"

    info "Linking userChrome CSS files..."
    for css_file in "$vagari_dir"/css/*; do
        ln -sfv "$css_file" "$chrome_dir/"
    done
    ok "userChrome CSS linked"

    # Install user.js preferences
    local userjs_src="$DOTFILES/config/firefox/user.js"
    local userjs_dst="$FIREFOX_PROFILE_DIR/user.js"

    if [[ ! -f "$userjs_src" ]]; then
        err "user.js not found at $userjs_src"
        return 1
    fi

    ln -sfv "$userjs_src" "$userjs_dst"
    ok "user.js linked"

    # Update newtab config with profile path
    local config_yaml="$DOTFILES/daemons/config.yaml"
    local new_db_path="$FIREFOX_PROFILE_REL/places.sqlite"

    if [[ ! -f "$config_yaml" ]]; then
        warn "daemons/config.yaml not found, skipping newtab config update"
    else
        local current_db
        current_db=$(grep 'firefox_db:' "$config_yaml" | sed 's/.*firefox_db:[[:space:]]*//' | tr -d '"')

        if [[ "$current_db" == "$new_db_path" ]]; then
            ok "newtab config already points to correct profile"
        else
            info "Updating newtab firefox_db path..."
            sed -i "s|firefox_db:.*|firefox_db: \"$new_db_path\"|" "$config_yaml"
            ok "Updated firefox_db to $new_db_path"
        fi
    fi

    echo
    ok "Firefox configured"
    warn "Restart Firefox for changes to take effect"
}

healthcheck_firefox() {
    if ! detect_firefox_profile; then
        return "$STEP_SKIPPED_RC"
    fi

    [[ -f "$FIREFOX_PROFILE_DIR/user.js" ]] || { err "Healthcheck failed: Firefox user.js missing"; return 1; }
    [[ -d "$FIREFOX_PROFILE_DIR/chrome" ]] || { err "Healthcheck failed: Firefox chrome/ directory missing"; return 1; }
    return 0
}

# }}}
# =================================================================================================

# =================================================================================================
#  STEP {SHELL}: Ensure zsh exists and set it as login shell  {{{

step_shell() {
    header "Setting default shell to zsh"

    needs_sudo

    local current_shell
    current_shell=$(getent passwd "$USER" | cut -d: -f7)

    if [[ -z "$current_shell" ]]; then
        err "Could not detect current shell for user '$USER'"
        return 1
    fi

    if ! has zsh; then
        warn "zsh not found; attempting to install it now..."
        sudo pacman -S --needed --noconfirm zsh
    fi

    local zsh_path
    zsh_path=$(command -v zsh || true)
    if [[ -z "$zsh_path" ]]; then
        err "zsh is still unavailable after install attempt"
        return 1
    fi

    if [[ "$current_shell" == "$zsh_path" ]]; then
        ok "Default shell is already zsh"
        return 0
    fi

    info "Changing default shell for $USER from $current_shell to $zsh_path..."
    sudo chsh -s "$zsh_path" "$USER"
    ok "Default shell changed to zsh (takes effect on next login)"
}

healthcheck_shell() {
    local current_shell zsh_path
    zsh_path=$(command -v zsh || true)
    [[ -n "$zsh_path" ]] || { err "Healthcheck failed: zsh is not installed"; return 1; }

    current_shell=$(getent passwd "$USER" | cut -d: -f7)
    [[ "$current_shell" == "$zsh_path" ]] || {
        err "Healthcheck failed: $USER shell is '$current_shell' (expected '$zsh_path')"
        return 1
    }
    return 0
}

# }}}
# =================================================================================================

# =================================================================================================
#  STEP {DNS}: Configure systemd-resolved + NetworkManager DNS wiring  {{{

step_dns() {
    header "Setting up DNS (systemd-resolved + Cloudflare DNS-over-TLS)"
    needs_sudo

    local chroot_mode=0
    if in_chroot; then
        chroot_mode=1
        warn "Chroot environment detected; runtime restarts/checks will be skipped"
    fi

    local RESOLVED_SRC="$DOTFILES/etc/systemd/resolved.conf"
    local RESOLVED_DST="/etc/systemd/resolved.conf"

    # Require a synced resolved.conf from step_system.
    if [[ ! -f "$RESOLVED_SRC" ]]; then
        err "Source resolved.conf not found at $RESOLVED_SRC"
        return 1
    fi
    if ! sudo test -f "$RESOLVED_DST"; then
        err "$RESOLVED_DST not found."
        err "Run the 'system' step first."
        return 1
    fi
    if ! sudo cmp -s "$RESOLVED_SRC" "$RESOLVED_DST"; then
        err "resolved.conf is missing or outdated."
        err "Run './install.sh system' before running 'dns'."
        return 1
    fi

    # Enable/start systemd-resolved
    if systemctl is-enabled systemd-resolved &>/dev/null; then
        ok "systemd-resolved already enabled"
    else
        info "Enabling systemd-resolved..."
        sudo systemctl enable systemd-resolved
    fi
    if [[ $chroot_mode -eq 0 ]]; then
        sudo systemctl restart systemd-resolved
    else
        warn "Skipping systemd-resolved restart in chroot"
    fi

    # Configure NetworkManager via dedicated drop-in.
    local NM_DIR="/etc/NetworkManager/conf.d"
    local NM_CONF="$NM_DIR/10-dotfiles-dns.conf"
    local nm_tmp
    nm_tmp=$(mktemp)
    printf "[main]\ndns=systemd-resolved\n" > "$nm_tmp"
    if sudo test -f "$NM_CONF" && sudo cmp -s "$nm_tmp" "$NM_CONF"; then
        ok "NetworkManager DNS drop-in already configured"
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

    # Link resolv.conf
    local STUB="/run/systemd/resolve/stub-resolv.conf"
    local stub_real resolv_real
    stub_real=$(readlink -f "$STUB" 2>/dev/null || true)
    resolv_real=$(readlink -f /etc/resolv.conf 2>/dev/null || true)

    if [[ -n "$stub_real" && -n "$resolv_real" && "$resolv_real" == "$stub_real" ]]; then
        ok "resolv.conf already linked to stub"
    else
        if [[ -e /etc/resolv.conf && ! -L /etc/resolv.conf ]]; then
            info "Backing up existing /etc/resolv.conf to /etc/resolv.conf.pre-dotfiles.bak"
            sudo cp /etc/resolv.conf /etc/resolv.conf.pre-dotfiles.bak
        fi
        info "Linking resolv.conf to systemd-resolved stub..."
        sudo ln -sfn "$STUB" /etc/resolv.conf
    fi

    if [[ $chroot_mode -eq 0 ]]; then
        info "Restarting NetworkManager..."
        sudo systemctl restart NetworkManager
    else
        warn "Skipping NetworkManager restart in chroot"
    fi

    if [[ $chroot_mode -eq 0 ]]; then
        if ! systemctl is-active systemd-resolved &>/dev/null; then
            err "systemd-resolved is not active after configuration"
            return 1
        fi
        resolv_real=$(readlink -f /etc/resolv.conf 2>/dev/null || true)
        if [[ -z "$stub_real" || -z "$resolv_real" || "$resolv_real" != "$stub_real" ]]; then
            err "/etc/resolv.conf is not linked to $STUB"
            return 1
        fi
    else
        resolv_real=$(readlink -f /etc/resolv.conf 2>/dev/null || true)
        if [[ -z "$stub_real" || -z "$resolv_real" || "$resolv_real" != "$stub_real" ]]; then
            warn "/etc/resolv.conf is not linked to $STUB yet; verify after first boot"
        fi
    fi

    if [[ $chroot_mode -eq 0 ]] && has resolvectl; then
        local runtime_dns
        runtime_dns=$(resolvectl dns 2>/dev/null | awk 'NF > 2 {for (i = 3; i <= NF; i++) print $i}' | sort -u | tr '\n' ' ')
        [[ -n "$runtime_dns" ]] && info "Runtime DNS servers: $runtime_dns"
    fi

    ok "DNS configured with Cloudflare DNS-over-TLS + strict DNSSEC"
}

healthcheck_dns() {
    local stub="/run/systemd/resolve/stub-resolv.conf"
    local stub_real resolv_real

    stub_real=$(readlink -f "$stub" 2>/dev/null || true)
    resolv_real=$(readlink -f /etc/resolv.conf 2>/dev/null || true)

    if [[ -z "$stub_real" || -z "$resolv_real" || "$resolv_real" != "$stub_real" ]]; then
        err "Healthcheck failed: /etc/resolv.conf is not linked to $stub"
        return 1
    fi

    if ! in_chroot; then
        if ! systemctl is-active systemd-resolved &>/dev/null; then
            err "Healthcheck failed: systemd-resolved is not active"
            return 1
        fi
    fi

    return 0
}

# }}}
# =================================================================================================

# =================================================================================================
#  Step Dispatcher / CLI  {{{

run_step_healthcheck() {
    local step_name="$1"
    local fn="healthcheck_${step_name}"

    if ! declare -f "$fn" &>/dev/null; then
        err "Missing healthcheck function for step '$step_name' ($fn)"
        return 1
    fi

    step "Healthcheck for step '$step_name'..."
    "$fn"
}

validate_healthcheck_coverage() {
    local entry step_name fn
    for entry in "${STEP_DEFS[@]}"; do
        IFS='|' read -r step_name _ _ _ <<< "$entry"
        fn="healthcheck_${step_name}"
        if ! declare -f "$fn" &>/dev/null; then
            die "Missing required healthcheck function: $fn"
        fi
    done
}

list_steps() {
    echo "Available steps:"
    echo
    local i=1
    for entry in "${STEP_DEFS[@]}"; do
        IFS='|' read -r name desc needs_root deps <<< "$entry"
        local badges=""
        [[ "$needs_root" == "yes" ]] && badges+=" ${YELLOW}(sudo)${RESET}"
        [[ -n "$deps" ]] && badges+=" ${BLUE}(after: $deps)${RESET}"
        printf '  %b%02d%b. %-12s %s%b\n' "$BOLD" "$i" "$RESET" "$name" "$desc" "$badges"
        ((i++))
    done
}

get_step_deps() {
    local target="$1"
    for entry in "${STEP_DEFS[@]}"; do
        IFS='|' read -r name _ _ deps <<< "$entry"
        if [[ "$name" == "$target" ]]; then
            echo "$deps"
            return
        fi
    done
}

get_step_desc() {
    local target="$1"
    for entry in "${STEP_DEFS[@]}"; do
        IFS='|' read -r name desc _ _ <<< "$entry"
        if [[ "$name" == "$target" ]]; then
            echo "$desc"
            return
        fi
    done
}

run_step() {
    local name="$1"
    local func="step_$name"

    if ! declare -f "$func" &>/dev/null; then
        err "Unknown step: $name"
        list_steps
        return 1
    fi

    # Skip if already ran (idempotent guard)
    if array_contains "$name" "${PASSED[@]}" || array_contains "$name" "${FAILED[@]}" || array_contains "$name" "${SKIPPED[@]}"; then
        return 0
    fi

    # Check deps are in PASSED[]
    local deps
    deps=$(get_step_deps "$name")
    if [[ -n "$deps" ]]; then
        IFS=',' read -ra dep_list <<< "$deps"
        for dep in "${dep_list[@]}"; do
            dep="${dep#"${dep%%[![:space:]]*}"}"
            dep="${dep%"${dep##*[![:space:]]}"}"
            [[ -n "$dep" ]] || continue
            if ! array_contains "$dep" "${PASSED[@]}"; then
                err "Step '$name' requires '$dep' — run '$dep' first"
                FAILED+=("$name")
                return 1
            fi
        done
    fi

    # Step banner + confirmation
    local desc
    desc=$(get_step_desc "$name")
    header "$name"
    [[ -n "$desc" ]] && dim "$desc"
    if ! is_auto; then
        if ! confirm "Run step '$name'?"; then
            SKIPPED+=("$name")
            return 0
        fi
    elif (( PAUSE > 0 )); then
        dim "Starting in ${PAUSE}s..."
        sleep "$PAUSE"
    fi

    # Run step function
    local rc=0
    ( "$func" ) || rc=$?

    if [[ $rc -eq "$STEP_SKIPPED_RC" ]]; then
        SKIPPED+=("$name")
        return 0
    fi
    if [[ $rc -ne 0 ]]; then
        FAILED+=("$name")
        err "Step '$name' failed (exit code $rc)"
        return 1
    fi

    # Run healthcheck
    local health_rc=0
    run_step_healthcheck "$name" || health_rc=$?
    if [[ $health_rc -eq "$STEP_SKIPPED_RC" ]]; then
        health_rc=0
    fi
    if [[ $health_rc -ne 0 ]]; then
        FAILED+=("$name")
        err "Step '$name' failed healthcheck"
        return 1
    fi

    ok "Step '$name' completed"
    PASSED+=("$name")
}

print_summary() {
    [[ ${#PASSED[@]} -eq 0 && ${#FAILED[@]} -eq 0 && ${#SKIPPED[@]} -eq 0 ]] && return

    header "Summary"
    for name in "${PASSED[@]}"; do
        _log "$GREEN" "OK" "$name"
    done
    for name in "${FAILED[@]}"; do
        _log "$RED" "FAIL" "$name"
    done
    for name in "${SKIPPED[@]}"; do
        _log "$YELLOW" "SKIP" "$name"
    done
    echo

    if [[ ${#FAILED[@]} -gt 0 ]]; then
        banner_end "DOTFILES INSTALL" "FAILED" "passed=${#PASSED[@]} failed=${#FAILED[@]} skipped=${#SKIPPED[@]}"
        return 1
    else
        banner_end "DOTFILES INSTALL" "SUCCESS" "passed=${#PASSED[@]} failed=0 skipped=${#SKIPPED[@]}"
        ok "All steps completed successfully"
        return 0
    fi
}

run_all() {
    for entry in "${STEP_DEFS[@]}"; do
        IFS='|' read -r name _ _ _ <<< "$entry"
        run_step "$name" || break
    done
    print_summary
}

run_healthchecks() {
    local steps=("$@")
    local passed=0 failed=0 skipped=0

    if [[ ${#steps[@]} -eq 0 ]]; then
        for entry in "${STEP_DEFS[@]}"; do
            IFS='|' read -r name _ _ _ <<< "$entry"
            steps+=("$name")
        done
    fi

    header "Healthchecks"
    for name in "${steps[@]}"; do
        local fn="healthcheck_${name}"
        if ! declare -f "$fn" &>/dev/null; then
            err "Unknown step: $name"
            ((++failed))
            continue
        fi
        local rc=0
        "$fn" || rc=$?
        if [[ $rc -eq "$STEP_SKIPPED_RC" ]]; then
            _log "$YELLOW" "SKIP" "$name"
            ((++skipped))
        elif [[ $rc -eq 0 ]]; then
            _log "$GREEN" "PASS" "$name"
            ((++passed))
        else
            _log "$RED" "FAIL" "$name"
            ((++failed))
        fi
    done
    echo
    info "passed=$passed failed=$failed skipped=$skipped"
    [[ $failed -eq 0 ]]
}

main() {
    if [[ "$ARCH" == "1" ]]; then
        run_arch_mode "$@"
        return
    fi

    ensure_dotfiles_checkout "$@"
    validate_healthcheck_coverage

    if ! [[ "$PAUSE" =~ ^[0-9]+$ ]]; then
        warn "Invalid PAUSE='$PAUSE' (expected integer seconds); defaulting to 0"
        PAUSE=0
    fi

    if [[ $# -eq 0 ]]; then
        usage
        exit 0
    fi

    case "$1" in
        --list|-l)
            printf '%b== dotfiles install v%s ==%b\n' "$BLUE" "$VERSION" "$RESET"
            list_steps
            ;;
        --help|-h)
            printf '%b== dotfiles install v%s ==%b\n' "$BLUE" "$VERSION" "$RESET"
            usage
            ;;
        --check|-c)
            shift
            run_healthchecks "$@"
            ;;
        all)
            banner "post-install" "all"
            run_all
            ;;
        *)
            banner "post-install" "$*"
            for name in "$@"; do
                run_step "$name" || break
            done
            print_summary
            ;;
    esac
}

# }}}
# =================================================================================================

main "$@"
