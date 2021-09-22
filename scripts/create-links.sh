#!/bin/bash

# TODO: do this better

ln -s $HOME/.dotfiles/.Xresources $HOME/
ln -s $HOME/.dotfiles/.xinitrc $HOME/
ln -s $HOME/.dotfiles/.latexmkrc $HOME/
ln -s $HOME/.dotfiles/.zshrc $HOME/
ln -s $HOME/.dotfiles/config/* $HOME/.config/
ln -s $HOME/.dotfiles/resources $HOME/
ln -s $HOME/.dotfiles/scripts $HOME/

mkdir -pv $HOME/.local/bin/
ln -s $HOME/.dotfiles/scripts/bonsai $HOME/.local/bin/
ln -s $HOME/.dotfiles/scripts/tmux-session $HOME/.local/bin/
