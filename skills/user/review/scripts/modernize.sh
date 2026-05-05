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

dirty_changed_lines() {
    git diff --cached --no-ext-diff --unified=0 -- "${safe_pathspec[@]}"
    git diff --no-ext-diff --unified=0 -- "${safe_pathspec[@]}"
}

legacy_scan() {
    local pattern='local\.gcfg|deprecated|legacy|compat(ibility)?|fallback|alias|hidden|TODO[^\n]*(legacy|compat|deprecated)|cobra\.Command|Use:[[:space:]]*"[^"]*(hidden|deprecated)|Deprecated:|Aliases:|\.Deprecated|\.Hidden'

    if dirty_changed_lines | grep -niE "^[-+][^-+].*($pattern)"; then
        printf 'modernize legacy-scan found legacy/compatibility patterns\n' >&2
        return 1
    fi

    printf 'modernize legacy-scan found no legacy/compatibility patterns\n'
}

case "${1:-help}" in
    help)
        cat <<'EOF'
modernize review helper

commands:
  legacy-scan  read-only scan of dirty diffs for legacy, deprecated, hidden, alias, and compatibility fallback paths
EOF
        ;;
    legacy-scan)
        legacy_scan
        ;;
    *)
        printf 'unknown modernize command: %s\n' "$1" >&2
        exit 2
        ;;
esac
