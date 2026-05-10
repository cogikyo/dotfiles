#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")" && pwd)"
SKILLS_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
SKILL_NAME="${1:-}"
ERRORS=()

CLR_RESET=$'\033[0m'
CLR_BLUE_BOLD=$'\033[1;34m'
CLR_GREEN=$'\033[32m'
CLR_RED=$'\033[31m'

PASS_MARK=$'\033[32m✔\033[0m'
FAIL_MARK=$'\033[31m✖\033[0m'
SKIP_MARK=$'\033[2m->\033[0m'

find_skill() {
    local name="$1"
    local path="$SKILLS_DIR/$name"

    [[ -d "$path" ]] || return 1
    printf '%s\n' "$path"
}

opencode_config_has_skills_path() {
    local config_file="${OPENCODE_CONFIG_FILE:-$HOME/.config/opencode/opencode.json}"
    local repo_path="$SKILLS_DIR"

    [[ -f "$config_file" ]] || return 1

    grep -Eq '"\{env:HOME\}/dotfiles/skills"|"/home/cullyn/dotfiles/skills"' "$config_file" && return 0
    grep -Fq "\"$repo_path\"" "$config_file"
}

check_opencode_config() {
    local config_file="${OPENCODE_CONFIG_FILE:-$HOME/.config/opencode/opencode.json}"
    local opencode_dir
    opencode_dir="$(dirname "$config_file")"
    local skills_link="$opencode_dir/skills"

    if opencode_config_has_skills_path; then
        eval "$1+=(\"pass|opencode-config|skills.paths includes $SKILLS_DIR\")"
    else
        ERRORS+=("OpenCode config missing skills.paths entry for {env:HOME}/dotfiles/skills")
        if [[ -f "$config_file" ]]; then
            eval "$1+=(\"fail|opencode-config|checked=$config_file\")"
        else
            eval "$1+=(\"fail|opencode-config|missing=$config_file\")"
        fi
    fi

    if [[ -L "$skills_link" && "$(readlink -f "$skills_link")" == "$SKILLS_DIR" ]]; then
        eval "$1+=(\"pass|opencode-link|$skills_link -> $SKILLS_DIR\")"
    elif [[ -e "$skills_link" && ! -L "$skills_link" ]]; then
        ERRORS+=("OpenCode skills path is a real directory, expected symlink to $SKILLS_DIR: $skills_link")
        eval "$1+=(\"fail|opencode-link|real directory at $skills_link\")"
    else
        ERRORS+=("OpenCode skills symlink missing or broken: $skills_link -> $SKILLS_DIR")
        eval "$1+=(\"fail|opencode-link|missing or broken: $skills_link\")"
    fi
}

