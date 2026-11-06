#!/usr/bin/env bash
set -euo pipefail

record() {
  printf '%s\n' "$2" >"$1/result.tmp"
  mv "$1/result.tmp" "$1/result"
}

if [[ $1 == --inner ]]; then
  record "$2" started
  SHELL=$3 exec "$3" -c "$4"
fi

dir=$1
shell=$2
reason=$3
command=$4

printf '\e[1m%s\e[0m\n\e[2min %q\e[0m\n\n$ %s\n\n' "$reason" "$PWD" "$command"

inner=$(printf '%q ' "$BASH" "${BASH_SOURCE[0]}" --inner "$dir" "$shell" "$command")
set +e
SHELL=$BASH script --quiet --return --flush --command "$inner" "$dir/output.log" 2>"$dir/runner.err"
status=$?
set -e

if [[ -e $dir/result ]]; then
  record "$dir" "exit $status"
  color=$((status == 0 ? 32 : 31))
  message="exit status $status"
else
  record "$dir" "failed $status"
  cat "$dir/runner.err" >&2
  color=31
  message="the runner failed with status $status before the command started"
fi
printf '\n\e[%sm%s\e[0m; press Enter to close\n' "$color" "$message"
while read -r -s -t 0.1 -n 4096; do :; done
read -r -s || true
