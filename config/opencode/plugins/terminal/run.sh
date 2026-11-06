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
session=$3
reason=$4
command=$5

case $PWD in
  "$HOME") where=\~ ;;
  "$HOME"/*) where=\~$(printf '%q' "${PWD#"$HOME"}") ;;
  *) printf -v where '%q' "$PWD" ;;
esac

wrapped=$(fmt -w 72 <<<"$reason")
ticks=${reason//[^\`]/}
if ((${#ticks} % 2 == 0)); then
  wrapped=$(awk -v on=$'\e[36m' -v off=$'\e[39m' '{
    line = code ? on : ""
    n = split($0, part, "`")
    for (i = 1; i <= n; i++) {
      if (i > 1) { code = !code; line = line (code ? on : off) }
      line = line part[i]
    }
    print line off
  }' <<<"$wrapped")
fi

highlighted=$(bat --no-config --language=bash --theme=ansi --style=plain --color=always --paging=never <<<"$command" 2>/dev/null) ||
  highlighted=$command

printf '\e]2;%s\a\n \e[1;30;41m %s \e[0m\n\n' "$session" "$session"
while IFS= read -r line; do printf '   %s\n' "$line"; done <<<"$wrapped"
printf '\n   \e[2m%s\e[0m\n' "$where"
prompt='$'
while IFS= read -r line; do
  printf '   \e[2m%s\e[0m %s\n' "$prompt" "$line"
  prompt=' '
done <<<"$highlighted"
printf -v rule '%76s' ''
printf '\e[2m%s\e[0m\n' "${rule// /─}"

inner=$(printf '%q ' "$BASH" "${BASH_SOURCE[0]}" --inner "$dir" "$shell" "$command")
set +e
SHELL=$BASH script --quiet --return --flush --command "$inner" "$dir/output.log" 2>"$dir/runner.err"
status=$?
set -e

if [[ -e $dir/result ]]; then
  record "$dir" "exit $status"
  if ((status == 0)); then color=32 glyph=✓; else color=31 glyph=✗; fi
  message="exit $status"
else
  record "$dir" "failed $status"
  cat "$dir/runner.err" >&2
  color=31 glyph=✗
  message="the runner failed with status $status before the command started"
fi
printf '\n \e[%sm%s %s\e[0m \e[2m· Enter to close\e[0m\n' "$color" "$glyph" "$message"
while read -r -s -t 0.1 -n 4096; do :; done
read -r -s || true
