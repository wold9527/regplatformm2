package model

import "time"

// User 用户模型
type User struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	Username           string    `gorm:"size:100;not null;uniqueIndex" json:"username"`
	PasswordHash       string    `gorm:"size:200;default:''" json:"-"`
	Name               string    `gorm:"size:200;default:''" json:"name"`
	Email              string    `gorm:"size:200;default:''" json:"email"`
	AvatarURL          string    `gorm:"size:500;default:''" json:"avatar_url"`
	Role               int       `gorm:"default:1" json:"role"` // 1=user, 10=admin, 100=root
	Credits            int       `gorm:"default:0" json:"credits"`
	FreeTrialUsed      bool      `gorm:"default:false" json:"free_trial_used"`
	FreeTrialRemaining int       `gorm:"default:0" json:"free_trial_remaining"`
	IsActive           bool      `gorm:"default:true" json:"is_active"`
	IsAdmin            bool      `gorm:"default:false" json:"is_admin"`
	CreatedAt          time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt          time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// 可选：SSO 对接字段（用于外部系统集成）
	LinuxdoID   int `gorm:"default:0" json:"linuxdo_id,omitempty"`
	NewapiID    int `gorm:"default:0;index" json:"newapi_id,omitempty"`
	TrustLevel  int `gorm:"default:0" json:"trust_level,omitempty"`
	NewapiQuota int `gorm:"default:0" json:"newapi_quota,omitempty"`
}

// IsRoot 是否超级管理员
func (u *User) IsRoot() bool {
	return u.Role >= 100
}
