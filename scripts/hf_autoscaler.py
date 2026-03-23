#!/usr/bin/env python3
"""HF Space 弹性管理器 — 自动检测封禁/扩缩容/替换

功能:
  1. 健康检查所有 Space，标记封禁/不可达节点
  2. 封禁节点自动删除 + 创建新 Space 替代（随机命名伪装）
  3. 健康节点不足时自动扩容
  4. 健康节点过剩时自动缩容（删除多余空闲节点）
  5. 更新 CF Worker 环境变量（通过 Cloudflare API）

用法:
  # 检查并自动修复（默认模式）
  python scripts/hf_autoscaler.py --service kiro

  # 只检查不修复（dry-run）
  python scripts/hf_autoscaler.py --service kiro --dry-run

  # 指定目标节点数
  python scripts/hf_autoscaler.py --service kiro --target 10

  # 所有服务
  python scripts/hf_autoscaler.py --service all

依赖:
  pip install huggingface_hub requests
"""

import argparse
import json
import os
import sys
import time
import concurrent.futures
from pathlib import Path

import requests

try:
    from huggingface_hub import HfApi
except ImportError:
    print("Error: pip install huggingface_hub requests", file=sys.stderr)
    sys.exit(1)

REPO_ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(REPO_ROOT / "scripts"))
from hf_deploy import (
    SERVICE_MAP, generate_space_name, generate_readme,
    load_tokens, get_accounts, deploy_one_space,
)

# ────────────────────────────────────────────────────────────────────────────
# 配置
# ────────────────────────────────────────────────────────────────────────────

# CF Worker 环境变量名映射
CF_ENV_MAP = {
    "openai": "OPENAI_SPACES",
    "grok":   "GROK_SPACES",
    "kiro":   "KIRO_SPACES",
    "gemini": "GEMINI_SPACES",
    "ts":     "TS_SPACES",
}

# 健康检查超时（秒）
HEALTH_TIMEOUT = 8

# 默认目标节点数（每个服务）
DEFAULT_TARGET = 5

# 缩容保护：健康节点超过目标数的倍数才缩容
SCALE_DOWN_RATIO = 1.5


# ────────────────────────────────────────────────────────────────────────────
# 健康检查
# ────────────────────────────────────────────────────────────────────────────


def check_space_health(space_url: str) -> dict:
    """检查单个 Space 健康状态，返回 {url, healthy, status, reason}"""
    result = {"url": space_url, "healthy": False, "status": 0, "reason": ""}
    try:
        resp = requests.get(
            space_url.rstrip("/") + "/health",
            timeout=HEALTH_TIMEOUT,
            allow_redirects=True,
        )
        result["status"] = resp.status_code
        if resp.ok:
            result["healthy"] = True
        elif resp.status_code == 403:
            result["reason"] = "banned_or_restricted"
        elif resp.status_code == 404:
            result["reason"] = "not_found_or_deleted"
        elif resp.status_code == 503:
            result["reason"] = "sleeping_or_building"
        else:
            result["reason"] = f"http_{resp.status_code}"
    except requests.exceptions.Timeout:
        result["reason"] = "timeout"
    except requests.exceptions.ConnectionError:
        result["reason"] = "connection_error"
    except Exception as e:
        result["reason"] = str(e)[:80]
    return result


def batch_health_check(space_urls: list) -> list:
    """并发健康检查所有 Space"""
    results = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=20) as pool:
        futures = {pool.submit(check_space_health, url): url for url in space_urls}
        for future in concurrent.futures.as_completed(futures):
            results.append(future.result())
    return results


# ────────────────────────────────────────────────────────────────────────────
# Space 管理
# ────────────────────────────────────────────────────────────────────────────


