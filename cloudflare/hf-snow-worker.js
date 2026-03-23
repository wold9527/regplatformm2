// hf-snow-worker.js — Edge load balancer + keepalive
//
// Deploy: Cloudflare Workers
// Cron Trigger: 0 */2 * * *
//
// Environment variables (space-separated HF Space URL lists):
//   OPENAI_SPACES — OpenAI node pool
//   GROK_SPACES   — Grok node pool
//   KIRO_SPACES   — Kiro node pool
//   GEMINI_SPACES — Gemini node pool
//   TS_SPACES     — Turnstile Solver node pool
//
// Routing:
//   POST {url}/openai/process  → match /openai/ prefix → strip → /api/v1/process
//   POST {url}/grok/process    → match /grok/ prefix   → strip → /api/v1/process
//   POST {url}/kiro/process    → match /kiro/ prefix   → strip → /api/v1/process
//   POST {url}/gemini/process  → match /gemini/ prefix → strip → /gemini/process
//   POST {url}/ts/turnstile    → match /ts/ prefix      → strip → /turnstile
//
// 负载均衡策略: least-loaded + round-robin（同负载节点严格轮询，跨请求持久化）

// 模块级 round-robin 计数器（同一 isolate 内跨请求持久化，最大化节点利用率）
const rrCounters = {};

// 模块级健康状态缓存（10s TTL，避免每次请求全量探测所有节点）
const statsCache = {};
const STATS_CACHE_TTL = 10_000; // 10 秒

const CORS_HEADERS = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
  "Access-Control-Allow-Headers": "Content-Type, Authorization",
};

// Route prefix → { envKey, rewrite(subpath) }
// rewrite: transform the subpath after stripping the prefix
const ROUTE_MAP = {
  "/openai/": { envKey: "OPENAI_SPACES", rewrite: (sub) => "/api/v1" + sub },
  "/grok/":   { envKey: "GROK_SPACES",   rewrite: (sub) => "/api/v1" + sub },
  "/kiro/":   { envKey: "KIRO_SPACES",   rewrite: (sub) => "/kiro" + sub },
  "/gemini/": { envKey: "GEMINI_SPACES", rewrite: (sub) => "/gemini" + sub },
  "/ts/":     { envKey: "TS_SPACES",     rewrite: (sub) => sub },
};

/**
 * Match request path to a Space pool
 * @param {string} pathname
 * @returns {{ prefix: string, subpath: string } | null}
 */
function matchRoute(pathname) {
  for (const [prefix, cfg] of Object.entries(ROUTE_MAP)) {
    if (pathname.startsWith(prefix)) {
      // Strip prefix, then apply rewrite
      const stripped = "/" + pathname.slice(prefix.length);
      const subpath = cfg.rewrite(stripped);
      return { prefix, subpath };
    }
  }
  return null;
}

/**
 * Parse Space URL list from env
 * @param {object} env
 * @param {string} prefix
 * @returns {string[]}
 */
function getPool(env, prefix) {
  const cfg = ROUTE_MAP[prefix];
  if (!cfg) return [];
  return (env[cfg.envKey] || "").split(/\s+/).filter(Boolean);
}

/**
 * Quick health probe (2s timeout)
 * @param {string} space
 * @returns {Promise<boolean>}
 */
async function probeHealth(space) {
  try {
    const resp = await fetch(space + "/health", {
      signal: AbortSignal.timeout(2000),
    });
    return resp.ok;
  } catch {
    return false;
  }
}

/**
 * Fetch node concurrency stats (3s timeout)
 * Health response: {"status":"ok","concurrency":{"load","queue","cap","avg"}}
 * @param {string} space
 * @returns {Promise<{active:number,waiting:number,max_concurrent:number,avg_seconds:number,healthy:boolean}|null>}
 */
async function fetchNodeStats(space) {
  try {
    const resp = await fetch(space + "/health", {
      signal: AbortSignal.timeout(3000),
    });
    if (!resp.ok) return null;
    const data = await resp.json();
    const q = data.concurrency || {};
    return {
      active: q.load || 0,
      waiting: q.queue || 0,
      max_concurrent: q.cap || 5,
      avg_seconds: q.avg || 0,
      healthy: true,
    };
  } catch {
    return null;
  }
}

