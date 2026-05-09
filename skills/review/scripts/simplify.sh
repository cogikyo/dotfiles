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

go_pathspec=(
    '*.go'
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

changed_lines() {
    {
        git diff --cached --numstat -- "${safe_pathspec[@]}"
        git diff --numstat -- "${safe_pathspec[@]}"
    } | awk '{ add[$3]+=$1; del[$3]+=$2 } END { for (file in add) printf "%d\t+%d\t-%d\t%s\n", add[file]+del[file], add[file], del[file], file }' | sort -nr
}

large_go() {
    changed_lines | awk '$4 ~ /\.go$/ { print }'
}

hotspots() {
    {
        git diff --cached --no-ext-diff --unified=0 -- "${go_pathspec[@]}"
        git diff --no-ext-diff --unified=0 -- "${go_pathspec[@]}"
    } | grep -niE '^(@@|\+[^+].*(func[[:space:]]+|if[[:space:]]+|for[[:space:]]+|switch[[:space:]]+|select[[:space:]]+|go[[:space:]]+|defer[[:space:]]+))' || true
}

case "${1:-help}" in
    help)
        cat <<'EOF'
simplify review helper

commands:
  changed-lines  print dirty files sorted by changed lines
  large-go       print changed Go files sorted by changed lines
  hotspots       print changed Go function/control-flow hotspots
EOF
        ;;
    changed-lines)
        changed_lines
        ;;
    large-go)
        large_go
        ;;
    hotspots)
        hotspots
        ;;
    *)
        printf 'unknown simplify command: %s\n' "$1" >&2
        exit 2
        ;;
esac
