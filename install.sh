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

# ========================
#  Runtime Config
# ========================

INSTALL_VERSION="2026.02.15.3"
DOTFILES="${DOTFILES:-$HOME/dotfiles}"
DOTFILES_REPO="${DOTFILES_REPO:-https://github.com/cogikyo/dotfiles.git}"
DOTFILES_REF="${DOTFILES_REF:-master}"
STEP_SKIPPED_RC=42
STEP_ABORT_RC=130
ARCH="${ARCH:-0}"
DOTFILES_INSTALL_PREBOOT="${DOTFILES_INSTALL_PREBOOT:-0}"
strict_default=1
if [[ "$ARCH" == "1" ]]; then
    strict_default=0
fi
STRICT="${STRICT:-$strict_default}"
PAUSE="${PAUSE:-3}"
DOTFILES_RAW_BASE="${DOTFILES_RAW_BASE:-https://raw.githubusercontent.com/cogikyo/dotfiles/$DOTFILES_REF}"
DOTFILES_TARGET_ROOT="${DOTFILES_TARGET_ROOT:-/mnt}"
DOTFILES_TARGET_USER="${DOTFILES_TARGET_USER:-}"
ARCHINSTALL_PROFILE_MODE="${ARCHINSTALL_PROFILE_MODE:-minimal}"
SKIP="${SKIP:-${ARCH_POST_SKIP:-0}}"
STEPS="${STEPS:-${STPES:-${ARCH_STEPS:-${ARCH_POST_STEPS:-all}}}}"
NONINTERACTIVE="${NONINTERACTIVE:-${ARCH_NONINTERACTIVE:-0}}"

# ========================
#  Colors / Logging
# ========================

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

# Backward-compatible color aliases used throughout this script.
R="$RED"
G="$GREEN"
Y="$YELLOW"
B="$BLUE"
M="$MAGENTA"
F="$FAINT"
BD="$BOLD"
N="$RESET"

BANNER_WIDTH=62

log_line() {
    local color="$1"
    local level="$2"
    shift 2
    printf '%b[%-5s]%b %s\n' "$color" "$level" "$RESET" "$*"
}

banner_line() {
    local rule
    rule=$(printf '%*s' "$BANNER_WIDTH" '' | tr ' ' '=')
    printf '%b+%s+%b\n' "$MAGENTA" "$rule" "$RESET"
}

banner_text() {
    local text="$1"
    printf '%b| %-*s |%b\n' "$MAGENTA" "$BANNER_WIDTH" "$text" "$RESET"
}

banner_kv() {
    local key="$1"
    local value="$2"
    banner_text "$key: $value"
}

script_header() { printf '%b== dotfiles install v%s ==%b\n' "$BLUE" "$INSTALL_VERSION" "$RESET"; }

info()    { log_line "$BLUE" "INFO" "$*"; }
step()    { printf '%b  ==>%b %s\n' "$BLUE" "$RESET" "$*"; }
success() { log_line "$GREEN" "OK" "$*"; }
warn()    { log_line "$YELLOW" "WARN" "$*"; }
error()   { log_line "$RED" "ERROR" "$*" >&2; }
ask()     { log_line "$MAGENTA" "ASK" "$*"; }
faint()   { printf '%b%s%b\n' "$FAINT" "$*" "$RESET"; }
finish()  { log_line "$GREEN" "DONE" "$*"; }
header()  { printf '\n%b--- %s ---%b\n\n' "$MAGENTA" "$*" "$RESET"; }
die()     { error "$*"; exit 1; }

print_start_banner() {
    local mode="$1"
    local selection="$2"

    script_header
    banner_line
    banner_text "DOTFILES INSTALL START"
    banner_line
    banner_kv "mode" "$mode"
    banner_kv "selection" "$selection"
    banner_kv "dotfiles" "$DOTFILES"
    banner_kv "strict" "$STRICT"
    banner_kv "pause" "$PAUSE"
    banner_kv "preboot" "$DOTFILES_INSTALL_PREBOOT"
    banner_kv "noninteractive" "${DOTFILES_INSTALL_NONINTERACTIVE:-0}"
    banner_kv "chroot-forced" "${DOTFILES_INSTALL_CHROOT:-0}"
    banner_line
}

print_arch_start_banner() {
    script_header
    banner_line
    banner_text "ARCH BOOTSTRAP START (ARCH=1)"
    banner_line
    banner_kv "target-root" "$DOTFILES_TARGET_ROOT"
    banner_kv "target-user" "${DOTFILES_TARGET_USER:-auto-detect}"
    banner_kv "profile-mode" "$ARCHINSTALL_PROFILE_MODE"
    banner_kv "steps" "$STEPS"
    banner_kv "skip-post-install" "$SKIP"
    banner_kv "strict" "$STRICT"
    banner_kv "pause" "$PAUSE"
    banner_kv "noninteractive" "$NONINTERACTIVE"
    banner_line
}

print_finish_banner() {
    local mode="$1"
    local status="$2"
    local details="${3:-}"

    banner_line
    banner_text "$mode FINISH ($status)"
    [[ -n "$details" ]] && banner_text "$details"
    banner_line
}

# ========================
#  Step Registry
# ========================

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
SOFT_FAILED=()
declare -A STEP_ACTIVE=()

# ========================
#  Core Helpers
# ========================

confirm() {
    local prompt="$1" default="${2:-y}"
    local yn
    if is_noninteractive; then
        if [[ "$default" == "y" ]]; then
            info "$prompt -> yes (non-interactive default)"
            return 0
        fi
        info "$prompt -> no (non-interactive default)"
        return 1
    fi

    if [[ "$default" == "y" ]]; then
        ask "$prompt [Y/n]"
        read -r yn
        yn="${yn:-y}"
    else
        ask "$prompt [y/N]"
        read -r yn
        yn="${yn:-n}"
    fi
    [[ "$yn" =~ ^[Yy] ]]
}

