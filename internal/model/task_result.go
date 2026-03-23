package model

import (
	"time"

	"gorm.io/datatypes"
)

// TaskResult 注册成功结果
type TaskResult struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	TaskID         uint           `gorm:"not null;index" json:"task_id"`
	UserID         uint           `gorm:"not null;index" json:"user_id"`
	Platform       string         `gorm:"size:20;default:'grok'" json:"platform"`
	Email          string         `gorm:"size:200;default:''" json:"email"`
	CredentialData datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"credential_data"` // JSONB 存储凭据
	SSOToken       string         `gorm:"type:text;default:''" json:"auth_token"`           // 向后兼容 Grok
	NsfwEnabled    bool           `gorm:"default:false" json:"feature_enabled"`
	IsArchived     bool           `gorm:"default:false;index" json:"is_archived"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`

	// 软禁用（验活失效时标记而非硬删除，可恢复）
	Disabled       bool       `gorm:"default:false;index" json:"disabled"`
	DisabledReason string     `gorm:"size:500;default:''" json:"disabled_reason"`
	DisabledAt     *time.Time `json:"disabled_at"`

	// 最后验活时间
	LastValidatedAt *time.Time `json:"last_validated_at"`

	// 关联
	Task Task `gorm:"foreignKey:TaskID" json:"-"`
	User User `gorm:"foreignKey:UserID" json:"-"`
}
