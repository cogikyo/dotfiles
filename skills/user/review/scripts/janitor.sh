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

dirty_go_files() {
    {
        git diff --cached --name-only -- "${safe_pathspec[@]}"
        git diff --name-only -- "${safe_pathspec[@]}"
    } | grep -E '\.go$' | sort -u || true
}

dirty_changed_lines() {
    git diff --cached --no-ext-diff --unified=0 -- "${go_pathspec[@]}"
    git diff --no-ext-diff --unified=0 -- "${go_pathspec[@]}"
}

callsites() {
    local pattern=${1:-'ApplyProfile|SaveConfig|RenderRunnerProfileConfig'}

    if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        printf 'janitor callsites requires a git work tree\n' >&2
        return 2
    fi

    local files
    files=$(dirty_go_files)
    if [[ -z "$files" ]]; then
        printf 'janitor callsites found no dirty Go files\n'
        return 0
    fi

    printf '%s\n' "$files" | xargs -r grep -nE "$pattern" -- || true
}

dirty_symbols() {
    dirty_changed_lines | grep -niE '^\+[^+].*(func[[:space:]]+|type[[:space:]]+|interface[[:space:]]*\{|struct[[:space:]]*\{|\.([[:alnum:]_]+)\()' || true
}

case "${1:-help}" in
    help)
        cat <<'EOF'
janitor review helper

commands:
  callsites [regex]  scan dirty Go files for ownership/coupling callsites
  dirty-symbols      list changed Go symbols and call-looking lines
EOF
        ;;
    callsites)
        shift
        callsites "${1:-}"
        ;;
    dirty-symbols)
        dirty_symbols
        ;;
    *)
        printf 'unknown janitor command: %s\n' "$1" >&2
        exit 2
        ;;
esac
