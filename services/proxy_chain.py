"""
代理链工具 — 在环境变量代理（Clash）和后端代理之间建立链式转发。

链路：App → Clash(HTTP_PROXY) → 后端代理 → 目标

使用方式：
    from proxy_chain import chain_proxy, curl_proxy_args

    # playwright / camoufox / requests — 替换代理 URL
    proxy_url = chain_proxy("http://user:pass@us-proxy:8080")
    # 返回 "http://127.0.0.1:xxxxx"（本地链式代理），或原样返回

    # curl — 获取额外参数
    extra_args = curl_proxy_args("http://user:pass@us-proxy:8080")
    # 返回 ["--preproxy", "http://clash:7890"] 或 []
"""

import base64
import logging
import os
import select
import socket
import threading
from urllib.parse import urlparse

logger = logging.getLogger("proxy-chain")

# ─── 环境变量检测 ─────────────────────────────────────────────────────────────

def _get_env_proxy() -> str:
    """读取环境变量中的代理地址（HTTPS_PROXY 优先）"""
    for var in ("HTTPS_PROXY", "HTTP_PROXY", "https_proxy", "http_proxy"):
        val = os.environ.get(var, "").strip()
        if val:
            return val
    return ""


# ─── curl 代理链 ─────────────────────────────────────────────────────────────

def curl_proxy_args(backend_proxy: str) -> list:
    """
    返回 curl 的代理链参数。
    当环境变量有代理（Clash）且有后端代理时，返回 ["--preproxy", clash_url]，
    curl 会先通过 Clash 建立隧道，再连后端代理。
    """
    env_proxy = _get_env_proxy()
    if env_proxy and backend_proxy:
        return ["--preproxy", env_proxy]
    return []


# ─── 通用代理链（playwright / camoufox / requests）─────────────────────────────

_chain_servers = {}   # backend_proxy_url -> "http://127.0.0.1:port"
_chain_lock = threading.Lock()


def chain_proxy(backend_proxy: str) -> str:
    """
    代理链入口。返回应该使用的代理 URL：
      - 有 Clash + 有后端代理 → 启动本地链式代理，返回 "http://127.0.0.1:port"
      - 只有后端代理 → 原样返回
      - 无后端代理 → 返回空串（让调用方走默认 / 环境变量）
    """
    if not backend_proxy:
        return ""

    env_proxy = _get_env_proxy()
    if not env_proxy:
        return backend_proxy  # 无 Clash，直连后端代理

    with _chain_lock:
        # double-check：锁内检查 + 创建，避免 TOCTOU 竞态
        if backend_proxy in _chain_servers:
            return _chain_servers[backend_proxy]

        # 启动本地链式代理
        srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        srv.bind(("127.0.0.1", 0))
        port = srv.getsockname()[1]
        srv.listen(32)

        bp = urlparse(backend_proxy)
        clash = urlparse(env_proxy)

        # 后端代理认证信息（注入到转发请求中）
        proxy_auth_header = ""
        if bp.username:
            cred = f"{bp.username}:{bp.password or ''}"
            b64 = base64.b64encode(cred.encode()).decode()
            proxy_auth_header = f"Proxy-Authorization: Basic {b64}\r\n"

        bp_host = bp.hostname
        bp_port = bp.port or (443 if bp.scheme == "https" else 80)
        clash_host = clash.hostname or "127.0.0.1"
        clash_port = clash.port or 7890

        t = threading.Thread(
            target=_accept_loop,
            args=(srv, clash_host, clash_port, bp_host, bp_port, proxy_auth_header, backend_proxy),
            daemon=True,
        )
        t.start()

        local_url = f"http://127.0.0.1:{port}"
        _chain_servers[backend_proxy] = local_url

        logger.info("代理链已启动: %s → Clash(%s:%d) → %s:%d",
                    local_url, clash_host, clash_port, bp_host, bp_port)
        return local_url


# ─── 内部实现 ─────────────────────────────────────────────────────────────────

