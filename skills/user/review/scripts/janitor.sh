#!/usr/bin/env bash

set -euo pipefail

case "${1:-help}" in
    help)
        printf 'janitor review helper: no commands implemented yet\n'
        ;;
    *)
        printf 'unknown janitor command: %s\n' "$1" >&2
        exit 2
        ;;
esac
