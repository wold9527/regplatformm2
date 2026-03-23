package handler

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xiaolajiaoyyds/regplatformm/internal/middleware"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"golang.org/x/net/proxy"
	"gorm.io/gorm"
)

// ProxyHandler 用户代理管理
type ProxyHandler struct {
	db *gorm.DB
}

// NewProxyHandler 创建代理处理器
func NewProxyHandler(db *gorm.DB) *ProxyHandler {
	return &ProxyHandler{db: db}
}

type createProxyReq struct {
	Name     string `json:"name" binding:"max=100"`
	Protocol string `json:"protocol" binding:"required,oneof=http https socks5"`
	Host     string `json:"host" binding:"required,max=200"`
	Port     int    `json:"port" binding:"required,min=1,max=65535"`
	Username string `json:"username" binding:"max=100"`
	Password string `json:"password" binding:"max=200"`
}

// List 获取当前用户的代理列表（GET /api/proxies）
func (h *ProxyHandler) List(c *gin.Context) {
	user := middleware.GetUser(c)
	var proxies []model.UserProxy
	h.db.Where("user_id = ?", user.ID).Order("created_at DESC").Find(&proxies)
	// 脱敏密码
	for i := range proxies {
		if proxies[i].Password != "" {
			proxies[i].Password = "******"
		}
	}
	c.JSON(http.StatusOK, proxies)
}

// Create 添加代理（POST /api/proxies）
func (h *ProxyHandler) Create(c *gin.Context) {
	user := middleware.GetUser(c)
	var req createProxyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	// 限制每用户最多 20 个代理
	var count int64
	h.db.Model(&model.UserProxy{}).Where("user_id = ?", user.ID).Count(&count)
	if count >= 20 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "最多保存 20 个代理"})
		return
	}

	p := model.UserProxy{
		UserID:   user.ID,
		Name:     req.Name,
		Protocol: req.Protocol,
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
	}
	if err := h.db.Create(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "保存失败"})
		return
	}
	if p.Password != "" {
		p.Password = "******"
	}
	c.JSON(http.StatusOK, p)
}

// Update 更新代理（PUT /api/proxies/:id）
func (h *ProxyHandler) Update(c *gin.Context) {
	user := middleware.GetUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 ID"})
		return
	}

	var p model.UserProxy
	if err := h.db.Where("id = ? AND user_id = ?", id, user.ID).First(&p).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "代理不存在"})
		return
	}

	var req createProxyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	p.Name = req.Name
	p.Protocol = req.Protocol
	p.Host = req.Host
	p.Port = req.Port
	p.Username = req.Username
	if req.Password != "******" {
		p.Password = req.Password
	}
	h.db.Save(&p)
	if p.Password != "" {
		p.Password = "******"
	}
	c.JSON(http.StatusOK, p)
}

// Delete 删除代理（DELETE /api/proxies/:id）
func (h *ProxyHandler) Delete(c *gin.Context) {
	user := middleware.GetUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 ID"})
		return
	}
	result := h.db.Where("id = ? AND user_id = ?", id, user.ID).Delete(&model.UserProxy{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "代理不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// Test 测试代理连通性（POST /api/proxies/test）
// 支持两种方式：传 proxy_id 测试已保存的，或直接传代理信息测试
func (h *ProxyHandler) Test(c *gin.Context) {
	user := middleware.GetUser(c)

	var req struct {
		ProxyID  uint   `json:"proxy_id"`
		Protocol string `json:"protocol"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	var proxyURL string
	if req.ProxyID > 0 {
		var p model.UserProxy
		if err := h.db.Where("id = ? AND user_id = ?", req.ProxyID, user.ID).First(&p).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"detail": "代理不存在"})
			return
		}
		proxyURL = p.URL()
	} else if req.Host != "" {
		auth := ""
		if req.Username != "" {
			auth = req.Username + ":" + req.Password + "@"
		}
		proxyURL = fmt.Sprintf("%s://%s%s:%d", req.Protocol, auth, req.Host, req.Port)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请提供代理信息"})
		return
	}

	ok, latency, err := testProxy(proxyURL)
	if ok {
		c.JSON(http.StatusOK, gin.H{"ok": true, "latency_ms": latency, "message": fmt.Sprintf("连接成功（延迟 %dms）", latency)})
	} else {
		msg := "连接失败"
		if err != nil {
			msg = "连接失败: " + err.Error()
		}
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": msg})
	}
}

// testProxy 测试代理连通性，返回是否成功、延迟毫秒、错误
func testProxy(proxyURL string) (bool, int64, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return false, 0, fmt.Errorf("代理地址格式错误: %w", err)
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
			return false, 0, fmt.Errorf("SOCKS5 初始化失败: %w", err)
		}
		var conn net.Conn
		if cd, ok := dialer.(proxy.ContextDialer); ok {
			conn, err = cd.DialContext(ctx, "tcp", "httpbin.org:80")
		} else {
			conn, err = dialer.Dial("tcp", "httpbin.org:80")
		}
		if err != nil {
			return false, 0, fmt.Errorf("SOCKS5 连接失败: %w", err)
		}
		conn.Close()

	case "http", "https":
		transport := &http.Transport{
			Proxy: http.ProxyURL(parsed),
		}
		client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://httpbin.org/ip", nil)
		resp, err := client.Do(req)
		if err != nil {
			return false, 0, fmt.Errorf("HTTP 代理连接失败: %w", err)
		}
		resp.Body.Close()

	default:
		return false, 0, fmt.Errorf("不支持的协议: %s", parsed.Scheme)
	}

	latency := time.Since(start).Milliseconds()
	return true, latency, nil
}
