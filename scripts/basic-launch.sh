#!/bin/bash

bspc config focus_follows_pointer false
bspc desktop -l tiled

alacritty &
sleep 0.25

bspc config split_ratio 0.302 && \
firefox --new-window https://github.com/nosvagor/$1 &
sleep 0.25

bspc config split_ratio 0.645 && \
firefox --newwindow &
sleep 0.25

bspc config split_ratio 0.77 && \
alacritty -e cava &
sleep 0.25

bspc config split_ratio 0.52
bspc config focus_follows_pointer true

notify-send -u low init $1 &
exit 0
