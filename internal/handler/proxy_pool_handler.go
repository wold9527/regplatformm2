package handler

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"github.com/xiaolajiaoyyds/regplatformm/internal/pkg/netutil"
	"github.com/xiaolajiaoyyds/regplatformm/internal/service"
	"gorm.io/gorm"
)

// ProxyPoolHandler 全局代理池管理（管理员）
type ProxyPoolHandler struct {
	db        *gorm.DB
	proxyPool *service.ProxyPool
}

// NewProxyPoolHandler 创建代理池管理处理器
func NewProxyPoolHandler(db *gorm.DB, proxyPool *service.ProxyPool) *ProxyPoolHandler {
	return &ProxyPoolHandler{db: db, proxyPool: proxyPool}
}

// ── 请求结构体 ──────────────────────────────────────────────────────

type createPoolProxyReq struct {
	Name     string `json:"name" binding:"max=100"`
	Protocol string `json:"protocol" binding:"required,oneof=http https socks5"`
	Host     string `json:"host" binding:"required,max=200"`
	Port     int    `json:"port" binding:"required,min=1,max=65535"`
	Username string `json:"username" binding:"max=100"`
	Password string `json:"password" binding:"max=200"`
	Country  string `json:"country" binding:"max=10"`
}

// ── CRUD ────────────────────────────────────────────────────────────

// List 获取代理池列表（GET /api/admin/proxy-pool，支持分页）
func (h *ProxyPoolHandler) List(c *gin.Context) {
	query := h.db.Where("protocol IN ?", []string{"http", "https", "socks5"}).Order("created_at DESC")

	// 可选过滤：healthy / unhealthy
	if filter := c.Query("filter"); filter == "healthy" {
		query = query.Where("is_healthy = ?", true)
	} else if filter == "unhealthy" {
		query = query.Where("is_healthy = ?", false)
	}

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 50
	}

	// 总数（Session 克隆防止 Count 污染后续 Find 查询）
	var total int64
	query.Session(&gorm.Session{}).Model(&model.ProxyPoolEntry{}).Count(&total)

	// 分页查询
	var entries []model.ProxyPoolEntry
	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "查询失败"})
		return
	}

	// 脱敏密码
	for i := range entries {
		if entries[i].Password != "" {
			entries[i].Password = "******"
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     entries,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Create 添加代理到池（POST /api/admin/proxy-pool）
func (h *ProxyPoolHandler) Create(c *gin.Context) {
	var req createPoolProxyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	// SSRF 防护：拒绝私网/保留地址
	if netutil.IsPrivateHost(req.Host) {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "不允许添加内网地址的代理"})
		return
	}

	// 去重检查
	var count int64
	if err := h.db.Model(&model.ProxyPoolEntry{}).Where("host = ? AND port = ?", req.Host, req.Port).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "查询失败"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": fmt.Sprintf("代理 %s:%d 已存在", req.Host, req.Port)})
		return
	}

	entry := model.ProxyPoolEntry{
		Name:     req.Name,
		Protocol: req.Protocol,
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
		Country:  req.Country,
		Source:   "manual",
	}
	if err := h.db.Create(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "创建失败"})
		return
	}

	h.proxyPool.Reload()

	if entry.Password != "" {
		entry.Password = "******"
	}
	c.JSON(http.StatusOK, entry)
}

// Delete 删除代理（DELETE /api/admin/proxy-pool/:id）
func (h *ProxyPoolHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 ID"})
		return
	}
	result := h.db.Where("id = ?", id).Delete(&model.ProxyPoolEntry{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "删除失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "代理不存在"})
		return
	}
	h.proxyPool.Reload()
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// BatchDelete 批量删除（POST /api/admin/proxy-pool/batch-delete）
func (h *ProxyPoolHandler) BatchDelete(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	if len(req.IDs) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "单次最多删除 500 条"})
		return
	}
	result := h.db.Where("id IN ?", req.IDs).Delete(&model.ProxyPoolEntry{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "删除失败"})
		return
	}
	h.proxyPool.Reload()
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("已删除 %d 条", result.RowsAffected)})
}

// ── 批量导入 ────────────────────────────────────────────────────────

