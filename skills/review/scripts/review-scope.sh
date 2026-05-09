#!/usr/bin/env bash

set -euo pipefail

branch=""
upstream=""
base=""
dirty_count=0
staged_count=0
unstaged_count=0
ahead_count=0
inside_work_tree=0

if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    inside_work_tree=1
    branch=$(git branch --show-current 2>/dev/null || true)
    upstream=$(git rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null || true)
    dirty_count=$(git status --porcelain=v1 2>/dev/null | wc -l)
    staged_count=$(git diff --staged --name-only 2>/dev/null | wc -l)
    unstaged_count=$(git diff --name-only 2>/dev/null | wc -l)

    if [[ -n "$upstream" ]]; then
        ahead_count=$(git rev-list --count "${upstream}..HEAD" 2>/dev/null || printf '0')
    else
        for candidate in origin/main origin/master main master; do
            if git rev-parse --verify --quiet "$candidate" >/dev/null; then
                base="$candidate"
                break
            fi
        done
    fi
fi

suggestion="module"
reason="no dirty files or upstream-ahead commits detected"

if (( dirty_count > 0 && ahead_count > 0 )); then
    suggestion="ask"
    reason="dirty files and upstream-ahead commits both exist"
elif (( dirty_count > 0 )); then
    suggestion="dirty"
    reason="dirty files exist"
elif (( ahead_count > 0 )); then
    suggestion="branch"
    reason="branch has commits ahead of upstream"
fi

printf 'inside_work_tree=%s\n' "$inside_work_tree"
printf 'branch=%s\n' "${branch:-unknown}"
printf 'upstream=%s\n' "${upstream:-none}"
printf 'base=%s\n' "${base:-none}"
printf 'dirty_files=%s\n' "$dirty_count"
printf 'staged_files=%s\n' "$staged_count"
printf 'unstaged_files=%s\n' "$unstaged_count"
printf 'ahead_commits=%s\n' "$ahead_count"
printf 'suggested_scope=%s\n' "$suggestion"
printf 'reason=%s\n' "$reason"

if (( inside_work_tree == 0 )); then
    exit 0
fi

printf 'dirty_status_command=%s\n' 'git status --short'
printf 'dirty_diff_command=%s\n' 'git diff'
printf 'staged_diff_command=%s\n' 'git diff --staged'
printf 'branch_status_command=%s\n' 'git status --short --branch'
printf 'recent_log_command=%s\n' 'git log --oneline --decorate --max-count=20'

if [[ -n "$upstream" ]]; then
    printf 'branch_diff_command=%s\n' 'git diff @{upstream}...HEAD'
elif [[ -n "$base" ]]; then
    printf 'branch_diff_command=git diff %s...HEAD\n' "$base"
else
    printf 'branch_diff_command=%s\n' 'none'
fi