confirm_continue_after_failure() {
    local step_name="$1"
    local step_log="${2:-}"
    local step_rc="${3:-1}"

    if is_noninteractive; then
        return 0
    fi

    warn "Step '$step_name' failed (exit code $step_rc)"
    if [[ -n "$step_log" && -f "$step_log" ]]; then
        warn "Recent output from '$step_name' (log: $step_log)"
        tail -n 25 "$step_log" | sed 's/^/  | /'
    fi

    if confirm "Continue to next step?" "n"; then
        return 0
    fi

    warn "Stopping at failed step '$step_name' by user request"
    return "$STEP_ABORT_RC"
}

has() { command -v "$1" &>/dev/null; }

is_noninteractive() {
    [[ "${DOTFILES_INSTALL_NONINTERACTIVE:-0}" == "1" || ! -t 0 ]]
}

is_headless_override() {
    [[ "${DOTFILES_INSTALL_ALLOW_HEADLESS:-0}" == "1" ]]
}

is_preboot_mode() {
    [[ "$DOTFILES_INSTALL_PREBOOT" == "1" ]]
}

is_best_effort_mode() {
    [[ "$STRICT" != "1" ]]
}

is_critical_step() {
    case "$1" in
        packages|link|system) return 0 ;;
        *) return 1 ;;
    esac
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

