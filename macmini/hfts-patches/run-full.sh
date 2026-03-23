#!/bin/bash
set -eu

# 清理可能残留的 Xvfb 锁文件（快速重启循环时可能残留）
rm -f /tmp/.X99-lock /tmp/.X11-unix/X99

# 启动虚拟显示
Xvfb :99 -screen 0 1280x720x24 -nolisten tcp &

# 等待 Xvfb 就绪（最多 10 秒），替代不可靠的 sleep 1
for i in $(seq 1 20); do
  if [ -e /tmp/.X11-unix/X99 ]; then
    break
  fi
  sleep 0.5
done

if [ ! -e /tmp/.X11-unix/X99 ]; then
  echo "[FATAL] Xvfb 启动失败，退出" >&2
  exit 1
fi

cd /opt/app
exec python -u app.py ${SVC_ARGS:---host 0.0.0.0 --port 7860 --browser_type camoufox --no-headless --thread 2}
