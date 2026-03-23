#!/usr/bin/env python3
"""HF Space 批量部署脚本 — 创建随机化 Space 并上传 Dockerfile + 入口脚本

用法:
  python scripts/hf_deploy.py --service openai --count 10 --release-url <GitHub Release URL>
  python scripts/hf_deploy.py --service grok   --count 5  --release-url <URL>
  python scripts/hf_deploy.py --service kiro   --count 5  --release-url <URL>
  python scripts/hf_deploy.py --service ts     --count 5  --release-url <URL> --extra-secrets '{"SVC_ARGS":"--host 0.0.0.0 --port 7860"}'

依赖:
  pip install huggingface_hub requests
"""

import argparse
import json
import os
import random
import secrets
import sys
import time
from pathlib import Path

try:
    from huggingface_hub import CommitOperationAdd, HfApi
except ImportError:
    print("错误: 请先安装依赖 → pip install huggingface_hub requests", file=sys.stderr)
    sys.exit(1)

# ─── 项目根目录 ───
REPO_ROOT = Path(__file__).resolve().parent.parent

# ────────────────────────────────────────────────────────────────────────────
# 随机化词库
# ────────────────────────────────────────────────────────────────────────────

ADJECTIVES = [
    # 原有 48 个
    "crystal", "amber", "silver", "iron", "coral", "jade", "onyx", "ruby",
    "azure", "cobalt", "bronze", "pearl", "maple", "cedar", "frost", "storm",
    "lunar", "solar", "arctic", "ember", "velvet", "carbon", "neon", "zinc",
    "copper", "golden", "misty", "rapid", "silent", "vivid", "stark", "swift",
    "noble", "prime", "keen", "bold", "calm", "deep", "fair", "warm",
    "light", "dark", "pure", "wild", "cool", "thin", "vast", "flat",
    # 新增 ~72 个（自然/技术/科学词汇，3-8 字母）
    "bright", "crisp", "dense", "eager", "fresh", "grand", "harsh", "ideal",
    "jolly", "lucid", "mellow", "narrow", "oval", "plain", "quiet", "rough",
    "sharp", "tight", "ultra", "vital", "woven", "young", "zonal", "agile",
    "blunt", "civic", "dusty", "exact", "fluid", "giant", "hazy", "inner",
    "joint", "lush", "minor", "natal", "outer", "polar", "raw", "solid",
    "tidal", "urban", "vivid", "wiry", "xenon", "yearly", "zen", "atomic",
    "binary", "cubic", "dual", "elastic", "focal", "global", "hex", "ionic",
    "kinetic", "linear", "modal", "neural", "optic", "pixel", "quartz", "radial",
    "scalar", "tensor", "unified", "vector", "warp", "xray", "yield", "zero",
]

NOUNS = [
    # 原有 48 个
    "wave", "spark", "field", "node", "flow", "gate", "link", "mesh",
    "hub", "core", "beam", "edge", "grid", "port", "sync", "task",
    "loop", "pipe", "rack", "dock", "shard", "vault", "pulse", "forge",
    "lens", "prism", "delta", "nexus", "ridge", "crest", "bloom", "drift",
    "shore", "cliff", "creek", "grove", "peak", "vale", "glade", "brook",
    "stone", "flame", "cloud", "leaf", "root", "seed", "wing", "arch",
    # 新增 ~72 个（自然/技术/科学词汇，3-8 字母）
    "bolt", "cage", "dart", "echo", "fern", "glow", "hive", "iris",
    "jade", "knot", "loom", "mist", "nest", "orb", "pond", "quay",
    "reef", "sail", "tide", "urn", "vine", "well", "yard", "zone",
    "alloy", "blade", "coil", "disc", "fiber", "grain", "helix", "index",
    "jewel", "kite", "lance", "motor", "nerve", "oxide", "plank", "quill",
    "relay", "scope", "tower", "unity", "valve", "wedge", "axiom", "byte",
    "cell", "diode", "flux", "graph", "hash", "input", "joule", "laser",
    "matrix", "nano", "orbit", "phase", "queue", "rune", "sigma", "token",
    "unit", "voxel", "watt", "xenon", "yoke", "zeta", "atlas", "basin",
]

