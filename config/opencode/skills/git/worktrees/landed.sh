#!/usr/bin/env bash
# Usage: landed.sh <worktree> [target-ref] (default: main checkout's branch or its upstream)
# Exit 0: target contains all non-ignored worktree changes.
# Exit 1: unlanded paths or nested repositories.
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
if (($# == 2)); then
  targets=("$2")
else
  branch=$(git -C "$main" symbolic-ref --quiet --short HEAD) || refuse "main checkout is detached; pass a target ref"
  targets=("$branch")
  if up=$(git -C "$main" rev-parse --abbrev-ref "$branch@{upstream}" 2>/dev/null) &&
    git -C "$wt" rev-parse --quiet --verify "$up^{commit}" >/dev/null &&
    [[ $(git -C "$wt" rev-parse "$up^{commit}") != $(git -C "$wt" rev-parse "$branch^{commit}") ]]; then
    targets+=("$up")
  fi
fi
for target in "${targets[@]}"; do
  git -C "$wt" rev-parse --quiet --verify "$target^{commit}" >/dev/null || refuse "unknown target $target"
done

for op in rebase-merge rebase-apply MERGE_HEAD CHERRY_PICK_HEAD REVERT_HEAD BISECT_LOG; do
  [[ ! -e $(git -C "$wt" rev-parse --path-format=absolute --git-path "$op") ]] || refuse "$op in progress in $wt"
done

head=$(git -C "$wt" rev-parse --quiet --verify 'HEAD^{commit}') || refuse "$wt has no commits"
objects=$(git -C "$wt" rev-parse --path-format=absolute --git-path objects)
index=$(git -C "$wt" rev-parse --path-format=absolute --git-path index)

tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
mkdir "$tmp/objects"
export GIT_INDEX_FILE=$tmp/index GIT_OBJECT_DIRECTORY=$tmp/objects GIT_ALTERNATE_OBJECT_DIRECTORIES=$objects
if [[ -e $index ]]; then
  cp -- "$index" "$GIT_INDEX_FILE"
else
  git -C "$wt" read-tree "$head"
fi

# Refuse stderr too: Git can warn and omit unreadable paths without failing.
list() {
  local out=$tmp/$1
  shift
  git -C "$wt" "$@" >"$out" 2>"$out.err" && [[ ! -s $out.err ]] || refuse "git $*: $(<"$out.err")"
}

list modes ls-files -z --format='%(objectmode)'
list tracked ls-tree -r -z --format='%(objectmode)' "$head"
modules=$(git -C "$wt" rev-parse --path-format=absolute --git-path modules)
[[ ! -e $wt/.gitmodules && ! -e $modules ]] || refuse "$wt has submodules"
! grep -qzx 160000 "$tmp/modes" "$tmp/tracked" || refuse "$wt has submodules"
staged=$(git -C "$wt" write-tree) || refuse "cannot write the index tree of $wt"

# The worktree may live under main, so exclude its subtree.
overlay() {
  local real
  real=$(realpath -m -- "$1")
  [[ $real == "$main"/* && $real != "$wt" && $real != "$wt"/* ]]
}

nested=()
: >"$tmp/paths"
: >"$tmp/flags"

list changed status -z --porcelain=v1 --no-renames --untracked-files=all --ignored=no
while IFS= read -r -d '' line; do
  path=${line:3}
  if [[ $path == */ ]]; then
    nested+=("$path (nested repository)")
  elif [[ ${line:0:2} == '??' && -L $wt/$path ]] && overlay "$wt/$path"; then
    continue
  elif [[ ${line:1:1} != ' ' ]]; then
    printf '%s\0' "$path" >>"$tmp/paths"
  fi
done <"$tmp/changed"

list flagged ls-files -z -v
while IFS= read -r -d '' line; do
  path=${line:2}
  [[ ${line:0:1} == [a-zS] ]] || continue
  [[ ${line:0:1} == [Ss] && ! -e $wt/$path && ! -L $wt/$path ]] && continue
  printf '%s\0' "$path" >>"$tmp/paths"
  printf '%s\0' "$path" >>"$tmp/flags"
done <"$tmp/flagged"

list unflag update-index --no-assume-unchanged -z --stdin <"$tmp/flags"
list unskip update-index --no-skip-worktree -z --stdin <"$tmp/flags"
list update update-index --add --remove -z --stdin <"$tmp/paths"
tree=$(git -C "$wt" write-tree) || refuse "cannot write the worktree's tree"
snapshot() {
  GIT_AUTHOR_NAME=landed GIT_AUTHOR_EMAIL=landed@localhost GIT_COMMITTER_NAME=landed GIT_COMMITTER_EMAIL=landed@localhost \
    git -C "$wt" commit-tree --no-gpg-sign -p "$head" -m landed "$1" || refuse "cannot snapshot $wt"
}
states=(committed "$head")
[[ $staged == "$(git -C "$wt" rev-parse "$head^{tree}")" ]] || states+=(staged "$(snapshot "$staged")")
[[ $tree == "$staged" ]] || states+=(uncommitted "$(snapshot "$tree")")

check() {
  local target=$1 label=$2 commit=$3 status=0 result path
  git -C "$wt" merge-tree --write-tree -z --name-only --no-messages "$target" "$commit" >"$tmp/merge" 2>"$tmp/merge.err" || status=$?
  ((status <= 1)) || refuse "git merge-tree $target: $(<"$tmp/merge.err")"
  { IFS= read -r -d '' result && mapfile -d '' -t conflicts; } <"$tmp/merge"
  ((status == 0)) && [[ $result == "$(git -C "$wt" rev-parse "$target^{tree}")" ]] && return
  while IFS= read -r -d '' path; do
    [[ -n $path && -z ${seen[$path]-} ]] || continue
    seen[$path]=1
    missing+=("$path ($label)")
  done < <(
    git -C "$wt" diff-tree -r -z --name-only --no-renames "$target" "$result"
    printf '%s\0' "${conflicts[@]}"
  )
  return 1
}

passed=
unique=()
declare -A seen
for target in "${targets[@]}"; do
  missing=()
  seen=()
  pass=1
  for ((i = 0; i < ${#states[@]}; i += 2)); do
    check "$target" "${states[i]}" "${states[i + 1]}" || pass=
  done
  if [[ -n $pass ]]; then
    passed=$target
    unique=()
    break
  fi
  if [[ $target == "${targets[0]}" ]] || ((${#missing[@]} < ${#unique[@]})); then
    unique=("${missing[@]}")
  fi
done
unique=("${nested[@]}" "${unique[@]}")

if [[ -z $passed ]] || ((${#unique[@]})); then
  printf 'unique: %s\n' "${unique[@]}"
  exit 1
fi
echo "landed: $wt holds nothing $passed lacks"
