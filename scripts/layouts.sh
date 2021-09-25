#!/usr/bin/env bash

rofi_command="rofi -theme ~/.config/rofi/layouts.rasi"

options=`echo "calculus linear cpp notes dotfiles\n" | tr ' ' '\n'`

chosen="$(echo -e "$options" | $rofi_command -p "ﱖ" -dmenu)"
case $chosen in
    "notes")
		./scripts/scripts/basic-launch.sh notes
        ;;
    "dotfiles")
		./scripts/scripts/basic-launch.sh .dotfiles
        ;;
    "cpp")
		./scripts/code-launch.sh water/cpp cpp
        ;;
    "calculus")
        ./scripts/scripts/tex-launch.sh fire/calculus calculus
        ;;
    "linear")
		./scripts/code-launch.sh fire/linear linear
        ;;
esac
