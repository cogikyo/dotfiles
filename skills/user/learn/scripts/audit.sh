#!/usr/bin/env bash
# Audit skill structure and symlinks
# Usage: audit.sh [skill-name]  (no arg = audit all)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")" && pwd)"
SKILLS_BASE="$(cd "$SCRIPT_DIR/../../.." && pwd)"
SKILL_NAME="${1:-}"
ERRORS=()
LINK_DETAIL=""

red() { printf '\033[31m%s\033[0m\n' "$1"; }
green() { printf '\033[32m%s\033[0m\n' "$1"; }

CLR_RESET=$'\033[0m'
CLR_BLUE_BOLD=$'\033[1;34m'
CLR_GREEN=$'\033[32m'
CLR_RED=$'\033[31m'

PASS_MARK=$'\033[32m✔\033[0m'
FAIL_MARK=$'\033[31m✖\033[0m'
SKIP_MARK=$'\033[2m->\033[0m'

project_links_for_skill() {
    local skill_name="$1"
    local skill_path="$2"
    local norm_skill
    local -a links=()
    norm_skill=$(readlink -f "$skill_path")

    while IFS= read -r link; do
        local target project_dir
        target=$(readlink -f "$link" 2>/dev/null || true)
        [[ "$target" == "$norm_skill" ]] || continue
        project_dir="${link%/.codex/skills/"$skill_name"}"
        links+=("$project_dir")
    done < <(
        find "$HOME" \
            \( -path "$HOME/.cache" -o -path "$HOME/.local/share" -o -path "$HOME/.cargo" -o -path "$HOME/.rustup" -o -path "$HOME/.npm" -o -path "$HOME/.pnpm-store" \) -prune \
            -o -type l -path "*/.codex/skills/$skill_name" -print 2>/dev/null
    )

    if (( ${#links[@]} == 0 )); then
        echo "none"
    else
        printf '%s\n' "${links[@]}" | sort -u | paste -sd',' -
    fi
}

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

# User skills should be linked in $CODEX_HOME/skills/<name>.
# Config-dir compatibility paths are also accepted:
#   ${CLAUDE_CONFIG_DIR:-~/.config/claude}/skills/<name>
#   ${AGENTS_CONFIG_DIR:-~/.config/agents}/skills/<name>
check_link() {
    local skill_name="$1"
    local skill_path="$2"
    local scope="$3"
    LINK_DETAIL="symlink=skipped-project"

    if [[ "$scope" == "user" ]]; then
        local link target
        local ok=0
        local norm_skill
        LINK_DETAIL="symlink=missing"
        norm_skill=$(readlink -f "$skill_path")
        local codex_home="${CODEX_HOME:-$HOME/.codex}"
        local claude_config_dir="${CLAUDE_CONFIG_DIR:-$HOME/.config/claude}"
        local agents_config_dir="${AGENTS_CONFIG_DIR:-$HOME/.config/agents}"
        local codex_link="$codex_home/skills/$skill_name"
        local candidates=(
            "$codex_link"
            "$claude_config_dir/skills/$skill_name"
            "$agents_config_dir/skills/$skill_name"
            "$HOME/.agents/skills/$skill_name"
        )

        for link in "${candidates[@]}"; do
            if [[ -L "$link" ]]; then
                local norm_target
                target=$(readlink -f "$link" 2>/dev/null || true)
                norm_target="${target%/}"
                if [[ "$norm_target" == "$norm_skill" ]]; then
                    ok=1
                    LINK_DETAIL="symlink=${link} -> ${norm_target}"
                    break
                fi
            fi
        done

        if [[ $ok -eq 0 ]]; then
            local agents_link="$HOME/.agents/skills/$skill_name"
            local codex_skill_link="$codex_link"
            local claude_link="$claude_config_dir/skills/$skill_name"
            local agents_config_link="$agents_config_dir/skills/$skill_name"

            if [[ -d "$codex_skill_link" && ! -L "$codex_skill_link" ]]; then
                ERRORS+=("$codex_skill_link is a real directory, not a symlink — run link.sh user")
            elif [[ -d "$agents_link" && ! -L "$agents_link" ]]; then
                ERRORS+=("$agents_link is a real directory, not a symlink — run link.sh user")
            elif [[ -d "$claude_link" && ! -L "$claude_link" ]]; then
                ERRORS+=("$claude_link is a real directory, not a symlink — run link.sh user")
            elif [[ -d "$agents_config_link" && ! -L "$agents_config_link" ]]; then
                ERRORS+=("$agents_config_link is a real directory, not a symlink — run link.sh user")
            else
                ERRORS+=("Not linked in \$CODEX_HOME/skills, \$CLAUDE_CONFIG_DIR/skills, or \$AGENTS_CONFIG_DIR/skills — run link.sh user")
            fi
        fi
    fi
    # project skills are linked per-project via link.sh project <name>, skip here
}

lint_skill() {
    local skill_path="$1"
    ERRORS=()
    local -a check_lines=()

    local rel_path="${skill_path#"$SKILLS_BASE"/}"
    local depth
    local skill_name scope
    skill_name=$(basename "$skill_path")
    scope=$(echo "$rel_path" | cut -d'/' -f1)
    depth=$(echo "$rel_path" | tr '/' '\n' | wc -l)

    # Path structure: should be {user,project}/<name>
    if [[ $depth -gt 2 ]]; then
        ERRORS+=("Path too deep: $rel_path")
        check_lines+=("fail|path-structure|depth=$depth (expected 2)")
    elif [[ "$rel_path" =~ ^(user|project)/(user|project)/ ]]; then
        ERRORS+=("Nested scope directory: $rel_path")
        check_lines+=("fail|path-structure|nested scope")
    else
        check_lines+=("pass|path-structure|depth=$depth")
    fi

    # Required files
    if [[ -f "$skill_path/SKILL.md" && -f "$skill_path/INSTRUCTIONS.md" ]]; then
        check_lines+=("pass|required-files|SKILL.md + INSTRUCTIONS.md")
    else
        local missing="none"
        if [[ ! -f "$skill_path/SKILL.md" ]]; then
            ERRORS+=("Missing SKILL.md")
            missing="SKILL.md"
        fi
        if [[ ! -f "$skill_path/INSTRUCTIONS.md" ]]; then
            ERRORS+=("Missing INSTRUCTIONS.md")
            if [[ "$missing" == "none" ]]; then
                missing="INSTRUCTIONS.md"
            else
                missing="$missing, INSTRUCTIONS.md"
            fi
        fi
        check_lines+=("fail|required-files|missing=$missing")
    fi

    # SKILL.md size
    if [[ -f "$skill_path/SKILL.md" ]]; then
        local lines words
        lines=$(wc -l < "$skill_path/SKILL.md")
        words=$(wc -w < "$skill_path/SKILL.md")
        if ((lines <= 30 && words <= 200)); then
            check_lines+=("pass|skill-size|$lines lines, $words words")
        else
            ((lines <= 30)) || ERRORS+=("SKILL.md: $lines lines (max 30)")
            ((words <= 200)) || ERRORS+=("SKILL.md: $words words (max 200)")
            check_lines+=("fail|skill-size|$lines lines, $words words (limits: 30/200)")
        fi

        local first_line
        first_line=$(head -1 "$skill_path/SKILL.md")
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

    # Forbidden files
    local forbidden=(README.md CHANGELOG.md INSTALLATION.md)
    local -a found_forbidden=()
    for f in "${forbidden[@]}"; do
        if [[ -f "$skill_path/$f" ]]; then
            ERRORS+=("Forbidden file: $f")
            found_forbidden+=("$f")
        fi
    done
    if (( ${#found_forbidden[@]} == 0 )); then
        check_lines+=("pass|forbidden-files|found=0")
    else
        local found_text
        found_text=$(printf '%s, ' "${found_forbidden[@]}")
        found_text="${found_text%, }"
        check_lines+=("fail|forbidden-files|found=${#found_forbidden[@]}; files=[$found_text]")
    fi

    # Keep slash-command docs in sync when both files declare them.
    if [[ -f "$skill_path/SKILL.md" && -f "$skill_path/INSTRUCTIONS.md" ]]; then
        local -a skill_cmds instruction_cmds
        local skill_cmds_joined="none" instruction_cmds_joined="none"
        mapfile -t skill_cmds < <(
            sed -nE "s/^[[:space:]]*-[[:space:]]*\`(\/[^\`]+)\`.*$/\1/p" "$skill_path/SKILL.md" | sort -u
        )
        mapfile -t instruction_cmds < <(
            sed -nE "s/^###[[:space:]]*\`(\/[^\`]+)\`.*$/\1/p" "$skill_path/INSTRUCTIONS.md" | sort -u
        )
        if (( ${#skill_cmds[@]} > 0 )); then
            skill_cmds_joined=$(printf '%s, ' "${skill_cmds[@]}")
            skill_cmds_joined="${skill_cmds_joined%, }"
        fi
        if (( ${#instruction_cmds[@]} > 0 )); then
            instruction_cmds_joined=$(printf '%s, ' "${instruction_cmds[@]}")
            instruction_cmds_joined="${instruction_cmds_joined%, }"
        fi

        if (( ${#skill_cmds[@]} > 0 && ${#instruction_cmds[@]} > 0 )); then
            if [[ "$skill_cmds_joined" != "$instruction_cmds_joined" ]]; then
                ERRORS+=("Command list mismatch: SKILL.md has [$skill_cmds_joined], INSTRUCTIONS.md has [$instruction_cmds_joined]")
                check_lines+=("fail|commands-sync|SKILL=[$skill_cmds_joined] | INSTRUCTIONS=[$instruction_cmds_joined]")
            else
                check_lines+=("pass|commands-sync|commands=[$skill_cmds_joined]")
            fi
        else
            check_lines+=("skip|commands-sync|SKILL=[$skill_cmds_joined] | INSTRUCTIONS=[$instruction_cmds_joined]")
        fi
    else
        check_lines+=("skip|commands-sync|requires SKILL.md and INSTRUCTIONS.md")
    fi

    # Symlink check
    local errors_before_link=${#ERRORS[@]}
    check_link "$skill_name" "$skill_path" "$scope"
    if [[ "$scope" == "user" ]]; then
        if (( ${#ERRORS[@]} == errors_before_link )); then
            check_lines+=("pass|symlink|$LINK_DETAIL")
        else
            check_lines+=("fail|symlink|$LINK_DETAIL")
        fi
    else
        check_lines+=("skip|symlink|$LINK_DETAIL")
    fi

    local pass_count=0 skip_count=0 fail_count=0
    local item status check_name check_meta
    for item in "${check_lines[@]}"; do
        IFS='|' read -r status check_name check_meta <<< "$item"
        case "$status" in
            pass) ((++pass_count)) ;;
            skip) ((++skip_count)) ;;
            fail) ((++fail_count)) ;;
        esac
    done

    # Report
    local scope_text
    if [[ "$scope" == "project" ]]; then
        local linked_projects
        linked_projects=$(project_links_for_skill "$skill_name" "$skill_path")
        scope_text="scope=project; linked_projects=[$linked_projects]"
    else
        scope_text="scope=user"
    fi

    if [[ ${#ERRORS[@]} -eq 0 ]]; then
        printf '\n%b%s%b: %bOK%b (%s)\n' "$CLR_BLUE_BOLD" "$skill_name" "$CLR_RESET" "$CLR_GREEN" "$CLR_RESET" "$scope_text"
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
    else
        printf '\n%b%s%b: %bFAIL%b (%s)\n' "$CLR_BLUE_BOLD" "$skill_name" "$CLR_RESET" "$CLR_RED" "$CLR_RESET" "$scope_text"
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
        printf '%b All %d skills passed\n' "$PASS_MARK" "$total"
    else
        printf '%b %d/%d skills failed\n' "$FAIL_MARK" "$failed" "$total"
        return 1
    fi
}

# Main
if [[ -z "$SKILL_NAME" ]]; then
    audit_all
else
    skill_path=$(find_skill "$SKILL_NAME") || {
        printf '%b No skill found: %s\n' "$FAIL_MARK" "$SKILL_NAME"
        exit 1
    }
    lint_skill "$skill_path"
fi
