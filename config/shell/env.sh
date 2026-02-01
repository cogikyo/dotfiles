#!/bin/sh

export PATH="$HOME/.cargo/bin:$HOME/.go/bin:$HOME/.local/bin:$PATH"
export GOPATH="$HOME/.go"

# export TERM=xterm-256color  # Let terminal set its own (kitty needs xterm-kitty for undercurls)
export PAGER=nvimpager
export EDITOR=nvim
export DOTS="$HOME/dotfiles"

export FZF_DEFAULT_OPTS="\
--color=bg+:#222536,bg:#222536,spinner:#b29ae8,hl:#f8b486 \
--color=fg:#9db2f4,header:#f8b486,info:#6380ec,pointer:#f8b486 \
--color=marker:#6380ec,fg+:#8aa4f3,prompt:#6380ec,hl+:#f8b486"