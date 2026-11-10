#!/usr/bin/env bash
# Usage: landed.sh <worktree> [target-ref]
# Exit 0: removing the worktree and its branch loses nothing the target lacks.
# Exit 1: prints each path whose content exists only in the worktree.
# Exit 2: refused.
set -euo pipefail
export GIT_LITERAL_PATHSPECS=1

refuse() {
  echo "refused: $*" >&2
  exit 2
}

[[ $# -ge 1 && $# -le 2 ]] || refuse "usage: landed.sh <worktree> [target-ref]"
wt=$(git -C "$1" rev-parse --show-toplevel) || refuse "$1 is not a worktree"
main=$(git -C "$wt" worktree list --porcelain | sed -n '1s/^worktree //p')
[[ $wt != "$main" ]] || refuse "$wt is the main checkout"
target=${2:-$(git -C "$main" symbolic-ref --quiet --short HEAD)} || refuse "main checkout is detached; pass a target ref"
git -C "$wt" rev-parse --quiet --verify "$target^{commit}" >/dev/null || refuse "unknown target $target"
[[ ! -e $wt/.gitmodules ]] || refuse "$wt has submodules"

for op in rebase-merge rebase-apply MERGE_HEAD CHERRY_PICK_HEAD REVERT_HEAD BISECT_LOG; do
  [[ ! -e $(git -C "$wt" rev-parse --path-format=absolute --git-path "$op") ]] || refuse "$op in progress in $wt"
done

tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT

# Refuse stderr too: Git can warn and omit unreadable paths without failing.
list() {
  local out=$tmp/$1
  shift
  git -C "$wt" "$@" >"$out" 2>"$out.err" && [[ ! -s $out.err ]] || refuse "git $*: $(<"$out.err")"
}

entry() {
  if [[ -z $1 ]]; then
    git -C "$wt" ls-files --format='%(objectmode) %(objectname)' -- "$2"
  else
    git -C "$wt" ls-tree --format='%(objectmode) %(objectname)' "$1" -- "$2"
  fi
}

# Strip only readlink's final newline; the target can contain newlines.
link() { readlink -- "$1" | head -c -1 | git -C "$wt" hash-object --stdin; }

# The worktree may live under main, so exclude its subtree.
overlay() {
  local real
  real=$(realpath -m -- "$1")
  [[ $real == "$main"/* && $real != "$wt" && $real != "$wt"/* ]]
}

here() {
  local path=$wt/$1 oid
  if [[ -L $path ]]; then
    oid=$(link "$path") || return 1
    echo "120000 $oid"
  elif [[ -d $path ]]; then
    echo directory
  elif [[ -e $path ]]; then
    [[ -f $path && -r $path ]] || return 1
    oid=$(git -C "$wt" hash-object -- "$1") || return 1
    if [[ -x $path ]]; then echo "100755 $oid"; else echo "100644 $oid"; fi
  fi
}

unique=()

# Accept unchanged branch content when the target has advanced since checkout.
check() {
  local code=$1 path=$2 want head index got
  if [[ $code == '??' && -L $wt/$path ]] && overlay "$wt/$path"; then return 0; fi
  want=$(entry "$target" "$path")
  head=$(entry HEAD "$path")
  index=$(entry "" "$path")
  [[ $index == "$head" || $index == "$want" ]] || { unique+=("$path (staged)") && return; }
  if [[ $code == [Ss] && ! -e $wt/$path && ! -L $wt/$path ]]; then
    got=$index
  else
    got=$(here "$path") || refuse "cannot read $wt/$path"
  fi
  [[ $got == "$index" || $got == "$want" ]] || unique+=("$path")
}

kept() {
  local copy=$main/$1
  overlay "$copy" && cmp -s -- "$wt/$1" "$copy" || return 1
  if [[ -x $wt/$1 ]]; then [[ -x $copy ]]; else [[ ! -x $copy ]]; fi
}

list committed diff -z --name-only --no-renames "$(git -C "$wt" merge-base HEAD "$target")" HEAD
while IFS= read -r -d '' path; do
  [[ $(entry HEAD "$path") == "$(entry "$target" "$path")" ]] || unique+=("$path (committed)")
done <"$tmp/committed"

list changed status -z --porcelain=v1 --no-renames --untracked-files=all --ignored=no
while IFS= read -r -d '' line; do
  path=${line:3}
  if [[ $path == */ ]]; then unique+=("$path (nested repository)"); else check "${line:0:2}" "$path"; fi
done <"$tmp/changed"

list flagged ls-files -z -v
while IFS= read -r -d '' line; do
  if [[ ${line:0:1} == [a-zS] ]]; then check "${line:0:1}" "${line:2}"; fi
done <"$tmp/flagged"

list ignored status -z --porcelain=v1 --ignored=traditional --untracked-files=all
while IFS= read -r -d '' line; do
  [[ ${line:0:2} == '!!' ]] || continue
  path=${line:3}
  if [[ $path == */ ]]; then
    unique+=("$path (nested repository)")
  elif [[ -L $wt/$path ]]; then
    overlay "$wt/$path" || [[ -L $main/$path && $(link "$wt/$path") == "$(link "$main/$path")" ]] ||
      unique+=("$path (ignored)")
  else
    kept "$path" || unique+=("$path (ignored)")
  fi
done <"$tmp/ignored"

if ((${#unique[@]})); then
  printf 'unique: %s\n' "${unique[@]}"
  exit 1
fi
echo "landed: $wt holds nothing $target lacks"
