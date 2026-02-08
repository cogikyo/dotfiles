#!/usr/bin/env bash
# archinstall - Automated Arch Linux installation from live ISO
#
# Usage (as root on live ISO):
#   curl -sL raw.githubusercontent.com/cogikyo/dotfiles/master/archinstall.sh | bash
#
# Pulls configuration from this repo, prompts for password, detects disk,
# patches config, and runs archinstall.
# After reboot, clone dotfiles and run ./install.sh for post-install setup.

set -euo pipefail

REPO_RAW="https://raw.githubusercontent.com/cogikyo/dotfiles/master"
CONFIG="/tmp/arch_config.json"
CREDS="/tmp/arch_creds.json"
PASSFILE="/tmp/arch_password.$$"

trap 'rm -f "$CONFIG" "$CREDS" "$PASSFILE"' EXIT

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

# Keep generated creds/config private in /tmp.
umask 077

# ── Download config ───────────────────────────────────────────────────────────

info "Downloading configuration..."
curl -fsSL "$REPO_RAW/etc/arch.json" -o "$CONFIG"
success "Config downloaded"

# ── Detect disk ───────────────────────────────────────────────────────────────

echo
info "Available disks:"
lsblk -dpno NAME,SIZE,MODEL | grep -v loop
echo

mapfile -t disks < <(lsblk -dpno NAME | grep -v loop)

if [[ ${#disks[@]} -eq 1 ]]; then
    disk="${disks[0]}"
    info "Auto-selected: $disk"
else
    echo "Select target disk:"
    select disk in "${disks[@]}"; do
        [[ -n "$disk" ]] && break
    done
fi

warn "ALL DATA on $disk will be erased"
read -rp "Continue? [y/N] " yn
[[ "$yn" =~ ^[Yy] ]] || die "Aborted"

# ── Prompt for hostname, username, and password ──────────────────────────────

echo
read -rp "Hostname: " hostname
[[ -n "$hostname" ]] || die "Hostname cannot be empty"

read -rp "Username [cullyn]: " username
username="${username:-cullyn}"

echo
read -rsp "Password for $username (+ root): " password
echo
read -rsp "Confirm: " password_confirm
echo

[[ "$password" == "$password_confirm" ]] || die "Passwords do not match"
[[ -n "$password" ]] || die "Password cannot be empty"

# ── Disk size ────────────────────────────────────────────────────────────────

echo
info "Btrfs partition sizing (boot partition uses 1 GiB)"
read -rp "Use full remaining disk? [Y/n] or enter size in GiB: " disk_size
disk_size="${disk_size:-y}"

if [[ ! "$disk_size" =~ ^[Yy]([Ee][Ss])?$ ]] && [[ ! "$disk_size" =~ ^[0-9]+\.?[0-9]*$ ]]; then
    die "Invalid disk size: '$disk_size' (enter 'y' or a number in GiB)"
fi

# ── Patch config and create credentials ───────────────────────────────────────

info "Preparing config for $disk..."

printf '%s\n' "$password" > "$PASSFILE"

python3 - "$CONFIG" "$CREDS" "$PASSFILE" "$hostname" "$username" "$disk" "$disk_size" << 'PYEOF'
import json
import sys

config_path, creds_path, passfile_path, hostname, username, disk, disk_size = sys.argv[1:8]
with open(passfile_path) as f:
    password = f.read().rstrip("\n")

with open(config_path) as f:
    config = json.load(f)

config["hostname"] = hostname

for mod in config["disk_config"]["device_modifications"]:
    mod["device"] = disk

# Custom GiB size: patch the btrfs partition (second partition)
if not disk_size.lower().startswith("y"):
    gib = float(disk_size)
    btrfs_part = config["disk_config"]["device_modifications"][0]["partitions"][1]
    btrfs_part["size"] = {
        "sector_size": {"unit": "B", "value": 512},
        "unit": "GiB",
        "value": gib,
    }

with open(config_path, "w") as f:
    json.dump(config, f, indent=4)

creds = {
    "!root-password": password,
    "!users": [
        {
            "username": username,
            "!password": password,
            "sudo": True,
        }
    ],
}

with open(creds_path, "w") as f:
    json.dump(creds, f, indent=4)
PYEOF

chmod 600 "$CREDS"
unset password password_confirm

success "Config patched for $disk"
success "Credentials ready"

# ── Run archinstall ───────────────────────────────────────────────────────────

info "Starting archinstall..."
echo

archinstall --config "$CONFIG" --creds "$CREDS" --silent

# ── Done ──────────────────────────────────────────────────────────────────────

finish "Installation complete!"
echo
info "Next steps:"
step "1. Reboot into the new system"
step "2. Log in as $username"
step "3. Clone dotfiles:  git clone https://github.com/cogikyo/dotfiles ~/dotfiles"
step "4. Run post-install: cd ~/dotfiles && ./install.sh"
echo
