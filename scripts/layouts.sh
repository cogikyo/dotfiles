#!/usr/bin/env bash

# ╦  ╦┌─┐┬─┐
# ╚╗╔╝├─┤├┬┘
#  ╚╝ ┴ ┴┴└─

rofi_command="rofi -theme ~/.config/rofi/layouts.rasi"
options=`echo "statistics algos discrete nio notes dotfiles" | tr ' ' '\n'`
statPath='stat-361/homework/hw4'
discPath='cs-250/hw_tex/hw4'

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
  basic_phat $2
  end $2
}

pdf () {
  bspc desktop -f 2
  case $1 in
    stat-solutions|discrete)
    bspc rule -a Zathura -o state=floating rectangle=1550x2008+1145+87
    ;;
  esac
  zathura ~/textbooks/$1.pdf & sleep 0.25
  bspc desktop -f 3
}

# ╦  ┌─┐┬ ┬┌─┐┬ ┬┌┬┐┌─┐
# ║  ├─┤└┬┘│ ││ │ │ └─┐
# ╩═╝┴ ┴ ┴ └─┘└─┘ ┴ └─┘

basic_phat () {

  basic_down () {
    bspc config split_ratio 0.3005
    alacritty -e cava & sleep 0.5
    bspc config split_ratio 0.895
    bspc node -p north
    firefox --new-window & sleep 1
    bspc node -f west.local
  }

  basic_up () {
    bspc config split_ratio 0.3005
    firefox --new-window & sleep 1
    bspc config split_ratio 0.105
    bspc node -p north
    alacritty -e cava & sleep 0.5
    bspc node -f west.local
  }


  tex () {
    bspc config split_ratio 0.3005
    firefox --new-window https://calendar.google.com/calendar/u/0/r \
    & sleep 1
    bspc config split_ratio 0.605
    bspc node -p east
    zathura ~/notes/$1/$2.pdf & sleep 0.5
    bspc config split_ratio 0.857
    bspc node -p south
    alacritty -e cava & sleep 0.25
    bspc node -f west
    bspc config split_ratio 0.954
    bspc node -p south
    zathura ~/textbooks/$3.pdf & sleep 0.25
    bspc node -f west.local
  }

  case $1 in
    dotfiles)
      basic_down
      ;;
    notes)
      basic_up
      ;;
    stat)
      tex python $statPath stat
      bspc node -f west.local
      pdf stat-solutions
      ;;
     algos)
      basic_down
      ;;
     discrete)
      tex python $discPath discrete
      bspc node -f west.local
  esac

  bspc config split_ratio 0.52
}

# ╔═╗┌─┐┬  ┌─┐┌─┐┌┬┐┬┌─┐┌┐┌
# ╚═╗├┤ │  ├┤ │   │ ││ ││││
# ╚═╝└─┘┴─┘└─┘└─┘ ┴ ┴└─┘┘└┘

chosen="$(echo -e "$options" | $rofi_command -p "ﱖ" -dmenu)"
case $chosen in
  dotfiles)
    open 4 dotfiles
    ;;
  notes)
    open 5 notes
    ;;
  statistics)
    open 3 stat
    ;;
  algos)
    open 3 algos
    ;;
  discrete)
    open 3 discrete
    ;;
  nio)
    open 4 dotfiles
    open 5 notes
    bspc desktop -f 6
    ;;
esac

exit 0
