#!/usr/bin/env bash

set -euo pipefail

safe_pathspec=(
    .
    ':!configs/local.gcfg'
    ':!**/.env'
    ':!**/.env.*'
    ':!**/*credential*'
    ':!**/*secret*'
    ':!**/*key*'
    ':!**/*.pem'
    ':!**/*.key'
    ':!**/runtime/*config*'
)

dirty_files() {
    git diff --cached --name-only -- "${safe_pathspec[@]}"
    git diff --name-only -- "${safe_pathspec[@]}"
}

scope() {
    if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        printf 'architect scope requires a git work tree\n' >&2
        return 2
    fi

    local files seams
    files=$(dirty_files | sort -u)

    if [[ -z "$files" ]]; then
        printf 'dirty_files=0\n'
        return 0
    fi

    printf 'dirty_files:\n'
    printf '%s\n' "$files"

    seams=$(printf '%s\n' "$files" | grep -Ei '(^|/)(workspace|state|process|runner|runtime|config|storage|transport|api|cmd|internal|pkg)(/|$)' || true)
    if [[ -n "$seams" ]]; then
        printf '\ncross_boundary_candidates:\n'
        printf '%s\n' "$seams"
    fi
}

case "${1:-help}" in
    help)
        cat <<'EOF'
architect review helper

commands:
  scope  print dirty files and likely cross-boundary seam changes without reading secret paths
EOF
        ;;
    scope)
        scope
        ;;
    *)
        printf 'unknown architect command: %s\n' "$1" >&2
        exit 2
        ;;
esac
