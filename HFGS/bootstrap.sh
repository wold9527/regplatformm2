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

if [ -z "${PKG_URL:-}" ]; then
  echo "Error: PKG_URL is not set." >&2
  exit 1
fi

mkdir -p /opt/run
cd /opt/run

# 缓存判断：volume 中已有可执行文件则跳过下载
_CACHED_BIN="$(find /opt/run -maxdepth 1 -type f ! -name '*.md' ! -name '*.txt' ! -name '*.json' ! -name '*.yml' 2>/dev/null | head -1)"
if [ -n "$_CACHED_BIN" ] && [ "${FORCE_DOWNLOAD:-0}" != "1" ]; then
  echo "[cache] 检测到已有缓存，跳过下载"
  chmod +x "$_CACHED_BIN"
  exec "$_CACHED_BIN" ${RUN_ARGS:-}
fi

# --- 首次下载 ---
ARCHIVE="bundle"

AUTH="$(printf '%s' "${GH_PAT:-}" | tr -d '\r\n "')"

if [ -n "$AUTH" ]; then
  SRC="$PKG_URL"

  case "$SRC" in
    https://github.com/*)
      REPO="$(echo "$SRC" | sed -e 's|https://github.com/||' -e 's|/releases/.*||')"
      VER="$(echo "$SRC" | sed -e 's|.*/releases/download/||' -e 's|/.*||')"
      FNAME="$(echo "$SRC" | sed 's|.*/releases/download/[^/]*/||')"

      ASSET="$(curl -fsSL \
        -H "Authorization: token $AUTH" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/$REPO/releases/tags/$VER" \
        | jq -r ".assets[] | select(.name==\"$FNAME\") | .url")"

      if [ -z "$ASSET" ] || [ "$ASSET" = "null" ]; then
        echo "Error: Package '$FNAME' not found in '$VER'." >&2
        exit 1
      fi

      curl -fsSL \
        -H "Authorization: token $AUTH" \
        -H "Accept: application/octet-stream" \
        "$ASSET" -o "$ARCHIVE"
      ;;
    *)
      curl -fsSL -H "Authorization: token $AUTH" "$SRC" -o "$ARCHIVE"
      ;;
  esac
else
  UA="Mozilla/5.0 (X11; Ubuntu; Linux x86_64) AppleWebKit/537.36"
  if ! curl -fsSL -A "$UA" "$PKG_URL" -o "$ARCHIVE"; then
    echo "Error: Fetch failed." >&2
    exit 1
  fi
fi

unzip -oq "$ARCHIVE" 2>/dev/null || tar -xzf "$ARCHIVE" 2>/dev/null
rm -f "$ARCHIVE"

BIN="$(find /opt/run -maxdepth 1 -type f ! -name '*.md' ! -name '*.txt' ! -name '*.json' ! -name '*.yml' | head -1)"
if [ -z "$BIN" ]; then
  echo "Error: No executable found." >&2
  exit 1
fi
chmod +x "$BIN"
exec "$BIN" ${RUN_ARGS:-}
