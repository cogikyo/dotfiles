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

ARCHINSTALL_VERSION="2026.02.09.2"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_CONFIG="$SCRIPT_DIR/etc/arch.json"
TMP_SOURCE_CONFIG="/tmp/arch_source_config.json"
CONFIG="/tmp/arch_config.json"
DOTFILES_REF="${DOTFILES_REF:-master}"
DOTFILES_RAW_BASE="${DOTFILES_RAW_BASE:-https://raw.githubusercontent.com/cogikyo/dotfiles/$DOTFILES_REF}"

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
EOF
}

# Print version banner before any early validation errors.
header

[[ $EUID -eq 0 ]] || die "Run as root from the live ISO"
command -v python3 >/dev/null || die "python3 is required"
command -v archinstall >/dev/null || die "archinstall is required"
command -v curl >/dev/null || die "curl is required"

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

finish "Installation complete!"
echo
info "Next steps:"
step "1. Reboot into the new system"
step "2. Log in as your configured user"
step "3. Clone dotfiles and run: ./install.sh all"
echo
