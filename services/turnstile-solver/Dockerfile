FROM python:3.12-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    xvfb xauth dbus-x11 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# 安装 Camoufox 浏览器二进制
RUN python -c "from camoufox.utils import install; install()"

COPY . .

ENV DISPLAY=:99

# 启动 Xvfb + Solver
CMD ["sh", "-c", "Xvfb :99 -screen 0 1280x720x24 -nolisten tcp & sleep 1 && python api_solver.py --browser_type camoufox --no-headless --thread ${SOLVER_THREADS:-2} --host 0.0.0.0 --port ${SOLVER_PORT:-5072}"]

EXPOSE 5072
