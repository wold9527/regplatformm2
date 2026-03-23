package service

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"github.com/xiaolajiaoyyds/regplatformm/internal/worker"
	"golang.org/x/net/proxy"
	"gorm.io/gorm"
)

// ProxyPool 代理池（线程安全轮询，多源加载，健康检查）
type ProxyPool struct {
	mu         sync.RWMutex
	proxies    []worker.ProxyEntry
	index      uint64
	settingSvc *SettingService
	db         *gorm.DB
}

// NewProxyPool 创建代理池
func NewProxyPool(settingSvc *SettingService, db *gorm.DB) *ProxyPool {
	p := &ProxyPool{
		settingSvc: settingSvc,
		db:         db,
	}
	p.Reload()
	return p
}

// GetNext 获取下一个健康代理（延迟感知轮询 + TCP 快检验证）
// 最多尝试 min(n, 3) 个代理，跳过 TCP 不可达的；全部失败则回退返回第一个
func (p *ProxyPool) GetNext() *worker.ProxyEntry {
	p.mu.RLock()
	n := len(p.proxies)
	if n == 0 {
		p.mu.RUnlock()
		return nil // 直连
	}

	start := atomic.AddUint64(&p.index, 1)
	proxiesCopy := make([]worker.ProxyEntry, n)
	copy(proxiesCopy, p.proxies)
	p.mu.RUnlock()

	// 最多尝试 min(n, 3) 个代理
	tries := n
	if tries > 3 {
		tries = 3
	}
	for i := 0; i < tries; i++ {
		entry := proxiesCopy[(start+uint64(i))%uint64(n)]
		if quickTCPCheck(entry.HTTP, 2*time.Second) {
			return &entry
		}
		// 异步标记不健康（不阻塞选择流程）
		go p.markUnhealthy(entry.HTTP)
	}

	// 全部快检失败，回退返回第一个候选（让上层决定是否直连）
	entry := proxiesCopy[start%uint64(n)]
	return &entry
}

// Count 返回可用代理数量
func (p *ProxyPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.proxies)
}

// GetNextForPlatform 根据平台代理策略获取代理
// 模式：pool=轮询代理池, fixed=使用平台专用固定代理, direct=不使用代理, smart=按延迟加权选择
func (p *ProxyPool) GetNextForPlatform(platform string) *worker.ProxyEntry {
	mode := p.settingSvc.Get(platform+"_proxy_mode", "pool")

	switch mode {
	case "direct":
		return nil

	case "fixed":
		fixedURL := p.settingSvc.Get(platform+"_proxy", "")
		if fixedURL == "" {
			log.Debug().Str("platform", platform).Msg("fixed 模式但未配置代理，回退 pool")
			return p.GetNext()
		}
		return &worker.ProxyEntry{HTTP: fixedURL, HTTPS: fixedURL}

	case "smart":
		return p.getSmartProxy()

	default: // "pool" 或未知值
		return p.GetNext()
	}
}

// getSmartProxy 基于延迟的加权随机选择（延迟低的代理被选中概率更高）
// 权重 = 1000 / max(latency_ms, 10)，未检测过的代理使用默认权重 50
func (p *ProxyPool) getSmartProxy() *worker.ProxyEntry {
	if p.db == nil {
		return p.GetNext()
	}

	var entries []model.ProxyPoolEntry
	if err := p.db.Where("is_healthy = ? AND protocol IN ?", true, []string{"socks5", "http", "https"}).
		Find(&entries).Error; err != nil || len(entries) == 0 {
		return p.GetNext()
	}

	type weightedEntry struct {
		entry  model.ProxyPoolEntry
		weight float64
	}
	candidates := make([]weightedEntry, 0, len(entries))
	var totalWeight float64
	for _, e := range entries {
		w := 50.0
		if e.LatencyMs > 0 {
			w = 1000.0 / float64(max(e.LatencyMs, 10))
		}
		totalWeight += w
		candidates = append(candidates, weightedEntry{entry: e, weight: w})
	}

	r := rand.Float64() * totalWeight
	var cumulative float64
	for _, c := range candidates {
		cumulative += c.weight
		if r <= cumulative {
			proxyURL := c.entry.URL()
			return &worker.ProxyEntry{HTTP: proxyURL, HTTPS: proxyURL}
		}
	}

	last := candidates[len(candidates)-1].entry.URL()
	return &worker.ProxyEntry{HTTP: last, HTTPS: last}
}

