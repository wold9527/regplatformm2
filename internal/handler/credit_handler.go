package handler

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xiaolajiaoyyds/regplatformm/internal/dto"
	"github.com/xiaolajiaoyyds/regplatformm/internal/middleware"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"github.com/xiaolajiaoyyds/regplatformm/internal/service"
	"gorm.io/gorm"
)

// CreditHandler 积分处理器
type CreditHandler struct {
	db         *gorm.DB
	creditSvc  *service.CreditService
	settingSvc *service.SettingService
}

// NewCreditHandler 创建积分处理器
func NewCreditHandler(db *gorm.DB, creditSvc *service.CreditService, settingSvc *service.SettingService) *CreditHandler {
	return &CreditHandler{db: db, creditSvc: creditSvc, settingSvc: settingSvc}
}

// dailyResetTime 返回今天的重置时间点（凌晨 6 点）
// 如果当前时间在 6 点之前，则返回昨天 6 点
func dailyResetTime() time.Time {
	now := time.Now()
	reset := time.Date(now.Year(), now.Month(), now.Day(), 6, 0, 0, 0, now.Location())
	if now.Before(reset) {
		reset = reset.AddDate(0, 0, -1)
	}
	return reset
}

// Balance 查询余额（GET /api/credits/balance）
func (h *CreditHandler) Balance(c *gin.Context) {
	user := middleware.GetUser(c)
	credits, freeRemaining, freeUsed := h.creditSvc.GetBalance(user.ID)

	// 读取系统限制
	maxTarget, _ := strconv.Atoi(h.settingSvc.Get("max_target", "1000"))
	maxThreads, _ := strconv.Atoi(h.settingSvc.Get("max_threads", "16"))
	dailyLimit, _ := strconv.Atoi(h.settingSvc.Get("daily_reg_limit", "0"))

	// 计算今日已注册数量（从凌晨 6 点算起）
	resetTime := dailyResetTime()
	var dailyUsed int64
	h.db.Model(&model.Task{}).
		Where("user_id = ? AND created_at >= ?", user.ID, resetTime).
		Select("COALESCE(SUM(success_count), 0)").Scan(&dailyUsed)

	dailyRemaining := -1 // -1 表示不限制
	if dailyLimit > 0 {
		dailyRemaining = dailyLimit - int(dailyUsed)
		if dailyRemaining < 0 {
			dailyRemaining = 0
		}
	}

	// 可用注册次数 = 本地积分 + 免费试用剩余
	registrationsAvailable := credits + freeRemaining

	// 免费试用信息
	freeTrialCount := h.settingSvc.GetInt("free_trial_count", 2)
	freeTrialEnabled := h.settingSvc.Get("free_trial_enabled", "true") == "true"

	trialEligible := !freeUsed && freeTrialEnabled && freeTrialCount > 0
	trialRemaining := 0
	trialTotal := 0
	if freeUsed {
		trialRemaining = freeRemaining
	} else if trialEligible {
		trialRemaining = freeTrialCount
		trialTotal = freeTrialCount
	}

	// New-API 余额（实时查询）
	var newapiBalance float64
	var newapiBalanceDisplay string
	var unitPrice float64
	var unitPriceDisplay string
	var newapiAvailable int
	mode := "local"
	display := fmt.Sprintf("%d 次", registrationsAvailable)

	costStr := h.settingSvc.Get("newapi_cost_per_reg", "0.004")
	costUSD, _ := strconv.ParseFloat(costStr, 64)
	if costUSD <= 0 {
		costUSD = 0.004
	}

	baseURL := h.settingSvc.Get("newapi_base_url", "")
	if baseURL != "" && user.NewapiID != 0 {
		mode = "newapi"
		unitPrice = costUSD
		unitPriceDisplay = fmt.Sprintf("$%.4g", costUSD)

		quotaRaw, balUSD, err := h.creditSvc.GetNewAPIBalance(user)
		if err == nil {
			newapiBalance = balUSD
			newapiBalanceDisplay = fmt.Sprintf("$%.2f", balUSD)
			newapiAvailable = int(math.Floor(float64(quotaRaw) / (costUSD * 500000)))
			display = fmt.Sprintf("$%.2f", balUSD)
		} else {
			newapiBalanceDisplay = "$0"
			display = fmt.Sprintf("%d 次", registrationsAvailable)
		}
	}

	c.JSON(http.StatusOK, dto.BalanceResp{
		Credits:                credits,
		FreeTrialRemaining:     freeRemaining,
		FreeTrialUsed:          freeUsed,
		CostPerReg:             1,
		Mode:                   mode,
		Display:                display,
		RegistrationsAvailable: registrationsAvailable,
		NewapiBalance:          newapiBalance,
		NewapiBalanceDisplay:   newapiBalanceDisplay,
		UnitPrice:              unitPrice,
		UnitPriceDisplay:       unitPriceDisplay,
		NewapiAvailable:        newapiAvailable,
		PlatformPrices:         buildPlatformPrices(h.settingSvc, costUSD),
		FreeTrial: &dto.FreeTrialResp{
			Eligible:  trialEligible,
			Remaining: trialRemaining,
			Total:     trialTotal,
		},
		Limits: &dto.LimitsResp{
			MaxTarget:               maxTarget,
			MaxThreads:              maxThreads,
			DailyRegLimit:           dailyLimit,
			DailyUsed:               int(dailyUsed),
			DailyRemaining:          dailyRemaining,
			PlatformGrokEnabled:     h.settingSvc.Get("platform_grok_enabled", "true") == "true",
			PlatformOpenaiEnabled:   h.settingSvc.Get("platform_openai_enabled", "true") == "true",
			PlatformKiroEnabled:     h.settingSvc.Get("platform_kiro_enabled", "true") == "true",
			PlatformGeminiEnabled:   h.settingSvc.Get("platform_gemini_enabled", "true") == "true",
			ThreadTiers:             h.settingSvc.Get("thread_tiers", ""),
			PlatformGrokFreeUntil:   h.settingSvc.Get("platform_grok_free_until", ""),
			PlatformOpenaiFreeUntil: h.settingSvc.Get("platform_openai_free_until", ""),
			PlatformKiroFreeUntil:   h.settingSvc.Get("platform_kiro_free_until", ""),
			PlatformGeminiFreeUntil: h.settingSvc.Get("platform_gemini_free_until", ""),
			FreeMode:                buildFreeModeStatus(h.db, h.settingSvc, user.ID),
			PlatformLimits:          buildPlatformLimits(h.db, h.settingSvc, user.ID),
		},
	})
}