// Import 批量导入代理（POST /api/admin/proxy-pool/import）
// 支持格式：每行一个，格式 protocol://[user:pass@]host:port 或 host:port（默认 socks5）
func (h *ProxyPoolHandler) Import(c *gin.Context) {
	var req struct {
		Proxies  string `json:"proxies" binding:"required"` // 换行分隔的代理列表
		Protocol string `json:"protocol"`                   // 默认协议（未指定时使用）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	defaultProto := req.Protocol
	if defaultProto == "" {
		defaultProto = "socks5"
	}

	// 协议白名单校验
	allowedProtos := map[string]bool{"http": true, "https": true, "socks5": true}
	if !allowedProtos[defaultProto] {
		c.JSON(http.StatusBadRequest, gin.H{"detail": fmt.Sprintf("不支持的协议: %s，仅支持 http/https/socks5", defaultProto)})
		return
	}

	const maxImportLines = 1000
	scanner := bufio.NewScanner(strings.NewReader(req.Proxies))
	var imported, skipped, failed int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if imported+skipped+failed >= maxImportLines {
			failed++
			continue
		}

		entry, err := parseProxyLine(line, defaultProto)
		if err != nil {
			log.Debug().Str("line", line).Err(err).Msg("代理导入解析失败")
			failed++
			continue
		}

		// 协议白名单（解析后二次校验）
		if !allowedProtos[entry.Protocol] {
			failed++
			continue
		}

		// SSRF 防护：拒绝私网/保留地址
		if netutil.IsPrivateHost(entry.Host) {
			log.Warn().Str("host", entry.Host).Msg("代理导入拒绝内网地址")
			failed++
			continue
		}

		// 去重检查
		var count int64
		h.db.Model(&model.ProxyPoolEntry{}).Where("host = ? AND port = ?", entry.Host, entry.Port).Count(&count)
		if count > 0 {
			skipped++
			continue
		}

		entry.Source = "imported"
		if err := h.db.Create(entry).Error; err != nil {
			failed++
			continue
		}
		imported++
	}

	h.proxyPool.Reload()
	c.JSON(http.StatusOK, gin.H{
		"imported": imported,
		"skipped":  skipped,
		"failed":   failed,
		"message":  fmt.Sprintf("导入完成：成功 %d，重复跳过 %d，失败 %d", imported, skipped, failed),
	})
}

// parseProxyLine 解析单行代理地址
// 支持格式：
//   - socks5://user:pass@host:port
//   - host:port（使用默认协议）
//   - host:port:user:pass（使用默认协议）
func parseProxyLine(line, defaultProto string) (*model.ProxyPoolEntry, error) {
	entry := &model.ProxyPoolEntry{}

	// 带协议前缀
	if strings.Contains(line, "://") {
		parts := strings.SplitN(line, "://", 2)
		entry.Protocol = parts[0]
		rest := parts[1]

		// 提取认证信息
		if atIdx := strings.LastIndex(rest, "@"); atIdx > 0 {
			authPart := rest[:atIdx]
			rest = rest[atIdx+1:]
			if colonIdx := strings.Index(authPart, ":"); colonIdx > 0 {
				entry.Username = authPart[:colonIdx]
				entry.Password = authPart[colonIdx+1:]
			}
		}

		host, port, err := parseHostPort(rest)
		if err != nil {
			return nil, err
		}
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("端口超出范围: %d", port)
		}
		entry.Host = host
		entry.Port = port
	} else {
		// 无协议前缀
		entry.Protocol = defaultProto
		parts := strings.Split(line, ":")
		switch len(parts) {
		case 2:
			// host:port
			port, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("端口格式错误: %s", parts[1])
			}
			if port < 1 || port > 65535 {
				return nil, fmt.Errorf("端口超出范围: %d", port)
			}
			entry.Host = parts[0]
			entry.Port = port
		case 4:
			// host:port:user:pass
			port, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("端口格式错误: %s", parts[1])
			}
			if port < 1 || port > 65535 {
				return nil, fmt.Errorf("端口超出范围: %d", port)
			}
			entry.Host = parts[0]
			entry.Port = port
			entry.Username = parts[2]
			entry.Password = parts[3]
		default:
			return nil, fmt.Errorf("无法解析: %s", line)
		}
	}

	return entry, nil
}

// parseHostPort 解析 host:port（支持 IPv6 如 [::1]:8080）
func parseHostPort(s string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		return "", 0, fmt.Errorf("无效 host:port: %s", s)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("端口格式错误: %s", portStr)
	}
	return host, port, nil
}

// ── 健康检查 ────────────────────────────────────────────────────────

// 健康检查冷却：防止频繁触发导致大量出站连接
var (
	healthCheckMu       sync.Mutex
	healthCheckLastTime time.Time
)

const healthCheckCooldown = 30 * time.Second // 最短间隔 30 秒

// HealthCheck 触发代理池健康检查（POST /api/admin/proxy-pool/health-check）
// 支持选择性检查：请求体含 ids 字段时仅检查指定代理（跳过冷却），为空时检查全部
func (h *ProxyPoolHandler) HealthCheck(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	// 忽略绑定错误，允许空 body（兼容旧调用）
	_ = c.ShouldBindJSON(&req)

	// 指定 ID 列表：小批量检查，跳过全局冷却
	if len(req.IDs) > 0 {
		if len(req.IDs) > 500 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "单次最多检查 500 个代理"})
			return
		}
		total, healthy, unhealthy := h.proxyPool.HealthCheckByIDs(req.IDs)
		c.JSON(http.StatusOK, gin.H{
			"total":     total,
			"healthy":   healthy,
			"unhealthy": unhealthy,
			"message":   fmt.Sprintf("检查完成：共 %d 个，健康 %d，不健康 %d", total, healthy, unhealthy),
		})
		return
	}

	// 全量检查：受 30 秒冷却限制
	healthCheckMu.Lock()
	if time.Since(healthCheckLastTime) < healthCheckCooldown {
		remaining := healthCheckCooldown - time.Since(healthCheckLastTime)
		healthCheckMu.Unlock()
		c.JSON(http.StatusTooManyRequests, gin.H{
			"detail":      fmt.Sprintf("健康检查冷却中，请 %d 秒后再试", int(remaining.Seconds())+1),
			"retry_after": int(remaining.Seconds()) + 1,
		})
		return
	}
	healthCheckLastTime = time.Now()
	healthCheckMu.Unlock()

	total, healthy, unhealthy := h.proxyPool.HealthCheckAll()
	c.JSON(http.StatusOK, gin.H{
		"total":     total,
		"healthy":   healthy,
		"unhealthy": unhealthy,
		"message":   fmt.Sprintf("检查完成：共 %d 个，健康 %d，不健康 %d", total, healthy, unhealthy),
	})
}

