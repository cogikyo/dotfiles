#!/usr/bin/env bash

# Skills linking script
# Usage:
#   link.sh user              - Link user skills for Claude/agents compatibility
#   link.sh project [name]    - Symlink project skill(s) to ./.opencode/skills/ (with compatibility links)

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
        echo "Choose a different target (for example ./.opencode/skills) to avoid self-referential links."
        exit 1
    fi
}

link_config_dir() {
    local consumer_name="$1"
    local target_dir="$2"
    local config_dir="$3"
    local link_name="$4"
    local rerun_cmd="$5"
    local config_link="$config_dir/$link_name"

    mkdir -p "$config_dir"
    if [[ -e "$config_link" && ! -L "$config_link" ]]; then
        echo "Keeping existing directory at $config_link"
        echo "If $consumer_name should read from $target_dir, remove it and run $rerun_cmd again."
    else
        ln -sfnv "$target_dir" "$config_link"
    fi
}

link_config_skills_dir() {
    local consumer_name="$1"
    local target_dir="$2"
    local config_dir="$3"
    local rerun_cmd="$4"

    link_config_dir "$consumer_name" "$target_dir" "$config_dir" "skills" "$rerun_cmd"
}

link_opencode_skill_commands() {
    local source_dir="$1"
    local commands_dir="$2"

    mkdir -p "$commands_dir"

    for skill in "$source_dir"/*; do
        [[ -d "$skill" ]] || continue
        local name skill_file command_file command_target
        name=$(basename "$skill")
        skill_file="$skill/SKILL.md"
        [[ -f "$skill_file" ]] || continue
        command_file="$commands_dir/$name.md"
        command_target=$(realpath --relative-to="$commands_dir" "$skill_file")

        if [[ -e "$command_file" && ! -L "$command_file" ]]; then
            echo "Keeping existing command file at $command_file"
            echo "If OpenCode should read $skill_file for /$name, remove it and run link.sh user again."
            continue
        fi

        ln -sfnv "$command_target" "$command_file"
    done
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
    local claude_home claude_config_dir agents_home opencode_config_dir

    claude_home="${CLAUDE_HOME:-${HOME}/.claude}"
    claude_config_dir="${CLAUDE_CONFIG_DIR:-${HOME}/.config/claude}"
    agents_home="${AGENTS_HOME:-${HOME}/.agents}"
    opencode_config_dir="${OPENCODE_CONFIG_DIR:-${HOME}/.config/opencode}"

    if [[ ! -d "$USER_SKILLS_DIR" ]]; then
        echo "No user skills directory found at $USER_SKILLS_DIR"
        exit 1
    fi

    echo "Linking user skills from $USER_SKILLS_DIR..."
    link_config_skills_dir "Claude" "$USER_SKILLS_DIR" "$claude_home" "link.sh user"
    link_config_skills_dir "Claude config" "$USER_SKILLS_DIR" "$claude_config_dir" "link.sh user"
    link_config_skills_dir "agents tools" "$USER_SKILLS_DIR" "$agents_home" "link.sh user"
    link_config_skills_dir "OpenCode" "$USER_SKILLS_DIR" "$opencode_config_dir" "link.sh user"
    link_opencode_skill_commands "$USER_SKILLS_DIR" "$opencode_config_dir/commands"
}

link_project_skill() {
    local skill_name="${1:-}"
    local skill_path
    local project_opencode_dir="$PWD/.opencode"
    local project_claude_dir="$PWD/.claude"
    local project_agents_dir="$PWD/.agents"
    local target_dir="$project_opencode_dir/skills"

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
    link_config_skills_dir "project agents" "$target_dir" "$project_agents_dir" "link.sh project"
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
        echo "  link.sh user              - Link user skills for Claude/agents compatibility"
        echo "  link.sh project [name]    - Link project skill(s) to ./.opencode/skills/"
        exit 1
        ;;
esac