# 35 个伪装主题，随机选择生成 README
THEMES = [
    {"title": "ML Inference Server", "emoji": "\U0001f916", "desc": "Lightweight model serving endpoint"},
    {"title": "Log Aggregator", "emoji": "\U0001f4ca", "desc": "Real-time log collection and forwarding"},
    {"title": "Config Sync Agent", "emoji": "\u2699\ufe0f", "desc": "Distributed configuration management"},
    {"title": "Health Monitor", "emoji": "\U0001f493", "desc": "Service health checking and alerting"},
    {"title": "Task Scheduler", "emoji": "\U0001f4c5", "desc": "Cron-like task scheduling service"},
    {"title": "Webhook Relay", "emoji": "\U0001f517", "desc": "HTTP webhook forwarding proxy"},
    {"title": "Metrics Collector", "emoji": "\U0001f4c8", "desc": "Prometheus-compatible metrics aggregation"},
    {"title": "Edge Cache", "emoji": "\u26a1", "desc": "CDN edge caching layer"},
    {"title": "Auth Gateway", "emoji": "\U0001f510", "desc": "OAuth2 authentication gateway"},
    {"title": "Stream Processor", "emoji": "\U0001f30a", "desc": "Real-time data stream processing"},
    {"title": "API Gateway Lite", "emoji": "\U0001f310", "desc": "Minimal reverse proxy and API gateway"},
    {"title": "Data Relay Service", "emoji": "\U0001f504", "desc": "Lightweight data forwarding service"},
    {"title": "Queue Worker", "emoji": "\U0001f4ec", "desc": "Background job processing engine"},
    {"title": "Feature Flag Service", "emoji": "\U0001f3c1", "desc": "Dynamic feature toggle management"},
    {"title": "Rate Limiter", "emoji": "\U0001f6a6", "desc": "Distributed rate limiting service"},
    {"title": "Secret Rotator", "emoji": "\U0001f511", "desc": "Automated credential rotation agent"},
    {"title": "DNS Resolver", "emoji": "\U0001f5fa\ufe0f", "desc": "Custom DNS resolution service"},
    {"title": "File Sync Agent", "emoji": "\U0001f4c1", "desc": "Cross-region file synchronization"},
    {"title": "Notification Hub", "emoji": "\U0001f514", "desc": "Multi-channel notification dispatcher"},
    {"title": "Circuit Breaker", "emoji": "\U0001f6e1\ufe0f", "desc": "Service resilience and fault isolation"},
    {"title": "Load Balancer", "emoji": "\u2696\ufe0f", "desc": "Layer 7 traffic distribution engine"},
    {"title": "Trace Collector", "emoji": "\U0001f50d", "desc": "Distributed tracing data aggregation"},
    {"title": "Schema Registry", "emoji": "\U0001f4cb", "desc": "API schema versioning and validation"},
    {"title": "Event Bus", "emoji": "\U0001f68c", "desc": "Lightweight event-driven messaging"},
    {"title": "Canary Deploy Agent", "emoji": "\U0001f424", "desc": "Gradual rollout and canary analysis"},
    {"title": "Service Mesh Proxy", "emoji": "\U0001f578\ufe0f", "desc": "Sidecar proxy for service mesh traffic"},
    {"title": "Artifact Store", "emoji": "\U0001f4e6", "desc": "Binary artifact storage and retrieval"},
    {"title": "Pipeline Runner", "emoji": "\u25b6\ufe0f", "desc": "CI/CD pipeline execution engine"},
    {"title": "Dependency Scanner", "emoji": "\U0001f50e", "desc": "Automated dependency vulnerability scanning"},
    {"title": "Config Validator", "emoji": "\u2705", "desc": "Configuration file linting and validation"},
    {"title": "Log Router", "emoji": "\U0001f4e8", "desc": "Multi-destination log routing service"},
    {"title": "Backup Agent", "emoji": "\U0001f4be", "desc": "Scheduled backup and snapshot management"},
    {"title": "Token Refresh Service", "emoji": "\U0001f504", "desc": "OAuth token lifecycle management"},
    {"title": "Proxy Pool Manager", "emoji": "\U0001f3ca", "desc": "Upstream proxy rotation and health tracking"},
    {"title": "Batch Processor", "emoji": "\u2699\ufe0f", "desc": "Scheduled batch data processing engine"},
]

