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

echo $size

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
  bspc config split_ratio 0.547
}

basic_phat () {

  basic () {
    bspc config split_ratio 0.3005
    alacritty -e cava & sleep 0.5
    bspc config split_ratio 0.896
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
  }

  case $1 in
    dotfiles|notes)
      basic
      firefox --new-window & sleep 1
      ;;
    cpp)
      basic
      firefox --new-window https://github.com/nosvagor/notes/tree/main/water/$1 \
      & sleep 1
      ;;
    linear)
      tex linear mth-343/applied-linear strang
      ;;
    calculus)
      tex calculus mth-253/calc-iii rogawski
      ;;
  esac

  bspc node -f west.local
  bspc config split_ratio 0.3005
}

###

end () {
  bspc config focus_follows_pointer true
  notify-send -u low -t 3000 init $1
  exit 0
}

open () {
  begin
  basic_$size $1
  end $1
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
