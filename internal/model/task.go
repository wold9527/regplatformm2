package model

import "time"

// Task 注册任务
type Task struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	UserID           uint       `gorm:"not null;index" json:"user_id"`
	Platform         string     `gorm:"size:20;default:'grok'" json:"platform"`
	Status           string     `gorm:"size:20;default:'pending'" json:"status"` // pending/queued/running/stopping/completed/failed/stopped
	TargetCount      int        `gorm:"not null" json:"target_count"`
	ThreadCount      int        `gorm:"default:4" json:"thread_count"`
	SuccessCount     int        `gorm:"default:0" json:"success_count"`
	FailCount        int        `gorm:"default:0" json:"fail_count"`
	CreditsReserved  int        `gorm:"default:0" json:"credits_reserved"`
	CreditsConsumed  int        `gorm:"default:0" json:"credits_consumed"`
	ProxyURL         string     `gorm:"size:500;default:''" json:"proxy_url"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	StartedAt        *time.Time `json:"started_at"`
	StoppedAt        *time.Time `json:"stopped_at"`

	// 关联
	User    User         `gorm:"foreignKey:UserID" json:"-"`
	Results []TaskResult `gorm:"foreignKey:TaskID" json:"-"`
}

// IsDone 任务是否已结束
func (t *Task) IsDone() bool {
	return t.Status == "completed" || t.Status == "failed" || t.Status == "stopped"
}