// History 交易记录（GET /api/credits/history）
func (h *CreditHandler) History(c *gin.Context) {
	user := middleware.GetUser(c)
	var txs []model.CreditTransaction
	h.db.Where("user_id = ?", user.ID).
		Order("created_at DESC").
		Limit(50).
		Find(&txs)
	c.JSON(http.StatusOK, txs)
}

// Redeem 兑换码（POST /api/credits/redeem）
func (h *CreditHandler) Redeem(c *gin.Context) {
	user := middleware.GetUser(c)
	var req dto.RedeemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	credits, err := h.creditSvc.RedeemCode(h.db, user.ID, req.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "兑换成功", "credits": credits})
}

// ClaimFreeTrial 领取免费试用（POST /api/credits/free-trial）
func (h *CreditHandler) ClaimFreeTrial(c *gin.Context) {
	user := middleware.GetUser(c)
	count, err := h.creditSvc.ClaimFreeTrial(h.db, user.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "领取成功", "count": count})
}

// Purchase 购买注册次数（POST /api/credits/purchase）
func (h *CreditHandler) Purchase(c *gin.Context) {
	user := middleware.GetUser(c)
	var req dto.PurchaseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请输入有效的购买数量（1-10000）"})
		return
	}

	if err := h.creditSvc.PurchaseCredits(h.db, user.ID, req.Amount, req.Platform); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	// 返回更新后的余额（复用 Balance 逻辑）
	h.Balance(c)
}

// buildPlatformPrices 构建各平台单价映射（优先用平台专属价，降级全局价）
func buildPlatformPrices(settingSvc *service.SettingService, globalCost float64) map[string]float64 {
	prices := map[string]float64{}
	for _, p := range []string{"grok", "openai", "kiro", "gemini"} {
		key := "platform_" + p + "_unit_price"
		val := settingSvc.Get(key, "")
		if val != "" {
			if parsed, err := strconv.ParseFloat(val, 64); err == nil && parsed > 0 {
				prices[p] = parsed
				continue
			}
		}
		prices[p] = globalCost
	}
	return prices
}

