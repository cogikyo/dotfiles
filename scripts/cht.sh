#!/usr/bin/env bash

languages=`echo "go lua cpp javascript typescript python rust slidity" | tr ' ' '\n'`
core_utils=`echo "cd ls cat mkdir rmdir rm cp mv ln chown chmod dd tar less find diff grep sed awk dmesg lsblk mount umount su kill" | tr ' ' '\n'`

selected=`printf "$languages\n$core_utils" | fzf `
read -p "query: " query

if printf $languages | grep -qs $selected; then
  tmux neww bash -c "curl cht.sh/$selected/`echo $query | tr ' ' '+'` & while [ : ]; do sleep 1; done"
else
  tmux neww bash -c "curl cht.sh/$selected~$query & while [ : ]; do sleep 1; done"
fi

