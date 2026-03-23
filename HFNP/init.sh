#!/bin/sh
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

if [ -z "${ARTIFACT_URL:-}" ]; then
  echo "Error: ARTIFACT_URL is not set." >&2
  exit 1
fi

mkdir -p /opt/svc
cd /opt/svc

# 缓存判断：volume 中已有可执行文件则跳过下载
_CACHED_BIN="$(find /opt/svc -maxdepth 1 -type f ! -name '*.md' ! -name '*.txt' ! -name '*.json' ! -name '*.yml' 2>/dev/null | head -1)"
if [ -n "$_CACHED_BIN" ] && [ "${FORCE_DOWNLOAD:-0}" != "1" ]; then
  echo "[cache] 检测到已有缓存，跳过下载"
  chmod +x "$_CACHED_BIN"
  exec "$_CACHED_BIN" ${SVC_ARGS:-}
fi

# --- 首次下载 ---
TMP_FILE="payload"

TOKEN="$(printf '%s' "${GH_PAT:-}" | tr -d '\r\n "')"

if [ -n "$TOKEN" ]; then
  URL="$ARTIFACT_URL"

  case "$URL" in
    https://github.com/*)
      REPO="$(echo "$URL" | sed -e 's|https://github.com/||' -e 's|/releases/.*||')"
      REL="$(echo "$URL" | sed -e 's|.*/releases/download/||' -e 's|/.*||')"
      FILE="$(echo "$URL" | sed 's|.*/releases/download/[^/]*/||')"

      ASSET="$(curl -fsSL \
        -H "Authorization: token $TOKEN" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/$REPO/releases/tags/$REL" \
        | jq -r ".assets[] | select(.name==\"$FILE\") | .url")"

      if [ -z "$ASSET" ] || [ "$ASSET" = "null" ]; then
        echo "Error: Artifact '$FILE' not found in '$REL'." >&2
        exit 1
      fi

      curl -fsSL \
        -H "Authorization: token $TOKEN" \
        -H "Accept: application/octet-stream" \
        "$ASSET" -o "$TMP_FILE"
      ;;
    *)
      curl -fsSL -H "Authorization: token $TOKEN" "$URL" -o "$TMP_FILE"
      ;;
  esac
else
  UA="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
  if ! curl -fsSL -A "$UA" "$ARTIFACT_URL" -o "$TMP_FILE"; then
    echo "Error: Download failed." >&2
    exit 1
  fi
fi

unzip -oq "$TMP_FILE" 2>/dev/null || tar -xzf "$TMP_FILE" 2>/dev/null
rm -f "$TMP_FILE"

BIN="$(find /opt/svc -maxdepth 1 -type f ! -name '*.md' ! -name '*.txt' ! -name '*.json' ! -name '*.yml' | head -1)"
if [ -z "$BIN" ]; then
  echo "Error: No executable found." >&2
  exit 1
fi
chmod +x "$BIN"
exec "$BIN" ${SVC_ARGS:-}
