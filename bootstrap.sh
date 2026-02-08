#!/usr/bin/env bash
# bootstrap - Single-command entrypoint for dotfiles installation scripts
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash -s -- arch
#   curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash -s -- install all
#   curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash -s -- auto

set -euo pipefail

REPO_RAW="${DOTFILES_RAW_BASE:-https://raw.githubusercontent.com/cogikyo/dotfiles/master}"

R='\033[0;31m'
G='\033[0;32m'
Y='\033[1;33m'
B='\033[1;34m'
N='\033[0m'

info()  { printf '%b(↓)%b %s\n' "$B" "$N" "$*"; }
warn()  { printf '\n%b(!) %s%b\n' "$Y" "$*" "$N"; }
ok()    { printf '%b(✓)%b %s\n' "$G" "$N" "$*"; }
die()   { printf '%b(✗)%b %s\n' "$R" "$N" "$*" >&2; exit 1; }

usage() {
    cat <<'EOF'
Usage: bootstrap.sh [MODE] [ARGS...]

Modes:
  arch       Download + verify + run archinstall.sh
  install    Download + verify + run install.sh (defaults to "all" if no args)
  auto       arch when root, install when non-root (default)
  help       Show this help

Examples:
  curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash -s -- arch
  curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash -s -- install all
EOF
}

require_bin() {
    command -v "$1" >/dev/null || die "Missing required command: $1"
}

verify_file_checksum() {
    local sums_file="$1" file_path="$2" file_name expected actual
    file_name=$(basename "$file_path")
    expected=$(awk -v f="$file_name" '$2==f {print $1}' "$sums_file")
    [[ -n "$expected" ]] || die "$file_name entry not found in SHA256SUMS"

    actual=$(sha256sum "$file_path" | awk '{print $1}')
    [[ "$actual" == "$expected" ]] || die "$file_name checksum mismatch: expected $expected, got $actual"
}

download_and_run() {
    local target_script="$1"
    shift

    local workdir sums_file target_file
    workdir=$(mktemp -d)
    trap 'rm -rf "$workdir"' EXIT

    sums_file="$workdir/SHA256SUMS"
    target_file="$workdir/$target_script"

    info "Downloading checksums..."
    curl -fsSL "$REPO_RAW/SHA256SUMS" -o "$sums_file"

    info "Downloading $target_script..."
    curl -fsSL "$REPO_RAW/$target_script" -o "$target_file"

    verify_file_checksum "$sums_file" "$target_file"
    ok "$target_script checksum verified"

    exec bash "$target_file" "$@"
}

main() {
    require_bin bash
    require_bin curl
    require_bin sha256sum
    require_bin mktemp
    require_bin awk

    local mode="${1:-auto}"
    if [[ $# -gt 0 ]]; then
        shift
    fi

    case "$mode" in
        arch|archinstall)
            download_and_run "archinstall.sh" "$@"
            ;;
        install)
            if [[ $# -eq 0 ]]; then
                set -- all
            fi
            download_and_run "install.sh" "$@"
            ;;
        auto)
            if [[ $EUID -eq 0 ]]; then
                download_and_run "archinstall.sh" "$@"
            else
                if [[ $# -eq 0 ]]; then
                    set -- all
                fi
                download_and_run "install.sh" "$@"
            fi
            ;;
        help|-h|--help)
            usage
            ;;
        *)
            warn "Unknown mode: $mode"
            usage
            exit 1
            ;;
    esac
}

main "$@"
