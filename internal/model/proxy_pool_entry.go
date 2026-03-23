package model

import (
	"fmt"
	"net/url"
	"time"
)

// ProxyPoolEntry 全局代理池条目（管理员管理，所有用户共享）
type ProxyPoolEntry struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	Name          string     `gorm:"size:100" json:"name"`
	Protocol      string     `gorm:"size:20;not null;default:'socks5'" json:"protocol"` // socks5, http, https
	Host          string     `gorm:"size:200;not null;uniqueIndex:idx_host_port" json:"host"`
	Port          int        `gorm:"not null;uniqueIndex:idx_host_port" json:"port"`
	Username      string     `gorm:"size:100" json:"username"`
	Password      string     `gorm:"size:200" json:"password"`
	Country       string     `gorm:"size:10" json:"country"` // 国家代码，如 US, JP
	Source        string     `gorm:"size:50" json:"source"`  // 来源：manual, imported, fetched
	IsHealthy     bool       `gorm:"default:true;index" json:"is_healthy"`
	LastCheckedAt *time.Time `json:"last_checked_at"`
	LatencyMs     int        `gorm:"default:0" json:"latency_ms"` // 最近一次检测延迟
	FailCount     int        `gorm:"default:0" json:"fail_count"` // 连续失败次数
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

// URL 生成完整代理地址（凭证经过 URL 编码，避免特殊字符破坏解析）
func (p *ProxyPoolEntry) URL() string {
	u := &url.URL{
		Scheme: p.Protocol,
		Host:   fmt.Sprintf("%s:%d", p.Host, p.Port),
	}
	if p.Username != "" {
		u.User = url.UserPassword(p.Username, p.Password)
	}
	return u.String()
}

// MaskedURL 脱敏代理地址（日志用）
func (p *ProxyPoolEntry) MaskedURL() string {
	auth := ""
	if p.Username != "" {
		auth = "***@"
	}
	return fmt.Sprintf("%s://%s%s:%d", p.Protocol, auth, p.Host, p.Port)
}

// HostPort 返回 host:port 格式（用于去重和 TCP 连通性检测）
func (p *ProxyPoolEntry) HostPort() string {
	return fmt.Sprintf("%s:%d", p.Host, p.Port)
}
