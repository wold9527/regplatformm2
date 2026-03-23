#!/bin/bash
set -eu


SHM_SIZE="${APP_SHM_SIZE:-512m}"
if mount -o remount,size="$SHM_SIZE" /dev/shm 2>/dev/null; then
  :
else
  if mount -t tmpfs -o size="$SHM_SIZE" tmpfs /tmp/browser-shm 2>/dev/null; then
    export TMPDIR=/tmp/browser-shm
  else
    mkdir -p /tmp/browser-shm
    export TMPDIR=/tmp/browser-shm
  fi
fi


Xvfb :99 -screen 0 1920x1080x24 -nolisten tcp &
sleep 1

cd /opt/app
exec python -u server.py --host 0.0.0.0 --port 7860 ${APP_ARGS:-}
