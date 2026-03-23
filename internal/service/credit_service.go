package service

import (
	"fmt"
	"math"
	"strconv"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"github.com/xiaolajiaoyyds/regplatformm/internal/pkg/newapi"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreditService 积分服务
type CreditService struct {
	db             *gorm.DB
	settingSvc     *SettingService
	refundLocks    map[uint]*sync.Mutex // task_id -> lock
	refundAmounts  map[uint]int         // task_id -> 已退还数量
	purchaseLocks  map[uint]*sync.Mutex // user_id -> lock，防止并发购买竞态
	mu             sync.Mutex
}

// NewCreditService 创建积分服务
func NewCreditService(db *gorm.DB, settingSvc *SettingService) *CreditService {
	return &CreditService{
		db:            db,
		settingSvc:    settingSvc,
		refundLocks:   make(map[uint]*sync.Mutex),
		refundAmounts: make(map[uint]int),
		purchaseLocks: make(map[uint]*sync.Mutex),
	}
}

// GetBalance 获取用户余额
func (s *CreditService) GetBalance(userID uint) (credits int, freeRemaining int, freeUsed bool) {
	var user model.User
	if err := s.db.Select("credits, free_trial_remaining, free_trial_used").
		Where("id = ?", userID).First(&user).Error; err != nil {
		return 0, 0, false
	}
	return user.Credits, user.FreeTrialRemaining, user.FreeTrialUsed
}

// ReserveCredits 预扣积分（创建任务时，使用 SELECT FOR UPDATE 防竞态）
func (s *CreditService) ReserveCredits(db *gorm.DB, userID uint, taskID uint, amount int) error {
	var user model.User
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", userID).First(&user).Error; err != nil {
		return err
	}

	// 优先使用免费试用
	freeUsed := 0
	paidUsed := 0
	if user.FreeTrialRemaining > 0 {
		freeUsed = min(user.FreeTrialRemaining, amount)
		paidUsed = amount - freeUsed
	} else {
		paidUsed = amount
	}

	// 检查余额
	if paidUsed > user.Credits {
		return fmt.Errorf("积分不足：需要 %d，当前 %d（含免费 %d）", amount, user.Credits+user.FreeTrialRemaining, user.FreeTrialRemaining)
	}

	// 扣减
	if freeUsed > 0 {
		db.Model(&user).Update("free_trial_remaining", gorm.Expr("free_trial_remaining - ?", freeUsed))
		s.recordTx(db, userID, &taskID, -freeUsed, model.TxTypeFreeTrialConsume, fmt.Sprintf("任务 #%d 预扣免费试用 %d 次", taskID, freeUsed))
	}
	if paidUsed > 0 {
		db.Model(&user).Update("credits", gorm.Expr("credits - ?", paidUsed))
		s.recordTx(db, userID, &taskID, -paidUsed, model.TxTypeConsume, fmt.Sprintf("任务 #%d 预扣积分 %d", taskID, paidUsed))
	}

	return nil
}

// RefundCredits 退还积分（任务结束时退还未消费部分）
func (s *CreditService) RefundCredits(db *gorm.DB, userID uint, taskID uint, amount int) {
	if amount <= 0 {
		return
	}

	// 获取任务级退款锁
	lock := s.getRefundLock(taskID)
	lock.Lock()
	defer lock.Unlock()

	already := s.refundAmounts[taskID]
	if already >= amount {
		return // 已全部退还
	}
	toRefund := amount - already

	db.Model(&model.User{}).Where("id = ?", userID).
		Update("credits", gorm.Expr("credits + ?", toRefund))
	s.recordTx(db, userID, &taskID, toRefund, model.TxTypeRefund,
		fmt.Sprintf("任务 #%d 退还积分 %d", taskID, toRefund))

	s.mu.Lock()
	s.refundAmounts[taskID] = already + toRefund
	s.mu.Unlock()

	log.Info().Uint("task_id", taskID).Int("refund", toRefund).Msg("积分已退还")
}

