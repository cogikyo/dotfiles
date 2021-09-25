#!/bin/bash

bspc config focus_follows_pointer false
bspc desktop -l tiled

alacritty &
sleep 0.25

bspc config split_ratio 0.547 && \
zathura ~/notes/$1/$2.pdf &
sleep 1

bspc config split_ratio 0.8505 && bspc node -p south && \
alacritty -e cava &
sleep 0.25

bspc config split_ratio 0.547
bspc config focus_follows_pointer true

notify-send -u low $2 notes &
exit 0

