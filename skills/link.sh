#!/usr/bin/env bash

# Skills linking script
# Usage:
#   link.sh user              - Symlink all user skills to $CODEX_HOME/skills (with Claude compatibility links)
#   link.sh project [name]    - Symlink project skill(s) to ./.codex/skills/ (with .claude compatibility)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
USER_SKILLS_DIR="$SCRIPT_DIR/user"
PROJECT_SKILLS_DIR="$SCRIPT_DIR/project"

canonpath() {
    readlink -f "$1" 2>/dev/null || true
}

guard_against_self_linking() {
    local source_dir="$1"
    local target_dir="$2"
    local source_real target_real

    source_real="$(canonpath "$source_dir")"
    target_real="$(canonpath "$target_dir")"

    if [[ -n "$source_real" && -n "$target_real" && "$source_real" == "$target_real" ]]; then
        echo "Refusing to link: $target_dir resolves to source directory $source_dir"
        echo "Choose a different target (for example \$CODEX_HOME/skills) to avoid self-referential links."
        exit 1
    fi
}

link_config_skills_dir() {
    local consumer_name="$1"
    local target_dir="$2"
    local config_dir="$3"
    local rerun_cmd="$4"
    local config_skills="$config_dir/skills"

    mkdir -p "$config_dir"
    if [[ -e "$config_skills" && ! -L "$config_skills" ]]; then
        echo "Keeping existing directory at $config_skills"
        echo "If $consumer_name should read from $target_dir, remove it and run $rerun_cmd again."
    else
        ln -sfnv "$target_dir" "$config_skills"
    fi
}

link_all_skills() {
    local source_dir="$1"
    local target_dir="$2"

    for skill in "$source_dir"/*; do
        [[ -d "$skill" ]] || continue
        local name source_real link_real
        name=$(basename "$skill")
        source_real="$(canonpath "$skill")"
        link_real="$(canonpath "$target_dir/$name")"

        if [[ -n "$source_real" && -n "$link_real" && "$source_real" == "$link_real" ]]; then
            echo "Skipping $name: already resolves to source path"
            continue
        fi

        ln -sfnv "$skill" "$target_dir/$name"
    done
}

link_user_skills() {
    local codex_home target_dir claude_config_dir agents_config_dir

    codex_home="${CODEX_HOME:-${HOME}/.codex}"
    target_dir="$codex_home/skills"
    claude_config_dir="${CLAUDE_CONFIG_DIR:-${HOME}/.config/claude}"
    agents_config_dir="${AGENTS_CONFIG_DIR:-${HOME}/.config/agents}"

    mkdir -p "$target_dir"

    if [[ ! -d "$USER_SKILLS_DIR" ]]; then
        echo "No user skills directory found at $USER_SKILLS_DIR"
        exit 1
    fi

    guard_against_self_linking "$USER_SKILLS_DIR" "$target_dir"

    echo "Linking user skills to $target_dir..."
    link_all_skills "$USER_SKILLS_DIR" "$target_dir"

    link_config_skills_dir "Claude" "$target_dir" "$claude_config_dir" "link.sh user"
    if [[ "$agents_config_dir" != "$claude_config_dir" && "$agents_config_dir" != "$codex_home" ]]; then
        link_config_skills_dir "agents tools" "$target_dir" "$agents_config_dir" "link.sh user"
    fi
}

link_project_skill() {
    local skill_name="${1:-}"
    local skill_path
    local project_codex_dir="$PWD/.codex"
    local project_claude_dir="$PWD/.claude"
    local target_dir="$project_codex_dir/skills"

    mkdir -p "$target_dir"
    guard_against_self_linking "$PROJECT_SKILLS_DIR" "$target_dir"

    if [[ -n "$skill_name" ]]; then
        skill_path="$PROJECT_SKILLS_DIR/$skill_name"
        if [[ ! -d "$skill_path" ]]; then
            echo "Skill '$skill_name' not found in $PROJECT_SKILLS_DIR"
            exit 1
        fi

        echo "Linking project skill '$skill_name' to $target_dir..."
        ln -sfnv "$skill_path" "$target_dir/$skill_name"
    else
        echo "Linking all project skills to $target_dir..."
        link_all_skills "$PROJECT_SKILLS_DIR" "$target_dir"
    fi

    link_config_skills_dir "project Claude" "$target_dir" "$project_claude_dir" "link.sh project"
}

case "${1:-}" in
    user)
        link_user_skills
        ;;
    project)
        link_project_skill "${2:-}"
        ;;
    *)
        echo "Usage:"
        echo "  link.sh user              - Link all user skills to \$CODEX_HOME/skills"
        echo "  link.sh project [name]    - Link project skill(s) to ./.codex/skills/"
        exit 1
        ;;
esac
