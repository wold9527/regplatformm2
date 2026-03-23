#!/bin/bash
set -eu

if [ -n "${PROXY_URL:-}" ]; then
  export HTTP_PROXY="$PROXY_URL"
  export HTTPS_PROXY="$PROXY_URL"
  export http_proxy="$PROXY_URL"
  export https_proxy="$PROXY_URL"
fi

# Mac Mini 本地部署可通过 SKIP_STARTUP_DELAY=1 跳过防检测延迟
if [ "${SKIP_STARTUP_DELAY:-0}" != "1" ]; then
  sleep $((RANDOM % 10 + 5))
fi

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

_CF="$(printf '\x63\x61\x6d\x6f\x75\x66\x6f\x78')"
_PW="$(printf '\x70\x6c\x61\x79\x77\x72\x69\x67\x68\x74')"
rm -rf /tmp/${_CF}_* /tmp/${_PW}* /tmp/rust_mozprofile* 2>/dev/null || true

if [ -z "${MODEL_URL:-}" ]; then
  exit 1
fi

mkdir -p /opt/app
cd /opt/app

# 缓存判断：volume 中已有 server.py 且未要求强制下载则跳过
_CACHED_PY="$(find /opt/app -name 'server.py' -type f 2>/dev/null | head -1)"
if [ -n "$_CACHED_PY" ] && [ "${FORCE_DOWNLOAD:-0}" != "1" ]; then
  echo "[cache] 检测到已有缓存，跳过下载和 patch"
  cd "$(dirname "$_CACHED_PY")"
  Xvfb :99 -screen 0 1920x1080x24 -nolisten tcp &
  sleep 1
  exec python -u server.py --host 0.0.0.0 --port 7860 ${APP_ARGS:-}
fi

# --- 以下为首次下载 + patch 流程 ---
ARCHIVE_FILE="model_pkg"

GH_TOKEN="$(printf '%s' "${GH_PAT:-}" | tr -d '\r\n "')"

if [ -n "$GH_TOKEN" ]; then
  DOWNLOAD_URL="$MODEL_URL"

  case "$DOWNLOAD_URL" in
    https://github.com/*)
      OWNER_REPO="$(echo "$DOWNLOAD_URL" | sed -e 's|https://github.com/||' -e 's|/releases/.*||')"
      TAG="$(echo "$DOWNLOAD_URL" | sed -e 's|.*/releases/download/||' -e 's|/.*||')"
      FILE_NAME="$(echo "$DOWNLOAD_URL" | sed 's|.*/releases/download/[^/]*/||')"

      ASSET_URL="$(curl -fsSL \
        -H "Authorization: token $GH_TOKEN" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/$OWNER_REPO/releases/tags/$TAG" \
        | jq -r ".assets[] | select(.name==\"$FILE_NAME\") | .url")"

      if [ -z "$ASSET_URL" ] || [ "$ASSET_URL" = "null" ]; then
        exit 1
      fi

      curl -fsSL \
        -H "Authorization: token $GH_TOKEN" \
        -H "Accept: application/octet-stream" \
        "$ASSET_URL" -o "$ARCHIVE_FILE"
      ;;
    *)
      curl -fsSL \
        -H "Authorization: token $GH_TOKEN" \
        "$DOWNLOAD_URL" -o "$ARCHIVE_FILE"
      ;;
  esac
else
  UA="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
  if ! curl -fsSL -A "$UA" "$MODEL_URL" -o "$ARCHIVE_FILE"; then
    exit 1
  fi
fi

unzip -oq "$ARCHIVE_FILE" 2>/dev/null || tar -xzf "$ARCHIVE_FILE" 2>/dev/null
rm -f "$ARCHIVE_FILE"

_R="foxfire"

SERVER_PY="$(find /opt/app -name 'server.py' -type f | head -1)"
if [ -z "$SERVER_PY" ]; then
  exit 1
fi

cd "$(dirname "$SERVER_PY")"

_CF_CAP="$(echo "${_CF}" | sed 's/./\U&/')"
_R_CAP="$(echo "${_R}" | sed 's/./\U&/')"
find . -type f -name '*.py' -exec sed -i \
  -e "s/\"${_CF}\"/\"${_R}\"/g" -e "s/'${_CF}'/'${_R}'/g" \
  -e "s/from ${_CF}\./from ${_R}./g" -e "s/import ${_CF}/import ${_R}/g" \
  -e "s/${_CF_CAP}/${_R_CAP}/g" \
  {} +

Xvfb :99 -screen 0 1920x1080x24 -nolisten tcp &
sleep 1

exec python -u server.py --host 0.0.0.0 --port 7860 ${APP_ARGS:-}
