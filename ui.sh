#!/bin/bash
export DISPLAY=:99
xdpyinfo >/dev/null 2>&1 || { echo "DISPLAY DEAD"; exit 1; }
case "$1" in
  geom) WID=$(xdotool search --name "LEMV Launcher"|head -1); echo "wid=$WID"; xdotool getwindowgeometry "$WID";;
  click) xdotool mousemove "$2" "$3" click 1; sleep "${5:-2}"; import -window root "/home/claude/grooky/shots/$4.png" && echo "shot: shots/$4.png";;
  move) xdotool mousemove "$2" "$3"; sleep 1; import -window root "/home/claude/grooky/shots/$4.png" && echo "shot: shots/$4.png";;
  shot) import -window root "/home/claude/grooky/shots/$2.png" && echo "shot: shots/$2.png";;
esac
