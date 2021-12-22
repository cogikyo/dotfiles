# ███████╗███████╗██╗  ██╗██████╗  ██████╗
# ╚══███╔╝██╔════╝██║  ██║██╔══██╗██╔════╝
#   ███╔╝ ███████╗███████║██████╔╝██║
#  ███╔╝  ╚════██║██╔══██║██╔══██╗██║
# ███████╗███████║██║  ██║██║  ██║╚██████╗
# ╚══════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝
# →-.   →  .-'.   →  .--.   →  .--.   →  .--.   →  .-→
# →::.\::::::::.\::::::::.\::::::::.\::::::::.\::::::: → →
# → →:.\ https://github.com/nosvagor/dotfiles \::: → →  →  →
# →::::.\::::::::.\::::::::.\::::::::.\::::::::.\::::→  →
#   → →   `--'  →   `.-'  →   `--'  →   `--'  →   `--' → →

# ╦┌┐┌┬┌┬┐
# ║││││ │
# ╩┘└┘┴ ┴

# autostart startx after login
if [[ -z $DISPLAY ]] && [[ $(tty) = /dev/tty1 ]]; then
startx
fi

~/.config/zsh/greeting.sh

eval "$(starship init zsh)"
eval "$(thefuck --alias)"
eval "$(jump shell)"

# history
HISTFILE=~/.histfile
HISTSIZE=10000
SAVEHIST=10000
setopt appendhistory
setopt share_history


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
export EDITOR=nvim


# ╔═╗┬  ┬┌─┐┌─┐┌─┐┌─┐
# ╠═╣│  │├─┤└─┐├┤ └─┐
# ╩ ╩┴─┘┴┴ ┴└─┘└─┘└─┘

alias pac='(){sudo pacman -S $1;}' alias niol='clear && zsh'

# git
alias gts='git status'
alias kurmit='(){git add . && git commit -m"$1" && git push;}'
alias amend='(){git add . && git commit --amend --no-edit;}'
alias gp='git pull'
alias n='nvim'

alias eyes='(){xrandr --output HDMI-A-0 --brightness $1;}'
alias eyess='(){xrandr --output eDP-1 --brightness $1;}'

# tmux
alias tls='tmux ls'
alias tad='tmux attach -t \ 🪙'
alias tan='tmux attach -t \ 📓'
alias tal='tmux attach -t \ 🌒'
alias tab='tmux attach -t \ 🐚'
alias tap='tmux attach -t \ 🐍'
alias tar='tmux attach -t \ 🦀'
alias tas='tmux attach -t \ 🦄'
alias tag='tmux attach -t \ 🦦'
alias tat='tmux attach -t \ 🎏'
alias td='(){tmux detach}'
alias tr='()tmux-session restore'
alias ts='tmux-session save'
alias tk='tmux kill-session'
alias tks='tmux kill-server'


# ╔═╗┬  ┬ ┬┌─┐┬┌┐┌┌─┐
# ╠═╝│  │ ││ ┬││││└─┐
# ╩  ┴─┘└─┘└─┘┴┘└┘└─┘

source ~/.local/zsh/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh
source ~/.local/zsh/zsh-autosuggestions/zsh-autosuggestions.zsh
source ~/.local/zsh/zsh-autocomplete/zsh-autocomplete.plugin.zsh
  zstyle ':autocomplete:*' min-input 2
source ~/.local/zsh/zsh-colored-man-pages/colored-man-pages.plugin.zsh


# ╔═╗─┐ ┬┌─┐┌─┐┬─┐┌┬┐┌─┐
# ║╣ ┌┴┬┘├─┘│ │├┬┘ │ └─┐
# ╚═╝┴ └─┴  └─┘┴└─ ┴ └─┘

export GOPATH="$HOME/.go"
export PATH="$PATH:$HOME/.go/bin"
export PATH="$PATH:$HOME/.local/bin"
export PATH="$HOME/.local/bin:$PATH"
export PATH="$HOME/.cargo/bin:$PATH"
export TEXINPUTS="$HOME/notes/.assets/tex//:"


# ┌─┐┬─┐ ┬  ┬┌─┌─┐┬ ┬┌─┐
# ├┤ │┌┴┬┘  ├┴┐├┤ └┬┘└─┐
# └  ┴┴ └─  ┴ ┴└─┘ ┴ └─┘

bindkey  "^[[1~"    beginning-of-line
bindkey  "^[[H"     beginning-of-line
bindkey  "^[[4~"    end-of-line
bindkey  "^[[F"     end-of-line
bindkey  "^[[3~"    delete-char
bindkey  "^[[1;5C"  forward-word
bindkey  "^[[1;5D"  backward-word
bindkey  "^S"       history-incremental-search-forward
bindkey  "^R"       history-incremental-search-backward