// Recharge 管理员充值
func (s *CreditService) Recharge(db *gorm.DB, userID uint, amount int, desc string) error {
	if err := db.Model(&model.User{}).Where("id = ?", userID).
		Update("credits", gorm.Expr("credits + ?", amount)).Error; err != nil {
		return err
	}
	s.recordTx(db, userID, nil, amount, model.TxTypeRecharge, desc)
	return nil
}

// ClaimFreeTrial 领取免费试用
func (s *CreditService) ClaimFreeTrial(db *gorm.DB, userID uint) (int, error) {
	enabled := s.settingSvc.Get("free_trial_enabled", "true")
	if enabled != "true" {
		return 0, fmt.Errorf("免费试用已关闭")
	}

	var user model.User
	if err := db.Where("id = ?", userID).First(&user).Error; err != nil {
		return 0, err
	}
	if user.FreeTrialUsed {
		return 0, fmt.Errorf("已领取过免费试用")
	}

	count := s.settingSvc.GetInt("free_trial_count", 2)
	db.Model(&user).Updates(map[string]interface{}{
		"free_trial_used":      true,
		"free_trial_remaining": count,
	})
	s.recordTx(db, userID, nil, count, model.TxTypeFreeTrial, fmt.Sprintf("领取免费试用 %d 次", count))
	return count, nil
}

// RedeemCode 兑换码兑换（事务 + 行锁防双花）
func (s *CreditService) RedeemCode(db *gorm.DB, userID uint, code string) (int, error) {
	var credits int
	err := db.Transaction(func(tx *gorm.DB) error {
		var rc model.RedemptionCode
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("code = ? AND is_used = ?", code, false).First(&rc).Error; err != nil {
			return fmt.Errorf("无效的兑换码")
		}

		// 标记已使用
		now := gorm.Expr("NOW()")
		tx.Model(&rc).Updates(map[string]interface{}{
			"is_used":     true,
			"redeemed_by": userID,
			"redeemed_at": now,
		})

		// 增加积分
		tx.Model(&model.User{}).Where("id = ?", userID).
			Update("credits", gorm.Expr("credits + ?", rc.Credits))
		s.recordTx(tx, userID, nil, rc.Credits, model.TxTypeRedeem,
			fmt.Sprintf("兑换码 %s 充值 %d 积分", code, rc.Credits))

		credits = rc.Credits
		return nil
	})
	if err != nil {
		return 0, err
	}
	return credits, nil
}

// recordTx 记录交易
func (s *CreditService) recordTx(db *gorm.DB, userID uint, taskID *uint, amount int, txType, desc string) {
	tx := model.CreditTransaction{
		UserID:      userID,
		Amount:      amount,
		Type:        txType,
		Description: desc,
		TaskID:      taskID,
	}
	if err := db.Create(&tx).Error; err != nil {
		log.Error().Err(err).Uint("user_id", userID).Str("type", txType).Msg("记录交易失败")
	}
}

// CleanupTask 清理已完成任务的退款锁和退款记录（防内存泄漏）
func (s *CreditService) CleanupTask(taskID uint) {
	s.mu.Lock()
	delete(s.refundLocks, taskID)
	delete(s.refundAmounts, taskID)
	s.mu.Unlock()
}

func (s *CreditService) getRefundLock(taskID uint) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.refundLocks[taskID]; ok {
		return l
	}
	l := &sync.Mutex{}
	s.refundLocks[taskID] = l
	return l
}

// getPurchaseLock 获取用户级购买锁（防止同一用户并发购买）
func (s *CreditService) getPurchaseLock(userID uint) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.purchaseLocks[userID]; ok {
		return l
	}
	l := &sync.Mutex{}
	s.purchaseLocks[userID] = l
	return l
}

