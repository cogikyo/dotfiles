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

# load starship prompt ~/.config/starship.toml
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
alias notes="cd ~/notes"
alias dot="cd ~/.dotfiles"

# common commands
alias sps="sudo pacman -S"
alias spr="sudo pacman -R"
alias MYEYES='(){xrandr --output HDMI-A-0 --brightness $1;}'
alias niol='clear && zsh'

# ╔═╗┬  ┬ ┬┌─┐┬┌┐┌┌─┐
# ╠═╝│  │ ││ ┬││││└─┐
# ╩  ┴─┘└─┘└─┘┴┘└┘└─┘

source ~/.local/zsh/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh
source ~/.local/zsh/zsh-autocomplete/zsh-autocomplete.plugin.zsh
source ~/.local/zsh/zsh-autosuggestions/zsh-autosuggestions.zsh
source ~/.local/zsh/zsh-colored-man-pages/colored-man-pages.plugin.zsh