/**
 * 带缓存的 fetchNodeStats（10s TTL，同一 isolate 内跨请求共享）
 * 避免每次请求都全量探测所有节点，sleeping 节点的 3s 超时不再拖慢健康节点
 * @param {string} space
 * @returns {Promise<{active:number,waiting:number,max_concurrent:number,avg_seconds:number,healthy:boolean}|null>}
 */
async function getCachedStats(space) {
  const now = Date.now();
  const cached = statsCache[space];
  if (cached && now - cached.ts < STATS_CACHE_TTL) {
    return cached.stats;
  }
  const stats = await fetchNodeStats(space);
  statsCache[space] = { stats, ts: now };
  return stats;
}

/**
 * /api/v1/status?t=openai|grok|kiro|gemini — aggregate queue status across all nodes
 * Returns: { total_active, total_waiting, max_concurrent, avg_seconds, healthy_nodes, total_nodes }
 */
async function handleQueueStatus(env, searchParams) {
  const platform = (searchParams.get("t") || "").toLowerCase();
  let prefix;
  if (platform === "openai") prefix = "/openai/";
  else if (platform === "grok") prefix = "/grok/";
  else if (platform === "kiro") prefix = "/kiro/";
  else if (platform === "gemini") prefix = "/gemini/";
  else {
    return new Response(
      JSON.stringify({ error: "missing parameter: t (openai/grok/kiro/gemini)" }),
      { status: 400, headers: { "Content-Type": "application/json", ...CORS_HEADERS } }
    );
  }

  const pool = getPool(env, prefix);
  if (pool.length === 0) {
    return new Response(
      JSON.stringify({ total_active: 0, total_waiting: 0, max_concurrent: 0, avg_seconds: 0, healthy_nodes: 0, total_nodes: 0 }),
      { headers: { "Content-Type": "application/json", ...CORS_HEADERS } }
    );
  }

  const results = await Promise.all(pool.map(fetchNodeStats));
  let totalActive = 0, totalWaiting = 0, maxConcurrent = 0, avgSum = 0, avgCount = 0, healthyNodes = 0;
  for (const r of results) {
    if (r && r.healthy) {
      healthyNodes++;
      totalActive += r.active;
      totalWaiting += r.waiting;
      maxConcurrent += r.max_concurrent;
      if (r.avg_seconds > 0) {
        avgSum += r.avg_seconds;
        avgCount++;
      }
    }
  }

  return new Response(JSON.stringify({
    total_active: totalActive,
    total_waiting: totalWaiting,
    max_concurrent: maxConcurrent,
    avg_seconds: avgCount > 0 ? Math.round(avgSum / avgCount) : 0,
    healthy_nodes: healthyNodes,
    total_nodes: pool.length,
  }), { headers: { "Content-Type": "application/json", ...CORS_HEADERS } });
}

/**
 * Space URL → 6 字符短哈希（djb2 算法，用 base36 编码）
 * 用于 sticky routing 的节点标识，不依赖数组索引，加减节点不影响已有任务
 */
function spaceHash(url) {
  let h = 5381;
  for (let i = 0; i < url.length; i++) {
    h = ((h << 5) + h + url.charCodeAt(i)) >>> 0;
  }
  return h.toString(36).padStart(6, "0").slice(-6);
}

/**
 * TS sticky routing: 基于 Space URL 哈希的粘性路由
 *
 * 设计要点:
 *   1. submit 时在 taskId 前加 Space 哈希（如 "a1b2c3:uuid"），不用数组索引
 *   2. poll 时从 id 前缀提取哈希，在 pool 里找到对应 Space 直连
 *   3. 加减节点不影响已有任务（哈希固定绑 URL），用完即销（TS 内部有 TTL）
 *   4. 多用户并发安全——每个 taskId 自带目标节点，不会混
 *
 * 格式: "{spaceHash}:{originalTaskId}"
 */
