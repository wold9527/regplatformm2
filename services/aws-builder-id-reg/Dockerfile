FROM python:3.12-slim

# Camoufox 运行时依赖（Xvfb 虚拟显示 + dbus）
RUN apt-get update && apt-get install -y --no-install-recommends \
    xvfb xauth dbus-x11 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# 预装 Camoufox 浏览器二进制（构建时完成，避免运行时下载）
RUN python -c "from camoufox.utils import install; install()"

COPY server.py .

ENV KIRO_REG_PORT=5076
ENV KIRO_REG_HOST=0.0.0.0
ENV DISPLAY=:99
ENV PYTHONUNBUFFERED=1

EXPOSE 5076

# Xvfb 虚拟显示器 + Python 服务
CMD ["sh", "-c", "Xvfb :99 -screen 0 1920x1080x24 -nolisten tcp & sleep 1 && exec python -u server.py"]
