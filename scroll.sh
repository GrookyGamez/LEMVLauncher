#!/bin/bash
export DISPLAY=:99
xdotool mousemove "$1" "$2"
for i in $(seq 1 "$3"); do xdotool click 5; sleep 0.15; done
sleep 1
import -window root "/home/claude/grooky/shots/$4.png" && echo "shot: shots/$4.png"
