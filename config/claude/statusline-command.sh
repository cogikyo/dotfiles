#!/bin/bash

# Vagari palette colors (RGB)
blu_2='\033[38;2;116;146;239m'   # #7492ef - separators
blu_3='\033[1;38;2;138;164;243m' # #8aa4f3 - directory (bold)
orn_2='\033[38;2;235;144;93m'    # #eb905d - git branch
orn_3='\033[38;2;242;161;112m'   # #f2a170 - staged
grn_3='\033[38;2;149;203;121m'   # #95cb79 - ahead, green
rby_3='\033[38;2;240;136;152m'   # #f08898 - behind, deleted, red
sky_2='\033[38;2;107;189;236m'   # #6bbdec - modified
sun_3='\033[38;2;244;206;136m'   # #f4ce88 - untracked, yellow
slt_2='\033[38;2;72;78;117m'     # #484e75 - stashed
prp_3='\033[38;2;178;154;232m'   # #b29ae8 - renamed
pnk_3='\033[38;2;232;135;195m'   # #e887c3 - conflicted
N='\033[0m'

# Nerd font icons (from starship config, using hex escapes for reliability)
icon_conflicted=$'\xef\x91\xbf'   #
icon_stashed=$'\xf3\xb0\xb8\xa7'  # 󰸧
icon_modified=$'\xee\xab\x9e'     #
icon_staged=$'\xef\x90\x97'       #
icon_untracked=$'\xee\x8d\xb0'    #
icon_renamed=$'\xef\x81\x84'      #
icon_ahead=$'\xe2\xac\x86'        # ⬆
icon_behind=$'\xe2\xac\x87'       # ⬇
icon_deleted=$'\xef\x91\x98'      #
icon_branch=$'\xe2\xbd\x80'       # ⽀
icon_model=$'\xf3\xb0\xaf\x89'    # 󰯉

# Read JSON input
input=$(cat)

# Extract values
current_dir=$(echo "$input" | jq -r '.workspace.current_dir')
model_name=$(echo "$input" | jq -r '.model.display_name')
context_data=$(echo "$input" | jq '.context_window')

# Get directory (replace home with ⾕)
dir="${current_dir/#$HOME/⾕}"

# Git info
git_info=""
git_status=""
if git -C "$current_dir" rev-parse --git-dir > /dev/null 2>&1; then
    branch=$(git -C "$current_dir" --no-optional-locks branch --show-current 2>/dev/null)
    if [ -n "$branch" ]; then
        git_info="${orn_2} ${icon_branch}${branch}${N}"
    fi

    # Git status indicators (starship style)
    # Ahead/behind
    ahead=$(git -C "$current_dir" --no-optional-locks rev-list --count @{upstream}..HEAD 2>/dev/null || echo 0)
    behind=$(git -C "$current_dir" --no-optional-locks rev-list --count HEAD..@{upstream} 2>/dev/null || echo 0)
    [ "$ahead" -gt 0 ] 2>/dev/null && git_status+="${grn_3}${icon_ahead}${ahead} ${N}"
    [ "$behind" -gt 0 ] 2>/dev/null && git_status+="${rby_3}${icon_behind}${behind} ${N}"

    # Staged
    staged=$(git -C "$current_dir" --no-optional-locks diff --cached --numstat 2>/dev/null | wc -l | tr -d ' ')
    [ "$staged" -gt 0 ] && git_status+="${orn_3}${icon_staged} ${staged} ${N}"

    # Modified
    modified=$(git -C "$current_dir" --no-optional-locks diff --numstat 2>/dev/null | wc -l | tr -d ' ')
    [ "$modified" -gt 0 ] && git_status+="${sky_2}${icon_modified} ${modified} ${N}"

    # Untracked
    untracked=$(git -C "$current_dir" --no-optional-locks ls-files --others --exclude-standard 2>/dev/null | wc -l | tr -d ' ')
    [ "$untracked" -gt 0 ] && git_status+="${sun_3}${icon_untracked} ${untracked} ${N}"

    # Deleted
    deleted=$(git -C "$current_dir" --no-optional-locks diff --diff-filter=D --numstat 2>/dev/null | wc -l | tr -d ' ')
    [ "$deleted" -gt 0 ] && git_status+="${rby_3}${icon_deleted} ${deleted} ${N}"

    # Stashed
    stashed=$(git -C "$current_dir" --no-optional-locks stash list 2>/dev/null | wc -l | tr -d ' ')
    [ "$stashed" -gt 0 ] && git_status+="${slt_2}${icon_stashed} ${stashed} ${N}"
fi

# Build context progress bar
context_bar=""
usage=$(echo "$context_data" | jq '.current_usage')
if [ "$usage" != "null" ]; then
    current=$(echo "$usage" | jq '.input_tokens + .cache_creation_input_tokens + .cache_read_input_tokens')
    size=$(echo "$context_data" | jq '.context_window_size')
    if [ "$current" != "null" ] && [ "$size" != "null" ] && [ "$size" -gt 0 ]; then
        pct=$((current * 100 / size))

        filled=$((pct / 10))
        empty=$((10 - filled))

        # Color: blue 0-15%, green 15-30%, yellow 30-60%, red 60%+
        if [ "$pct" -lt 15 ]; then
            bar_color="$blu_2"
        elif [ "$pct" -lt 30 ]; then
            bar_color="$grn_3"
        elif [ "$pct" -lt 60 ]; then
            bar_color="$sun_3"
        else
            bar_color="$rby_3"
        fi

        bar=""
        for ((i=0; i<filled; i++)); do bar+="▰"; done
        for ((i=0; i<empty; i++)); do bar+="▱"; done

        context_bar="${bar_color}㊋ ${bar} ${pct}%${N}"
    fi
fi

# Build status line
printf "${blu_2}╞╾${N}"
printf "${blu_3}%s${N}" "$dir"

if [ -n "$git_info" ]; then
    printf "%b" "$git_info"
fi

if [ -n "$git_status" ]; then
    printf " %b" "$git_status"
fi

printf "${blu_2} ╼╾ ${N}"
printf "${blu_2}${icon_model} %s${N}" "$model_name"

if [ -n "$context_bar" ]; then
    printf "${blu_2} ╼╾ ${N}%b" "$context_bar"
fi

# Two newlines for spacing
printf "\n\n"
