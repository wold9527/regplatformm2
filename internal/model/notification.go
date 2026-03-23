package model

import "time"

// Notification 系统通知（UserID=0 表示广播）
type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"` // 0 = 广播给所有用户
	Title     string    `gorm:"size:200;not null" json:"title"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	IsRead    bool      `gorm:"default:false;index" json:"is_read"` // 仅用于个人通知（user_id>0）
	CreatedBy uint      `json:"created_by"`
	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

// NotificationRead 广播通知的用户已读记录
type NotificationRead struct {
	NotificationID uint      `gorm:"primaryKey" json:"notification_id"`
	UserID         uint      `gorm:"primaryKey" json:"user_id"`
	ReadAt         time.Time `gorm:"autoCreateTime" json:"read_at"`
}