def find_space_repo_id(api: HfApi, space_url: str) -> str | None:
    """从 Space URL 反推 repo_id（格式: username/space-name）
    URL 格式: https://username-space-name.hf.space
    """
    try:
        host = space_url.replace("https://", "").replace("http://", "").split("/")[0]
        slug = host.replace(".hf.space", "")
        # HF slug 格式: username-space-name，第一个 - 分隔用户名和空间名
        # 但用户名本身可能含 -，所以需要通过 API 查找
        # 尝试列出用户的 spaces 来匹配
        parts = slug.split("-")
        for i in range(1, len(parts)):
            username = "-".join(parts[:i])
            space_name = "-".join(parts[i:])
            repo_id = f"{username}/{space_name}"
            try:
                api.repo_info(repo_id=repo_id, repo_type="space")
                return repo_id
            except Exception:
                continue
    except Exception:
        pass
    return None


def delete_space(api: HfApi, space_url: str) -> bool:
    """删除一个 Space，返回是否成功"""
    repo_id = find_space_repo_id(api, space_url)
    if not repo_id:
        print(f"  [warn] 无法解析 repo_id: {space_url}", file=sys.stderr)
        return False
    try:
        api.delete_repo(repo_id=repo_id, repo_type="space")
        print(f"  [del] {repo_id}")
        return True
    except Exception as e:
        print(f"  [del-fail] {repo_id}: {e}", file=sys.stderr)
        return False


# ────────────────────────────────────────────────────────────────────────────
# CF Worker 环境变量更新
# ────────────────────────────────────────────────────────────────────────────


def update_cf_worker_env(
    cf_account_id: str,
    cf_api_token: str,
    worker_name: str,
    env_key: str,
    space_urls: list,
) -> bool:
    """通过 CF API 更新 Worker 的环境变量（空格分隔的 URL 列表）

    使用 Settings API 的 PATCH 方式，只更新指定的 env var，不影响其他变量。
    """
    url = (
        f"https://api.cloudflare.com/client/v4/accounts/{cf_account_id}"
        f"/workers/scripts/{worker_name}/settings"
    )
    headers = {
        "Authorization": f"Bearer {cf_api_token}",
        "Content-Type": "application/json",
    }

    # 读取当前 settings
    try:
        resp = requests.get(url, headers=headers, timeout=10)
        if not resp.ok:
            print(f"  [cf-err] GET settings: {resp.status_code} {resp.text[:200]}", file=sys.stderr)
            return False
    except Exception as e:
        print(f"  [cf-err] GET settings: {e}", file=sys.stderr)
        return False

    current = resp.json().get("result", {})
    bindings = current.get("bindings", [])

    # 更新或添加目标 env var
    value = " ".join(space_urls)
    found = False
    for b in bindings:
        if b.get("type") == "plain_text" and b.get("name") == env_key:
            b["text"] = value
            found = True
            break
    if not found:
        bindings.append({"type": "plain_text", "name": env_key, "text": value})

    # PATCH 回去
    try:
        patch_resp = requests.patch(
            url,
            headers=headers,
            json={"bindings": bindings},
            timeout=10,
        )
        if patch_resp.ok:
            print(f"  [cf] {env_key} updated ({len(space_urls)} urls)")
            return True
        else:
            print(f"  [cf-err] PATCH: {patch_resp.status_code} {patch_resp.text[:200]}", file=sys.stderr)
            return False
    except Exception as e:
        print(f"  [cf-err] PATCH: {e}", file=sys.stderr)
        return False


# ────────────────────────────────────────────────────────────────────────────
# 弹性管理核心逻辑
# ────────────────────────────────────────────────────────────────────────────


