#!/bin/bash
set -e

PORT="${KIRO_REG_PORT:-5076}"
HOST="${KIRO_REG_HOST:-0.0.0.0}"

echo "[aws-builder-id-reg] 启动 AWS Builder ID 注册服务 ${HOST}:${PORT}"
exec python server.py --host "$HOST" --port "$PORT"
