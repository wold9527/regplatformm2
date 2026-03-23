#!/bin/bash
# Turnstile Solver 启动脚本
# 用法: ./start.sh [--threads N] [--port PORT] [--proxy] [--browser chrome|chromium|camoufox]

set -e

THREADS=${SOLVER_THREADS:-2}
PORT=${SOLVER_PORT:-5072}
HOST=${SOLVER_HOST:-0.0.0.0}
BROWSER_TYPE=${SOLVER_BROWSER:-chrome}
USE_PROXY=false
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# 激活 venv（如果存在）
if [ -f "${SCRIPT_DIR}/.venv/bin/python" ]; then
    PYTHON="${SCRIPT_DIR}/.venv/bin/python"
else
    PYTHON="python"
fi

# 解析命令行参数
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --threads) THREADS="$2"; shift ;;
        --port) PORT="$2"; shift ;;
        --host) HOST="$2"; shift ;;
        --browser) BROWSER_TYPE="$2"; shift ;;
        --proxy) USE_PROXY=true ;;
        --debug) DEBUG=true ;;
        --default-proxy) DEFAULT_PROXY="$2"; shift ;;
        *) echo "未知参数: $1"; exit 1 ;;
    esac
    shift
done

echo "========================================="
echo "  Turnstile Solver"
echo "========================================="
echo "[*] 浏览器: ${BROWSER_TYPE}"
echo "[*] 线程数: ${THREADS}"
echo "[*] 监听: ${HOST}:${PORT}"
echo "[*] 代理: ${USE_PROXY}"
echo "[*] Python: ${PYTHON}"
echo "========================================="

PROXY_FLAG=""
if [ "$USE_PROXY" = true ]; then
    PROXY_FLAG="--proxy"
fi

DEBUG_FLAG=""
if [ "$DEBUG" = true ]; then
    DEBUG_FLAG="--debug"
fi

DEFAULT_PROXY_FLAG=""
if [ -n "$DEFAULT_PROXY" ]; then
    DEFAULT_PROXY_FLAG="--default-proxy ${DEFAULT_PROXY}"
fi

exec "${PYTHON}" api_solver.py \
    --browser_type "${BROWSER_TYPE}" \
    --thread "${THREADS}" \
    --host "${HOST}" \
    --port "${PORT}" \
    ${PROXY_FLAG} \
    ${DEBUG_FLAG} \
    ${DEFAULT_PROXY_FLAG}
