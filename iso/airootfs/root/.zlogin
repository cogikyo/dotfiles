# Fix permissions (mkarchiso --no-preserve=mode strips execute bits from overlay)
chmod +x ~/dotfiles/install.sh 2>/dev/null
find ~/dotfiles/bin -type f -exec chmod +x {} + 2>/dev/null
find ~/dotfiles -name '*.sh' -exec chmod +x {} + 2>/dev/null

# Print install banner
printf '\n'
printf '\033[1;34m ██████╗  ██████╗ ████████╗███████╗██╗██╗     ███████╗███████╗\033[0m\n'
printf '\033[1;34m ██╔══██╗██╔═══██╗╚══██╔══╝██╔════╝██║██║     ██╔════╝██╔════╝\033[0m\n'
printf '\033[1;34m ██║  ██║██║   ██║   ██║   █████╗  ██║██║     █████╗  ███████╗\033[0m\n'
printf '\033[1;34m ██║  ██║██║   ██║   ██║   ██╔══╝  ██║██║     ██╔══╝  ╚════██║\033[0m\n'
printf '\033[1;34m ██████╔╝╚██████╔╝   ██║   ██║     ██║███████╗███████╗███████║\033[0m\n'
printf '\033[1;34m ╚═════╝  ╚═════╝    ╚═╝   ╚═╝     ╚═╝╚══════╝╚══════╝╚══════╝\033[0m\n'
printf '\n'
printf '  \033[1;33mCustom Arch ISO — offline install with pre-cached packages\033[0m\n'
printf '\n'
printf '  \033[1mInstall steps:\033[0m\n'
printf '    1. Connect to Wi-Fi (optional):  \033[35miwctl\033[0m\n'
printf '    2. Run the installer:            \033[35mbootstrap\033[0m\n'
printf '    3. Reboot and run:               \033[35m~/dotfiles/install.sh all\033[0m\n'
printf '\n'
printf '  \033[90mLocal repo packages: pacman -Sl localrepo\033[0m\n'
printf '  \033[90mDotfiles source:     ~/dotfiles/\033[0m\n'
printf '\n'

# Run automated script (from releng)
~/.automated_script.sh