HF_COLORS = ["red", "yellow", "green", "blue", "indigo", "purple", "pink", "gray"]

LICENSES = ["mit", "apache-2.0", "bsd-3-clause", "bsd-2-clause", "isc", "mpl-2.0", "unlicense"]

# README Features 描述池（随机抽 3 条）
FEATURES_POOL = [
    "HTTP/HTTPS reverse proxy",
    "Health check endpoints",
    "Configurable routing rules",
    "Binary protocol support",
    "Minimal resource footprint",
    "REST API with streaming responses",
    "Docker-based deployment",
    "Concurrent session handling",
    "Low-latency edge compute",
    "Configurable upstream routing",
    "Real-time data processing",
    "Batch inference support",
    "WebSocket support",
    "Prometheus metrics export",
    "Graceful shutdown handling",
    "TLS termination",
    "Request rate limiting",
    "gRPC endpoint support",
    "Horizontal auto-scaling",
    "JSON structured logging",
]

# ────────────────────────────────────────────────────────────────────────────
# Service 类型映射（读取仓库中现有的 Dockerfile / 入口脚本）
# ────────────────────────────────────────────────────────────────────────────

SERVICE_MAP = {
    "openai": {
        "dir": REPO_ROOT / "HFNP",
        "dockerfile": "Dockerfile",
        "script": "init.sh",
        "url_secret": "ARTIFACT_URL",
    },
    "grok": {
        "dir": REPO_ROOT / "HFGS",
        "dockerfile": "Dockerfile",
        "script": "bootstrap.sh",
        "url_secret": "PKG_URL",
    },
    "kiro": {
        "dir": REPO_ROOT / "HFKR",
        "dockerfile": "Dockerfile",
        "script": "start.sh",
        "url_secret": "MODEL_URL",
    },
    "gemini": {
        "dir": REPO_ROOT / "HFGM",
        "dockerfile": "Dockerfile",
        "script": "start.sh",
        "url_secret": "MODEL_URL",
    },
    "ts": {
        "dir": REPO_ROOT / "HFTS",
        "dockerfile": "Dockerfile",
        "script": "run.sh",
        "url_secret": "DATA_URL",
    },
}


# ────────────────────────────────────────────────────────────────────────────
# 随机化生成函数
# ────────────────────────────────────────────────────────────────────────────


def generate_space_name() -> str:
    """生成随机 Space 名称，随机选择多种命名模式之一"""
    pattern = random.randint(0, 4)
    if pattern == 0:
        # adj-noun-hex4
        return f"{random.choice(ADJECTIVES)}-{random.choice(NOUNS)}-{secrets.token_hex(2)}"
    elif pattern == 1:
        # noun-adj-hex4（反转）
        return f"{random.choice(NOUNS)}-{random.choice(ADJECTIVES)}-{secrets.token_hex(2)}"
    elif pattern == 2:
        # adj-noun-noun（双名词，确保不同）
        n1, n2 = random.sample(NOUNS, 2)
        return f"{random.choice(ADJECTIVES)}-{n1}-{n2}"
    elif pattern == 3:
        # noun-hex6（简短）
        return f"{random.choice(NOUNS)}-{secrets.token_hex(3)}"
    else:
        # adj-adj-noun（双形容词，确保不同）
        a1, a2 = random.sample(ADJECTIVES, 2)
        return f"{a1}-{a2}-{random.choice(NOUNS)}"