// buildFreeModeStatus 构建各平台的免费模式状态（仅限时免费期内的平台）
// 包级函数，供 CreditHandler 和 InitHandler 共用
func buildFreeModeStatus(db *gorm.DB, settingSvc *service.SettingService, userID uint) map[string]*dto.FreeModeInfo {
	freeMode := map[string]*dto.FreeModeInfo{}
	resetTime := dailyResetTime()

	for _, platform := range []string{"grok", "openai", "kiro", "gemini"} {
		if !isPlatformFreeByHandler(settingSvc, platform) {
			continue
		}

		prefix := "platform_" + platform + "_free_"
		dailyLimit, _ := strconv.Atoi(settingSvc.Get(prefix+"daily_limit", "5"))
		taskLimit, _ := strconv.Atoi(settingSvc.Get(prefix+"task_limit", "2"))
		cooldownMin, _ := strconv.Atoi(settingSvc.Get(prefix+"cooldown", "30"))

		// 查今日免费使用量
		var dailyFreeUsed int64
		db.Model(&model.Task{}).
			Where("user_id = ? AND platform = ? AND credits_reserved = 0 AND created_at >= ?", userID, platform, resetTime).
			Select("COALESCE(SUM(target_count), 0)").Scan(&dailyFreeUsed)

		dailyRemaining := -1
		if dailyLimit > 0 {
			dailyRemaining = dailyLimit - int(dailyFreeUsed)
			if dailyRemaining < 0 {
				dailyRemaining = 0
			}
		}

		// 查冷却剩余（上一轮用满 task_limit 才触发，没用满不冷却）
		cooldownRemainingSec := 0
		lastTaskMaxed := false
		if cooldownMin > 0 && taskLimit > 0 {
			var lastTask model.Task
			err := db.Where("user_id = ? AND platform = ? AND credits_reserved = 0 AND stopped_at IS NOT NULL", userID, platform).
				Order("stopped_at DESC").First(&lastTask).Error
			if err == nil && lastTask.StoppedAt != nil && lastTask.TargetCount >= taskLimit {
				lastTaskMaxed = true
				cooldownEnd := lastTask.StoppedAt.Add(time.Duration(cooldownMin) * time.Minute)
				if time.Now().Before(cooldownEnd) {
					cooldownRemainingSec = int(time.Until(cooldownEnd).Seconds())
				}
			}
		}

		// 判断是否可用
		available := true
		reason := ""
		dailyLimitReached := dailyLimit > 0 && dailyRemaining <= 0
		if dailyLimitReached {
			// 今日额度用完
			available = false
			reason = "今日免费额度已用完"
		} else if lastTaskMaxed && cooldownRemainingSec > 0 {
			// 上一轮用满了 task_limit，冷却中
			available = false
			reason = fmt.Sprintf("上一轮已用满 %d 个，冷却中还需 %d 分钟", taskLimit, cooldownRemainingSec/60+1)
		}

		freeMode[platform] = &dto.FreeModeInfo{
			Available:         available,
			DailyUsed:         int(dailyFreeUsed),
			DailyLimit:        dailyLimit,
			DailyRemaining:    dailyRemaining,
			TaskLimit:         taskLimit,
			CooldownSec:       cooldownMin * 60,
			CooldownRemaining: cooldownRemainingSec,
			Reason:            reason,
		}
	}

	if len(freeMode) == 0 {
		return nil
	}
	return freeMode
}

// buildPlatformLimits 构建各平台的独立限制（付费/免费均生效）
// 包级函数，供 CreditHandler 和 InitHandler 共用
func buildPlatformLimits(db *gorm.DB, settingSvc *service.SettingService, userID uint) map[string]*dto.PlatformLimitInfo {
	result := map[string]*dto.PlatformLimitInfo{}
	resetTime := dailyResetTime()

	for _, platform := range []string{"grok", "openai", "kiro", "gemini"} {
		taskLimit := settingSvc.GetInt("platform_"+platform+"_task_limit", 0)
		dailyLimit := settingSvc.GetInt("platform_"+platform+"_daily_limit", 0)

		// 两个都是 0 表示该平台没有独立限制，跳过
		if taskLimit == 0 && dailyLimit == 0 {
			continue
		}

		dailyUsed := 0
		dailyRemaining := -1 // -1 = 不限
		if dailyLimit > 0 {
			var used int64
			db.Model(&model.Task{}).
				Where("user_id = ? AND platform = ? AND created_at >= ?", userID, platform, resetTime).
				Select("COALESCE(SUM(success_count), 0)").Scan(&used)
			dailyUsed = int(used)
			dailyRemaining = dailyLimit - dailyUsed
			if dailyRemaining < 0 {
				dailyRemaining = 0
			}
		}

		result[platform] = &dto.PlatformLimitInfo{
			TaskLimit:      taskLimit,
			DailyLimit:     dailyLimit,
			DailyUsed:      dailyUsed,
			DailyRemaining: dailyRemaining,
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// isPlatformFreeByHandler 判断平台是否限时免费（handler 层复用）
func isPlatformFreeByHandler(settingSvc *service.SettingService, platform string) bool {
	dateStr := settingSvc.Get("platform_"+platform+"_free_until", "")
	if dateStr == "" {
		return false
	}
	deadline, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}
	return time.Now().Before(deadline.AddDate(0, 0, 1))
}
