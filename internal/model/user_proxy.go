package model

import (
	"fmt"
	"time"
)

// UserProxy 用户保存的代理
type UserProxy struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Name      string    `gorm:"size:100" json:"name"`
	Protocol  string    `gorm:"size:10;not null" json:"protocol"` // http, https, socks5
	Host      string    `gorm:"size:200;not null" json:"host"`
	Port      int       `gorm:"not null" json:"port"`
	Username  string    `gorm:"size:100" json:"username"`
	Password  string    `gorm:"size:200" json:"password"`
	IsDefault bool      `gorm:"default:false" json:"is_default"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// URL 生成完整代理地址
func (p *UserProxy) URL() string {
	auth := ""
	if p.Username != "" {
		auth = p.Username + ":" + p.Password + "@"
	}
	return fmt.Sprintf("%s://%s%s:%d", p.Protocol, auth, p.Host, p.Port)
}

// MaskedURL 脱敏代理地址（日志用）
func (p *UserProxy) MaskedURL() string {
	auth := ""
	if p.Username != "" {
		auth = "***@"
	}
	return fmt.Sprintf("%s://%s%s:%d", p.Protocol, auth, p.Host, p.Port)
}