def generate_readme() -> str:
    """从模板池随机生成 README（不同 title/emoji/colors/license/description/features/usage/api/badges）"""
    theme = random.choice(THEMES)
    c1, c2 = random.sample(HF_COLORS, 2)
    lic = random.choice(LICENSES)
    features = random.sample(FEATURES_POOL, k=min(4, len(FEATURES_POOL)))

    features_md = "\n".join(f"- {f}" for f in features)

    # 随机 badge 图片
    badge_pool = [
        f"![Build](https://img.shields.io/badge/build-passing-brightgreen)",
        f"![License](https://img.shields.io/badge/license-{lic}-blue)",
        f"![Docker](https://img.shields.io/badge/docker-ready-blue)",
        f"![Version](https://img.shields.io/badge/version-{random.randint(1,5)}.{random.randint(0,9)}.{random.randint(0,9)}-green)",
        f"![Status](https://img.shields.io/badge/status-active-success)",
        f"![Coverage](https://img.shields.io/badge/coverage-{random.randint(80,99)}%25-brightgreen)",
    ]
    badges = " ".join(random.sample(badge_pool, k=random.randint(1, 3)))

    # Usage section 模板
    usage_templates = [
        "```bash\ndocker pull ghcr.io/app:latest\ndocker run -p 7860:7860 ghcr.io/app:latest\n```",
        "```bash\ncurl -X POST http://localhost:7860/api/v1/run -H 'Content-Type: application/json'\n```",
        "```bash\ndocker compose up -d\ncurl http://localhost:7860/health\n```",
        "```bash\nexport PORT=7860\n./entrypoint.sh\n```",
    ]

    # API section 模板
    api_templates = [
        "| Endpoint | Method | Description |\n|----------|--------|-------------|\n| `/health` | GET | Health check |\n| `/api/v1/run` | POST | Execute task |",
        "- `GET /health` - Service health check\n- `POST /api/v1/process` - Submit processing job\n- `GET /api/v1/status` - Check job status",
        "| Route | Description |\n|-------|-------------|\n| `GET /` | Service info |\n| `GET /health` | Liveness probe |\n| `POST /submit` | Submit workload |",
    ]

    readme = (
        f"---\n"
        f"title: {theme['title']}\n"
        f"emoji: {theme['emoji']}\n"
        f"colorFrom: {c1}\n"
        f"colorTo: {c2}\n"
        f"sdk: docker\n"
        f"pinned: false\n"
        f"license: {lic}\n"
        f"short_description: {theme['desc']}\n"
        f"---\n"
        f"\n"
        f"# {theme['title']}\n"
        f"\n"
        f"{badges}\n"
        f"\n"
        f"{theme['desc']}.\n"
        f"Built for edge deployment with minimal resource footprint.\n"
        f"\n"
        f"## Features\n"
        f"{features_md}\n"
        f"\n"
        f"## Usage\n"
        f"{random.choice(usage_templates)}\n"
        f"\n"
        f"## API\n"
        f"{random.choice(api_templates)}\n"
    )
    return readme


# 入口脚本文件名池（随机选择，增加指纹多样性）
SCRIPT_NAME_POOL = [
    "entrypoint.sh", "main.sh", "launch.sh", "setup.sh", "app.sh",
    "init.sh", "bootstrap.sh", "start.sh", "run.sh", "serve.sh",
]