def _accept_loop(srv, clash_host, clash_port, bp_host, bp_port, proxy_auth, cache_key):
    try:
        while True:
            client, _ = srv.accept()
            threading.Thread(
                target=_handle_client,
                args=(client, clash_host, clash_port, bp_host, bp_port, proxy_auth),
                daemon=True,
            ).start()
    except Exception:
        pass
    finally:
        _safe_close(srv)
        # 清理缓存，下次调用 chain_proxy 时会重建
        with _chain_lock:
            _chain_servers.pop(cache_key, None)


def _handle_client(client, clash_host, clash_port, bp_host, bp_port, proxy_auth):
    """处理单个客户端连接：Clash → 后端代理 双跳 CONNECT"""
    tunnel = None
    try:
        # 1. 读取客户端请求头
        header = b""
        while b"\r\n\r\n" not in header:
            chunk = client.recv(8192)
            if not chunk:
                return
            header += chunk

        first_line = header.split(b"\r\n")[0].decode("utf-8", errors="replace")

        # 2. TCP 连接 Clash
        tunnel = socket.create_connection((clash_host, clash_port), timeout=15)

        # 3. 通过 Clash CONNECT 到后端代理
        connect_to_bp = (
            f"CONNECT {bp_host}:{bp_port} HTTP/1.1\r\n"
            f"Host: {bp_host}:{bp_port}\r\n"
            f"\r\n"
        )
        tunnel.sendall(connect_to_bp.encode())

        resp = _read_http_header(tunnel)
        if resp is None or b"200" not in resp.split(b"\r\n")[0]:
            client.sendall(b"HTTP/1.1 502 Bad Gateway\r\n\r\n")
            return

        # 4. 现在 tunnel 是到后端代理的连接
        if first_line.upper().startswith("CONNECT"):
            # CONNECT 请求：注入认证后转发给后端代理
            target = first_line.split()[1]  # host:port
            fwd = (
                f"CONNECT {target} HTTP/1.1\r\n"
                f"Host: {target}\r\n"
                f"{proxy_auth}"
                f"\r\n"
            )
            tunnel.sendall(fwd.encode())

            bp_resp = _read_http_header(tunnel)
            if bp_resp is None:
                client.sendall(b"HTTP/1.1 502 Bad Gateway\r\n\r\n")
                return
            client.sendall(bp_resp)
            _pipe(client, tunnel)
        else:
            # 普通 HTTP 代理请求：注入认证头后转发
            if proxy_auth:
                # 在请求头中插入 Proxy-Authorization
                parts = header.split(b"\r\n", 1)
                header = parts[0] + b"\r\n" + proxy_auth.encode() + parts[1]
            tunnel.sendall(header)
            _pipe(client, tunnel)

    except Exception as e:
        logger.debug("代理链连接异常: %s", e)
        try:
            client.sendall(b"HTTP/1.1 502 Bad Gateway\r\n\r\n")
        except Exception:
            pass
    finally:
        _safe_close(client)
        _safe_close(tunnel)


def _read_http_header(sock, timeout=30) -> bytes | None:
    """读取完整的 HTTP 响应头（直到 \\r\\n\\r\\n）"""
    sock.settimeout(timeout)
    buf = b""
    try:
        while b"\r\n\r\n" not in buf:
            chunk = sock.recv(8192)
            if not chunk:
                return None
            buf += chunk
    except socket.timeout:
        return None
    finally:
        sock.settimeout(None)
    return buf


def _pipe(sock1, sock2):
    """双向数据管道"""
    pair = [sock1, sock2]
    try:
        while True:
            readable, _, errored = select.select(pair, [], pair, 120)
            if errored or not readable:
                break
            for s in readable:
                data = s.recv(65536)
                if not data:
                    return
                other = sock2 if s is sock1 else sock1
                other.sendall(data)
    except Exception:
        pass


def _safe_close(sock):
    if sock:
        try:
            sock.close()
        except Exception:
            pass