// Stats 代理池统计（GET /api/admin/proxy-pool/stats）
func (h *ProxyPoolHandler) Stats(c *gin.Context) {
	stats := h.proxyPool.PoolStats()
	c.JSON(http.StatusOK, stats)
}

// PurgeUnhealthy 清除所有不健康代理（POST /api/admin/proxy-pool/purge）
func (h *ProxyPoolHandler) PurgeUnhealthy(c *gin.Context) {
	result := h.db.Where("is_healthy = ?", false).Delete(&model.ProxyPoolEntry{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "清除失败"})
		return
	}
	h.proxyPool.Reload()
	c.JSON(http.StatusOK, gin.H{
		"deleted": result.RowsAffected,
		"message": fmt.Sprintf("已清除 %d 个不健康代理", result.RowsAffected),
	})
}

// ResetHealth 重置指定代理的健康状态（POST /api/admin/proxy-pool/:id/reset）
func (h *ProxyPoolHandler) ResetHealth(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 ID"})
		return
	}
	now := time.Now()
	result := h.db.Model(&model.ProxyPoolEntry{}).Where("id = ?", id).Updates(map[string]interface{}{
		"is_healthy":      true,
		"fail_count":      0,
		"last_checked_at": now,
	})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "代理不存在"})
		return
	}
	h.proxyPool.Reload()
	c.JSON(http.StatusOK, gin.H{"message": "已重置健康状态"})
}

// ── 从 URL 抓取 ────────────────────────────────────────────────────

// FetchURL 从 URL 抓取代理列表并导入（POST /api/admin/proxy-pool/fetch-url）
func (h *ProxyPoolHandler) FetchURL(c *gin.Context) {
	var req struct {
		URL      string `json:"url" binding:"required"`
		Protocol string `json:"protocol"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	defaultProto := req.Protocol
	if defaultProto == "" {
		defaultProto = "socks5"
	}

	allowedProtos := map[string]bool{"http": true, "https": true, "socks5": true}
	if !allowedProtos[defaultProto] {
		c.JSON(http.StatusBadRequest, gin.H{"detail": fmt.Sprintf("不支持的协议: %s，仅支持 http/https/socks5", defaultProto)})
		return
	}

	// SSRF 防护：校验 URL 协议白名单
	if err := netutil.ValidateURLScheme(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	// SSRF 防护：校验 URL 主机不指向内网
	if err := netutil.ValidateURLHost(req.URL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	// 使用 SSRF 安全客户端下载
	client := netutil.NewSSRFSafeClient(30 * time.Second)
	httpReq, err := http.NewRequest("GET", req.URL, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": fmt.Sprintf("无效的 URL: %s", err.Error())})
		return
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	httpReq.Header.Set("Accept", "text/plain, */*")

	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"detail": fmt.Sprintf("抓取失败: %s", err.Error())})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"detail": fmt.Sprintf("远程返回 HTTP %d", resp.StatusCode)})
		return
	}

	// 限制读取大小（5MB）
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"detail": fmt.Sprintf("读取内容失败: %s", err.Error())})
		return
	}

	// 逐行解析并导入
	const maxLines = 2000
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	var imported, skipped, failed int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		if imported+skipped+failed >= maxLines {
			failed++
			continue
		}

		entry, err := parseProxyLine(line, defaultProto)
		if err != nil {
			failed++
			continue
		}

		// 协议白名单
		if !allowedProtos[entry.Protocol] {
			failed++
			continue
		}

		// SSRF 防护
		if netutil.IsPrivateHost(entry.Host) {
			failed++
			continue
		}

		// 去重
		var count int64
		h.db.Model(&model.ProxyPoolEntry{}).Where("host = ? AND port = ?", entry.Host, entry.Port).Count(&count)
		if count > 0 {
			skipped++
			continue
		}

		entry.Source = "fetched"
		if err := h.db.Create(entry).Error; err != nil {
			failed++
			continue
		}
		imported++
	}

	h.proxyPool.Reload()
	c.JSON(http.StatusOK, gin.H{
		"imported": imported,
		"skipped":  skipped,
		"failed":   failed,
		"message":  fmt.Sprintf("抓取完成：成功 %d，重复跳过 %d，失败 %d", imported, skipped, failed),
	})
}
