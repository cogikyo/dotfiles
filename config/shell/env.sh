#!/bin/sh
# Shared env vars sourced by zshenv (and any POSIX shell).

path_prepend() {
    [ -d "$1" ] || return 0
    case ":$PATH:" in
        *":$1:"*) ;;
        *) PATH="$1:$PATH" ;;
    esac
}

path_prepend "$HOME/.local/bin"
path_prepend "$HOME/.go/bin"
path_prepend "$HOME/.cargo/bin"

if [ "$(uname)" = "Darwin" ]; then
    path_prepend "/usr/local/sbin"
    path_prepend "/usr/local/bin"
    path_prepend "/usr/local/go/bin"
    path_prepend "/opt/homebrew/sbin"
    path_prepend "/opt/homebrew/bin"
else
    export SSH_AUTH_SOCK="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}/ssh-agent.socket"
fi

export PATH
export GOPATH="$HOME/.go"
export GOPRIVATE="${GOPRIVATE:-git.linecode.dev/*,git.linecode.net/*}"

export PAGER=nvimpager
export EDITOR=nvim
export VISUAL=nvim
export DOTS="$HOME/dotfiles"

if [ "$(uname)" = "Darwin" ]; then
    export CLAUDE_CONFIG_DIR="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
else
    export CLAUDE_CONFIG_DIR="${CLAUDE_CONFIG_DIR:-$HOME/.config/claude}"
fi

export FZF_DEFAULT_OPTS="\
--color=bg+:#222536,bg:#222536,spinner:#b29ae8,hl:#f8b486 \
--color=fg:#9db2f4,header:#f8b486,info:#6380ec,pointer:#f8b486 \
--color=marker:#6380ec,fg+:#8aa4f3,prompt:#6380ec,hl+:#f8b486"