async function handleTSRoute(pool, subpath, url, request, headers, bodyBuf) {
  // 降级时需要用还原后的 search（去掉 hash 前缀），默认用原始 search
  let fallbackSearch = url.search;

  // ── Poll: 从 id 参数提取 Space 哈希，直连对应节点 ──
  if (subpath.startsWith("/result")) {
    const id = url.searchParams.get("id");
    if (id) {
      const sep = id.indexOf(":");
      if (sep > 0) {
        const hash = id.substring(0, sep);
        const realId = id.substring(sep + 1);
        // 预先构建还原后的 search，fallback 时也用这个（TS Space 不认识 hash 前缀）
        const cleanParams = new URLSearchParams(url.searchParams);
        cleanParams.set("id", realId);
        fallbackSearch = "?" + cleanParams.toString();

        const target = pool.find((s) => spaceHash(s) === hash);
        if (target) {
          try {
            return await forwardRequest(target, "/result", fallbackSearch, request.method, headers, bodyBuf);
          } catch (e) {
            console.error(`[ts-sticky] ${hash} failed: ${e.message}`);
            // 目标节点不可达，降级到随机健康节点
          }
        }
        // 哈希匹配不到（节点已下线）或目标不可达，降级到随机健康节点
        console.log(`[ts-sticky] hash ${hash} not in pool or unreachable, fallback`);
      }
    }
  }

  // ── 健康探测 + 随机选节点 ──
  const probed = await Promise.all(
    pool.map(async (space) => ({ space, hash: spaceHash(space), ok: await probeHealth(space) }))
  );
  const up = probed.filter((p) => p.ok);
  if (up.length === 0) {
    return new Response(
      JSON.stringify({ ok: false, error: "all TS nodes unavailable" }),
      { status: 502, headers: { "Content-Type": "application/json", ...CORS_HEADERS } }
    );
  }
  // 随机打散，负载均衡
  for (let i = up.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [up[i], up[j]] = [up[j], up[i]];
  }

  // ── 尝试转发 ──
  for (let k = 0; k < up.length; k++) {
    const node = up[k];
    try {
      const resp = await forwardRequest(node.space, subpath, fallbackSearch, request.method, headers, bodyBuf);
      if (resp.status === 503 && k < up.length - 1) continue;

      // Submit 成功：在 taskId 前加 Space 哈希，下次 poll 可直连
      if (subpath.startsWith("/turnstile") && resp.ok) {
        try {
          const text = await resp.text();
          const body = JSON.parse(text);
          if (body.taskId) {
            body.taskId = node.hash + ":" + body.taskId;
          }
          const newHeaders = new Headers(resp.headers);
          newHeaders.set("Content-Type", "application/json");
          for (const [ck, cv] of Object.entries(CORS_HEADERS)) {
            newHeaders.set(ck, cv);
          }
          return new Response(JSON.stringify(body), { status: resp.status, headers: newHeaders });
        } catch {
          // JSON 解析失败，原样返回
        }
      }

      return resp;
    } catch (e) {
      console.error(`[ts-fail] ${node.space}: ${e.message}`);
      continue;
    }
  }

  return new Response(
    JSON.stringify({ ok: false, error: "all TS nodes failed" }),
    { status: 502, headers: { "Content-Type": "application/json", ...CORS_HEADERS } }
  );
}

/**
 * Forward request to HF Space
 */
