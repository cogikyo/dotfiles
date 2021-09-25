#!/usr/bin/env bash

# TODO: do this better

ln -sfn $HOME/.dotfiles/.Xresources $HOME/
ln -sfn $HOME/.dotfiles/.xinitrc $HOME/
ln -sfn $HOME/.dotfiles/.latexmkrc $HOME/
ln -sfn $HOME/.dotfiles/.zshrc $HOME/
ln -sfn $HOME/.dotfiles/config/* $HOME/.config/
ln -sfn $HOME/.dotfiles/resources $HOME/
ln -sfn $HOME/.dotfiles/scripts $HOME/

mkdir -pv $HOME/.local/bin/
ln -sfn $HOME/.dotfiles/bin/* $HOME/.local/bin/