lint_skill() {
    local skill_path="$1"
    ERRORS=()
    local -a check_lines=()

    local skill_name rel_path depth
    skill_name=$(basename "$skill_path")
    rel_path="${skill_path#"$SKILLS_DIR"/}"
    depth=$(awk -F/ '{print NF}' <<< "$rel_path")

    if [[ $depth -eq 1 ]]; then
        check_lines+=("pass|path-structure|$rel_path")
    else
        ERRORS+=("Path must be skills/<name>: $rel_path")
        check_lines+=("fail|path-structure|depth=$depth (expected 1)")
    fi

    if [[ -f "$skill_path/SKILL.md" && -f "$skill_path/INSTRUCTIONS.md" ]]; then
        check_lines+=("pass|required-files|SKILL.md + INSTRUCTIONS.md")
    else
        local missing=()
        [[ -f "$skill_path/SKILL.md" ]] || missing+=("SKILL.md")
        [[ -f "$skill_path/INSTRUCTIONS.md" ]] || missing+=("INSTRUCTIONS.md")
        ERRORS+=("Missing required files: ${missing[*]}")
        check_lines+=("fail|required-files|missing=${missing[*]}")
    fi

    if [[ -f "$skill_path/SKILL.md" ]]; then
        local lines words first_line
        lines=$(wc -l < "$skill_path/SKILL.md")
        words=$(wc -w < "$skill_path/SKILL.md")
        if (( lines <= 30 && words <= 200 )); then
            check_lines+=("pass|skill-size|$lines lines, $words words")
        else
            (( lines <= 30 )) || ERRORS+=("SKILL.md: $lines lines (max 30)")
            (( words <= 200 )) || ERRORS+=("SKILL.md: $words words (max 200)")
            check_lines+=("fail|skill-size|$lines lines, $words words (limits: 30/200)")
        fi

        first_line=$(head -n 1 "$skill_path/SKILL.md")
        if [[ "$first_line" == "---" ]]; then
            check_lines+=("pass|frontmatter|present")
        else
            ERRORS+=("SKILL.md missing YAML frontmatter")
            check_lines+=("fail|frontmatter|missing")
        fi
    else
        check_lines+=("skip|skill-size|SKILL.md missing")
        check_lines+=("skip|frontmatter|SKILL.md missing")
    fi

    local forbidden=(README.md CHANGELOG.md INSTALLATION.md)
    local -a found_forbidden=()
    for file in "${forbidden[@]}"; do
        if [[ -f "$skill_path/$file" ]]; then
            ERRORS+=("Forbidden file: $file")
            found_forbidden+=("$file")
        fi
    done
    if (( ${#found_forbidden[@]} == 0 )); then
        check_lines+=("pass|forbidden-files|found=0")
    else
        check_lines+=("fail|forbidden-files|files=${found_forbidden[*]}")
    fi

    if [[ -f "$skill_path/SKILL.md" && -f "$skill_path/INSTRUCTIONS.md" ]]; then
        local -a skill_cmds instruction_cmds
        local skill_cmds_joined="none" instruction_cmds_joined="none"
        mapfile -t skill_cmds < <(grep -E "^[[:space:]]*-[[:space:]]*\`/" "$skill_path/SKILL.md" | sed -E "s/^[[:space:]]*-[[:space:]]*\`([^\`]+)\`.*$/\1/" | sort -u)
        mapfile -t instruction_cmds < <(grep -E "^###[[:space:]]*\`/" "$skill_path/INSTRUCTIONS.md" | sed -E "s/^###[[:space:]]*\`([^\`]+)\`.*$/\1/" | sort -u)

        (( ${#skill_cmds[@]} == 0 )) || skill_cmds_joined=$(printf '%s, ' "${skill_cmds[@]}")
        skill_cmds_joined="${skill_cmds_joined%, }"
        (( ${#instruction_cmds[@]} == 0 )) || instruction_cmds_joined=$(printf '%s, ' "${instruction_cmds[@]}")
        instruction_cmds_joined="${instruction_cmds_joined%, }"

        if (( ${#skill_cmds[@]} > 0 && ${#instruction_cmds[@]} > 0 )); then
            if [[ "$skill_cmds_joined" == "$instruction_cmds_joined" ]]; then
                check_lines+=("pass|commands-sync|commands=[$skill_cmds_joined]")
            else
                ERRORS+=("Command list mismatch: SKILL.md has [$skill_cmds_joined], INSTRUCTIONS.md has [$instruction_cmds_joined]")
                check_lines+=("fail|commands-sync|SKILL=[$skill_cmds_joined] | INSTRUCTIONS=[$instruction_cmds_joined]")
            fi
        else
            check_lines+=("skip|commands-sync|SKILL=[$skill_cmds_joined] | INSTRUCTIONS=[$instruction_cmds_joined]")
        fi

        local config_file="${OPENCODE_CONFIG_FILE:-$HOME/.config/opencode/opencode.json}"
        local commands_dir
        commands_dir="$(dirname "$config_file")/commands"
        local command_file="$commands_dir/$skill_name.md"

        if [[ ! -e "$command_file" ]]; then
            ERRORS+=("Missing OpenCode command shim: $command_file")
            check_lines+=("fail|command-shims|missing=$command_file")
        elif [[ ! -L "$command_file" || "$(readlink -f "$command_file")" != "$SKILLS_DIR/$skill_name/SKILL.md" ]]; then
            ERRORS+=("OpenCode command shim must symlink to matching SKILL.md: $command_file")
            check_lines+=("fail|command-shims|invalid=$command_file")
        else
            check_lines+=("pass|command-shims|$command_file -> $SKILLS_DIR/$skill_name/SKILL.md")
        fi
    else
        check_lines+=("skip|commands-sync|requires SKILL.md and INSTRUCTIONS.md")
        check_lines+=("skip|command-shims|requires SKILL.md and INSTRUCTIONS.md")
    fi

    check_opencode_config check_lines

    local pass_count=0 skip_count=0 fail_count=0 item status check_name check_meta
    for item in "${check_lines[@]}"; do
        IFS='|' read -r status check_name check_meta <<< "$item"
        case "$status" in
            pass) ((++pass_count)) ;;
            skip) ((++skip_count)) ;;
            fail) ((++fail_count)) ;;
        esac
    done

    if [[ ${#ERRORS[@]} -eq 0 ]]; then
        printf '\n%b%s%b: %bOK%b\n' "$CLR_BLUE_BOLD" "$skill_name" "$CLR_RESET" "$CLR_GREEN" "$CLR_RESET"
        printf '  summary: %d passed, %d skipped, %d failed\n' "$pass_count" "$skip_count" "$fail_count"
        for item in "${check_lines[@]}"; do
            IFS='|' read -r status check_name check_meta <<< "$item"
            if [[ "$status" == "pass" ]]; then
                printf '  %b %-16s %s\n' "$PASS_MARK" "$check_name" "$check_meta"
            else
                printf '  %b %-16s %s\n' "$SKIP_MARK" "$check_name" "$check_meta"
            fi
        done
        return 0
    fi

    printf '\n%b%s%b: %bFAIL%b\n' "$CLR_BLUE_BOLD" "$skill_name" "$CLR_RESET" "$CLR_RED" "$CLR_RESET"
    printf '  summary: %d passed, %d skipped, %d failed\n' "$pass_count" "$skip_count" "$fail_count"
    for item in "${check_lines[@]}"; do
        IFS='|' read -r status check_name check_meta <<< "$item"
        case "$status" in
            pass) printf '  %b %-16s %s\n' "$PASS_MARK" "$check_name" "$check_meta" ;;
            skip) printf '  %b %-16s %s\n' "$SKIP_MARK" "$check_name" "$check_meta" ;;
            fail) printf '  %b %-16s %s\n' "$FAIL_MARK" "$check_name" "$check_meta" ;;
        esac
    done
    printf '  - %s\n' "${ERRORS[@]}"
    return 1
}

audit_all() {
    local failed=0 total=0

    for skill_dir in "$SKILLS_DIR"/*/; do
        [[ -d "$skill_dir" ]] || continue
        ((++total))
        lint_skill "${skill_dir%/}" || ((++failed)) || true
    done

    echo "---"
    if [[ $failed -eq 0 ]]; then
        printf '%b All %d skills passed\n' "$PASS_MARK" "$total"
    else
        printf '%b %d/%d skills failed\n' "$FAIL_MARK" "$failed" "$total"
        return 1
    fi
}

if [[ -z "$SKILL_NAME" ]]; then
    audit_all
else
    skill_path=$(find_skill "$SKILL_NAME") || {
        printf '%b No skill found: %s\n' "$FAIL_MARK" "$SKILL_NAME"
        exit 1
    }
    lint_skill "$skill_path"
fi
