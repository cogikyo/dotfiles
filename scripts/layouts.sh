#!/usr/bin/env bash
# ╦  ╦┌─┐┬─┐
# ╚╗╔╝├─┤├┬┘
#  ╚╝ ┴ ┴┴└─

rofi_command="rofi -theme ~/.config/rofi/layouts.rasi"
options=`echo "go python typeScript solidity react nio notes dotfiles" | tr ' ' '\n'`
reactPath='typeScript/react'

# ╔═╗┬ ┬┌┐┌┌─┐
# ╠╣ │ │││││
# ╚  └─┘┘└┘└─┘

begin () {
    bspc config focus_follows_pointer false
    bspc desktop -f ^$1
    bspc desktop -l tiled
    alacritty & sleep 0.25
}

end () {
    bspc config focus_follows_pointer true
    notify-send -u low -t 1500 init $1 -i "$HOME/dotfiles/resources/icons/tmux_icon.svg"
}

open () {
    begin $1
    basics $2
    end $2
}

web () {
    firefox --new-window & sleep 1
}

term () {
    alacritty -e cava & sleep 0.5
}

# ╦  ┌─┐┬ ┬┌─┐┬ ┬┌┬┐┌─┐
# ║  ├─┤└┬┘│ ││ │ │ └─┐
# ╩═╝┴ ┴ ┴ └─┘└─┘ ┴ └─┘

basics () {
    bspc config split_ratio 0.3005

basic_down () {
    term
    bspc config split_ratio 0.895
    bspc node -p north
    web
}

basic_up () {
    web
    bspc config split_ratio 0.105
    bspc node -p north
    term
}

case $1 in
    dotfiles)
        basic_down
        ;;
    notes)
        basic_up
        ;;
    react|go|typescript|solidity|python)
        basic_down
        ;;
esac

bspc node -f west.local
bspc config split_ratio 0.52
}

# ╔═╗┌─┐┬  ┌─┐┌─┐┌┬┐┬┌─┐┌┐┌
# ╚═╗├┤ │  ├┤ │   │ ││ ││││
# ╚═╝└─┘┴─┘└─┘└─┘ ┴ ┴└─┘┘└┘

chosen="$(echo -e "$options" | $rofi_command -p "ﱖ" -dmenu)"
case $chosen in
    dotfiles)
        open 4 $chosen
        ;;
    notes)
        open 5 $chosen
        ;;
    go|typeScript|solidity|python)
        open 3 $chosen
        ;;
    react)
        open 3 $chosen
        ;;
    nio)
        open 4 dotfiles
        open 5 notes
        bspc desktop -f 6
        ;;
esac

exit 0
