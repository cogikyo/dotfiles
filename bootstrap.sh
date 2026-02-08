#!/usr/bin/env bash
# bootstrap - Single-command entrypoint for dotfiles installation scripts
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash -s -- arch
#   curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash -s -- auto
#   curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash -s -- auto --download-only

set -euo pipefail

BOOTSTRAP_VERSION="2026.02.08.1"
REPO_RAW_ROOT="${DOTFILES_RAW_ROOT:-https://raw.githubusercontent.com/cogikyo/dotfiles}"
DEFAULT_REF="${DOTFILES_REF:-master}"

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
Usage: bootstrap.sh [MODE] [OPTIONS] [-- SCRIPT_ARGS...]

Modes:
  auto       root => archinstall.sh, non-root => install.sh (default)
  arch       Download + verify + run archinstall.sh
  install    Download + verify + run install.sh
  help       Show this help

Options:
  --download-only, --save-only
             Download + verify only (do not execute)
  -o, --output PATH
             Save path for download-only mode (default: ./SCRIPT_NAME)
  -r, --ref REF
             Git ref to download from (branch, tag, or commit SHA). Default: master
  --cache-bust TOKEN
             Append ?v=TOKEN to download URLs to bypass caches
  --version  Show bootstrap version and exit
  -h, --help Show this help

Examples:
  curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash
  curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash -s -- auto
  curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash -s -- install -- all
  curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash -s -- auto --download-only -o /tmp/archinstall.sh
  curl -fsSL https://raw.githubusercontent.com/cogikyo/dotfiles/master/bootstrap.sh | bash -s -- arch --ref master --cache-bust "$(date +%s)"
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

download_target_script() {
    local repo_raw="$1" target_script="$2" workdir="$3" cache_bust="$4"
    local sums_file target_file
    local sums_url target_url

    sums_file="$workdir/SHA256SUMS"
    target_file="$workdir/$target_script"

    sums_url="$repo_raw/SHA256SUMS"
    target_url="$repo_raw/$target_script"
    if [[ -n "$cache_bust" ]]; then
        sums_url="${sums_url}?v=${cache_bust}"
        target_url="${target_url}?v=${cache_bust}"
    fi

    info "Downloading checksums from $repo_raw ..."
    curl -fsSL -H "Cache-Control: no-cache" -H "Pragma: no-cache" "$sums_url" -o "$sums_file"

    info "Downloading $target_script..."
    curl -fsSL -H "Cache-Control: no-cache" -H "Pragma: no-cache" "$target_url" -o "$target_file"

    verify_file_checksum "$sums_file" "$target_file"
    ok "$target_script checksum verified"
}

save_target_script() {
    local source_path="$1" output_path="$2"
    local run_cmd

    mkdir -p "$(dirname "$output_path")"
    cp -f "$source_path" "$output_path"
    chmod +x "$output_path"

    ok "Saved script: $output_path"
    if [[ "$output_path" == */* || "$output_path" == ./* || "$output_path" == /* ]]; then
        run_cmd="$output_path"
    else
        run_cmd="./$output_path"
    fi
    info "Run next: $run_cmd"
}

main() {
    require_bin bash
    require_bin curl
    require_bin sha256sum
    require_bin mktemp
    require_bin awk

    local mode="auto"
    local download_only="0"
    local output_path=""
    local ref="$DEFAULT_REF"
    local cache_bust=""
    local workdir target_script target_file
    local repo_raw
    local -a script_args=()

    if [[ $# -gt 0 && "${1#-}" == "$1" ]]; then
        mode="$1"
        shift
    fi

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --download-only|--save-only)
                download_only="1"
                ;;
            -o|--output)
                shift
                [[ $# -gt 0 ]] || die "Missing path for --output"
                output_path="$1"
                ;;
            -r|--ref)
                shift
                [[ $# -gt 0 ]] || die "Missing value for --ref"
                ref="$1"
                ;;
            --cache-bust)
                shift
                [[ $# -gt 0 ]] || die "Missing value for --cache-bust"
                cache_bust="$1"
                ;;
            --version)
                printf '%s\n' "$BOOTSTRAP_VERSION"
                return 0
                ;;
            -h|--help)
                usage
                return 0
                ;;
            --)
                shift
                script_args+=("$@")
                break
                ;;
            *)
                script_args+=("$1")
                ;;
        esac
        shift
    done

    case "$mode" in
        auto)
            if [[ $EUID -eq 0 ]]; then
                target_script="archinstall.sh"
            else
                target_script="install.sh"
                if [[ ${#script_args[@]} -eq 0 ]]; then
                    script_args=(all)
                fi
            fi
            ;;
        install)
            target_script="install.sh"
            if [[ ${#script_args[@]} -eq 0 ]]; then
                script_args=(all)
            fi
            ;;
        arch|archinstall)
            target_script="archinstall.sh"
            ;;
        help|-h|--help)
            usage
            return 0
            ;;
        *)
            warn "Unknown mode: $mode"
            usage
            exit 1
            ;;
    esac

    if [[ "$download_only" != "1" && -n "$output_path" ]]; then
        die "--output is only valid with --download-only/--save-only"
    fi

    if [[ -n "${DOTFILES_RAW_BASE:-}" ]]; then
        repo_raw="$DOTFILES_RAW_BASE"
    else
        repo_raw="$REPO_RAW_ROOT/$ref"
    fi

    workdir=$(mktemp -d)
    trap 'rm -rf "${workdir:-}"' EXIT
    target_file="$workdir/$target_script"

    download_target_script "$repo_raw" "$target_script" "$workdir" "$cache_bust"

    if [[ "$download_only" == "1" ]]; then
        output_path="${output_path:-./$target_script}"
        save_target_script "$target_file" "$output_path"
        return 0
    fi

    info "Running $target_script..."
    if [[ ! -t 0 && -r /dev/tty ]]; then
        exec bash "$target_file" "${script_args[@]}" < /dev/tty
    fi
    exec bash "$target_file" "${script_args[@]}"
}

main "$@"
