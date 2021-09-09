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
options="$golang\n$solidity\n$javascript\n$bash\n$c\n$python\n\
$notes\n$dotfiles\n$calc\n$linear\n$pandas\n$stat"

chosen="$(echo -e "$options" | $rofi_command -p "" -dmenu)"
case $chosen in
    $notes)
		./scripts/basic-launch.sh notes
        ;;
    $dotfiles)
		./scripts/basic-launch.sh .dotfiles
        ;;
    $golang)
		./scripts/code-launch.sh golang
        ;;
    $c)
		./scripts/code-launch.sh c++
        ;;
    $bash)
        ./scripts/code-launch.sh bash
        ;;
    $calc)
        ./scripts/tex-launch.sh fire/calculus calculus
        ;;
    $solidity)
        ./scripts/code-launch.sh solidity
        ;;
    $python)
        ./scripts/code-launch.sh python
        ;;
    $javascript)
        ./scripts/code-launch.sh javascript
        ;;
    $pandas)
		./scripts/code-launch.sh water/python/pandas
        ;;
    $linear)
		./scripts/code-launch.sh fire/linear
        ;;
    $stat)
		./scripts/code-launch.sh fire/statistics
        ;;
esac
