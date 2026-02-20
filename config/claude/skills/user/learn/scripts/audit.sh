#!/usr/bin/env bash
# Audit skill structure and symlinks
# Usage: audit.sh [skill-name]  (no arg = audit all)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")" && pwd)"
SKILLS_BASE="$(cd "$SCRIPT_DIR/../../.." && pwd)"
SKILL_NAME="${1:-}"
ERRORS=()

red() { printf '\033[31m%s\033[0m\n' "$1"; }
green() { printf '\033[32m%s\033[0m\n' "$1"; }
yellow() { printf '\033[33m%s\033[0m\n' "$1"; }

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

# User skills → ~/.config/claude/skills/<name> should be a symlink to dotfiles source
check_link() {
    local skill_name="$1"
    local skill_path="$2"
    local scope="$3"

    if [[ "$scope" == "user" ]]; then
        local target_dir="$HOME/.config/claude/skills"
        local link="$target_dir/$skill_name"

        if [[ -L "$link" ]]; then
            local target
            target=$(readlink "$link")
            local norm_target="${target%/}"
            local norm_skill="${skill_path%/}"
            if [[ "$norm_target" != "$norm_skill" ]]; then
                ERRORS+=("Symlink $link → $target (expected $skill_path)")
            fi
        elif [[ -d "$link" ]]; then
            ERRORS+=("$link is a real directory, not a symlink — run link.sh user")
        else
            ERRORS+=("Not linked — run link.sh user")
        fi
    fi
    # project skills are linked per-project via link.sh project <name>, skip here
}

lint_skill() {
    local skill_path="$1"
    ERRORS=()

    local rel_path="${skill_path#"$SKILLS_BASE"/}"
    local depth
    depth=$(echo "$rel_path" | tr '/' '\n' | wc -l)

    # Path structure: should be {user,project}/<name>
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

    # Symlink check
    local skill_name scope
    skill_name=$(basename "$skill_path")
    scope=$(echo "$rel_path" | cut -d'/' -f1)
    check_link "$skill_name" "$skill_path" "$scope"

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
