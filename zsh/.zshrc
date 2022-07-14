export GOPATH="$HOME/.go"
export PATH="$PATH:$HOME/.go/bin"
export PATH="$PATH:$HOME/.local/bin"
export PATH="$HOME/.local/bin:$PATH"
export PATH="$HOME/.cargo/bin:$PATH"

eval "$(starship init zsh)"
eval "$(jump shell)"

# history
HISTFILE=~/.histfile
HISTSIZE=1000
SAVEHIST=800
setopt INC_APPEND_HISTORY
setopt HIST_IGNORE_ALL_DUPS
setopt HIST_IGNORE_DUPS

# ┌┐┌┌┐┌┌┐┌
# │││││││││
# ┘└┘┘└┘┘└┘

f ()
{
  if [ -n $NNNLVL ] && [ "${NNNLVL:-0}" -ge 1 ]; then
    echo "nnn is already running"
    return
  fi

  NNN_TMPFILE="${XDG_CONFIG_HOME:-$HOME/.config}/nnn/.lastd"

  nnn -e "$@"

  if [ -f "$NNN_TMPFILE" ]; then
    . "$NNN_TMPFILE"
    rm -f "$NNN_TMPFILE" > /dev/null
  fi
}

export NNN_COLORS='4321'
BLK="00" CHR="00" REG="00"
DIR="04" EXE="02"
HARDLINK="00" SYMLINK="06"
MISSING="00"  ORPHAN="09"
FIFO="00" SOCK="00" OTHER="01"
export NNN_FCOLORS="$BLK$CHR$DIR$EXE$REG$HARDLINK$SYMLINK$MISSING$ORPHAN$FIFO$SOCK$OTHER"
export NNN_PLUG='t:autojump;f:bulknew;z:preview-tui'
export NNN_FIFO='/tmp/nnn.fifo'
export NNN_OPENER=nuke
export EDITOR=nvim


# ╔═╗┬  ┬┌─┐┌─┐┌─┐┌─┐
# ╠═╣│  │├─┤└─┐├┤ └─┐
# ╩ ╩┴─┘┴┴ ┴└─┘└─┘└─┘

alias pac='(){sudo pacman -S $1;}'
alias niol='clear && source ~/.zshrc'
alias dot='cd ~/dotfiles'
alias not='cd ~/notes'
alias x='exit'

# git
alias g='lazygit'
alias gck='(){git checkout $1}'
alias gts='git status'
kurmit () {
  git add .
  commit=""
  vared -p "Commit message: " commit && git commit -m "$commit"
  git push
}
alias amend='(){git add . && git commit --amend --no-edit;}'
alias gpf='git push --force'
alias gp='git pull'
alias gsync='git fetch -p && git branch -vv | awk "/: gone]/{print \$1}" | xargs git branch -D'
alias n='nvim'

alias eyes='(){xrandr --output HDMI-A-0 --brightness $1;}'
alias eyess='(){xrandr --output eDP-1 --brightness $1;}'

# tmux
alias tmux='TERM=xterm-256color tmux'
alias tls='tmux ls'
alias td='(){tmux detach}'
alias tk='tmux kill-session'
alias tks='tmux kill-server'



# ╔═╗┬  ┬ ┬┌─┐┬┌┐┌┌─┐
# ╠═╝│  │ ││ ┬││││└─┐
# ╩  ┴─┘└─┘└─┘┴┘└┘└─┘

source ~/.local/zsh/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh
source ~/.local/zsh/zsh-autosuggestions/zsh-autosuggestions.zsh
source ~/.local/zsh/zsh-autocomplete/zsh-autocomplete.plugin.zsh
  zstyle ':autocomplete:*' fzf-completion yes
  zstyle ':autocomplete:*' add-space
  zstyle ':autocomplete:*' min-input 1
source ~/.local/zsh/zsh-colored-man-pages/colored-man-pages.plugin.zsh

# ┌─┐┬─┐ ┬  ┬┌─┌─┐┬ ┬┌─┐
# ├┤ │┌┴┬┘  ├┴┐├┤ └┬┘└─┐
# └  ┴┴ └─  ┴ ┴└─┘ ┴ └─┘

bindkey '^[[1~'    beginning-of-line
bindkey '^[[H'     beginning-of-line
bindkey '^[[4~'    end-of-line
bindkey '^[[F'     end-of-line
bindkey '^[[3~'    delete-char
bindkey '^[[1;5C'  forward-word   # C-left
bindkey '^[[1;5D'  backward-word  # C-right
bindkey '^[[1;5A'  history-incremental-search-backward # C-up
bindkey '^[[1;5B'  history-incremental-search-forward  # C-down

$HOME/.local/bin/dna