def randomize_dockerfile(original: str) -> str:
    """对 Dockerfile 进行随机化处理，使每份文件 hash 唯一

    - 随机在不同位置插入注释
    - 随机化 WORKDIR 路径
    - 随机添加 EXPOSE 端口注释
    - 随机化 ENV 变量名前缀
    """
    lines = original.rstrip().split("\n")
    build_id = secrets.token_hex(4)

    # 1. 随机在不同位置插入 build-id 注释（不在第一行 FROM 之前）
    comment_lines = [
        f"# build-id: {build_id}",
        f"# generated: {secrets.token_hex(3)}",
    ]
    # 随机选一个插入位置（跳过第一行 FROM）
    insert_pos = random.randint(1, max(1, len(lines) - 1))
    lines.insert(insert_pos, random.choice(comment_lines))

    # 2. 随机化 WORKDIR 路径（如果存在 WORKDIR /app，替换为随机路径）
    workdir_variants = ["/app", "/opt/app", "/srv/app", "/home/app", "/workspace", "/opt/service"]
    for i, line in enumerate(lines):
        stripped = line.strip()
        if stripped.startswith("WORKDIR "):
            lines[i] = f"WORKDIR {random.choice(workdir_variants)}"
            break

    # 3. 随机添加 EXPOSE 端口注释
    if random.random() < 0.5:
        expose_comment = f"# EXPOSE {random.choice([7860, 8080, 8000, 3000, 5000])}"
        lines.append(expose_comment)

    # 4. 随机化 ENV 变量名前缀（添加一个无害的 ENV）
    env_prefixes = ["APP", "SVC", "NODE", "RUNTIME", "PROC"]
    env_line = f"ENV {random.choice(env_prefixes)}_BUILD_HASH={build_id}"
    # 插入到 FROM 之后的某个位置
    env_pos = random.randint(1, min(3, len(lines) - 1))
    lines.insert(env_pos, env_line)

    return "\n".join(lines) + "\n"


# ────────────────────────────────────────────────────────────────────────────
# HF API 操作
# ────────────────────────────────────────────────────────────────────────────


def load_tokens(token_file: str) -> list:
    """从 JSON 文件加载 HF Token 列表"""
    path = Path(token_file)
    if not path.exists():
        print(f"错误: Token 文件不存在 -> {path}", file=sys.stderr)
        sys.exit(1)
    with open(path) as f:
        tokens = json.load(f)
    if not isinstance(tokens, list) or len(tokens) == 0:
        print("错误: Token 文件应为非空 JSON 数组", file=sys.stderr)
        sys.exit(1)
    return tokens


def get_accounts(tokens: list) -> list:
    """通过 whoami 接口获取每个 Token 对应的 HF 用户名"""
    accounts = []
    for i, token in enumerate(tokens):
        api = HfApi(token=token)
        try:
            info = api.whoami()
            username = info["name"]
            accounts.append({"token": token, "username": username, "api": api})
            print(f"  Token #{i + 1}: {username}")
        except Exception as e:
            print(f"  Token #{i + 1}: 认证失败 -> {e}", file=sys.stderr)
    if not accounts:
        print("错误: 没有可用的 HF 账号", file=sys.stderr)
        sys.exit(1)
    return accounts


def deploy_one_space(
    account: dict,
    service_cfg: dict,
    release_url: str,
    gh_pat: str,
    extra_secrets: dict,
) -> str:
    """部署单个 HF Space，返回 Space URL

    流程: 创建 Space -> 上传文件（单次 commit） -> 设置 Secrets
    Space 名称重名时自动重试（最多 3 次）
    """
    api: HfApi = account["api"]
    username: str = account["username"]
    svc_dir: Path = service_cfg["dir"]

    # 读取模板文件（所有 Space 共享同一份 Dockerfile 和入口脚本内容）
    dockerfile_text = (svc_dir / service_cfg["dockerfile"]).read_text()
    script_text = (svc_dir / service_cfg["script"]).read_text()
    original_script_name = service_cfg["script"]

    # 随机选择入口脚本文件名（增加指纹多样性）
    script_name = random.choice(SCRIPT_NAME_POOL)

    # 尝试创建 Space（名称冲突时重试）
    space_name = None
    repo_id = None
    for _attempt in range(3):
        space_name = generate_space_name()
        repo_id = f"{username}/{space_name}"
        try:
            api.create_repo(
                repo_id=repo_id,
                repo_type="space",
                space_sdk="docker",
                private=False,
            )
            break
        except Exception as e:
            if "already exists" in str(e).lower() and _attempt < 2:
                continue
            raise

    # Dockerfile 随机化处理（替换脚本名 + 多维度随机化）
    dockerfile_text = dockerfile_text.replace(original_script_name, script_name)
    dockerfile_final = randomize_dockerfile(dockerfile_text)

    # 生成随机 README
    readme_text = generate_readme()

    # 一次性上传所有文件（单次 commit，只触发一次构建）
    operations = [
        CommitOperationAdd(path_in_repo="README.md", path_or_fileobj=readme_text.encode()),
        CommitOperationAdd(path_in_repo="Dockerfile", path_or_fileobj=dockerfile_final.encode()),
        CommitOperationAdd(path_in_repo=script_name, path_or_fileobj=script_text.encode()),
    ]
    api.create_commit(
        repo_id=repo_id,
        repo_type="space",
        operations=operations,
        commit_message="Initial deployment",
    )

    # 设置 Secrets
    api.add_space_secret(repo_id=repo_id, key=service_cfg["url_secret"], value=release_url)
    if gh_pat:
        api.add_space_secret(repo_id=repo_id, key="GH_PAT", value=gh_pat)
    for k, v in extra_secrets.items():
        api.add_space_secret(repo_id=repo_id, key=k, value=v)

    # 构造 Space URL（HF 格式: username-spacename.hf.space，全小写）
    slug = f"{username}-{space_name}".lower()
    return f"https://{slug}.hf.space"


