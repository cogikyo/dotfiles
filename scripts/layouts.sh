#!/usr/bin/env bash

# ╦  ╦┌─┐┬─┐
# ╚╗╔╝├─┤├┬┘
#  ╚╝ ┴ ┴┴└─

rofi_command="rofi -theme ~/.config/rofi/layouts.rasi"
options=`echo "notes dotfiles latex\n" | tr ' ' '\n'`

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
  notify-send -u low -t 3000 init $1
  exit 0
}

open () {
  begin $1
  basic_phat $2
  end $2
}

pdf () {
  bspc desktop -f 2
  case $1 in
    strang-solutions)
    bspc rule -a Zathura -o state=floating rectangle=1160x1960+1361+69
    ;;
    thomas)
    bspc rule -a Zathura -o state=floating rectangle=1520x2000+1161+69
    ;;
  esac
  zathura ~/textbooks/$1.pdf & sleep 0.25
  bspc desktop -f 3
}

# ╦  ┌─┐┬ ┬┌─┐┬ ┬┌┬┐┌─┐
# ║  ├─┤└┬┘│ ││ │ │ └─┐
# ╩═╝┴ ┴ ┴ └─┘└─┘ ┴ └─┘

basic_phat () {

  basic () {
    bspc config split_ratio 0.3005
    alacritty -e cava & sleep 0.5
    bspc config split_ratio 0.895
    bspc node -p north
  }

  tex () {
    bspc config split_ratio 0.3005
    firefox --new-window https://calendar.google.com/calendar/u/0/r \
    & sleep 1
    bspc config split_ratio 0.6439
    bspc node -p east
    zathura ~/notes/fire/$1/$2.pdf & sleep 0.5
    bspc config split_ratio 0.78
    bspc node -p south
    alacritty -e cava & sleep 0.25
    bspc config split_ratio 0.295
    bspc node -p north
    zathura ~/notes/fire/$1/$1.pdf & sleep 0.5
    bspc node -f west
    bspc config split_ratio 0.954
    bspc node -p south
    zathura ~/textbooks/$3.pdf & sleep 0.25
    bspc node -f west.local
    bspc config split_ratio 0.725
    alacritty & sleep 0.25
  }

  case $1 in
    dotfiles|notes|latex)
      basic
      firefox --new-window https://github.com/nosvagor/$1 & sleep 1
      bspc node -f west.local
      ;;
  esac

  bspc config split_ratio 0.52
}

# ╔═╗┌─┐┬  ┌─┐┌─┐┌┬┐┬┌─┐┌┐┌
# ╚═╗├┤ │  ├┤ │   │ ││ ││││
# ╚═╝└─┘┴─┘└─┘└─┘ ┴ ┴└─┘┘└┘

chosen="$(echo -e "$options" | $rofi_command -p "ﱖ" -dmenu)"
case $chosen in
  notes)
    open 3 notes
    ;;
  dotfiles)
    open 4 dotfiles
    ;;
  latex)
    open 5 latex
    ;;
esac