// Reload 重新加载代理列表（多源：DB ProxyPoolEntry + settings default_proxy）
// 在锁外执行 DB I/O，仅在交换时加锁，避免阻塞 GetNext()
func (p *ProxyPool) Reload() {
	var newProxies []worker.ProxyEntry

	// 源 1: 从 proxy_pool_entries 表加载健康代理（仅 http/https/socks5）
	if p.db != nil {
		var entries []model.ProxyPoolEntry
		if err := p.db.Where("is_healthy = ? AND protocol IN ? AND fail_count < ?", true, []string{"socks5", "http", "https"}, 3).
			Order("latency_ms ASC").Find(&entries).Error; err != nil {
			log.Warn().Err(err).Msg("代理池加载 DB 条目失败")
			p.mu.RLock()
			currentCount := len(p.proxies)
			p.mu.RUnlock()
			if currentCount > 0 {
				log.Warn().Int("kept", currentCount).Msg("DB 查询失败，保留现有代理池")
				return
			}
		} else {
			for _, e := range entries {
				entry := worker.ProxyEntry{HTTP: e.URL(), HTTPS: e.URL()}
				newProxies = appendUniqueProxy(newProxies, entry)
			}
		}
	}

	// 源 2: settings 中的 default_proxy（兜底）
	if dbProxy := p.settingSvc.Get("default_proxy", ""); dbProxy != "" {
		entry := worker.ProxyEntry{HTTP: dbProxy, HTTPS: dbProxy}
		newProxies = appendUniqueProxy(newProxies, entry)
	}

	// 原子交换：仅在此处加写锁
	p.mu.Lock()
	p.proxies = newProxies
	atomic.StoreUint64(&p.index, 0)
	p.mu.Unlock()

	log.Info().Int("count", len(newProxies)).Msg("代理池已加载")
}

// appendUniqueProxy 去重追加代理
func appendUniqueProxy(list []worker.ProxyEntry, entry worker.ProxyEntry) []worker.ProxyEntry {
	for _, existing := range list {
		if existing.HTTP == entry.HTTP {
			return list
		}
	}
	return append(list, entry)
}

// ── 健康检查 ────────────────────────────────────────────────────────

// HealthCheckAll 对所有代理池条目执行健康检查（并发，每批 10 个）
// 返回 (总数, 健康数, 不健康数)
func (p *ProxyPool) HealthCheckAll() (total, healthy, unhealthy int) {
	if p.db == nil {
		return 0, 0, 0
	}

	var entries []model.ProxyPoolEntry
	if err := p.db.Where("protocol IN ?", []string{"socks5", "http", "https"}).
		Find(&entries).Error; err != nil {
		log.Error().Err(err).Msg("健康检查查询代理失败")
		return 0, 0, 0
	}

	total = len(entries)
	if total == 0 {
		return 0, 0, 0
	}

	const batchSize = 10
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}
		batch := entries[i:end]

		for j := range batch {
			wg.Add(1)
			go func(entry *model.ProxyPoolEntry) {
				defer wg.Done()
				ok, latency := checkProxyHealth(entry.URL(), entry.Protocol)
				now := time.Now()

				mu.Lock()
				if ok {
					healthy++
				} else {
					unhealthy++
				}
				mu.Unlock()

				updates := map[string]interface{}{
					"last_checked_at": now,
				}
				if ok {
					updates["is_healthy"] = true
					updates["fail_count"] = 0
					updates["latency_ms"] = latency
				} else {
					newFail := entry.FailCount + 1
					updates["fail_count"] = newFail
					updates["is_healthy"] = false
				}
				if err := p.db.Model(&model.ProxyPoolEntry{}).Where("id = ?", entry.ID).Updates(updates).Error; err != nil {
					log.Warn().Err(err).Uint("id", entry.ID).Msg("更新代理健康状态失败")
				}
			}(&batch[j])
		}
		wg.Wait()
	}

	// 重新加载代理池（踢除不健康的）
	p.Reload()

	return total, healthy, unhealthy
}

// HealthCheckByIDs 对指定 ID 的代理执行健康检查（并发，每批 10 个）
// 返回 (总数, 健康数, 不健康数)
func (p *ProxyPool) HealthCheckByIDs(ids []uint) (total, healthy, unhealthy int) {
	if p.db == nil || len(ids) == 0 {
		return 0, 0, 0
	}

	var entries []model.ProxyPoolEntry
	if err := p.db.Where("id IN ? AND protocol IN ?", ids, []string{"socks5", "http", "https"}).
		Find(&entries).Error; err != nil {
		log.Error().Err(err).Msg("选择性健康检查查询代理失败")
		return 0, 0, 0
	}

	total = len(entries)
	if total == 0 {
		return 0, 0, 0
	}

	const batchSize = 10
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}
		batch := entries[i:end]

		for j := range batch {
			wg.Add(1)
			go func(entry *model.ProxyPoolEntry) {
				defer wg.Done()
				ok, latency := checkProxyHealth(entry.URL(), entry.Protocol)
				now := time.Now()

				mu.Lock()
				if ok {
					healthy++
				} else {
					unhealthy++
				}
				mu.Unlock()

				updates := map[string]interface{}{
					"last_checked_at": now,
				}
				if ok {
					updates["is_healthy"] = true
					updates["fail_count"] = 0
					updates["latency_ms"] = latency
				} else {
					newFail := entry.FailCount + 1
					updates["fail_count"] = newFail
					updates["is_healthy"] = false
				}
				if err := p.db.Model(&model.ProxyPoolEntry{}).Where("id = ?", entry.ID).Updates(updates).Error; err != nil {
					log.Warn().Err(err).Uint("id", entry.ID).Msg("更新代理健康状态失败")
				}
			}(&batch[j])
		}
		wg.Wait()
	}

	// 重新加载代理池（踢除不健康的）
	p.Reload()

	return total, healthy, unhealthy
}

