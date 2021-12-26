#!/usr/bin/env bash

TMPFILE="$(mktemp -t screencast-XXXXXXX).mkv"
OUTPUT="$HOME/recordings/gifs/$(date +%m-%d-%y__%H':'%M':'%S)"

read -r X Y W H G ID < <(slop -f "%x %y %w %h %g %i")
notify-send -u Critical -t 3000 started Recording

ffmpeg -f x11grab -s "$W"x"$H" -i :0.0+$X,$Y "$TMPFILE"
ffmpeg -y -i "$TMPFILE"  -vf fps=10,palettegen /tmp/palette.jpg
ffmpeg -i "$TMPFILE" -i /tmp/palette.jpg -filter_complex "paletteuse" $OUTPUT.gif
mv $TMPFILE $OUTPUT.mkv

notify-send -t 5000 "total size: $(du -h $OUTPUT.gif | awk '{print $1}')" "Gif Done"

trap "rm -f '$TMPFILE'; rm ~/recordings/gifs/*.mkv" 0
