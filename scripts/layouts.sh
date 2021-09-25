#!/usr/bin/env bash

rofi_command="rofi -theme ~/.config/rofi/layouts.rasi"

options=`echo "calculus linear cpp notes dotfiles\n" | tr ' ' '\n'`

# smol = surface pro, phat = 3840 x 2160
if xrandr | grep "eDP-1"
then
  size=smol
else
  size=phat
fi

begin () {
  bspc config focus_follows_pointer false
  bspc desktop -l tiled

  alacritty & sleep 0.25
}

###

basic_smol () {
  bspc config split_ratio 0.547
  bspc node -p east
  alacritty -e cava & sleep 0.5

  bspc config split_ratio 0.851
  bspc node -p north
  case $1 in
    dotfiles|notes)
      firefox --new-window & sleep 2
      ;;
    cpp)
      firefox --new-window https://github.com/nosvagor/notes/tree/main/water/$1 \
      & sleep 2
      ;;
    linear)
      firefox --new-window https://github.com/nosvagor/notes/tree/main/fire/$1 \
      & sleep 2
      ;;
    calculus)
      zathura ~/notes/fire/calculus/$1.pdf & sleep 1
      ;;
  esac

  bspc node -f west.local
}

###

end () {
  bspc config split_ratio 0.547
  bspc config focus_follows_pointer true
  notify-send -u low -t 3000 init $1 &
  exit 0
}

open () {
  begin
  basic_$size $1
  end
}


chosen="$(echo -e "$options" | $rofi_command -p "ﱖ" -dmenu)"
case $chosen in
  notes)
    open notes
    ;;
  dotfiles)
    open dotfiles
    ;;
  cpp)
    open cpp
    ;;
  calculus)
    open calculus
    ;;
  linear)
    open linear
    ;;
esac
