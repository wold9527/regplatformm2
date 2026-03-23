package model

import "time"

// RedemptionCode 兑换码
type RedemptionCode struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Code       string     `gorm:"size:32;uniqueIndex;not null" json:"code"` // XXXX-XXXX-XXXX-XXXX
	Credits    int        `gorm:"not null" json:"credits"`
	BatchName  string     `gorm:"size:100;default:''" json:"batch_name"`
	CreatedBy  *uint      `json:"created_by"`
	RedeemedBy *uint      `json:"redeemed_by"`
	IsUsed     bool       `gorm:"default:false" json:"is_used"`
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
	RedeemedAt *time.Time `json:"redeemed_at"`

	Creator  *User `gorm:"foreignKey:CreatedBy" json:"-"`
	Redeemer *User `gorm:"foreignKey:RedeemedBy" json:"-"`
}
