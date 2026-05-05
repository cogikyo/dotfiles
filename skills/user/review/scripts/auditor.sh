#!/usr/bin/env bash

set -euo pipefail

dirty_changed_lines() {
    git diff --cached --no-ext-diff --unified=0
    git diff --no-ext-diff --unified=0
}

dirty_scan() {
    local pattern='RemoveAll|os\.Rename|docker[[:space:]]+compose|COMPOSE_PROJECT_NAME|ReadPassword|Authorization|token'

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
  RemoveAll, os.Rename, docker compose, COMPOSE_PROJECT_NAME, ReadPassword, Authorization, token
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
