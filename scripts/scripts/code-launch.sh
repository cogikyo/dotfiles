#!/bin/bash

# TODO: update for code workflows, not sure what I need yet (copy of basic for now).

bspc config focus_follows_pointer false
bspc desktop -l tiled

alacritty &
sleep 0.25

bspc config split_ratio 0.547 && \
firefox --new-window https://github.com/nosvagor &
sleep 2

bspc config split_ratio 0.8505 && bspc node -p south && \
alacritty -e cava &
sleep 0.25

bspc config split_ratio 0.547
bspc config focus_follows_pointer true

notify-send -u low init $1 &
exit 0

