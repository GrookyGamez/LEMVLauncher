#!/bin/bash
# GUI harness: Xvfb + wine + screenshot.  usage: ./gui.sh [shotname] [seconds]
# Both Xvfb and wine are setsid'd so they survive past this script's session —
# without that they take a SIGHUP when the shell exits and the display dies.
SHOT="${1:-shot}"; WAIT="${2:-8}"
export DISPLAY=:99
export WINEPREFIX=/home/claude/.winelemv
export WINEDEBUG=-all
if ! xdpyinfo >/dev/null 2>&1; then
  pkill -f "Xvfb :99" 2>/dev/null; sleep 1
  rm -f /tmp/.X99-lock /tmp/.X11-unix/X99
  (setsid nohup Xvfb :99 -screen 0 1280x860x24 -nolisten tcp > /tmp/xvfb.log 2>&1 < /dev/null &)
  sleep 3
fi
xdpyinfo >/dev/null 2>&1 || { echo "FATAL: display :99 won't start"; tail -5 /tmp/xvfb.log; exit 1; }
export LEMV_MANIFEST_URL=http://127.0.0.1:8765/manifest.json
export LEMV_JRE_INDEX_URL=http://127.0.0.1:8765/jre/all.json
export LEMV_RESOURCES_URL=http://127.0.0.1:8765/res/
export LEMV_LIBRARIES_URL=http://127.0.0.1:8765/lib/
export LEMV_MSA_URL=http://127.0.0.1:8765/msa
export LEMV_XBL_URL=http://127.0.0.1:8765/xbl
export LEMV_XSTS_URL=http://127.0.0.1:8765/xsts
export LEMV_MCSVC_URL=http://127.0.0.1:8765/mcsvc
export LEMV_LAUNCHWRAPPER_MAVEN=http://127.0.0.1:8765/lwmaven/
export LEMV_OMNIARCHIVE_MANIFEST=http://127.0.0.1:8765/omni/v1/manifest.json
export LEMV_ROOT='Z:\tmp\lemv-wine'
mkdir -p /tmp/lemv-wine
(setsid nohup wine /home/claude/grooky/LEMVLauncher.exe > /tmp/wine.log 2>&1 < /dev/null &)
sleep "$WAIT"
xdotool search --name "LEMV Launcher" >/dev/null 2>&1 && echo "window mapped" || echo "NO WINDOW"
import -window root "/home/claude/grooky/shots/$SHOT.png" && echo "shot: shots/$SHOT.png"
