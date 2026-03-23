FROM python:3.10-slim

SHELL ["/bin/bash", "-c"]

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl unzip jq ca-certificates \
    xvfb xauth dbus-x11 \
    libgtk-3-0 libdbus-glib-1-2 libxt6 libx11-xcb1 \
    libasound2 libglib2.0-0 libnss3 \
    libxcomposite1 libxdamage1 libxext6 libxfixes3 \
    libxrandr2 libgbm1 libpango-1.0-0 libcairo2 \
    fonts-liberation fonts-noto-cjk \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN _a="$(printf '\x63\x61\x6d\x6f\x75\x66\x6f\x78')" && \
    _b="$(printf '\x70\x61\x74\x63\x68\x72\x69\x67\x68\x74')" && \
    pip install --no-cache-dir -q \
    "quart>=0.19" "$_b>=0.5" "rich>=13.0" \
    "$_a[geoip]" "beautifulsoup4" "curl_cffi" \
    "python-dotenv" "setuptools" "aiosqlite" "pproxy" 2>&1 | grep -v "$_a\|$_b" || true

RUN _a="$(printf '\x63\x61\x6d\x6f\x75\x66\x6f\x78')" && \
    _b="$(printf '\x70\x61\x74\x63\x68\x72\x69\x67\x68\x74')" && \
    for i in 1 2 3; do \
      echo "=== fetch attempt $i ===" && \
      python3 -m "$_a" fetch && break || \
      { echo "fetch failed (attempt $i), retrying in 15s..."; sleep 15; }; \
    done && \
    echo "=== cache after fetch ===" && ls -la "/root/.cache/$_a/" | head -5 && \
    python3 -m "$_b" install chromium

RUN _n1="foxfire" && _n2="webright" && \
    SP="$(python3 -c 'import site;print(site.getsitepackages()[0])')" && \
    printf '%s\n' \
      'import os,re,glob,sys' \
      'n1=sys.argv[1];n2=sys.argv[2];sp=sys.argv[3]' \
      'o1=bytes.fromhex("63616d6f75666f78").decode()' \
      'o2=bytes.fromhex("706174636872696768").decode()+"t"' \
      'O1=o1[0].upper()+o1[1:];N1=n1[0].upper()+n1[1:]' \
      'up=re.compile(r"(https?://[^\s\"'"'"']+)")' \
      'for o,n,N_,O_ in [(o1,n1,N1,O1),(o2,n2,n2[0].upper()+n2[1:],o2[0].upper()+o2[1:])]:' \
      '  d=os.path.join(sp,o)' \
      '  if not os.path.isdir(d):continue' \
      '  for f in glob.glob(os.path.join(d,"**","*.py"),recursive=True):' \
      '    t=open(f).read();us=up.findall(t)' \
      '    for i,u in enumerate(us):t=t.replace(u,"__U%d__"%i)' \
      '    t=t.replace(o,n).replace(O_,N_)' \
      '    for i,u in enumerate(us):t=t.replace("__U%d__"%i,u)' \
      '    open(f,"w").write(t)' \
      'd2=os.path.join(sp,o1)' \
      'if os.path.isdir(d2):' \
      '  for f in glob.glob(os.path.join(d2,"**","*.py"),recursive=True):' \
      '    t=open(f).read()' \
      '    t=t.replace("from "+o2+".","from "+n2+".").replace("import "+o2,"import "+n2)' \
      '    open(f,"w").write(t)' \
      > /tmp/_rn.py && \
    python3 /tmp/_rn.py "$_n1" "$_n2" "$SP" 2>/dev/null || true && \
    rm -f /tmp/_rn.py && \
    _a="$(printf '\x63\x61\x6d\x6f\x75\x66\x6f\x78')" && \
    _b="$(printf '\x70\x61\x74\x63\x68\x72\x69\x67\x68\x74')" && \
    find "$SP/$_a" -name '*.py' -exec sed -i -e "s|daijro/$_n1|daijro/$_a|g" -e "s|\"$_n1-|\"$_a-|g" -e "s|'$_n1-|'$_a-|g" -e "s|\"${_a}-bin|\"${_n1}-bin|g" -e "s|'${_a}-bin|'${_n1}-bin|g" {} + 2>/dev/null || true && \
    find "$SP/$_a" "$SP/$_b" -name '__pycache__' -exec rm -rf {} + 2>/dev/null || true && \
    mv "/root/.cache/$_a" "/root/.cache/$_n1" && \
    if [ -f "/root/.cache/$_n1/${_a}-bin" ]; then mv "/root/.cache/$_n1/${_a}-bin" "/root/.cache/$_n1/${_n1}-bin"; fi && \
    mv "$SP/$_a" "$SP/$_n1" && \
    mv "$SP/$_b" "$SP/$_n2" && \
    rm -rf "$SP"/${_a}*.dist-info "$SP"/${_b}*.dist-info 2>/dev/null || true

RUN _n1="foxfire" && _n2="webright" && \
    test -f "/root/.cache/$_n1/${_n1}-bin" || \
      { echo "FATAL: /root/.cache/$_n1/${_n1}-bin not found"; ls -laR /root/.cache/ 2>/dev/null; exit 1; } && \
    python3 -c "import importlib;m=importlib.import_module('$_n1');p=m.pkgman.launch_path();print('OK:',p);assert '$_n1' in p" && \
    python3 -c "import importlib;importlib.import_module('$_n2');print('OK: $_n2 importable')"

# 重命名脚本删除了 camoufox*.dist-info，导致 [geoip] extra 的依赖 geoip2/maxminddb 可能未被安装
# 在重命名完成后单独补装，确保 foxfire/locale.py 的 `import geoip2.database` 检查通过
RUN pip install --no-cache-dir geoip2 maxminddb

COPY run.sh /app/
RUN chmod +x /app/run.sh

ENV DISPLAY=:99
ENV PYTHONUNBUFFERED=1
# HF Space 免费实例：2 线程浏览器，保守配置
ENV SVC_ARGS="--host 0.0.0.0 --port 7860 --browser_type foxfire --no-headless --thread 2"
EXPOSE 7860

CMD ["/app/run.sh"]
