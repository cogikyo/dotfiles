#!/usr/bin/env bash
# Audit skill structure
# Usage: audit.sh [skill-name]  (no arg = audit all)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_BASE="$(cd "$SCRIPT_DIR/../../.." && pwd)"
SKILL_NAME="${1:-}"
ERRORS=()

red() { printf '\033[31m%s\033[0m\n' "$1"; }
green() { printf '\033[32m%s\033[0m\n' "$1"; }

find_skill() {
    local name="$1"
    for scope in user project; do
        local path="$SKILLS_BASE/$scope/$name"
        if [[ -d "$path" ]]; then
            echo "$path"
            return 0
        fi
    done
    return 1
}

lint_skill() {
    local skill_path="$1"
    ERRORS=()

    local rel_path="${skill_path#"$SKILLS_BASE"/}"
    local depth
    depth=$(echo "$rel_path" | tr '/' '\n' | wc -l)

    # Path structure
    if [[ $depth -gt 2 ]]; then
        ERRORS+=("Path too deep: $rel_path")
    fi
    if [[ "$rel_path" =~ ^(user|project)/(user|project)/ ]]; then
        ERRORS+=("Nested scope directory: $rel_path")
    fi

    # Required files
    [[ -f "$skill_path/SKILL.md" ]] || ERRORS+=("Missing SKILL.md")
    [[ -f "$skill_path/INSTRUCTIONS.md" ]] || ERRORS+=("Missing INSTRUCTIONS.md")

    # SKILL.md size
    if [[ -f "$skill_path/SKILL.md" ]]; then
        local lines words
        lines=$(wc -l < "$skill_path/SKILL.md")
        words=$(wc -w < "$skill_path/SKILL.md")
        ((lines <= 30)) || ERRORS+=("SKILL.md: $lines lines (max 30)")
        ((words <= 200)) || ERRORS+=("SKILL.md: $words words (max 200)")

        local first_line
        first_line=$(head -1 "$skill_path/SKILL.md")
        [[ "$first_line" == "---" ]] || ERRORS+=("SKILL.md missing YAML frontmatter")
    fi

    # Forbidden files
    local forbidden=(README.md CHANGELOG.md INSTALLATION.md)
    for f in "${forbidden[@]}"; do
        [[ ! -f "$skill_path/$f" ]] || ERRORS+=("Forbidden file: $f")
    done

    # Report
    if [[ ${#ERRORS[@]} -eq 0 ]]; then
        green "OK: $rel_path"
        return 0
    else
        red "FAIL: $rel_path"
        printf '  - %s\n' "${ERRORS[@]}"
        return 1
    fi
}

audit_all() {
    local failed=0
    local total=0

    for scope in user project; do
        local scope_dir="$SKILLS_BASE/$scope"
        [[ -d "$scope_dir" ]] || continue

        for skill_dir in "$scope_dir"/*/; do
            [[ -d "$skill_dir" ]] || continue
            ((++total))
            lint_skill "${skill_dir%/}" || ((++failed)) || true
        done
    done

    echo "---"
    if [[ $failed -eq 0 ]]; then
        green "All $total skills passed"
    else
        red "$failed/$total skills failed"
        return 1
    fi
}

# Main
if [[ -z "$SKILL_NAME" ]]; then
    audit_all
else
    skill_path=$(find_skill "$SKILL_NAME") || {
        red "No skill found: $SKILL_NAME"
        exit 1
    }
    lint_skill "$skill_path"
fi