// quickTCPCheck 对代理执行 TCP 快连测试，仅验证端口可达性
func quickTCPCheck(proxyURL string, timeout time.Duration) bool {
	parsed, err := url.Parse(proxyURL)
	if err != nil || parsed.Host == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", parsed.Host, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// markUnhealthy 异步标记代理为不健康并触发代理池重载
func (p *ProxyPool) markUnhealthy(proxyURL string) {
	if p.db == nil {
		return
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return
	}
	host := parsed.Hostname()
	portStr := parsed.Port()
	if host == "" || portStr == "" {
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return
	}
	if err := p.db.Model(&model.ProxyPoolEntry{}).
		Where("host = ? AND port = ?", host, port).
		Updates(map[string]interface{}{
			"fail_count": gorm.Expr("fail_count + 1"),
			"is_healthy": false,
		}).Error; err != nil {
		log.Warn().Str("proxy", proxyURL).Err(err).Msg("标记代理不健康失败")
	}
	p.Reload()
}

// PoolStats 返回代理池统计信息
func (p *ProxyPool) PoolStats() map[string]interface{} {
	if p.db == nil {
		return map[string]interface{}{"total": 0, "healthy": 0, "unhealthy": 0, "active": p.Count()}
	}

	var total, healthyCount int64
	p.db.Model(&model.ProxyPoolEntry{}).Where("protocol IN ?", []string{"socks5", "http", "https"}).Count(&total)
	p.db.Model(&model.ProxyPoolEntry{}).Where("protocol IN ? AND is_healthy = ?", []string{"socks5", "http", "https"}, true).Count(&healthyCount)

	return map[string]interface{}{
		"total":     total,
		"healthy":   healthyCount,
		"unhealthy": total - healthyCount,
		"active":    p.Count(),
	}
}

// ── 代理连通性检测 ──────────────────────────────────────────────────

// checkProxyHealth 检测代理是否可用，返回 (是否健康, 延迟毫秒)
func checkProxyHealth(proxyURL, protocol string) (bool, int64) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return false, 0
	}

	switch parsed.Scheme {
	case "socks5":
		auth := &proxy.Auth{}
		if parsed.User != nil {
			auth.User = parsed.User.Username()
			auth.Password, _ = parsed.User.Password()
		} else {
			auth = nil
		}
		dialer, err := proxy.SOCKS5("tcp", parsed.Host, auth, proxy.Direct)
		if err != nil {
			return false, 0
		}
		var conn net.Conn
		if cd, ok := dialer.(proxy.ContextDialer); ok {
			conn, err = cd.DialContext(ctx, "tcp", "httpbin.org:80")
		} else {
			conn, err = dialer.Dial("tcp", "httpbin.org:80")
		}
		if err != nil {
			return false, 0
		}
		conn.Close()

	case "http", "https":
		transport := &http.Transport{Proxy: http.ProxyURL(parsed)}
		client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://httpbin.org/ip", nil)
		resp, err := client.Do(req)
		if err != nil {
			return false, 0
		}
		resp.Body.Close()

	default:
		return false, 0
	}

	latency := time.Since(start).Milliseconds()
	return true, latency
}

// BuildProxyHTTPClient 构建通过指定代理的 HTTP Client（供验活等场景使用）
func BuildProxyHTTPClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	if proxyURL == "" {
		return &http.Client{Timeout: timeout}, nil
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("代理地址格式错误: %w", err)
	}

	switch parsed.Scheme {
	case "socks5":
		auth := &proxy.Auth{}
		if parsed.User != nil {
			auth.User = parsed.User.Username()
			auth.Password, _ = parsed.User.Password()
		} else {
			auth = nil
		}
		dialer, err := proxy.SOCKS5("tcp", parsed.Host, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("SOCKS5 初始化失败: %w", err)
		}
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if cd, ok := dialer.(proxy.ContextDialer); ok {
					return cd.DialContext(ctx, network, addr)
				}
				return dialer.Dial(network, addr)
			},
		}
		return &http.Client{Transport: transport, Timeout: timeout}, nil

	case "http", "https":
		transport := &http.Transport{Proxy: http.ProxyURL(parsed)}
		return &http.Client{Transport: transport, Timeout: timeout}, nil

	default:
		return nil, fmt.Errorf("不支持的代理协议: %s", parsed.Scheme)
	}
}

// StartHealthChecker 启动代理池定时健康检查后台协程
// 每 5 分钟对所有代理执行连通性检测，失败即标记不健康并踢出代理池
func (p *ProxyPool) StartHealthChecker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		log.Info().Msg("代理池定时健康检查已启动")
		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("代理池定时健康检查已停止")
				return
			case <-ticker.C:
				total, healthy, unhealthy := p.HealthCheckAll()
				log.Info().Int("total", total).Int("healthy", healthy).Int("unhealthy", unhealthy).Msg("定时健康检查完成")
			}
		}
	}()
}
