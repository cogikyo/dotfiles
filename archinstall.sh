#!/usr/bin/env bash
# archinstall - Local partial-config launcher
#
# Usage:
#   ./archinstall.sh
#   ./archinstall.sh --auto
#
# This script only prepares a partial config from ./etc/arch.json and launches
# non-silent archinstall. Disk partitioning and authentication are done in the UI.

set -euo pipefail

ARCHINSTALL_VERSION="2026.02.15.2"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_CONFIG="$SCRIPT_DIR/etc/arch.json"
TMP_SOURCE_CONFIG="/tmp/arch_source_config.json"
CONFIG="/tmp/arch_config.json"
DOTFILES_REF="${DOTFILES_REF:-master}"
DOTFILES_REPO="${DOTFILES_REPO:-https://github.com/cogikyo/dotfiles.git}"
DOTFILES_RAW_BASE="${DOTFILES_RAW_BASE:-https://raw.githubusercontent.com/cogikyo/dotfiles/$DOTFILES_REF}"
DOTFILES_TARGET_ROOT="${DOTFILES_TARGET_ROOT:-/mnt}"
DOTFILES_TARGET_USER="${DOTFILES_TARGET_USER:-}"
SKIP="${SKIP:-0}"
STEPS="${STEPS:-all}"
STRICT="${STRICT:-0}"
NONINTERACTIVE="${NONINTERACTIVE:-0}"
PAUSE="${PAUSE:-3}"

trap 'rm -f "$CONFIG" "$TMP_SOURCE_CONFIG"' EXIT

R='\033[0;31m'
G='\033[0;32m'
Y='\033[1;33m'
B='\033[1;34m'
N='\033[0m'

info()    { printf '%b(↓)%b %s\n' "$B" "$N" "$*"; }
step()    { printf '%b(→)%b %s\n' "$B" "$N" "$*"; }
success() { printf '%b(✓)%b %s\n' "$G" "$N" "$*"; }
finish()  { printf '\n%b(✓✓) %s%b\n' "$G" "$*" "$N"; }
warn()    { printf '\n%b(!) %s%b\n' "$Y" "$*" "$N"; }
error()   { printf '%b(✗)%b %s\n' "$R" "$N" "$*" >&2; }
die()     { printf '%b(✗)%b %s\n' "$R" "$N" "$*" >&2; exit 1; }
header()  { printf '%b== dotfiles archinstall v%s ==%b\n' "$B" "$ARCHINSTALL_VERSION" "$N"; }

usage() {
    cat <<'EOF'
Usage: archinstall.sh [--auto]

Notes:
  --auto is accepted for compatibility, but default behavior is already auto.
  Set SKIP=1 to skip chroot post-install automation.
  Set STEPS to run a subset (default: all).
  Set STRICT=1 for strict fail-fast post-install.
  Set PAUSE=3 to pause N seconds before each install step.
  Set NONINTERACTIVE=1 for unattended post-install.
EOF
}

# Print version banner before any early validation errors.
header

if [[ $# -gt 0 ]]; then
    case "$1" in
        --auto)
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            die "Unknown argument: $1"
            ;;
    esac
fi
[[ $# -eq 0 ]] || die "Unexpected extra arguments: $*"

[[ $EUID -eq 0 ]] || die "Run as root from the live ISO"
command -v python3 >/dev/null || die "python3 is required"
command -v archinstall >/dev/null || die "archinstall is required"
command -v curl >/dev/null || die "curl is required"

firmware_mode="bios"
if [[ -d /sys/firmware/efi ]]; then
    firmware_mode="uefi"
fi
info "Detected firmware mode: $firmware_mode"

if [[ -f "$SOURCE_CONFIG" ]]; then
    info "Loading local config: $SOURCE_CONFIG"
    cp -f "$SOURCE_CONFIG" "$CONFIG"
else
    info "Local config not found at $SOURCE_CONFIG"
    info "Downloading config: $DOTFILES_RAW_BASE/etc/arch.json"
    curl -fsSL "$DOTFILES_RAW_BASE/etc/arch.json" -o "$TMP_SOURCE_CONFIG" || die "Failed to download etc/arch.json"
    cp -f "$TMP_SOURCE_CONFIG" "$CONFIG"
fi

info "Preparing partial config..."
python3 - "$CONFIG" "$firmware_mode" << 'PYEOF'
import json
import sys

config_path, firmware_mode = sys.argv[1:3]

with open(config_path) as f:
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

with open(config_path, "w") as f:
    json.dump(config, f, indent=4)
PYEOF
success "Partial config ready: $CONFIG"

info "Starting archinstall..."
warn "Set disk partitioning and authentication in the archinstall UI"
if ! archinstall --config "$CONFIG"; then
    error "archinstall failed"
    warn "Recent archinstall log output:"
    if [[ -f /var/log/archinstall/install.log ]]; then
        tail -n 120 /var/log/archinstall/install.log || true
    else
        warn "No /var/log/archinstall/install.log found"
    fi
    exit 1
fi

run_post_install() {
    local target_root="$DOTFILES_TARGET_ROOT"
    local target_user="$DOTFILES_TARGET_USER"

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
        "$STEPS" \
        "$STRICT" \
        "$NONINTERACTIVE" \
        "$PAUSE" << 'CHROOT_EOF'
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
        DOTFILES="$dotfiles_dir" \
        DOTFILES_REF="$dotfiles_ref" \
        DOTFILES_INSTALL_TARGET_USER="$target_user" \
        DOTFILES_INSTALL_ALLOW_HEADLESS=1 \
        DOTFILES_INSTALL_NONINTERACTIVE="$install_noninteractive" \
        DOTFILES_INSTALL_PREBOOT=1 \
        STRICT="$install_strict" \
        PAUSE="$install_pause" \
        "$dotfiles_dir/install.sh" "${step_args[@]}" < /dev/tty
else
    echo "No interactive TTY detected; running without secrets decrypt prompt" >&2
    sudo -H -u "$target_user" env \
        DOTFILES="$dotfiles_dir" \
        DOTFILES_REF="$dotfiles_ref" \
        DOTFILES_INSTALL_TARGET_USER="$target_user" \
        DOTFILES_INSTALL_ALLOW_HEADLESS=1 \
        DOTFILES_INSTALL_NONINTERACTIVE=1 \
        DOTFILES_INSTALL_PREBOOT=1 \
        STRICT="$install_strict" \
        PAUSE="$install_pause" \
        DOTFILES_SKIP_SECRETS=1 \
        "$dotfiles_dir/install.sh" "${step_args[@]}"
fi
CHROOT_EOF
}

if [[ "$SKIP" == "1" ]]; then
    warn "Skipping post-install automation (SKIP=1)"
else
    if ! run_post_install; then
        error "Post-install automation failed"
        warn "You can reboot and run ~/dotfiles/install.sh all manually"
        exit 1
    fi
fi

finish "Installation complete!"
echo
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