# ────────────────────────────────────────────────────────────────────────────
# 入口
# ────────────────────────────────────────────────────────────────────────────


def main():
    parser = argparse.ArgumentParser(
        description="HF Space 批量部署脚本",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument("--service", required=True, choices=list(SERVICE_MAP.keys()), help="服务类型")
    parser.add_argument("--count", type=int, required=True, help="部署数量")
    parser.add_argument("--release-url", required=True, help="GitHub Release 下载地址（写入 Space Secret）")
    parser.add_argument("--gh-pat", default=os.environ.get("GH_PAT", ""), help="GitHub PAT（默认读 GH_PAT 环境变量）")
    parser.add_argument("--tokens", default=str(REPO_ROOT / "scripts" / "hf_tokens.json"), help="HF Token JSON 文件路径")
    parser.add_argument("--extra-secrets", type=json.loads, default={}, help='额外 Secrets，JSON 格式')
    args = parser.parse_args()

    service_cfg = SERVICE_MAP[args.service]
    if not service_cfg["dir"].exists():
        print(f"错误: 模板目录不存在 -> {service_cfg['dir']}", file=sys.stderr)
        sys.exit(1)

    print("=" * 50)
    print(f"  HF Space 批量部署")
    print(f"  服务: {args.service}  数量: {args.count}")
    print(f"  Release: {args.release_url}")
    print("=" * 50)

    print(f"\n[1/3] 加载 HF Token...")
    tokens = load_tokens(args.tokens)
    print(f"  共 {len(tokens)} 个 Token")

    print(f"\n[2/3] 获取账号信息...")
    accounts = get_accounts(tokens)
    print(f"  可用账号: {len(accounts)} 个")

    print(f"\n[3/3] 开始部署...")
    deployed = []
    for i in range(args.count):
        # Round-robin 分配到各账号
        account = accounts[i % len(accounts)]
        try:
            url = deploy_one_space(account, service_cfg, args.release_url, args.gh_pat, args.extra_secrets)
            deployed.append(url)
            print(f"  [{i + 1}/{args.count}] OK {url}")
        except Exception as e:
            print(f"  [{i + 1}/{args.count}] FAIL ({account['username']}): {e}", file=sys.stderr)
        # 短暂间隔，避免 HF API 限流
        if i < args.count - 1:
            time.sleep(1)

    print(f"\n{'=' * 60}")
    print(f"部署完成: {len(deployed)}/{args.count} 成功")
    if deployed:
        env_key = args.service.upper() + "_SPACES"
        print(f"\nCF Worker 环境变量（复制到 {env_key}）:")
        print(" ".join(deployed))
        print()


if __name__ == "__main__":
    main()