def manage_service(
    service_name: str,
    space_urls: list,
    accounts: list,
    release_url: str,
    gh_pat: str,
    extra_secrets: dict,
    target: int,
    dry_run: bool,
    cf_account_id: str = "",
    cf_api_token: str = "",
    cf_worker_name: str = "",
) -> list:
    """对单个服务执行弹性管理，返回最终的 Space URL 列表"""
    service_cfg = SERVICE_MAP[service_name]
    env_key = CF_ENV_MAP[service_name]

    print(f"\n{'='*60}")
    print(f"  Service: {service_name}  Pool: {len(space_urls)}  Target: {target}")
    print(f"{'='*60}")

    # 1. 健康检查
    print(f"\n[1] Health check ({len(space_urls)} spaces)...")
    results = batch_health_check(space_urls)

    healthy = [r for r in results if r["healthy"]]
    unhealthy = [r for r in results if not r["healthy"]]
    banned = [r for r in unhealthy if r["reason"] in ("banned_or_restricted", "not_found_or_deleted")]
    sleeping = [r for r in unhealthy if r["reason"] == "sleeping_or_building"]
    dead = [r for r in unhealthy if r["reason"] not in ("banned_or_restricted", "not_found_or_deleted", "sleeping_or_building")]

    print(f"  healthy={len(healthy)} banned={len(banned)} sleeping={len(sleeping)} dead={len(dead)}")

    for r in unhealthy:
        print(f"  [-] {r['url']}  status={r['status']}  reason={r['reason']}")

    # 2. 删除封禁/死亡节点
    to_delete = [r["url"] for r in banned + dead]
    to_keep = [r["url"] for r in healthy + sleeping]

    if to_delete:
        print(f"\n[2] Removing {len(to_delete)} dead/banned spaces...")
        if not dry_run:
            # 用第一个账号的 API 来删除（需要是 Space 所有者）
            for url in to_delete:
                for acc in accounts:
                    if delete_space(acc["api"], url):
                        break
        else:
            for url in to_delete:
                print(f"  [dry-run] would delete: {url}")
    else:
        print(f"\n[2] No dead/banned spaces to remove")

    # 3. 计算需要创建的数量
    active_count = len(to_keep)
    need_create = max(0, target - active_count)

    # 额外：替换被删除的节点（至少补回被删的数量）
    need_replace = len(to_delete)
    need_create = max(need_create, need_replace)

    if need_create > 0:
        print(f"\n[3] Scaling up: creating {need_create} new spaces...")
        if not dry_run:
            new_urls = []
            for i in range(need_create):
                account = accounts[i % len(accounts)]
                try:
                    url = deploy_one_space(account, service_cfg, release_url, gh_pat, extra_secrets)
                    new_urls.append(url)
                    print(f"  [+] {url}")
                except Exception as e:
                    print(f"  [create-fail] {e}", file=sys.stderr)
                if i < need_create - 1:
                    time.sleep(1)
            to_keep.extend(new_urls)
        else:
            print(f"  [dry-run] would create {need_create} spaces")
    else:
        print(f"\n[3] No scale-up needed (active={active_count} >= target={target})")

    # 4. 缩容：健康节点远超目标时，删除多余的
    healthy_urls = [r["url"] for r in healthy]
    scale_down_threshold = int(target * SCALE_DOWN_RATIO)
    excess = len(to_keep) - scale_down_threshold

    if excess > 0 and len(healthy_urls) > target:
        # 只删除健康的多余节点（保留 sleeping 的，它们不占资源）
        removable = healthy_urls[target:]  # 保留前 target 个
        to_remove = removable[:excess]

        print(f"\n[4] Scaling down: removing {len(to_remove)} excess spaces...")
        if not dry_run:
            for url in to_remove:
                for acc in accounts:
                    if delete_space(acc["api"], url):
                        break
                to_keep.remove(url)
        else:
            for url in to_remove:
                print(f"  [dry-run] would delete excess: {url}")
    else:
        print(f"\n[4] No scale-down needed")

    # 5. 更新 CF Worker 环境变量
    final_urls = to_keep
    print(f"\n[5] Final pool: {len(final_urls)} spaces")
    for url in final_urls:
        print(f"  {url}")

    if cf_account_id and cf_api_token and cf_worker_name and not dry_run:
        print(f"\n[6] Updating CF Worker env: {env_key}...")
        update_cf_worker_env(cf_account_id, cf_api_token, cf_worker_name, env_key, final_urls)
    elif not dry_run:
        # 没有 CF 配置，输出手动更新指令
        print(f"\n[6] CF Worker env not configured. Manual update:")
        print(f"  {env_key}={' '.join(final_urls)}")

    return final_urls


