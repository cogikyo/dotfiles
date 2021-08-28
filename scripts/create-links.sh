#!/bin/bash

ln -s $HOME/.dotfiles/.xinitrc $HOME/
ln -s $HOME/.dotfiles/.Xresources $HOME/
ln -s $HOME/.dotfiles/.zshrc $HOME/
ln -s $HOME/.dotfiles/config/* $HOME/.config/
ln -s $HOME/.dotfiles/resources $HOME/
ln -s $HOME/.dotfiles/scripts $HOME/

exit 0

