#!/usr/bin/env bash

# TODO: do this better

ln -sfn $HOME/.dotfiles/config-surface/.Xresources $HOME/
for i in $(ls  ~/.dotfiles/config-surface/config/); do rm -rf "~/.config/${i%%}"; done
ln -sfn $HOME/.dotfiles/config-surface/config/* $HOME/.config/