canonpath() {
    if has realpath; then
        realpath -m -- "$1" 2>/dev/null || realpath "$1" 2>/dev/null || printf '%s\n' "$1"
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

# ========================
#  Arch Bootstrap Mode
# ========================

arch_mode_usage() {
    cat <<'EOF'
Usage: ARCH=1 install.sh [--auto]

Notes:
  --auto is accepted for compatibility, but default behavior is already auto.
  Set SKIP=1 to skip chroot post-install automation.
  Set STEPS to run a subset (default: all).
    Back-compat aliases: STPES, ARCH_STEPS, ARCH_POST_STEPS.
  Set STRICT=1 for strict fail-fast post-install.
  Set PAUSE=3 to pause N seconds before each install step.
  Set NONINTERACTIVE=1 for unattended post-install.
    Back-compat alias: ARCH_NONINTERACTIVE.
  Set ARCHINSTALL_PROFILE_MODE=minimal|preset (default: minimal).
EOF
}

prepare_arch_partial_config() {
    local config_path="$1"
    local firmware_mode="$2"
    local profile_mode="$3"

    python3 - "$config_path" "$firmware_mode" "$profile_mode" <<'PYEOF'
import json
import sys

config_path, firmware_mode, profile_mode = sys.argv[1:4]

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

# Keep install minimal by default; full desktop setup is handled post-install.
if profile_mode == "minimal":
    config["profile_config"] = None
elif profile_mode != "preset":
    raise SystemExit(
        f"Invalid ARCHINSTALL_PROFILE_MODE={profile_mode!r} "
        "(expected 'minimal' or 'preset')"
    )

with open(config_path, "w", encoding="utf-8") as f:
    json.dump(config, f, indent=4)
PYEOF
}

run_arch_post_install() {
    local target_root="$1"
    local target_user="$2"
    local post_install_steps="$3"
    local install_strict="$4"
    local install_noninteractive="$5"
    local install_pause="$6"

    command -v arch-chroot >/dev/null || die "arch-chroot is required for post-install"
    [[ -d "$target_root/etc" ]] || die "Target root not found: $target_root"
    [[ -f "$target_root/etc/passwd" ]] || die "Target root is missing /etc/passwd: $target_root"

    if [[ -z "$target_user" ]]; then
        target_user=$(
            awk -F: '
                $3 >= 1000 && $1 != "nobody" && $7 !~ /(nologin|false)$/ {
                    print $1
                    exit
                }
            ' "$target_root/etc/passwd"
        )
    fi
    [[ -n "$target_user" ]] || die "Could not detect target user. Set DOTFILES_TARGET_USER=<user>."

    local user_home
    user_home=$(awk -F: -v user="$target_user" '$1 == user { print $6 }' "$target_root/etc/passwd")
    [[ -n "$user_home" ]] || user_home="/home/$target_user"

    info "Running post-install in chroot for user '$target_user'..."
    arch-chroot "$target_root" /bin/bash -s -- \
        "$target_user" \
        "$user_home" \
        "$DOTFILES_REF" \
        "$DOTFILES_REPO" \
        "$post_install_steps" \
        "$install_strict" \
        "$install_noninteractive" \
        "$install_pause" <<'CHROOT_EOF'
set -euo pipefail

target_user="$1"
user_home="$2"
dotfiles_ref="$3"
dotfiles_repo="$4"
post_install_steps="$5"
install_strict="$6"
install_noninteractive="$7"
install_pause="$8"
dotfiles_dir="$user_home/dotfiles"
sudoers_file="/etc/sudoers.d/99-dotfiles-install"
step_args=()

read -r -a step_args <<< "$post_install_steps"
if [[ ${#step_args[@]} -eq 0 ]]; then
    step_args=(all)
fi

retry_cmd() {
    local attempts="$1"
    local delay="$2"
    shift 2
    local n=1
    while true; do
        if "$@"; then
            return 0
        fi
        if (( n >= attempts )); then
            return 1
        fi
        echo "Retry $n/$attempts failed for: $*" >&2
        sleep "$delay"
        ((n++))
    done
}

if ! id "$target_user" &>/dev/null; then
    echo "Target user does not exist in chroot: $target_user" >&2
    exit 1
fi

retry_cmd 3 3 pacman -Sy --noconfirm archlinux-keyring
retry_cmd 3 3 pacman -S --needed --noconfirm sudo git base-devel age

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
    retry_cmd 3 2 sudo -H -u "$target_user" git -C "$dotfiles_dir" fetch --depth 1 origin "$dotfiles_ref"
    sudo -H -u "$target_user" git -C "$dotfiles_dir" checkout -f FETCH_HEAD
elif [[ -e "$dotfiles_dir" ]]; then
    echo "dotfiles path exists but is not a git checkout: $dotfiles_dir" >&2
    exit 1
else
    retry_cmd 3 2 sudo -H -u "$target_user" git clone --depth 1 --branch "$dotfiles_ref" "$dotfiles_repo" "$dotfiles_dir"
fi
chown -R "$target_user:$target_user" "$dotfiles_dir"

if [[ -r /dev/tty ]]; then
    sudo -H -u "$target_user" env \
        ARCH=0 \
        DOTFILES="$dotfiles_dir" \
        DOTFILES_REF="$dotfiles_ref" \
        DOTFILES_INSTALL_TARGET_USER="$target_user" \
        DOTFILES_INSTALL_ALLOW_HEADLESS=1 \
        DOTFILES_INSTALL_NONINTERACTIVE="$install_noninteractive" \
        DOTFILES_INSTALL_PREBOOT=1 \
        DOTFILES_INSTALL_CHROOT=1 \
        STRICT="$install_strict" \
        PAUSE="$install_pause" \
        "$dotfiles_dir/install.sh" "${step_args[@]}" < /dev/tty
else
    echo "No interactive TTY detected; running without secrets decrypt prompt" >&2
    sudo -H -u "$target_user" env \
        ARCH=0 \
        DOTFILES="$dotfiles_dir" \
        DOTFILES_REF="$dotfiles_ref" \
        DOTFILES_INSTALL_TARGET_USER="$target_user" \
        DOTFILES_INSTALL_ALLOW_HEADLESS=1 \
        DOTFILES_INSTALL_NONINTERACTIVE=1 \
        DOTFILES_INSTALL_PREBOOT=1 \
        DOTFILES_INSTALL_CHROOT=1 \
        STRICT="$install_strict" \
        PAUSE="$install_pause" \
        DOTFILES_SKIP_SECRETS=1 \
        "$dotfiles_dir/install.sh" "${step_args[@]}"
fi
CHROOT_EOF
}

run_arch_mode() {
    local script_path script_dir source_config tmp_source_config config firmware_mode
    script_path="${BASH_SOURCE[0]:-$0}"
    script_dir="$(cd "$(dirname "$script_path")" && pwd)"
    source_config="$script_dir/etc/arch.json"
    tmp_source_config="/tmp/arch_source_config.json"
    config="/tmp/arch_config.json"

    if [[ $# -gt 0 ]]; then
        case "$1" in
            --auto)
                shift
                ;;
            -h|--help)
                arch_mode_usage
                exit 0
                ;;
            *)
                die "Unknown argument in ARCH mode: $1"
                ;;
        esac
    fi
    [[ $# -eq 0 ]] || die "Unexpected extra arguments in ARCH mode: $*"

    print_arch_start_banner

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
        info "Local config not found at $source_config"
        info "Downloading config: $DOTFILES_RAW_BASE/etc/arch.json"
        curl -fsSL "$DOTFILES_RAW_BASE/etc/arch.json" -o "$tmp_source_config" || die "Failed to download etc/arch.json"
        cp -f "$tmp_source_config" "$config"
    fi

    info "Preparing partial config..."
    prepare_arch_partial_config "$config" "$firmware_mode" "$ARCHINSTALL_PROFILE_MODE"
    success "Partial config ready: $config"

    info "Starting archinstall..."
    warn "Set disk partitioning and authentication in the archinstall UI"
    if ! archinstall --config "$config"; then
        error "archinstall failed"
        warn "Recent archinstall log output:"
        if [[ -f /var/log/archinstall/install.log ]]; then
            tail -n 120 /var/log/archinstall/install.log || true
        else
            warn "No /var/log/archinstall/install.log found"
        fi
        print_finish_banner "ARCH BOOTSTRAP" "FAILED" "archinstall returned non-zero"
        exit 1
    fi

    if [[ "$SKIP" == "1" ]]; then
        warn "Skipping post-install automation (SKIP=1)"
    else
        if ! run_arch_post_install "$DOTFILES_TARGET_ROOT" "$DOTFILES_TARGET_USER" "$STEPS" "$STRICT" "$NONINTERACTIVE" "$PAUSE"; then
            error "Post-install automation failed"
            warn "You can reboot and run ~/dotfiles/install.sh all manually"
            print_finish_banner "ARCH BOOTSTRAP" "FAILED" "chroot post-install failed"
            exit 1
        fi
    fi

    finish "Installation complete"
    print_finish_banner "ARCH BOOTSTRAP" "SUCCESS" "reboot then run deferred desktop steps"
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
    if [[ "${DOTFILES_INSTALL_CHROOT:-0}" == "1" ]]; then
        return 0
    fi

    if has systemd-detect-virt; then
        systemd-detect-virt --quiet --chroot &>/dev/null && return 0
        if [[ $EUID -ne 0 ]] && has sudo && sudo -n true 2>/dev/null; then
            sudo systemd-detect-virt --quiet --chroot &>/dev/null && return 0
        fi
    fi

    local root_id init_root_id
    root_id=$(stat -Lc '%d:%i' / 2>/dev/null || true)
    init_root_id=$(stat -Lc '%d:%i' /proc/1/root/. 2>/dev/null || true)
    if [[ -n "$root_id" && -n "$init_root_id" && "$root_id" != "$init_root_id" ]]; then
        return 0
    fi

    # In chroot-like environments, systemctl often reports "offline".
    if has systemctl; then
        local system_state=""
        system_state=$(systemctl is-system-running 2>/dev/null || true)
        [[ "$system_state" == "offline" ]] && return 0
    fi

    return 1
}

is_paru_usable() {
    has paru || return 1
    paru --version &>/dev/null
}

require_desktop_environment() {
    if is_headless_override; then
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
    if [[ $EUID -eq 0 ]]; then
        return 0
    fi

    if ! has sudo; then
        error "sudo not found. Install sudo first."
        return 1
    fi

    if ! sudo -n true 2>/dev/null; then
        info "Some steps require sudo access"
        sudo -v
    fi
}

resolve_target_user() {
    if [[ -n "${DOTFILES_INSTALL_TARGET_USER:-}" ]]; then
        printf '%s\n' "$DOTFILES_INSTALL_TARGET_USER"
        return 0
    fi

    if [[ $EUID -eq 0 ]]; then
        if [[ -n "${SUDO_USER:-}" && "$SUDO_USER" != "root" ]]; then
            printf '%s\n' "$SUDO_USER"
            return 0
        fi
        printf '%s\n' "${USER:-$(id -un)}"
        return 0
    fi

    printf '%s\n' "${USER:-$(id -un)}"
}

bootstrap_paru() {
    if is_paru_usable; then
        success "paru already installed"
        return 0
    fi

    if has paru; then
        warn "paru is installed but not runnable (likely libalpm mismatch)"
        info "Attempting to reinstall paru from official repos first..."
        if [[ $EUID -eq 0 ]]; then
            pacman -S --needed --noconfirm paru || warn "pacman reinstall of paru failed; falling back to AUR bootstrap"
        else
            sudo pacman -S --needed --noconfirm paru || warn "pacman reinstall of paru failed; falling back to AUR bootstrap"
        fi

        if is_paru_usable; then
            success "paru repaired via pacman"
            return 0
        fi
    fi

    info "paru not found; using AUR prebuilt package (paru-bin)"
    info "This does not compile paru locally"
    has git || { error "git not found. Install git first."; return 1; }
    has makepkg || { error "makepkg not found. Install base-devel first."; return 1; }

    if [[ $EUID -eq 0 ]]; then
        error "Cannot build AUR packages as root. Re-run as your regular user."
        return 1
    fi

    local cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}"
    local build_root="$cache_dir/paru-bootstrap"
    local makepkg_log="$cache_dir/paru-bootstrap.log"
    rm -rf "$build_root"
    mkdir -p "$cache_dir"
    if ! git clone --depth 1 "https://aur.archlinux.org/paru-bin.git" "$build_root"; then
        error "Failed to clone paru-bin AUR repo"
        return 1
    fi
    if ! (cd "$build_root" && makepkg -si --noconfirm --needed 2>&1 | tee "$makepkg_log"); then
        error "makepkg failed while bootstrapping paru-bin (log: $makepkg_log)"
        rm -rf "$build_root"
        if [[ "${DOTFILES_PARU_ALLOW_SOURCE_FALLBACK:-0}" == "1" ]]; then
            warn "DOTFILES_PARU_ALLOW_SOURCE_FALLBACK=1 set; retrying with source package (paru)"
            if ! git clone --depth 1 "https://aur.archlinux.org/paru.git" "$build_root"; then
                error "Failed to clone paru AUR repo"
                return 1
            fi
            if ! (cd "$build_root" && makepkg -si --noconfirm --needed 2>&1 | tee "$makepkg_log"); then
                error "makepkg failed while bootstrapping paru from source (log: $makepkg_log)"
                rm -rf "$build_root"
                return 1
            fi
        else
            return 1
        fi
    fi
    rm -rf "$build_root"

    if ! is_paru_usable; then
        error "paru bootstrap failed"
        return 1
    fi

    success "paru bootstrapped from AUR"
}

# ========================
#  STEP {PACKAGES}: Install pacman/AUR package sets and Rust stable toolchain
# ========================

step_packages() {
    header "Installing packages"

    needs_sudo
    bootstrap_paru

    "$DOTFILES/bin/update" --install
    ensure_rustup_stable
}

# ========================
#  STEP {LINK}: Symlink dotfiles config, scripts, and shell profile
# ========================

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

    success "Linking complete"
}

# ========================
#  STEP {SECRETS}: Decrypt age-managed secrets into local target paths
# ========================

step_secrets() {
    header "Decrypting secrets"

    if [[ "${DOTFILES_SKIP_SECRETS:-0}" == "1" ]]; then
        warn "Skipping secrets (DOTFILES_SKIP_SECRETS=1)"
        return "$STEP_SKIPPED_RC"
    fi

    if ! has age; then
        error "age not found - install with: pacman -S age"
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

# ========================
#  STEP {REPOS}: Create base directories and clone repos from manifest
# ========================

step_repos() {
    header "Cloning repositories and creating directories"

    local REPOS_FILE="$DOTFILES/etc/repos.toml"

    if ! has git; then
        error "git not found. Install it from packages first."
        return 1
    fi

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
                        error "Failed to clone $repo"
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
                error "Failed to clone $repo"
                ((failed++))
            fi
        fi
    fi

    echo
    success "Cloned $cloned repos ($skipped already exist)"
    if [[ $failed -gt 0 ]]; then
        warn "$failed repo(s) failed to clone"
        return 1
    fi
}

# ========================
#  STEP {SYSTEM}: Install system-level config files and enable core services
# ========================

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
            if sudo systemctl enable "$svc" &>/dev/null; then
                success "$svc enabled"
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

# ========================
#  STEP {SWAP}: Provision btrfs swap subvolume/file and fstab entry
# ========================

step_swap() {
    header "Setting up swap"
    needs_sudo

    local SWAP_SIZE="${DOTFILES_SWAP_SIZE:-16G}"
    local SWAP_DIR="/swap"
    local SWAP_FILE="$SWAP_DIR/swapfile"
    local root_fs

    if ! has btrfs; then
        warn "btrfs command not found. Skipping btrfs swap setup."
        return "$STEP_SKIPPED_RC"
    fi

    root_fs=$(findmnt -no FSTYPE / 2>/dev/null || true)
    if [[ "$root_fs" != "btrfs" ]]; then
        warn "Root filesystem is '$root_fs' (not btrfs). Skipping btrfs swap setup."
        return "$STEP_SKIPPED_RC"
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

# ========================
#  STEP {HIBERNATE}: Configure resume args, hooks, and initramfs for hibernate
# ========================

step_hibernate() {
    header "Configuring hibernation"
    needs_sudo

    local SWAP_FILE="/swap/swapfile"

    if [[ ! -f "$SWAP_FILE" ]]; then
        error "Swapfile not found at $SWAP_FILE. Run the 'swap' step first."
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
        error "Failed to determine a valid numeric resume_offset for $SWAP_FILE"
        error "Check output of: sudo btrfs inspect-internal map-swapfile -r $SWAP_FILE"
        return 1
    fi
    local RESUME_UUID
    RESUME_UUID=$(findmnt -no UUID -T "$SWAP_FILE" 2>/dev/null || true)
    if [[ -z "$RESUME_UUID" ]]; then
        error "Failed to determine resume UUID for $SWAP_FILE"
        return 1
    fi

    info "Resume UUID:   $RESUME_UUID"
    info "Resume Offset: $RESUME_OFFSET"

    # Update bootloader config (systemd-boot or GRUB).
    local LOADER_ENTRY
    local GRUB_DEFAULT="/etc/default/grub"
    LOADER_ENTRY=$(find /boot/loader/entries/ -name '*_linux.conf' 2>/dev/null | head -1)

    if [[ -n "$LOADER_ENTRY" ]]; then
        if ! grep -q '^options' "$LOADER_ENTRY"; then
            error "No 'options' line found in $LOADER_ENTRY"
            return 1
        fi

        info "Updating resume parameters in $LOADER_ENTRY..."
        sudo sed -i -E \
            -e 's/[[:space:]]resume=UUID=[^[:space:]]+//g' \
            -e 's/[[:space:]]resume_offset=[^[:space:]]+//g' \
            -e "s|^options.*|& resume=UUID=$RESUME_UUID resume_offset=$RESUME_OFFSET|" \
            "$LOADER_ENTRY"
        success "Updated systemd-boot entry"
    elif [[ -f "$GRUB_DEFAULT" ]]; then
        info "Updating resume parameters in $GRUB_DEFAULT..."
        if grep -q '^GRUB_CMDLINE_LINUX_DEFAULT=' "$GRUB_DEFAULT"; then
            sudo sed -i -E \
                -e '/^GRUB_CMDLINE_LINUX_DEFAULT=/{s/[[:space:]]resume=UUID=[^[:space:]]+//g; s/[[:space:]]resume_offset=[^[:space:]]+//g; s/"$/ resume=UUID='"$RESUME_UUID"' resume_offset='"$RESUME_OFFSET"'"/}' \
                "$GRUB_DEFAULT"
        elif grep -q '^GRUB_CMDLINE_LINUX=' "$GRUB_DEFAULT"; then
            sudo sed -i -E \
                -e '/^GRUB_CMDLINE_LINUX=/{s/[[:space:]]resume=UUID=[^[:space:]]+//g; s/[[:space:]]resume_offset=[^[:space:]]+//g; s/"$/ resume=UUID='"$RESUME_UUID"' resume_offset='"$RESUME_OFFSET"'"/}' \
                "$GRUB_DEFAULT"
        else
            printf 'GRUB_CMDLINE_LINUX_DEFAULT="resume=UUID=%s resume_offset=%s"\n' "$RESUME_UUID" "$RESUME_OFFSET" | sudo tee -a "$GRUB_DEFAULT" > /dev/null
        fi

        if has grub-mkconfig; then
            info "Regenerating GRUB config..."
            sudo grub-mkconfig -o /boot/grub/grub.cfg
            success "Updated GRUB config"
        else
            warn "grub-mkconfig not found; regenerate GRUB config manually"
        fi
    else
        warn "No supported bootloader config found; skipping bootloader resume parameters"
        return "$STEP_SKIPPED_RC"
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
        warn "sleep.conf.d/hibernate.conf not found - run the 'system' step"
    fi

    success "Hibernation configured"
}

# ========================
#  STEP {FONTS}: Install bundled fonts and refresh font cache
# ========================

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
    else
        info "Skipping Iosevka build"
    fi

    info "Refreshing font cache..."
    fc-cache -f
    success "Fonts installed"
}

# ========================
#  STEP {GO}: Build Go tools and install/enable user services
# ========================

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
        error "Module not found: $full_path"
        return 1
    fi

    info "Building $name from $module_dir/$build_path"
    (cd "$full_path" && go build -o "$install_dir/$name" "$build_path")
    success "Installed $name -> $install_dir/$name"
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

    local service_rc=0
    install_daemon_services || service_rc=$?
    if [[ $service_rc -eq "$STEP_SKIPPED_RC" ]]; then
        return 0
    fi
    return "$service_rc"
}

# ========================
#  STEP {FIREFOX}: Link Firefox theme/prefs and update newtab DB path
# ========================

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

    if ! detect_firefox_profile; then
        if in_chroot || is_headless_override || is_noninteractive; then
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

# ========================
#  STEP {SHELL}: Ensure zsh exists and set it as login shell
# ========================

step_shell() {
    header "Setting default shell to zsh"

    needs_sudo

    local target_user
    target_user=$(resolve_target_user)
    local current_shell
    current_shell=$(getent passwd "$target_user" | cut -d: -f7)

    if [[ -z "$current_shell" ]]; then
        error "Could not detect current shell for user '$target_user'"
        return 1
    fi

    if ! has zsh; then
        warn "zsh not found; attempting to install it now..."
        if [[ $EUID -eq 0 ]]; then
            pacman -S --needed --noconfirm zsh
        else
            sudo pacman -S --needed --noconfirm zsh
        fi
    fi

    local zsh_path
    zsh_path=$(command -v zsh || true)
    if [[ -z "$zsh_path" ]]; then
        error "zsh is still unavailable after install attempt"
        return 1
    fi

    if [[ "$current_shell" == "$zsh_path" ]]; then
        success "Default shell is already zsh"
        return 0
    fi

    info "Changing default shell for $target_user from $current_shell to $zsh_path..."
    if [[ $EUID -eq 0 ]]; then
        chsh -s "$zsh_path" "$target_user"
    else
        sudo chsh -s "$zsh_path" "$target_user"
    fi
    success "Default shell changed to zsh (takes effect on next login)"
}

# ========================
#  STEP {DNS}: Configure systemd-resolved + NetworkManager DNS wiring
# ========================

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
    local stub_real resolv_real
    stub_real=$(readlink -f "$STUB" 2>/dev/null || true)
    resolv_real=$(readlink -f /etc/resolv.conf 2>/dev/null || true)

    if [[ -n "$stub_real" && -n "$resolv_real" && "$resolv_real" == "$stub_real" ]]; then
        success "resolv.conf already linked to stub"
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
            error "systemd-resolved is not active after configuration"
            return 1
        fi
        resolv_real=$(readlink -f /etc/resolv.conf 2>/dev/null || true)
        if [[ -z "$stub_real" || -z "$resolv_real" || "$resolv_real" != "$stub_real" ]]; then
            error "/etc/resolv.conf is not linked to $STUB"
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

    success "DNS configured with Cloudflare DNS-over-TLS + strict DNSSEC"
}

# ========================
#  Step Healthchecks
# ========================

healthcheck_packages() {
    if ! is_paru_usable; then
        error "Healthcheck failed: paru is not runnable"
        return 1
    fi
    return 0
}

healthcheck_link() {
    local zsh_src zsh_dst
    zsh_src=$(canonpath "$DOTFILES/config/zsh/zshrc")
    zsh_dst=$(canonpath "$HOME/.zshrc")

    [[ -L "$HOME/.zshrc" ]] || { error "Healthcheck failed: ~/.zshrc is not a symlink"; return 1; }
    [[ "$zsh_src" == "$zsh_dst" ]] || { error "Healthcheck failed: ~/.zshrc does not point to $DOTFILES/config/zsh/zshrc"; return 1; }
    [[ -d "$HOME/.config" ]] || { error "Healthcheck failed: ~/.config does not exist"; return 1; }
    [[ -d "$HOME/.local/bin" ]] || { error "Healthcheck failed: ~/.local/bin does not exist"; return 1; }
    return 0
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
            error "Healthcheck failed: missing secret target '$expanded' for '$name'"
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

healthcheck_repos() {
    local repos_file="$DOTFILES/etc/repos.toml"
    local line key val path expanded
    local expected=0
    local missing=0

    [[ -f "$repos_file" ]] || { error "Healthcheck failed: missing repos manifest"; return 1; }

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
                error "Healthcheck failed: repo path missing '$expanded'"
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
            error "Healthcheck failed: missing system file '$path'"
            return 1
        fi
    done

    return 0
}

healthcheck_swap() {
    local root_fs
    root_fs=$(findmnt -no FSTYPE / 2>/dev/null || true)
    [[ "$root_fs" == "btrfs" ]] || return "$STEP_SKIPPED_RC"

    [[ -f /swap/swapfile ]] || { error "Healthcheck failed: /swap/swapfile missing"; return 1; }
    if ! awk '$0 !~ /^[[:space:]]*#/ && $1 == "/swap/swapfile" && $3 == "swap" {found=1} END {exit(found ? 0 : 1)}' /etc/fstab; then
        error "Healthcheck failed: /swap/swapfile is missing from /etc/fstab"
        return 1
    fi
    return 0
}

healthcheck_hibernate() {
    if [[ ! -f /swap/swapfile ]]; then
        error "Healthcheck failed: /swap/swapfile missing"
        return 1
    fi
    if ! grep -q 'HOOKS=.*resume' /etc/mkinitcpio.conf; then
        error "Healthcheck failed: mkinitcpio resume hook is missing"
        return 1
    fi
    return 0
}

healthcheck_fonts() {
    local font_dir="$HOME/.local/share/fonts"
    [[ -d "$font_dir" ]] || { error "Healthcheck failed: font directory missing"; return 1; }
    [[ -n "$(ls -A "$font_dir" 2>/dev/null)" ]] || { error "Healthcheck failed: font directory is empty"; return 1; }
    return 0
}

healthcheck_go() {
    local name module_dir build_path desc is_daemon output_dir install_dir
    for name in "${!GO_BINARIES[@]}"; do
        IFS='|' read -r module_dir build_path desc is_daemon output_dir <<< "${GO_BINARIES[$name]}"
        install_dir="$HOME/.local/bin"
        [[ -n "$output_dir" ]] && install_dir="$DOTFILES/$output_dir"
        if [[ ! -x "$install_dir/$name" ]]; then
            error "Healthcheck failed: missing Go binary '$install_dir/$name'"
            return 1
        fi
    done
    return 0
}

healthcheck_firefox() {
    if ! detect_firefox_profile; then
        return "$STEP_SKIPPED_RC"
    fi

    [[ -f "$FIREFOX_PROFILE_DIR/user.js" ]] || { error "Healthcheck failed: Firefox user.js missing"; return 1; }
    [[ -d "$FIREFOX_PROFILE_DIR/chrome" ]] || { error "Healthcheck failed: Firefox chrome/ directory missing"; return 1; }
    return 0
}

healthcheck_shell() {
    local target_user current_shell zsh_path
    target_user=$(resolve_target_user)
    zsh_path=$(command -v zsh || true)
    [[ -n "$zsh_path" ]] || { error "Healthcheck failed: zsh is not installed"; return 1; }

    current_shell=$(getent passwd "$target_user" | cut -d: -f7)
    [[ "$current_shell" == "$zsh_path" ]] || {
        error "Healthcheck failed: $target_user shell is '$current_shell' (expected '$zsh_path')"
        return 1
    }
    return 0
}

healthcheck_dns() {
    local stub="/run/systemd/resolve/stub-resolv.conf"
    local stub_real resolv_real

    stub_real=$(readlink -f "$stub" 2>/dev/null || true)
    resolv_real=$(readlink -f /etc/resolv.conf 2>/dev/null || true)

    if [[ -z "$stub_real" || -z "$resolv_real" || "$resolv_real" != "$stub_real" ]]; then
        error "Healthcheck failed: /etc/resolv.conf is not linked to $stub"
        return 1
    fi

    if ! in_chroot; then
        if ! systemctl is-active systemd-resolved &>/dev/null; then
            error "Healthcheck failed: systemd-resolved is not active"
            return 1
        fi
    fi

    return 0
}

run_step_healthcheck() {
    local step_name="$1"
    local fn="healthcheck_${step_name}"

    if ! declare -f "$fn" &>/dev/null; then
        error "Missing healthcheck function for step '$step_name' ($fn)"
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

# ========================
#  Step Dispatcher / CLI
# ========================

list_steps() {
    echo "Available steps:"
    echo
    local i=1
    for entry in "${STEP_DEFS[@]}"; do
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

print_step_banner() {
    local name="$1"
    local desc="$2"
    local pause_seconds="${PAUSE:-0}"

    printf '\n%b%s%b\n' "$BD" "===== STEP: $name =====" "$N"
    [[ -n "$desc" ]] && faint "$desc"

    if [[ "$pause_seconds" =~ ^[0-9]+$ ]] && (( pause_seconds > 0 )); then
        info "Starting in ${pause_seconds}s..."
        sleep "$pause_seconds"
    fi
}

run_step() {
    local name="$1"
    local func="step_$name"

    if ! declare -f "$func" &>/dev/null; then
        error "Unknown step: $name"
        list_steps
        return 1
    fi

    if array_contains "$name" "${PASSED[@]}" || array_contains "$name" "${FAILED[@]}" || array_contains "$name" "${SKIPPED[@]}" || array_contains "$name" "${SOFT_FAILED[@]}"; then
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

            local dep_rc=0
            run_step "$dep" || dep_rc=$?
            if [[ $dep_rc -eq "$STEP_ABORT_RC" ]]; then
                STEP_ACTIVE["$name"]=0
                return "$STEP_ABORT_RC"
            fi
            if [[ $dep_rc -ne 0 ]]; then
                warn "Skipping '$name' - dependency '$dep' failed"
                SKIPPED+=("$name")
                STEP_ACTIVE["$name"]=0
                return 0
            fi

            if array_contains "$dep" "${FAILED[@]}" || array_contains "$dep" "${SKIPPED[@]}" || array_contains "$dep" "${SOFT_FAILED[@]}"; then
                warn "Skipping '$name' - dependency '$dep' did not complete successfully"
                SKIPPED+=("$name")
                STEP_ACTIVE["$name"]=0
                return 0
            fi
        done
    fi

    print_step_banner "$name" "$(get_step_desc "$name")"
    step "Running step '$name'..."

    # Run in subshell to isolate set -e failures
    local rc=0
    local health_rc=0
    local cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}"
    local step_log_dir="$cache_dir/dotfiles/install-logs"
    local step_log=""
    mkdir -p "$step_log_dir"
    step_log="$step_log_dir/${name}.log"

    if ( "$func" ) 2>&1 | tee "$step_log"; then
        rc=0
    else
        rc=$?
    fi

    if [[ $rc -eq 0 ]]; then
        if run_step_healthcheck "$name" 2>&1 | tee -a "$step_log"; then
            health_rc=0
        else
            health_rc=$?
        fi

        if [[ $health_rc -eq "$STEP_SKIPPED_RC" ]]; then
            warn "Healthcheck for '$name' skipped"
            health_rc=0
        fi

        if [[ $health_rc -ne 0 ]]; then
            rc="$health_rc"
            error "Step '$name' failed healthcheck"
        fi
    fi

    if [[ $rc -eq 0 ]]; then
        success "Step '$name' completed"
        PASSED+=("$name")
        STEP_ACTIVE["$name"]=0
    elif [[ $rc -eq "$STEP_SKIPPED_RC" ]]; then
        SKIPPED+=("$name")
        STEP_ACTIVE["$name"]=0
    else
        if is_best_effort_mode && ! is_critical_step "$name"; then
            SOFT_FAILED+=("$name")
            STEP_ACTIVE["$name"]=0
            warn "Step '$name' failed (best-effort soft failure)"
            confirm_continue_after_failure "$name" "$step_log" "$rc" || return $?
            return 0
        fi

        FAILED+=("$name")
        STEP_ACTIVE["$name"]=0

        # Fail-fast for strict mode and all critical step failures.
        if ! is_best_effort_mode || is_critical_step "$name"; then
            warn "Step '$name' failed (aborting installation)"
            return "$STEP_ABORT_RC"
        fi

        warn "Step '$name' failed (continuing)"
        confirm_continue_after_failure "$name" "$step_log" "$rc" || return $?
        return 1
    fi
}

print_summary() {
    [[ ${#PASSED[@]} -eq 0 && ${#FAILED[@]} -eq 0 && ${#SKIPPED[@]} -eq 0 && ${#SOFT_FAILED[@]} -eq 0 ]] && return

    header "Summary"
    for name in "${PASSED[@]}"; do
        printf '  %b[OK ]%b %s\n' "$G" "$N" "$name"
    done
    for name in "${FAILED[@]}"; do
        printf '  %b[ERR]%b %s\n' "$R" "$N" "$name"
    done
    for name in "${SOFT_FAILED[@]}"; do
        printf '  %b[SOFT]%b %s (soft-failed)\n' "$Y" "$N" "$name"
    done
    for name in "${SKIPPED[@]}"; do
        printf '  %b[SKIP]%b %s (skipped)\n' "$Y" "$N" "$name"
    done
    echo

    if [[ ${#FAILED[@]} -gt 0 ]]; then
        print_finish_banner "DOTFILES INSTALL" "FAILED" "passed=${#PASSED[@]} failed=${#FAILED[@]} skipped=${#SKIPPED[@]} soft=${#SOFT_FAILED[@]}"
        warn "${#FAILED[@]} step(s) failed"
        return 1
    else
        print_finish_banner "DOTFILES INSTALL" "SUCCESS" "passed=${#PASSED[@]} failed=0 skipped=${#SKIPPED[@]} soft=${#SOFT_FAILED[@]}"
        if [[ ${#SOFT_FAILED[@]} -gt 0 ]]; then
            warn "${#SOFT_FAILED[@]} non-critical step(s) soft-failed in best-effort mode"
        fi
        finish "All steps completed successfully"
        return 0
    fi
}

run_all() {
    local rc=0
    for entry in "${STEP_DEFS[@]}"; do
        IFS='|' read -r name _ _ _ <<< "$entry"
        if is_preboot_mode; then
            case "$name" in
                repos|go|firefox)
                    warn "Preboot mode: deferring '$name' until after first login"
                    SKIPPED+=("$name")
                    continue
                    ;;
            esac
        fi
        rc=0
        run_step "$name" || rc=$?
        if [[ $rc -eq "$STEP_ABORT_RC" ]]; then
            warn "Installation halted after failure by user request"
            break
        fi
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

    local selections=()
    read -r -a selections <<< "$selection"

    for sel in "${selections[@]}"; do
        if [[ "$sel" =~ ^[0-9]+$ ]] && (( sel >= 1 && sel <= ${#STEP_DEFS[@]} )); then
            local entry="${STEP_DEFS[$((sel-1))]}"
            IFS='|' read -r name _ _ _ <<< "$entry"
            local rc=0
            run_step "$name" || rc=$?
            if [[ $rc -eq "$STEP_ABORT_RC" ]]; then
                warn "Installation halted after failure by user request"
                break
            fi
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
    echo "Arch bootstrap mode:"
    echo "  ARCH=1 ./install.sh [--auto]"
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
    echo "Env toggles:"
    echo "  DOTFILES_INSTALL_PREBOOT=1       Defer desktop/user-session steps for first login"
    echo "  DOTFILES_INSTALL_CHROOT=1        Force chroot mode when auto-detection is blocked"
    echo "  STRICT=1                         Fail immediately when a step fails (default)"
    echo "  STRICT=0                         Continue past non-critical step failures"
    echo "  PAUSE=3                          Pause N seconds before each step (default: 3)"
    echo "  ARCH=1                           Run archinstall bootstrap mode"
    echo "  SKIP=1                           Skip chroot post-install automation in ARCH mode"
    echo "  STEPS=\"...\"                     Select post-install steps in ARCH mode"
    echo "  NONINTERACTIVE=1                 Run ARCH mode post-install unattended"
    echo
    list_steps
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
        print_start_banner "post-install" "interactive-menu"
        if ! require_desktop_environment; then
            error "Aborting. Use DOTFILES_INSTALL_ALLOW_HEADLESS=1 to bypass this check."
            exit 1
        fi
        interactive_menu
        exit 0
    fi

    case "$1" in
        --list|-l)
            script_header
            list_steps
            ;;
        --help|-h)
            script_header
            usage
            ;;
        all)
            print_start_banner "post-install" "all"
            if ! require_desktop_environment; then
                error "Aborting. Use DOTFILES_INSTALL_ALLOW_HEADLESS=1 to bypass this check."
                exit 1
            fi
            run_all
            ;;
        *)
            print_start_banner "post-install" "$*"
            if ! require_desktop_environment; then
                error "Aborting. Use DOTFILES_INSTALL_ALLOW_HEADLESS=1 to bypass this check."
                exit 1
            fi
            for step in "$@"; do
                local rc=0
                run_step "$step" || rc=$?
                if [[ $rc -eq "$STEP_ABORT_RC" ]]; then
                    warn "Installation halted after failure by user request"
                    break
                fi
            done
            print_summary
            ;;
    esac
}

main "$@"
