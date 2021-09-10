#!/usr/bin/env bash

rofi_command="rofi -theme ~/.config/rofi/layouts.rasi"

#Options
notes="notes"
dotfiles="dotfiles"
golang="golang"
calc="calculus"
javascript="javascript"
bash="bash"
python="python"
linear="linear"
pandas="pandas"
solidity="solidity"
c="c++"
stat="statistics"

# Variable passed to rofi
options="$calc\n$linear\n$c\n$golang\n$solidity\n$javascript\n$bash\n$python\n\
  $notes\n$dotfiles\n$pandas\n$stat"

chosen="$(echo -e "$options" | $rofi_command -p "" -dmenu)"
case $chosen in
    $notes)
		./scripts/basic-launch.sh notes
        ;;
    $dotfiles)
		./scripts/basic-launch.sh .dotfiles
        ;;
    $golang)
		./scripts/code-launch.sh water/golang golang
        ;;
    $c)
		./scripts/code-launch.sh water/c++ c++
        ;;
    $bash)
        ./scripts/code-launch.sh water/bash bash
        ;;
    $calc)
        ./scripts/tex-launch.sh fire/calculus calculus
        ;;
    $solidity)
        ./scripts/code-launch.sh water/solidity
        ;;
    $python)
        ./scripts/code-launch.sh water/python python
        ;;
    $javascript)
        ./scripts/code-launch.sh water/javascript javascript
        ;;
    $pandas)
		./scripts/code-launch.sh water/python/pandas
        ;;
    $linear)
		./scripts/code-launch.sh fire/linear linear
        ;;
    $stat)
		./scripts/code-launch.sh fire/statistics statistics
        ;;
esac
