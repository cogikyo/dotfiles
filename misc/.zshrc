eval "$(starship init zsh)"
eval "$(zoxide init zsh)"
eval "$(thefuck --alias f)"

# history
HISTFILE=~/.histfile
HISTSIZE=1000
SAVEHIST=800
setopt INC_APPEND_HISTORY
setopt HIST_IGNORE_ALL_DUPS
setopt HIST_IGNORE_DUPS

# aliases
alias niol='clear && source ~/.zshrc'
alias ls='lsd'
alias n='nvim'
alias d='cd ~/dotfiles'
alias v='cd ~/vagari'
alias c='cd ~/.config'
alias x='exit'
alias g='lazygit'
alias p='paru'
alias man='man --pager=nvimpager'

# plugins
if [[ ! -f ${ZDOTDIR:-${HOME}}/.zcomet/bin/zcomet.zsh ]]; then
  command git clone https://github.com/agkozak/zcomet.git ${ZDOTDIR:-${HOME}}/.zcomet/bin
fi

source ${ZDOTDIR:-${HOME}}/.zcomet/bin/zcomet.zsh

zcomet load zsh-users/zsh-syntax-highlighting
zcomet load zsh-users/zsh-autosuggestions
zcomet load marlonrichert/zsh-autocomplete

# fix keys
bindkey -v
export KEYTIMEOUT=1
bindkey '^[[H'     vi-beginning-of-line
bindkey '^[[F'     vi-end-of-line
bindkey '^[[3~'    delete-char
bindkey '^[[1;5C'  forward-word   # C-left
bindkey '^[[1;5D'  backward-word  # C-right
bindkey '^[[1;5A'  history-incremental-search-backward # C-up
bindkey '^[[1;5B'  history-incremental-search-forward  # C-down
