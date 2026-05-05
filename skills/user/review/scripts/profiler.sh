#!/usr/bin/env bash

set -euo pipefail

dirty_changed_lines() {
    git diff --cached --no-ext-diff --unified=0
    git diff --no-ext-diff --unified=0
}

dirty_scan() {
    local pattern='RunStreamingEnv|docker[[:space:]]+compose|\.View\([[:space:]]*\)'

    if dirty_changed_lines | grep -niE "^[-+][^-+].*($pattern)"; then
        printf 'profiler dirty-scan found performance-sensitive patterns\n' >&2
        return 1
    fi

    printf 'profiler dirty-scan found no performance-sensitive patterns\n'
}

case "${1:-help}" in
    help)
        cat <<'EOF'
profiler review helper

commands:
  dirty-scan  read-only scan of staged and unstaged diffs for performance-sensitive patterns

patterns:
  RunStreamingEnv, docker compose, changed View() callsites
EOF
        ;;
    dirty-scan)
        dirty_scan
        ;;
    *)
        printf 'unknown profiler command: %s\n' "$1" >&2
        exit 2
        ;;
esac
