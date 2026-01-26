#!/usr/bin/env bash
# cmd/install.sh - Build and install all Go binaries from dotfiles
#
# Usage:
#   ./cmd/install.sh           # Build all binaries
#   ./cmd/install.sh hyprd     # Build specific binary
#   ./cmd/install.sh ewwd      # Build specific binary
#   ./cmd/install.sh --list    # List available binaries

set -euo pipefail

DOTFILES="${DOTFILES:-$HOME/dotfiles}"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

info() { printf "${BLUE}==>${NC} %s\n" "$*"; }
success() { printf "${GREEN}==>${NC} %s\n" "$*"; }
warn() { printf "${YELLOW}==>${NC} %s\n" "$*"; }
error() { printf "${RED}==>${NC} %s\n" "$*" >&2; }

# Binary definitions: name|source_dir|description
declare -A BINARIES=(
    ["hyprd"]="cmd/hyprd|Hyprland window management daemon"
    ["ewwd"]="cmd/ewwd|System utilities daemon for eww"
    ["statusline"]="config/claude/statusline|Claude Code statusline generator"
    ["newtab"]="share/newtab|Firefox new tab HTTP server"
)

list_binaries() {
    echo "Available binaries:"
    echo
    for name in "${!BINARIES[@]}"; do
        IFS='|' read -r src_dir desc <<< "${BINARIES[$name]}"
        printf "  %-12s %s\n" "$name" "$desc"
    done | sort
}

build_binary() {
    local name="$1"

    if [[ ! -v BINARIES[$name] ]]; then
        error "Unknown binary: $name"
        echo "Run with --list to see available binaries"
        return 1
    fi

    IFS='|' read -r src_dir desc <<< "${BINARIES[$name]}"
    local full_path="$DOTFILES/$src_dir"

    if [[ ! -d "$full_path" ]]; then
        error "Source directory not found: $full_path"
        return 1
    fi

    if [[ ! -f "$full_path/go.mod" ]]; then
        error "No go.mod found in $full_path"
        return 1
    fi

    info "Building $name from $src_dir"
    (
        cd "$full_path"
        go build -o "$INSTALL_DIR/$name" .
    )
    success "Installed $name → $INSTALL_DIR/$name"
}

build_all() {
    local failed=0
    local built=0

    for name in "${!BINARIES[@]}"; do
        if build_binary "$name"; then
            ((built++))
        else
            ((failed++))
        fi
    done

    echo
    if [[ $failed -eq 0 ]]; then
        success "All $built binaries installed successfully"
    else
        warn "$built succeeded, $failed failed"
        return 1
    fi
}

main() {
    mkdir -p "$INSTALL_DIR"

    # Ensure INSTALL_DIR is in PATH
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        warn "Note: $INSTALL_DIR is not in your PATH"
    fi

    case "${1:-all}" in
        all)
            build_all
            ;;
        --list|-l)
            list_binaries
            ;;
        --help|-h)
            echo "Usage: $0 [binary|--list|--help]"
            echo
            list_binaries
            ;;
        *)
            build_binary "$1"
            ;;
    esac
}

main "$@"
