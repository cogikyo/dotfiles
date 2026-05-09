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

dirty_scan() {
    local pattern='0\.0\.0\.0|http\.Serve|POST[[:space:]]+/|extractTar|HasPrefix\([[:space:]]*filepath\.Clean|tar[[:alnum:]_]*\.NewReader|\.\./|filepath\.Join|RemoveAll|os\.Rename|docker[[:space:]]+compose|COMPOSE_PROJECT_NAME|ReadPassword|Authorization|token|--drop-[[:alnum:]-]+|drop[[:space:]_-]*(database|schema|table|all)'

    if dirty_changed_lines | grep -niE "^[-+][^-+].*($pattern)"; then
        printf 'auditor dirty-scan found high-blast-radius patterns\n' >&2
        return 1
    fi

    printf 'auditor dirty-scan found no high-blast-radius patterns\n'
}

case "${1:-help}" in
    help)
        cat <<'EOF'
auditor review helper

commands:
  dirty-scan  read-only scan of staged and unstaged diffs for high-blast-radius patterns

patterns:
  bind-all hosts, http serving, POST routes, tar extraction, path traversal, destructive drops, secrets, broad filesystem/process operations
EOF
        ;;
    dirty-scan)
        dirty_scan
        ;;
    *)
        printf 'unknown auditor command: %s\n' "$1" >&2
        exit 2
        ;;
esac
