#!/usr/bin/env bash
# archinstall - Automated Arch Linux installation from live ISO
#
# Usage (as root on live ISO):
#   curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash -s -- arch
#   ./archinstall.sh
#
# Pulls configuration from this repo, prompts for password, detects disk,
# patches config, and runs archinstall.
# After reboot, run the post-install script (it can bootstrap dotfiles automatically).

set -euo pipefail

REPO_RAW="https://raw.githubusercontent.com/cogikyo/dotfiles/master"
SHA256SUMS_URL="$REPO_RAW/SHA256SUMS"
CONFIG="/tmp/arch_config.json"
CREDS="/tmp/arch_creds.json"
PASSFILE="/tmp/arch_password.$$"
ARCH_JSON_SHA256="7c27924a0d22d7e5588e27ab7da32706022e24328c01e555579dcc36fd83ff20"

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
command -v sha256sum >/dev/null || die "sha256sum is required"
command -v curl >/dev/null || die "curl is required"

# Keep generated creds/config private in /tmp.
umask 077

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

info "Downloading configuration..."
curl -fsSL "$REPO_RAW/etc/arch.json" -o "$CONFIG"

actual_sha256=$(sha256sum "$CONFIG" | awk '{print $1}')
if [[ "$actual_sha256" != "$ARCH_JSON_SHA256" ]]; then
    die "arch.json checksum mismatch: expected $ARCH_JSON_SHA256, got $actual_sha256"
fi
success "Config downloaded and verified"

# ── Detect disk ───────────────────────────────────────────────────────────────

echo
info "Available disks:"
lsblk -dpno NAME,SIZE,MODEL | grep -v loop
echo

mapfile -t disks < <(lsblk -dpno NAME | grep -v loop)
disk=""

if [[ ${#disks[@]} -eq 0 ]]; then
    die "No target disks detected"
elif [[ ${#disks[@]} -eq 1 ]]; then
    disk="${disks[0]}"
    info "Auto-selected: $disk"
else
    echo "Select target disk:"
    select disk in "${disks[@]}"; do
        [[ -n "${disk:-}" ]] && break
    done
fi

[[ -n "${disk:-}" ]] || die "No disk selected (input closed before a selection was made)"

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
while :; do
    read -rsp "Password for $username (+ root): " password
    echo
    read -rsp "Confirm: " password_confirm
    echo

    if [[ -z "$password" ]]; then
        warn "Password cannot be empty. Try again."
        continue
    fi

    if [[ "$password" != "$password_confirm" ]]; then
        warn "Passwords do not match. Try again."
        continue
    fi

    break
done

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


def load_size_model():
    try:
        from archinstall.lib.models.device import Size  # type: ignore

        return Size
    except Exception:
        pass

    try:
        from archinstall.lib.models.device_model import Size  # type: ignore

        return Size
    except Exception:
        return None


def is_valid_size_arg(size_model, size_arg):
    if size_model is None:
        return True

    try:
        size_model.parse_arg(size_arg)
        return True
    except Exception:
        return False


def resolve_percent_unit(size_model):
    probe = {"sector_size": None, "value": 100}
    for candidate in ("Percent", "percent", "PERCENT", "Percentage", "percentage", "PERCENTAGE", "%"):
        probe["unit"] = candidate
        if is_valid_size_arg(size_model, probe):
            return candidate
    return None

config["hostname"] = hostname

for mod in config["disk_config"]["device_modifications"]:
    mod["device"] = disk

device_mod = config["disk_config"]["device_modifications"][0]
btrfs_part = device_mod["partitions"][1]
size_model = load_size_model()

# Full-disk installs: normalize legacy "Percent" unit to current archinstall unit name.
if disk_size.lower().startswith("y"):
    size_arg = btrfs_part.get("size")
    if isinstance(size_arg, dict) and not is_valid_size_arg(size_model, size_arg):
        percent_unit = resolve_percent_unit(size_model)
        if percent_unit is not None:
            size_arg["unit"] = percent_unit
            size_arg["value"] = 100
            size_arg["sector_size"] = None
        # Fallback for incompatible schema changes: omit explicit size for final partition.
        if not is_valid_size_arg(size_model, size_arg):
            btrfs_part.pop("size", None)

# Custom GiB size: patch the btrfs partition (second partition).
else:
    btrfs_part["size"] = {
        "sector_size": {"unit": "B", "value": 512},
        "unit": "GiB",
        "value": float(disk_size),
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
step "3. Run post-install: curl -fsSL $REPO_RAW/bootstrap.sh | bash -s -- auto"
echo
