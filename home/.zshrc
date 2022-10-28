# exports
export GOPATH="$HOME/.go"
export PATH="$PATH:$HOME/.go/bin"
export PATH="$PATH:$HOME/.local/bin"
export PATH="$HOME/.local/bin:$PATH"
export PATH="$HOME/.config/nnn/plugins:$PATH"
export PATH="$HOME/.cargo/bin:$PATH"
export PAGER=nvimpager

# evals (shell "apps")
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

e ()
{
  if [ -n $NNNLVL ] && [ "${NNNLVL:-0}" -ge 1 ]; then
    echo "nnn is already running"
    return
  fi

  NNN_TMPFILE="${XDG_CONFIG_HOME:-$HOME/.config}/nnn/.lastd"

  nnn -c "$@"

  if [ -f "$NNN_TMPFILE" ]; then
    . "$NNN_TMPFILE"
    rm -f "$NNN_TMPFILE" > /dev/null
  fi
}

# --------------------------------------------------------------\
# black   red     green    blue     magenta  cyan     white   | color
# color0  color1  color2   color4   color5   color6   color7  | normal
# color8  color9  color10  color12  color13  color14  color15 | bright
# --------------------------------------------------------------/
export NNN_COLORS='4321'
BLK="00" CHR="00" REG="00" HARDLINK="00" FIFO="00" SOCK="00" MISSING="00"
OTHER="01"
EXE="02"
DIR="04"
SYMLINK="06"
ORPHAN="09"
export NNN_FCOLORS="$BLK$CHR$DIR$EXE$REG$HARDLINK$SYMLINK$MISSING$ORPHAN$FIFO$SOCK$OTHER"
export NNN_PLUG='a:autojump;f:bulknew;d:preview-tui;o:fzopen;p:fzplug;O:xdgdefault'
export NNN_FIFO='/tmp/nnn.fifo'
export NNN_OPENER=nuke
dots="$HOME/dotfiles"
export NNN_BMS="d:$HOME/downloads/;.:$dots/;b:$dots/$USER/.local/bin;c:$dots/$USER/.config/;e:$dots/$USER/.config/nvim/;s:$HOME/media/screenshots/;r:$HOME/media/recordings/"
export EDITOR=nvim


# ╔═╗┬  ┬┌─┐┌─┐┌─┐┌─┐
# ╠═╣│  │├─┤└─┐├┤ └─┐
# ╩ ╩┴─┘┴┴ ┴└─┘└─┘└─┘

alias niol='clear && source ~/.zshrc'
alias ls='lsd'
alias tree='gtree'
alias dot='cd ~/dotfiles'
alias d='cd ~/downloads'
alias not='cd ~/notes'
alias x='exit'
alias n='nvim'
bk () { mpvpaper -o "--loop --brightness=$2 --contrast=$3 --saturation=$4 --gamma=$5 --hue=$6" 'HDMI-A-1' ~/dotfiles/resources/videos/"$1".mp4 }
alias ew='eww-open'
alias ews='eww inspector & disown && exit'
alias el='eww logs'
choas () {
    kitty --hold --title="$1" -e $1 & disown
}
alias man='man -r=""'
pacclean () {
    while ! [ "$(pacman -Qdtq)" = "" ]; do
	sudo pacman -Rn $(pacman -Qdtq)
    done
}
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
            git add .
            git commit --message="$commit_msg"
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
alias gP='git push'
# alias gsync='git fetch -p && git branch -vv | awk "/: gone]/{print \$1}" | xargs git branch -D'

# ╔═╗┬  ┬ ┬┌─┐┬┌┐┌┌─┐
# ╠═╝│  │ ││ ┬││││└─┐
# ╩  ┴─┘└─┘└─┘┴┘└┘└─┘
source ~/.local/zsh/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh
source ~/.local/zsh/zsh-autosuggestions/zsh-autosuggestions.zsh
  ZSH_AUTOSUGGEST_STRATEGY=(history completion match_prev_cmd)
source ~/.local/zsh/zsh-autocomplete/zsh-autocomplete.plugin.zsh
  zstyle ':autocomplete:*' fzf-completion yes
  zstyle ':autocomplete:*' min-delay 0.69
  zstyle ':autocomplete:*' list-lines 32
  zstyle ':autocomplete:history-search:*' list-lines 32
  zstyle ':autocomplete:history-incremental-search-*:*' list-lines 32

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

# TODO: FIX
# $HOME/.local/bin/dna
