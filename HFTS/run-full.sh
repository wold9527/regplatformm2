#!/bin/bash
set -eu

# 启动虚拟显示
Xvfb :99 -screen 0 1280x720x24 -nolisten tcp &
sleep 1

cd /opt/app
exec python -u app.py ${SVC_ARGS:---host 0.0.0.0 --port 7860 --browser_type camoufox --no-headless --thread 2}