async function forwardRequest(space, subpath, search, method, headers, bodyBuf) {
  const target = space + subpath + search;
  const fwdHeaders = new Headers();
  fwdHeaders.set(
    "Content-Type",
    headers.get("Content-Type") || "application/json"
  );
  fwdHeaders.set("Accept", headers.get("Accept") || "*/*");
  fwdHeaders.set(
    "User-Agent",
    headers.get("User-Agent") || "CF-Worker-Edge"
  );

  const resp = await fetch(target, {
    method,
    headers: fwdHeaders,
    body: bodyBuf,
  });

  const newResp = new Response(resp.body, resp);
  for (const [k, v] of Object.entries(CORS_HEADERS)) {
    newResp.headers.set(k, v);
  }
  return newResp;
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const path = url.pathname;

    // CORS preflight
    if (request.method === "OPTIONS") {
      return new Response(null, { headers: CORS_HEADERS });
    }

    // Worker health check
    if (path === "/health") {
      return new Response("ok", { headers: CORS_HEADERS });
    }

    // Aggregated queue status (VPS heartbeat polling)
    if (path === "/api/v1/status") {
      return handleQueueStatus(env, url.searchParams);
    }

    // Route matching
    const route = matchRoute(path);
    if (!route) {
      return new Response(
        JSON.stringify({ name: "edge-service", version: "1.0.0", status: "healthy" }),
        { headers: { "Content-Type": "application/json", ...CORS_HEADERS } }
      );
    }

    const pool = getPool(env, route.prefix);
    if (pool.length === 0) {
      return new Response(
        JSON.stringify({ ok: false, error: "no nodes configured" }),
        {
          status: 503,
          headers: { "Content-Type": "application/json", ...CORS_HEADERS },
        }
      );
    }

    const bodyBuf =
      request.method !== "GET" && request.method !== "HEAD"
        ? await request.arrayBuffer()
        : null;

    // /ts/ 路由走专用 sticky handler（submit 打标 + poll 粘性路由）
    if (route.prefix === "/ts/") {
      return handleTSRoute(pool, route.subpath, url, request, request.headers, bodyBuf);
    }

    // 其他平台（grok/openai/kiro/gemini）：最小负载优先 + round-robin 路由
    // 查询每个节点的实时并发负载（10s 缓存），优先分给最闲的节点；
    // 同负载节点用 round-robin 严格轮询（而非随机），确保请求均匀分散
    const nodeStats = await Promise.all(
      pool.map(async (space) => {
        const stats = await getCachedStats(space);
        return {
          space,
          healthy: stats !== null,
          load: stats ? stats.active + stats.waiting : Infinity,
        };
      })
    );

    const healthy = nodeStats
      .filter((r) => r.healthy)
      .sort((a, b) => a.load - b.load)   // 负载最低的排前面
      .map((r) => r.space);
    const unhealthy = nodeStats
      .filter((r) => !r.healthy)
      .map((r) => r.space);

    // 同负载节点用 round-robin 轮询（模块级计数器跨请求持久化）
    // 比随机打散更均匀：100 个请求 / 18 节点 → 每节点稳定 5-6 个
    const loadMap = new Map(nodeStats.map((n) => [n.space, n.load]));
    if (!rrCounters[route.prefix]) rrCounters[route.prefix] = 0;
    if (healthy.length > 0) {
      const minLoad = loadMap.get(healthy[0]);
      // 找出所有最小负载节点
      const minGroup = healthy.filter((s) => loadMap.get(s) === minLoad);
      if (minGroup.length > 1) {
        // round-robin 选一个最小负载节点放到最前面
        const pick = rrCounters[route.prefix]++ % minGroup.length;
        const chosen = minGroup[pick];
        // 把选中的节点移到 healthy 数组最前面
        const idx = healthy.indexOf(chosen);
        if (idx > 0) {
          healthy.splice(idx, 1);
          healthy.unshift(chosen);
        }
      }
    }

    const tryOrder = [...healthy, ...unhealthy];

    // Try each node
    for (const space of tryOrder) {
      try {
        const resp = await forwardRequest(
          space,
          route.subpath,
          url.search,
          request.method,
          request.headers,
          bodyBuf
        );

        // 503 = node busy, try next
        if (resp.status === 503 && tryOrder.indexOf(space) < tryOrder.length - 1) {
          console.log(`[skip] ${space}: 503, trying next`);
          continue;
        }

        return resp;
      } catch (e) {
        console.error(`[fail] ${space}: ${e.message}`);
        continue;
      }
    }

    // All nodes failed
    return new Response(
      JSON.stringify({ ok: false, error: "all nodes unavailable" }),
      {
        status: 502,
        headers: { "Content-Type": "application/json", ...CORS_HEADERS },
      }
    );
  },

  // Keepalive: ping all pools every 2 hours to prevent HF Space hibernation
  async scheduled(event, env) {
    const allSpaces = Object.values(ROUTE_MAP).flatMap((cfg) =>
      (env[cfg.envKey] || "").split(/\s+/).filter(Boolean)
    );
    const unique = [...new Set(allSpaces)];

    await Promise.all(
      unique.map(async (space) => {
        try {
          const resp = await fetch(space + "/health", {
            signal: AbortSignal.timeout(5000),
          });
          console.log(`[keepalive] ${space}: ${resp.status}`);
        } catch (err) {
          console.error(`[keepalive] ${space}: ${err.message}`);
        }
      })
    );
  },
};
