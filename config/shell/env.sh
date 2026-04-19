#!/bin/sh
# Shared env vars sourced by zshenv (and any POSIX shell).

export PATH="$HOME/.cargo/bin:$HOME/.go/bin:$HOME/.local/bin:$PATH"
export GOPATH="$HOME/.go"
export SSH_AUTH_SOCK="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}/ssh-agent.socket"
export GOPRIVATE="git.linecode.dev/*"

export PAGER=nvimpager
export EDITOR=nvim
export VISUAL=nvim
export DOTS="$HOME/dotfiles"
export CLAUDE_CONFIG_DIR="${CLAUDE_CONFIG_DIR:-$HOME/.config/claude}"
export CODEX_HOME="${CODEX_HOME:-$HOME/.config/agents}"

export FZF_DEFAULT_OPTS="\
--color=bg+:#222536,bg:#222536,spinner:#b29ae8,hl:#f8b486 \
--color=fg:#9db2f4,header:#f8b486,info:#6380ec,pointer:#f8b486 \
--color=marker:#6380ec,fg+:#8aa4f3,prompt:#6380ec,hl+:#f8b486"
