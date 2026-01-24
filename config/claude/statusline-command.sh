#!/bin/bash

# ─────────────────────────────────────────────────
# Modifiers & Helpers
# ─────────────────────────────────────────────────
BOLD='\033[1m'
N='\033[0m'

cprint() { printf '%b%s%b' "$1" "$2" "$N"; }

# ─────────────────────────────────────────────────
# Colors
# ─────────────────────────────────────────────────
# Blues
RED='\033[31m'
GREEN='\033[32m'
YELLOW='\033[33m'
BLUE='\033[34m'
CYAN='\033[36m'
MAGENTA='\033[35m'
GRAY='\033[90m'

BR_RED='\033[91m'
BR_GREEN='\033[92m'
BR_YELLOW='\033[93m'
BR_BLUE='\033[94m'
PINK='\033[95m'


SEP='╼╾'

# Git
GIT_BRANCH=' ⽀'
GIT_AHEAD='⮭'
GIT_BEHIND='⮯'
GIT_STAGED=''
GIT_MODIFIED=' '
GIT_UNTRACKED=' '
GIT_DELETED='󰚃 '
GIT_STASHED='󰸧 '
GIT_RENAMED='󰑕 '
GIT_CONFLICT=' '

# Progress bar
BAR_FILLED='◉'
BAR_EMPTY='○'
BAR_CONTEXT='㊋'
BAR_PROGRESS=("󰪞" "󰪟" "󰪠" "󰪡" "󰪢" "󰪣" "󰪤" "󰪥" "󰪦" "󰪧")

# Session
ICON_MODEL='󰯉 '

# ─────────────────────────────────────────────────
# Functions
# ─────────────────────────────────────────────────

# Get progress bar icon based on percentage (0-100)
get_progress_icon() {
    local pct=$1
    local idx=$((pct / 10))
    (( idx = idx > 9 ? 9 : (idx < 0 ? 0 : idx) ))
    echo "${BAR_PROGRESS[$idx]}"
}

pct_color() {
    local pct=${1%.*}
    if   (( pct < 10 )); then echo "$BLUE"
    elif (( pct < 20 )); then echo "$BR_BLUE"
    elif (( pct < 30 )); then echo "$GREEN"
    elif (( pct < 40 )); then echo "$BR_GREEN"
    elif (( pct < 50 )); then echo "$BR_YELLOW"
    elif (( pct < 60 )); then echo "$YELLOW"
    elif (( pct < 70 )); then echo "$RED"
    elif (( pct < 90 )); then echo "$BR_RED"
    else echo "$PINK"; fi
}

format_date() {
    local iso=$1 fmt=$2
    [[ -z "$iso" ]] && return 1
    if [[ "$OSTYPE" == darwin* ]]; then
        date -j -f "%Y-%m-%dT%H:%M:%S%z" "${iso%%.*}+0000" "+$fmt" 2>/dev/null
    else
        date -d "$iso" "+$fmt" 2>/dev/null
    fi
}

# Git command wrapper with common flags
gitc() { git -C "$current_dir" --no-optional-locks "$@" 2>/dev/null; }

# Count lines from piped input
count_lines() { wc -l | tr -d ' '; }

# Append to git_status if count > 0
git_stat() {
    local color=$1 icon=$2 count=$3
    (( count > 0 )) && git_status+="${color}${icon}${count} ${N}"
}

# Get credentials file path (Linux)
get_creds_file() {
    local xdg="${XDG_CONFIG_HOME:-$HOME/.config}/claude/.credentials.json"
    [[ -f "$xdg" ]] && echo "$xdg" || echo "$HOME/.claude/.credentials.json"
}

# Fetch usage limits from API
get_usage_limits() {
    local creds token
    if [[ "$OSTYPE" == darwin* ]]; then
        creds=$(security find-generic-password -s "Claude Code-credentials" -w 2>/dev/null) || return 1
    else
        creds=$(cat "$(get_creds_file)" 2>/dev/null) || return 1
    fi
    token=$(jq -r '.claudeAiOauth.accessToken // empty' <<< "$creds") || return 1
    [[ -z "$token" ]] && return 1
    curl -s --max-time 2 \
        -H "Authorization: Bearer $token" \
        -H "anthropic-beta: oauth-2025-04-20" \
        "https://api.anthropic.com/api/oauth/usage" 2>/dev/null
}

# Build progress bar string
build_bar() {
    local filled=$1 total=${2:-10} bar=""
    for ((i=0; i<filled; i++)); do bar+="$BAR_FILLED"; done
    for ((i=filled; i<total; i++)); do bar+="$BAR_EMPTY"; done
    echo "$bar"
}

