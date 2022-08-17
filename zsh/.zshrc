
# paths
export GOPATH="$HOME/.go"
export PATH="$PATH:$HOME/.go/bin"
export PATH="$PATH:$HOME/.local/bin"
export PATH="$HOME/.local/bin:$PATH"
export PATH="$HOME/.cargo/bin:$PATH"
export TERM=xterm-256color

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

alias niol='clear && source ~/.zshrc'
alias dot='cd ~/dotfiles'
alias not='cd ~/notes'
alias x='exit'
alias n='nvim'
alias cat='bat'
touchsh () {
    file="$1"
    if [[ -z "$1" ]]; then
        vared -p "script name: " file
    fi
    touch $file
    chmod +x $file
    echo "#!/usr/bin/env bash" >> $file
}

# git
alias g='lazygit'
alias gts='git status'
kurmit () {
    commit_msg="$*"
    echo "git status:\n"
    git status
    echo
    while true; do
    yn=""
    vared -p "Are you sure you want to kurmit all files? [y/n]: " yn
    case $yn in
        [Yy]* )
            if [[ -z $commit_msg ]]; then
                vared -p "Commit message: " commit_msg
            fi
            git commit --all --message="$commit_msg"
            git push
            return;;
        [Nn]* )
            echo "\nAll right, then. Keep your secrets."
            return;;
        * ) echo "Please answer (y)es or (n)o.";;
    esac
    done
}
alias amend='(){git add . && git commit --amend --no-edit;}'
alias gpf='git push --force'
alias gp='git pull'
alias gsync='git fetch -p && git branch -vv | awk "/: gone]/{print \$1}" | xargs git branch -D'

# tmux
alias tls='tmux ls'
alias tk='tmux kill-session'
alias tks='tmux kill-server'
alias tr='tmux-session restore'
alias tad='tmux attach -t '
alias tan='tmux attach -t '
alias tap='tmux attach -t ﳣ'
alias tat='tmux attach -t פּ'

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
