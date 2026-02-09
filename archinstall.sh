#!/usr/bin/env bash
# archinstall - Automated Arch Linux installation from live ISO
#
# Usage (as root on live ISO):
#   bash <(curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh) arch
#   ./archinstall.sh            # partial config + interactive archinstall (default)
#   ./archinstall.sh --auto     # same as default (explicit)
#
# Pulls partial configuration from this repo and runs non-silent archinstall.
# Disk partitioning and user authentication are completed in the archinstall UI.
# After reboot, run the post-install script (it can bootstrap dotfiles automatically).

set -euo pipefail

REPO_RAW="https://raw.githubusercontent.com/cogikyo/dotfiles/master"
SHA256SUMS_URL="$REPO_RAW/SHA256SUMS"
CONFIG="/tmp/arch_config.json"
ARCH_JSON_SHA256="db6644f33d47515285ec948528713345cf9eba73d24df633ec8146f9278f233d"

trap 'rm -f "$CONFIG"' EXIT

R='\033[0;31m'
G='\033[0;32m'
Y='\033[1;33m'
B='\033[1;34m'
M='\033[0;35m'
F='\033[90m'
N='\033[0m'

info()    { printf '%b(↓)%b %s\n' "$B" "$N" "$*"; }
step()    { printf '%b(→)%b %s\n' "$B" "$N" "$*"; }
success() { printf '%b(✓)%b %s\n' "$G" "$N" "$*"; }
finish()  { printf '\n%b(✓✓) %s%b\n' "$G" "$*" "$N"; }
warn()    { printf '\n%b(!) %s%b\n' "$Y" "$*" "$N"; }
error()   { printf '%b(✗)%b %s\n' "$R" "$N" "$*" >&2; }
die()     { printf '%b(✗)%b %s\n' "$R" "$N" "$*" >&2; exit 1; }
ask()     { printf '%b(?)%b %s\n' "$Y" "$N" "$*"; }
header()  { printf '\n%b━━━ %s ━━━%b\n\n' "$M" "$*" "$N"; }
faint()   { printf '%b%s%b\n' "$F" "$*" "$N"; }

[[ $EUID -eq 0 ]] || die "Run as root from the live ISO"
command -v python3 >/dev/null || die "python3 is required"
command -v archinstall >/dev/null || die "archinstall is required"
command -v sha256sum >/dev/null || die "sha256sum is required"
command -v curl >/dev/null || die "curl is required"

# Keep generated creds/config private in /tmp.
umask 077

firmware_mode="bios"
if [[ -d /sys/firmware/efi ]]; then
    firmware_mode="uefi"
fi
info "Detected firmware mode: $firmware_mode"

usage() {
    cat <<'EOF'
Usage: archinstall.sh [--auto]

Modes:
  --auto     Run archinstall with partial repo config (default behavior)
EOF
}

if [[ $# -gt 0 ]]; then
    case "$1" in
        --auto)
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        --guided)
            die "--guided has been removed. Use default/--auto."
            ;;
        *)
            die "Unknown argument: $1 (use --auto)"
            ;;
    esac
