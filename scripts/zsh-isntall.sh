#!/bin/bash
DIR="$HOME/.local/zsh"
mkdir -pv "$DIR"
git -C "$DIR" clone https://github.com/zsh-users/zsh-syntax-highlighting.git
git -C "$DIR" clone https://github.com/zsh-users/zsh-autosuggestions.git
git -C "$DIR" clone https://github.com/marlonrichert/zsh-autocomplete.git
git -C "$DIR" clone https://github.com/ael-code/zsh-colored-man-pages.git