// PurchaseCredits 用 New-API 余额购买注册次数
// 流程：验证 → 查 New-API 余额 → 扣 New-API quota → 加内部 credits → 记录交易
func (s *CreditService) PurchaseCredits(db *gorm.DB, userID uint, amount int, platform string) error {
	// 获取用户信息
	var user model.User
	if err := db.Where("id = ?", userID).First(&user).Error; err != nil {
		return fmt.Errorf("用户不存在")
	}

	if user.NewapiID == 0 {
		return fmt.Errorf("未绑定 New-API 账号")
	}

	// per-user 购买锁，防止并发购买竞态
	lock := s.getPurchaseLock(userID)
	lock.Lock()
	defer lock.Unlock()

	// 获取 New-API 客户端配置
	client, err := s.newAPIClient()
	if err != nil {
		return err
	}

	// 计算需要扣减的 quota（500000 = $1 USD）
	// 优先使用平台专属单价，降级全局单价
	costUSD := 0.0
	if platform != "" {
		platformKey := "platform_" + platform + "_unit_price"
		platformCostStr := s.settingSvc.Get(platformKey, "")
		if platformCostStr != "" {
			if parsed, err := strconv.ParseFloat(platformCostStr, 64); err == nil && parsed > 0 {
				costUSD = parsed
			}
		}
	}
	if costUSD <= 0 {
		costStr := s.settingSvc.Get("newapi_cost_per_reg", "0.004")
		costUSD, _ = strconv.ParseFloat(costStr, 64)
		if costUSD <= 0 {
			return fmt.Errorf("单价配置错误")
		}
	}
	quotaCost := int(math.Ceil(float64(amount) * costUSD * 500000))
	if err := client.DeductQuota(user.NewapiID, quotaCost); err != nil {
		return err
	}

	// 扣减成功，增加内部 credits
	if err := db.Model(&model.User{}).Where("id = ?", userID).
		Update("credits", gorm.Expr("credits + ?", amount)).Error; err != nil {
		log.Error().Err(err).Uint("user_id", userID).Msg("增加积分失败（New-API 已扣款）")
		return fmt.Errorf("增加积分失败，请联系管理员")
	}

	// 同步更新本地 newapi_quota 缓存
	if newQuota, err := client.GetUserQuota(user.NewapiID); err == nil {
		db.Model(&model.User{}).Where("id = ?", userID).Update("newapi_quota", newQuota)
	}

	// 记录交易
	costDisplay := fmt.Sprintf("%.4f", float64(quotaCost)/500000.0)
	s.recordTx(db, userID, nil, amount, model.TxTypePurchase,
		fmt.Sprintf("购买 %d 次注册（消耗 $%s）", amount, costDisplay))

	log.Info().Uint("user_id", userID).Int("amount", amount).Int("quota_cost", quotaCost).Msg("购买注册次数成功")
	return nil
}

// newAPIClient 创建 New-API 客户端（统一读取配置）
func (s *CreditService) newAPIClient() (*newapi.Client, error) {
	baseURL := s.settingSvc.Get("newapi_base_url", "")
	adminToken := s.settingSvc.Get("newapi_admin_token", "")
	if baseURL == "" || adminToken == "" {
		return nil, fmt.Errorf("New-API 未配置")
	}
	adminUserID := s.settingSvc.GetInt("newapi_admin_id", 0)
	if adminUserID == 0 {
		return nil, fmt.Errorf("New-API 管理员用户 ID 未配置")
	}
	return newapi.NewClient(baseURL, adminToken, adminUserID), nil
}

// GetNewAPIBalance 实时查询 New-API 余额
func (s *CreditService) GetNewAPIBalance(user *model.User) (quotaRaw int, balanceUSD float64, err error) {
	if user.NewapiID == 0 {
		return 0, 0, fmt.Errorf("未绑定 New-API 账号")
	}

	client, err := s.newAPIClient()
	if err != nil {
		return 0, 0, err
	}

	quota, err := client.GetUserQuota(user.NewapiID)
	if err != nil {
		return 0, 0, err
	}

	return quota, float64(quota) / 500000.0, nil
}
