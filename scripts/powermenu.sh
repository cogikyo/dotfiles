
#!/usr/bin/env bash

uptime=$(uptime -p | sed -e 's/up //g')
rofi_command="rofi -theme ~/.config/rofi/powermenu.rasi"

# Options
shutdown="襤"
reboot="累"
lock=""

# Variable passed to rofi
options="$shutdown\n$lock\n$reboot"

chosen="$(echo -e "$options" | $rofi_command -p "$uptime" -dmenu -selected-row 1)"
case $chosen in
    $shutdown)
		systemctl poweroff -i
        ;;
    $reboot)
		systemctl reboot
        ;;
    $lock)
		env XSECURELOCK_SAVER=saver_mplayer XSECURELOCK_LIST_VIDEOS_COMMAND="find ~/resources/videos -type f" XSECURELOCK_SHOW_HOSTNAME=0 xsecurelock
        ;;
esac
