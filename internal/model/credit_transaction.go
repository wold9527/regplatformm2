package model

import "time"

// CreditTransaction 积分交易记录
type CreditTransaction struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	Amount      int       `gorm:"not null" json:"amount"` // 正=充值，负=消费
	Type        string    `gorm:"size:30;not null" json:"type"`
	Description string    `gorm:"size:500;default:''" json:"description"`
	TaskID      *uint     `gorm:"index" json:"task_id"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

// 交易类型常量
const (
	TxTypeRecharge         = "recharge"
	TxTypeConsume          = "consume"
	TxTypeRefund           = "refund"
	TxTypeFreeTrial        = "free_trial"
	TxTypeFreeTrialConsume = "free_trial_consume"
	TxTypeFreeTrialRefund  = "free_trial_refund"
	TxTypeRedeem           = "redeem"
	TxTypePurchase         = "purchase"       // New-API 余额购买 credits
	TxTypeNewUserBonus     = "new_user_bonus" // 新用户赠送积分
)
