#!/usr/bin/env bash

# TODO: do this better

ln -sfn $HOME/dotfiles/home/.* $HOME/
ln -sfn $HOME/dotfiles/config/* $HOME/.config/
ln -sfn $HOME/dotfiles/scripts/* $HOME/.local/bin/

ln -sfn $HOME/dotfiles/resources/page.css $HOME/.config/youtube-music-desktop-app/custom/css
