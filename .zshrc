# ███████╗███████╗██╗  ██╗██████╗  ██████╗
# ╚══███╔╝██╔════╝██║  ██║██╔══██╗██╔════╝
#   ███╔╝ ███████╗███████║██████╔╝██║
#  ███╔╝  ╚════██║██╔══██║██╔══██╗██║
# ███████╗███████║██║  ██║██║  ██║╚██████╗
# ╚══════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝
# →-.   →  .-'.   →  .--.   →  .--.   →  .--.   →  .-→
# →::.\::::::::.\::::::::.\::::::::.\::::::::.\::::::: → →
# → →:.\ https://github.com/nosvagor/.dotfiles \::: → →  →  →
# →::::.\::::::::.\::::::::.\::::::::.\::::::::.\::::→  →
#   → →   `--'  →   `.-'  →   `--'  →   `--'  →   `--' → →

# ╦┌┐┌┬┌┬┐
# ║││││ │
# ╩┘└┘┴ ┴
eval "$(starship init zsh)"
eval "$(thefuck --alias)"
~/.config/zsh/greeting.sh

# autostart startx after login
if [[ -z $DISPLAY ]] && [[ $(tty) = /dev/tty1 ]]; then
startx
fi

# history
HISTFILE=~/.histfile
HISTSIZE=10000
SAVEHIST=10000
setopt appendhistory
setopt share_history

# ╔═╗┬  ┬┌─┐┌─┐┌─┐┌─┐
# ╠═╣│  │├─┤└─┐├┤ └─┐
# ╩ ╩┴─┘┴┴ ┴└─┘└─┘└─┘

# common navigation
alias dd="cd ~/downloads"
alias not="cd ~/notes"
alias dot="cd ~/.dotfiles"

# common commands
alias pac='(){sudo pacman -S $1;}'
alias MYEYES='(){xrandr --output HDMI-A-0 --brightness $1;}'
alias niol='clear && zsh'
alias gts='git status'
alias kurmit='(){git add . && git commit -m"$1" && git push;}'
alias gp='git pull'
alias bltm='sudo systemctl start bluetooth && bluetoothctl connect DF:C5:8D:53:1B:94'
alias wifi='(){sudo ~/scripts/wifi.sh "$1";}'
alias cn='bltm && wifi psu'
alias n='nvim .'
alias inkwatch='inkscape-figures watch'
alias inktex='setsid python3 $HOME/.local/bin/inktex/main.py &'
alias sl='source ~/notes/fire/linear/linear-venv/bin/activate'

# tmux
alias tls='tmux ls'
alias ta='(){tmux attach -t \ $1;}'
alias taf='tmux attach -t \ 🔥'
alias taw='tmux attach -t \ 🌊'
alias tae='tmux attach -t \ 🌳'
alias taa='tmux attach -t \ ☁ '
alias tad='tmux attach -t \ 🪙'
alias tan='tmux attach -t \ 📓'
alias td='(){tmux detach}'
alias tr='()tmux-session restore'
alias ts='tmux-session save'
alias tk='tmux kill-session'

# ╔═╗┬  ┬ ┬┌─┐┬┌┐┌┌─┐
# ╠═╝│  │ ││ ┬││││└─┐
# ╩  ┴─┘└─┘└─┘┴┘└┘└─┘

source ~/.local/zsh/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh
source ~/.local/zsh/zsh-autocomplete/zsh-autocomplete.plugin.zsh
source ~/.local/zsh/zsh-autosuggestions/zsh-autosuggestions.zsh
source ~/.local/zsh/zsh-colored-man-pages/colored-man-pages.plugin.zsh

# ╔═╗─┐ ┬┌─┐┌─┐┬─┐┌┬┐┌─┐
# ║╣ ┌┴┬┘├─┘│ │├┬┘ │ └─┐
# ╚═╝┴ └─┴  └─┘┴└─ ┴ └─┘

export EDITOR=nvim
export VISUAL=nvim
export GOPATH="$HOME/.go"
export PATH="$PATH:$HOME/.go/bin"

PATH="$HOME/.perl5/bin${PATH:+:${PATH}}"; export PATH;
PERL5LIB="$HOME/.perl5/lib/perl5${PERL5LIB:+:${PERL5LIB}}"; export PERL5LIB;
PERL_LOCAL_LIB_ROOT="$HOME/.perl5${PERL_LOCAL_LIB_ROOT:+:${PERL_LOCAL_LIB_ROOT}}"; export PERL_LOCAL_LIB_ROOT;
PERL_MB_OPT="--install_base \"$HOME/.perl5\""; export PERL_MB_OPT;
PERL_MM_OPT="INSTALL_BASE=$HOME/.perl5"; export PERL_MM_OPT;

export TEXINPUTS="$HOME/notes/assets/latex//:"