# ─────────────────────────────────────────────────
# Main
# ─────────────────────────────────────────────────
input=$(cat)
current_dir=$(jq -r '.workspace.current_dir' <<< "$input")
context_data=$(jq '.context_window' <<< "$input")

# Git info
git_info="" git_status=""
if git -C "$current_dir" rev-parse --git-dir &>/dev/null; then
    branch=$(gitc branch --show-current)
    [[ -n "$branch" ]] && git_info="${YELLOW}${GIT_BRANCH}${branch}${N}"

    # shellcheck disable=SC1083
    ahead=$(gitc rev-list --count '@{upstream}..HEAD' || echo 0)
    behind=$(gitc rev-list --count 'HEAD..@{upstream}' || echo 0)
    git_stat "$GREEN"     "$GIT_AHEAD"   "$ahead"
    git_stat "$BR_RED"    "$GIT_BEHIND"  "$behind"
    git_stat "$YELLOW"    "$GIT_STAGED"  "$(gitc diff --cached --numstat | count_lines)"
    git_stat "$CYAN"      "$GIT_MODIFIED" "$(gitc diff --numstat | count_lines)"
    git_stat "$BR_YELLOW" "$GIT_UNTRACKED" "$(gitc ls-files --others --exclude-standard | count_lines)"
    git_stat "$RED"       "$GIT_DELETED" "$(gitc diff --diff-filter=D --numstat | count_lines)"
    git_stat "$GRAY"      "$GIT_STASHED" "$(gitc stash list | count_lines)"
    git_stat "$MAGENTA"   "$GIT_RENAMED" "$(gitc diff --cached --diff-filter=R --numstat | count_lines)"
    git_stat "$PINK"      "$GIT_CONFLICT" "$(gitc diff --name-only --diff-filter=U | count_lines)"
fi

# Context progress bar
context_bar=""
usage=$(jq '.current_usage' <<< "$context_data")
if [[ "$usage" != "null" ]]; then
    current=$(jq '.input_tokens + .cache_creation_input_tokens + .cache_read_input_tokens' <<< "$usage")
    size=$(jq '.context_window_size' <<< "$context_data")
    if [[ "$current" != "null" && "$size" != "null" && "$size" -gt 0 ]]; then
        pct=$((current * 100 / size))
        context_bar="$(pct_color "$pct")${BAR_CONTEXT}$(build_bar $((pct / 10))) ${pct}%${N}"
    fi
fi

# Usage limits
usage_info=""
if usage_response=$(get_usage_limits); then
    five_hr=$(jq -r '.five_hour.utilization // empty' <<< "$usage_response")
    five_hr_reset=$(jq -r '.five_hour.resets_at // empty' <<< "$usage_response")
    seven_day=$(jq -r '.seven_day.utilization // empty' <<< "$usage_response")
    seven_day_reset=$(jq -r '.seven_day.resets_at // empty' <<< "$usage_response")

    if [[ -n "$five_hr" ]]; then
        hr_color=$(pct_color "$five_hr")
        hr_icon=$(get_progress_icon "${five_hr%.*}")
        hr_reset=$(format_date "$five_hr_reset" "%-I%P")
        [[ -n "$hr_reset" ]] && hr_reset=" (${hr_reset})"
        usage_info+="${hr_color}${hr_icon} ${five_hr%.*}%${hr_reset}${N}"
    fi

    if [[ -n "$seven_day" ]]; then
        [[ -n "$usage_info" ]] && usage_info+="${BLUE} ${SEP} ${N}"
        day_color=$(pct_color "$seven_day")
        day_icon=$(get_progress_icon "${seven_day%.*}")
        day_reset=$(format_date "$seven_day_reset" "%-m/%-d")
        [[ -n "$day_reset" ]] && day_reset=" (${day_reset})"
        usage_info+="${day_color}${day_icon} ${seven_day%.*}%${day_reset}${N}"
    fi
fi

# ─────────────────────────────────────────────────
# Output (buffered to prevent partial output leaking)
# ─────────────────────────────────────────────────
out="${BLUE}${ICON_MODEL}${BOLD}${BLUE}${current_dir/#$HOME\//}${N}"
[[ -n "$git_info" ]]    && out+="$git_info"
[[ -n "$git_status" ]]  && out+=" $git_status"
[[ -n "$context_bar" ]] && out+="${BR_BLUE}${SEP} ${N}${context_bar} "
[[ -n "$usage_info" ]]  && out+="${BR_BLUE}${SEP} ${N}${usage_info}"
# Output line (Claude Code handles positioning)
printf '%b\033[0m' "$out"