# ────────────────────────────────────────────────────────────────────────────
# 入口
# ────────────────────────────────────────────────────────────────────────────


def load_space_registry(path: str) -> dict:
    """加载 Space 注册表 JSON（记录每个服务当前的 Space URL 列表）

    格式: {"openai": ["url1", "url2"], "grok": [...], ...}
    """
    p = Path(path)
    if p.exists():
        with open(p) as f:
            return json.load(f)
    return {}


def save_space_registry(path: str, registry: dict):
    """保存 Space 注册表"""
    with open(path, "w") as f:
        json.dump(registry, f, indent=2, ensure_ascii=False)
    print(f"\n[saved] Registry -> {path}")


def main():
    parser = argparse.ArgumentParser(description="HF Space Autoscaler")
    parser.add_argument("--service", required=True, help="服务类型 (openai/grok/kiro/gemini/ts/all)")
    parser.add_argument("--target", type=int, default=DEFAULT_TARGET, help=f"目标节点数 (默认 {DEFAULT_TARGET})")
    parser.add_argument("--dry-run", action="store_true", help="只检查不修改")
    parser.add_argument("--release-url", default="", help="GitHub Release URL（创建新 Space 时使用）")
    parser.add_argument("--gh-pat", default=os.environ.get("GH_PAT", ""), help="GitHub PAT")
    parser.add_argument("--tokens", default=str(REPO_ROOT / "scripts" / "hf_tokens.json"), help="HF Token JSON")
    parser.add_argument("--registry", default=str(REPO_ROOT / "scripts" / "hf_registry.json"), help="Space 注册表路径")
    parser.add_argument("--extra-secrets", type=json.loads, default={}, help="额外 Secrets (JSON)")
    # CF Worker 配置（可选，不配置则只输出手动更新指令）
    parser.add_argument("--cf-account-id", default=os.environ.get("CF_ACCOUNT_ID", ""), help="Cloudflare Account ID")
    parser.add_argument("--cf-api-token", default=os.environ.get("CF_API_TOKEN", ""), help="Cloudflare API Token")
    parser.add_argument("--cf-worker-name", default=os.environ.get("CF_WORKER_NAME", "hf-snow-worker"), help="CF Worker 名称")
    args = parser.parse_args()

    services = list(SERVICE_MAP.keys()) if args.service == "all" else [args.service]
    for svc in services:
        if svc not in SERVICE_MAP:
            print(f"Error: unknown service '{svc}'", file=sys.stderr)
            sys.exit(1)

    # 加载 Token 和账号
    print("[init] Loading HF tokens...")
    tokens = load_tokens(args.tokens)
    accounts = get_accounts(tokens)

    # 加载注册表
    registry = load_space_registry(args.registry)

    # 逐服务处理
    for svc in services:
        space_urls = registry.get(svc, [])
        if not space_urls and not args.release_url:
            print(f"\n[skip] {svc}: no spaces in registry and no --release-url for creating new ones")
            continue

        final = manage_service(
            service_name=svc,
            space_urls=space_urls,
            accounts=accounts,
            release_url=args.release_url,
            gh_pat=args.gh_pat,
            extra_secrets=args.extra_secrets,
            target=args.target,
            dry_run=args.dry_run,
            cf_account_id=args.cf_account_id,
            cf_api_token=args.cf_api_token,
            cf_worker_name=args.cf_worker_name,
        )
        registry[svc] = final

    # 保存注册表
    if not args.dry_run:
        save_space_registry(args.registry, registry)
    else:
        print(f"\n[dry-run] Registry not saved")

    print(f"\nDone.")


if __name__ == "__main__":
    main()
