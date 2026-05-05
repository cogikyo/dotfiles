#!/usr/bin/env bash

set -euo pipefail

go_test() {
    local -a args=("$@")
    local output status

    set +e
    output=$(go test "${args[@]}" 2>&1)
    status=$?
    set -e

    printf '%s\n' "$output"

    if (( status == 0 )); then
        return 0
    fi

    if [[ "$output" == *'directory prefix '*" does not contain modules listed in go.work"* ]]; then
        printf 'go test hit parent go.work prefix error; retrying once with GOWORK=off\n' >&2
        GOWORK=off go test "${args[@]}"
        return $?
    fi

    return "$status"
}

case "${1:-help}" in
    help)
        cat <<'EOF'
debugger review helper

commands:
  go-test [go test args...]  run go test; retry once with GOWORK=off on parent go.work prefix errors
EOF
        ;;
    go-test)
        shift
        go_test "$@"
        ;;
    *)
        printf 'unknown debugger command: %s\n' "$1" >&2
        exit 2
        ;;
esac
