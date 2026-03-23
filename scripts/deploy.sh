#!/usr/bin/env bash
# 服务器一键部署脚本
# 用法：./scripts/deploy.sh [--all | app | turnstile-solver | openai-reg | ...]
# 默认只更新 app 服务；--all 更新全部服务

set -euo pipefail

COMPOSE_FILE="docker-compose.prod.yml"
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[deploy]${NC} $*"; }
warn() { echo -e "${YELLOW}[deploy]${NC} $*"; }

# 登录 GitHub Container Registry（首次需要）
login_ghcr() {
  # 从 docker-compose.prod.yml 中读取镜像名，或使用环境变量
  local IMAGE="${GHCR_IMAGE:-ghcr.io/xiaolajiaoyyds/regplatformm:latest}"
  if ! docker pull "$IMAGE" >/dev/null 2>&1; then
    warn "ghcr.io 未登录或无权限，请先执行："
    warn "  echo \$GITHUB_TOKEN | docker login ghcr.io -u xiaolajiaoyyds --password-stdin"
    exit 1
  fi
}

# 确保 .env 存在
if [ ! -f .env ]; then
  warn ".env 文件不存在，从 .env.example 复制..."
  cp .env.example .env
  warn "请编辑 .env 后重新运行部署脚本"
  exit 1
fi

SERVICE="${1:-app}"

if [ "$SERVICE" = "--all" ]; then
  log "拉取全部镜像..."
  docker compose -f "$COMPOSE_FILE" pull
  log "重启全部服务..."
  docker compose -f "$COMPOSE_FILE" up -d
else
  log "拉取 $SERVICE 镜像..."
  docker compose -f "$COMPOSE_FILE" pull "$SERVICE"
  log "重启 $SERVICE..."
  docker compose -f "$COMPOSE_FILE" up -d "$SERVICE"
fi

log "清理旧镜像..."
docker image prune -f >/dev/null 2>&1 || true

log "当前服务状态："
docker compose -f "$COMPOSE_FILE" ps

log "部署完成！"
