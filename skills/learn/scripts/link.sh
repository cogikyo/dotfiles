#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")" && pwd)"
SKILLS_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
OPENCODE_DIR="${OPENCODE_CONFIG_DIR:-$HOME/.config/opencode}"
OPENCODE_CONFIG="$OPENCODE_DIR/opencode.json"

warn() { printf 'warning: %s\n' "$*" >&2; }
info() { printf '%s\n' "$*"; }

ensure_opencode_config() {
    mkdir -p "$OPENCODE_DIR"

    if [[ ! -f "$OPENCODE_CONFIG" ]]; then
        warn "$OPENCODE_CONFIG not found; create it with skills.paths including {env:HOME}/dotfiles/skills"
        return 0
    fi

    if grep -Eq '"\{env:HOME\}/dotfiles/skills"|"/home/cullyn/dotfiles/skills"' "$OPENCODE_CONFIG" || grep -Fq "\"$SKILLS_DIR\"" "$OPENCODE_CONFIG"; then
        info "OpenCode skills path already configured: $SKILLS_DIR"
    else
        warn "$OPENCODE_CONFIG should include skills.paths entry {env:HOME}/dotfiles/skills"
    fi
}

link_compat_skills_dir() {
    local label="$1"
    local config_dir="$2"
    local link_path="$config_dir/skills"

    [[ -d "$config_dir" ]] || return 0

    if [[ -e "$link_path" && ! -L "$link_path" ]]; then
        warn "keeping real $label skills directory at $link_path"
        return 0
    fi

    ln -sfn "$SKILLS_DIR" "$link_path"
    info "$label skills -> $SKILLS_DIR"
}

link_opencode_commands() {
    local commands_dir="$OPENCODE_DIR/commands"
    mkdir -p "$commands_dir"

    local skill_dir skill_name command_path target
    for skill_dir in "$SKILLS_DIR"/*/; do
        [[ -d "$skill_dir" && -f "$skill_dir/SKILL.md" ]] || continue

        skill_name="$(basename "${skill_dir%/}")"
        command_path="$commands_dir/$skill_name.md"
        target="${skill_dir%/}/SKILL.md"

        if [[ -e "$command_path" && ! -L "$command_path" ]] && ! grep -Fq "Load the \`$skill_name\` skill and execute \`/$skill_name \$ARGUMENTS\`." "$command_path"; then
            warn "keeping real OpenCode command at $command_path"
            continue
        fi

        rm -f "$command_path"
        ln -s "$target" "$command_path"
        info "OpenCode command /$skill_name -> $target"
    done
}

ensure_opencode_config
link_compat_skills_dir "OpenCode" "$OPENCODE_DIR"
link_opencode_commands
link_compat_skills_dir "Claude" "${CLAUDE_HOME:-$HOME/.claude}"
link_compat_skills_dir "Claude config" "${CLAUDE_CONFIG_DIR:-$HOME/.config/claude}"
