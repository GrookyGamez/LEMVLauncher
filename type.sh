#!/bin/bash
export DISPLAY=:99
xdotool mousemove "$1" "$2" click 1; sleep 0.5
xdotool type --delay 60 "$3"; sleep 1
import -window root "/home/claude/grooky/shots/$4.png" && echo "shot: shots/$4.png"