fi
[[ $# -eq 0 ]] || die "Unexpected extra arguments: $*"

refresh_archinstall_tooling() {
    if [[ "${DOTFILES_SKIP_ARCHINSTALL_UPDATE:-0}" == "1" ]]; then
        warn "Skipping archinstall/keyring refresh (DOTFILES_SKIP_ARCHINSTALL_UPDATE=1)"
        return 0
    fi

    info "Refreshing archinstall + archlinux-keyring from repos..."
    if pacman -Sy --noconfirm archlinux-keyring archinstall >/dev/null 2>&1; then
        success "archinstall tooling refreshed"
    else
        warn "Failed to refresh archinstall tooling; continuing with ISO-provided versions"
    fi
}

verify_self_checksum() {
    if [[ "${DOTFILES_SKIP_SELF_VERIFY:-0}" == "1" ]]; then
        warn "Skipping script checksum verification (DOTFILES_SKIP_SELF_VERIFY=1)"
        return 0
    fi

    local script_path script_name expected actual tmp_sums
    script_path=$(readlink -f -- "${BASH_SOURCE[0]:-$0}" 2>/dev/null || printf '%s' "${BASH_SOURCE[0]:-$0}")
    script_name=$(basename "$script_path")

    if [[ "$script_name" != "archinstall.sh" ]]; then
        warn "Skipping script checksum verification (unexpected script name: $script_name)"
        warn "Save as archinstall.sh to enable automatic self-verification"
        return 0
    fi

    tmp_sums=$(mktemp)
    curl -fsSL "$SHA256SUMS_URL" -o "$tmp_sums" || die "Failed to download SHA256SUMS from $SHA256SUMS_URL"
    expected=$(awk '$2=="archinstall.sh" {print $1}' "$tmp_sums")
    rm -f "$tmp_sums"

    [[ -n "$expected" ]] || die "archinstall.sh entry not found in SHA256SUMS"

    actual=$(sha256sum "$script_path" | awk '{print $1}')
    if [[ "$actual" != "$expected" ]]; then
        die "archinstall.sh checksum mismatch: expected $expected, got $actual"
    fi

    success "Script checksum verified"
}

# ── Download config ───────────────────────────────────────────────────────────

verify_self_checksum

refresh_archinstall_tooling

run_archinstall() {
    if ! archinstall "$@"; then
        error "archinstall failed"
        warn "Recent archinstall log output:"
        if [[ -f /var/log/archinstall/install.log ]]; then
            tail -n 120 /var/log/archinstall/install.log || true
        else
            warn "No /var/log/archinstall/install.log found"
        fi
        exit 1
    fi
}

info "Downloading configuration..."
curl -fsSL "$REPO_RAW/etc/arch.json" -o "$CONFIG"

actual_sha256=$(sha256sum "$CONFIG" | awk '{print $1}')
if [[ "$actual_sha256" != "$ARCH_JSON_SHA256" ]]; then
    die "arch.json checksum mismatch: expected $ARCH_JSON_SHA256, got $actual_sha256"
fi
success "Config downloaded and verified"

# ── Patch partial config ──────────────────────────────────────────────────────

info "Preparing partial config..."

python3 - "$CONFIG" "$firmware_mode" << 'PYEOF'
import json
import sys

config_path, firmware_mode = sys.argv[1:3]

with open(config_path) as f:
    config = json.load(f)

# Let archinstall collect these interactively in the UI.
config.pop("disk_config", None)
config.pop("auth_config", None)

# Avoid a known archinstall hang/failure path around keymap application
# ("Setting keyboard language to us") on some ISO versions.
locale_cfg = config.get("locale_config")
if isinstance(locale_cfg, dict):
    locale_cfg.pop("kb_layout", None)

# Systemd-boot is UEFI-only; use Grub in BIOS/legacy mode.
bootloader_cfg = config.setdefault("bootloader_config", {})
if firmware_mode == "bios":
    bootloader_cfg["bootloader"] = "Grub"
else:
    bootloader_cfg["bootloader"] = "Systemd-boot"

with open(config_path, "w") as f:
    json.dump(config, f, indent=4)
PYEOF

success "Partial config ready"

# ── Run archinstall ───────────────────────────────────────────────────────────

info "Starting archinstall..."
echo

warn "Disk partitioning and user authentication will be set in archinstall UI"
run_archinstall --config "$CONFIG"

# ── Done ──────────────────────────────────────────────────────────────────────

finish "Installation complete!"
echo
info "Next steps:"
step "1. Reboot into the new system"
step "2. Log in as your configured user"
step "3. Run post-install: bash <(curl -fsSL $REPO_RAW/bootstrap.sh) install -- all"
echo
