#!/usr/bin/env bash

options=$(echo "go python typeScript solidity bash lua rust react nio notes dotfiles" | tr ' ' '\n')
tsLearn="https://fireship.io/courses/react-next-firebase/"

# ╔═╗┬ ┬┌┐┌┌─┐
# ╠╣ │ │││││
# ╚  └─┘┘└┘└─┘

begin () {
    bspc config focus_follows_pointer false
    bspc desktop -f ^"$1"
    bspc config single_monocle  false
    bspc desktop -l tiled
    alacritty & sleep 0.25
}

end () {
    bspc config focus_follows_pointer true
    bspc config single_monocle  true
    notify-send -u low -t 1500 init "$1" -i "$HOME/dotfiles/resources/icons/tmux_icon.svg"
}

open () {
    begin "$1"
    layout "$2"
    end "$2"
}

web () {
    firefox --new-window "$1" & sleep 1
}

cava () {
    alacritty -e cava & sleep 0.5
}

basic_down () {
    cava
    bspc config split_ratio 0.895
    bspc node -p north
    web "$1"
}

basic_up () {
    web "$1"
    bspc config split_ratio 0.105
    bspc node -p north
    cava
}

# ╦  ┌─┐┬ ┬┌─┐┬ ┬┌┬┐┌─┐
# ║  ├─┤└┬┘│ ││ │ │ └─┐
# ╩═╝┴ ┴ ┴ └─┘└─┘ ┴ └─┘

layout () {
    bspc config split_ratio 0.321

    case $1 in
        dotfiles)
            basic_down https://github.com/nosvagor/"$1"
            ;;
        notes)
            basic_up https://github.com/nosvagor/"$1"
            ;;
        react|go|solidity|python|lua|bash|rust)
            basic_down
            ;;
        typeScript)
            web "$tsLearn"
            bspc config split_ratio 0.60
            bspc node -p south
            web localhost:3000
            ;;
    esac

    bspc node -f west.local
    bspc config split_ratio 0.528
}

# ╔═╗┌─┐┬  ┌─┐┌─┐┌┬┐┬┌─┐┌┐┌
# ╚═╗├┤ │  ├┤ │   │ ││ ││││
# ╚═╝└─┘┴─┘└─┘└─┘ ┴ ┴└─┘┘└┘

chosen="$(echo -e "$options" | rofi -theme ~/.config/rofi/layouts.rasi -p "ﱖ" -dmenu)"
case $chosen in
    dotfiles)
        open 4 "$chosen"
        ;;
    notes)
        open 5 "$chosen"
        ;;
    go|typeScript|solidity|python|bash|lua|rust|react)
        open 3 "$chosen"
        ;;
    nio)
        open 4 dotfiles
        open 5 notes
        bspc desktop -f 6
        ;;
esac

exit 0
