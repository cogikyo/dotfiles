#!/usr/bin/env bash
# Collect git activity across all LeadPier repos for a given date
# Usage: git-activity.sh YYYY-MM-DD [author]
set -euo pipefail

DATE="${1:?Usage: git-activity.sh YYYY-MM-DD [author]}"
AUTHOR="${2:-cullyn}"

# Next day for --before boundary (macOS date)
NEXT_DAY=$(date -j -v+1d -f "%Y-%m-%d" "$DATE" "+%Y-%m-%d" 2>/dev/null) || {
    # fallback for GNU date
    NEXT_DAY=$(date -d "$DATE + 1 day" "+%Y-%m-%d")
}

find "$HOME/LeadPier" -maxdepth 3 -name ".git" -type d | sort | while read -r git_dir; do
    repo="$(dirname "$git_dir")"
    rel="${repo#"$HOME"/LeadPier/}"

    log=$(cd "$repo" && git log \
        --author="$AUTHOR" \
        --after="${DATE}T00:00:00" \
        --before="${NEXT_DAY}T00:00:00" \
        --format="%n%h %s%n%b" \
        --stat --all 2>/dev/null) || true

    [ -z "$log" ] && continue

    echo "=== $rel ==="
    echo "$log"
    echo ""
done
