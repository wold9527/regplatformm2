package model

import "time"

// HFToken — HF 账号令牌
type HFToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Label     string    `gorm:"size:100;not null" json:"label"`
	Username  string    `gorm:"size:100" json:"username"`          // whoami 自动填充
	Token     string    `gorm:"size:200;not null" json:"token"`
	IsValid   bool      `gorm:"default:true" json:"is_valid"`
	SpaceUsed int       `gorm:"default:0" json:"space_used"`      // 关联 Space 数量
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// HFSpace — Space 注册表（替代 hf_registry.json）
type HFSpace struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Service     string     `gorm:"size:20;not null;index" json:"service"`   // openai/grok/kiro/gemini/ts
	RepoID      string     `gorm:"size:200;not null" json:"repo_id"`
	URL         string     `gorm:"size:500;not null" json:"url"`
	TokenID     uint       `gorm:"index" json:"token_id"`
	Status      string     `gorm:"size:20;default:'unknown'" json:"status"` // healthy/banned/sleeping/dead/unknown
	StatusCode  int        `gorm:"default:0" json:"status_code"`
	LastCheckAt *time.Time `json:"last_check_at"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}
