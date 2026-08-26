#!/bin/sh
# start the mock Mojang/Microsoft server if it isn't already up
pgrep -x mockserver >/dev/null && { echo "mockserver already running"; exit 0; }
cd /home/claude/grooky
(setsid nohup ./mockserver -addr 127.0.0.1:8765 -javaexe /home/claude/grooky/fakejava.exe > mock.log 2>&1 &)
sleep 1
pgrep -x mockserver >/dev/null && echo "mockserver up on 127.0.0.1:8765" || { echo "FAILED"; cat mock.log; }
