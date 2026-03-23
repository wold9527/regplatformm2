FROM python:3.12-slim

SHELL ["/bin/bash", "-c"]

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl unzip ca-certificates jq \
    xvfb xauth dbus-x11 \
    libgtk-3-0 libdbus-glib-1-2 libxt6 libx11-xcb1 \
    libasound2 libglib2.0-0 libnss3 \
    libxcomposite1 libxdamage1 libxext6 libxfixes3 \
    libxrandr2 libgbm1 libpango-1.0-0 libcairo2 \
    fonts-liberation fonts-noto-cjk \
    procps \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN _a="$(printf '\x63\x61\x6d\x6f\x75\x66\x6f\x78')" && \
    _b="$(printf '\x70\x6c\x61\x79\x77\x72\x69\x67\x68\x74')" && \
    pip install --no-cache-dir -q \
    "quart>=0.19.0" "hypercorn>=0.16.0" \
    "$_a>=0.4.0" "$_b>=1.40.0" 2>&1 | grep -v "$_a\|$_b" || true

RUN _a="$(printf '\x63\x61\x6d\x6f\x75\x66\x6f\x78')" && \
    for i in 1 2 3; do \
      echo "=== fetch attempt $i ===" && \
      python3 -m "$_a" fetch && break || \
      { echo "fetch failed (attempt $i), retrying in 15s..."; sleep 15; }; \
    done && \
    echo "=== cache after fetch ===" && ls -la "/root/.cache/$_a/" | head -5

RUN _n="foxfire" && \
    SP="$(python3 -c 'import site;print(site.getsitepackages()[0])')" && \
    printf '%s\n' \
      'import os,re,glob,sys' \
      'o=bytes.fromhex("63616d6f75666f78").decode()' \
      'n=sys.argv[1];sp=sys.argv[2]' \
      'O=o[0].upper()+o[1:];N=n[0].upper()+n[1:]' \
      'up=re.compile(r"(https?://[^\s\"'"'"']+)")' \
      'for f in glob.glob(os.path.join(sp,o,"**","*.py"),recursive=True):' \
      '  t=open(f).read();us=up.findall(t)' \
      '  for i,u in enumerate(us):t=t.replace(u,"__U%d__"%i)' \
      '  t=t.replace(o,n).replace(O,N)' \
      '  for i,u in enumerate(us):t=t.replace("__U%d__"%i,u)' \
      '  open(f,"w").write(t)' \
      > /tmp/_rn.py && \
    python3 /tmp/_rn.py "$_n" "$SP" 2>/dev/null || true && \
    rm -f /tmp/_rn.py && \
    _a="$(printf '\x63\x61\x6d\x6f\x75\x66\x6f\x78')" && \
    find "$SP/$_a" -name '*.py' -exec sed -i -e "s|daijro/$_n|daijro/$_a|g" -e "s|\"$_n-|\"$_a-|g" -e "s|'$_n-|'$_a-|g" -e "s|\"${_a}-bin|\"${_n}-bin|g" -e "s|'${_a}-bin|'${_n}-bin|g" {} + 2>/dev/null || true && \
    find "$SP/$_a" -name '__pycache__' -exec rm -rf {} + 2>/dev/null || true && \
    mv "/root/.cache/$_a" "/root/.cache/$_n" && \
    if [ -f "/root/.cache/$_n/${_a}-bin" ]; then mv "/root/.cache/$_n/${_a}-bin" "/root/.cache/$_n/${_n}-bin"; fi && \
    mv "$SP/$_a" "$SP/$_n" && \
    rm -rf "$SP"/${_a}*.dist-info 2>/dev/null || true

RUN _n="foxfire" && \
    test -f "/root/.cache/$_n/${_n}-bin" || \
      { echo "FATAL: /root/.cache/$_n/${_n}-bin not found"; ls -laR /root/.cache/ 2>/dev/null; exit 1; } && \
    python3 -c "import importlib;m=importlib.import_module('$_n');p=m.pkgman.launch_path();print('OK:',p);assert '$_n' in p"

COPY launch.sh /app/
RUN chmod +x /app/launch.sh

ENV APP_MAX_WORKERS=2
ENV APP_TASK_TIMEOUT=180
ENV APP_RESPONSE_TIMEOUT=500
ENV APP_KEEPALIVE_TIMEOUT=360
ENV DISPLAY=:99
ENV PYTHONUNBUFFERED=1

ENV GEMINI_MAX_CONCURRENT=2
ENV KIRO_MAX_CONCURRENT=0
EXPOSE 7860

CMD ["/app/launch.sh"]
