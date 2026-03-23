package model

import (
	"fmt"
	"time"
)

// CardPoolEntry 全局虚拟卡池条目（管理员管理，Team 开通共享）
type CardPoolEntry struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Name           string     `gorm:"size:100" json:"name"`                          // 卡别名
	CardNumber     string     `gorm:"size:30;not null;uniqueIndex" json:"card_number"` // 加密存储
	ExpMonth       int        `gorm:"not null" json:"exp_month"`
	ExpYear        int        `gorm:"not null" json:"exp_year"`
	CVC            string     `gorm:"size:10;not null" json:"cvc"`
	BillingName    string     `gorm:"size:200" json:"billing_name"`
	BillingEmail   string     `gorm:"size:200" json:"billing_email"`
	BillingCountry string     `gorm:"size:10;default:'US'" json:"billing_country"`
	BillingCity    string     `gorm:"size:100" json:"billing_city"`
	BillingLine1   string     `gorm:"size:300" json:"billing_line1"`
	BillingZip     string     `gorm:"size:20" json:"billing_zip"`
	Provider       string     `gorm:"size:50" json:"provider"`  // manual, privacy, wise, api
	Source         string     `gorm:"size:50" json:"source"`    // manual, imported, api
	IsValid        bool       `gorm:"default:true;index" json:"is_valid"`
	UseCount       int        `gorm:"default:0" json:"use_count"`
	FailCount      int        `gorm:"default:0" json:"fail_count"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	LastValidAt    *time.Time `json:"last_valid_at"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

// MaskedNumber 返回脱敏卡号（仅显示后 4 位）
func (c *CardPoolEntry) MaskedNumber() string {
	if len(c.CardNumber) <= 4 {
		return c.CardNumber
	}
	return fmt.Sprintf("**** **** **** %s", c.CardNumber[len(c.CardNumber)-4:])
}
