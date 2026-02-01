#!/usr/bin/env bash

# Skills linking script
# Usage:
#   link.sh user              - Symlink all user skills to ~/.claude/skills/
#   link.sh project <name>    - Symlink a project skill to ./.claude/skills/

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
USER_SKILLS_DIR="$SCRIPT_DIR/user"
PROJECT_SKILLS_DIR="$SCRIPT_DIR/project"

link_user_skills() {
    local target_dir="${HOME}/.claude/skills"
    mkdir -p "$target_dir"

    if [[ ! -d "$USER_SKILLS_DIR" ]]; then
        echo "No user skills directory found at $USER_SKILLS_DIR"
        exit 1
    fi

    echo "Linking user skills to $target_dir..."
    for skill in "$USER_SKILLS_DIR"/*/; do
        [[ -d "$skill" ]] || continue
        local name
        name=$(basename "$skill")
        ln -sfnv "$skill" "$target_dir/$name"
    done
}

link_project_skill() {
    local skill_name="$1"
    local skill_path="$PROJECT_SKILLS_DIR/$skill_name"
    local target_dir="./.claude/skills"

    if [[ ! -d "$skill_path" ]]; then
        echo "Skill '$skill_name' not found in $PROJECT_SKILLS_DIR"
        exit 1
    fi

    mkdir -p "$target_dir"
    echo "Linking project skill '$skill_name' to $target_dir..."
    ln -sfnv "$skill_path" "$target_dir/$skill_name"
}

case "${1:-}" in
    user)
        link_user_skills
        ;;
    project)
        if [[ -z "${2:-}" ]]; then
            echo "Usage: link.sh project <skill-name>"
            exit 1
        fi
        link_project_skill "$2"
        ;;
    *)
        echo "Usage:"
        echo "  link.sh user              - Link all user skills to ~/.claude/skills/"
        echo "  link.sh project <name>    - Link a project skill to ./.claude/skills/"
        exit 1
        ;;
esac
