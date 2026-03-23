package handler

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xiaolajiaoyyds/regplatformm/internal/middleware"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"github.com/xiaolajiaoyyds/regplatformm/internal/service"
	"gorm.io/gorm"
)

// InitHandler 页面初始化批量接口，一次返回所有数据
type InitHandler struct {
	db         *gorm.DB
	creditSvc  *service.CreditService
	settingSvc *service.SettingService
	engine     *service.TaskEngine
}

// NewInitHandler 创建初始化处理器
func NewInitHandler(db *gorm.DB, creditSvc *service.CreditService, settingSvc *service.SettingService, engine *service.TaskEngine) *InitHandler {
	return &InitHandler{db: db, creditSvc: creditSvc, settingSvc: settingSvc, engine: engine}
}

// Init 页面初始化（GET /api/init）
// 并行查询用户、余额、当前任务、结果、交易记录、公告，一次返回
func (h *InitHandler) Init(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "未登录"})
		return
	}

	var wg sync.WaitGroup
	resp := make(map[string]interface{})
	var mu sync.Mutex

	// 1. 用户信息（直接从 middleware 获取，无需查库）
	resp["user"] = gin.H{
		"id":                   user.ID,
		"newapi_id":            user.NewapiID,
		"username":             user.Username,
		"name":                 user.Name,
		"email":                user.Email,
		"avatar_url":           user.AvatarURL,
		"trust_level":          user.TrustLevel,
		"role":                 user.Role,
		"credits":              user.Credits,
		"newapi_quota":         user.NewapiQuota,
		"free_trial_used":      user.FreeTrialUsed,
		"free_trial_remaining": user.FreeTrialRemaining,
		"is_admin":             user.IsAdmin,
	}

	// 2. 余额（并行）
	wg.Add(1)
	go func() {
		defer wg.Done()
		credits, freeRemaining, freeUsed := h.creditSvc.GetBalance(user.ID)
		maxTarget, _ := strconv.Atoi(h.settingSvc.Get("max_target", "1000"))
		maxThreads, _ := strconv.Atoi(h.settingSvc.Get("max_threads", "16"))
		dailyLimit, _ := strconv.Atoi(h.settingSvc.Get("daily_reg_limit", "0"))

		resetTime := dailyResetTime()
		var dailyUsed int64
		h.db.Model(&model.Task{}).
			Where("user_id = ? AND created_at >= ?", user.ID, resetTime).
			Select("COALESCE(SUM(success_count), 0)").Scan(&dailyUsed)

		dailyRemaining := -1
		if dailyLimit > 0 {
			dailyRemaining = dailyLimit - int(dailyUsed)
			if dailyRemaining < 0 {
				dailyRemaining = 0
			}
		}

		registrationsAvailable := credits + freeRemaining

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

		mu.Lock()
		resp["balance"] = gin.H{
			"credits":                credits,
			"free_trial_remaining":   freeRemaining,
			"free_trial_used":        freeUsed,
			"cost_per_reg":           1,
			"mode":                   mode,
			"display":                display,
			"registrations_available": registrationsAvailable,
			"newapi_balance":          newapiBalance,
			"newapi_balance_display":  newapiBalanceDisplay,
			"unit_price":              unitPrice,
			"unit_price_display":      unitPriceDisplay,
			"newapi_available":        newapiAvailable,
			"platform_prices":         buildInitPlatformPrices(h.settingSvc, costUSD),
			"free_trial": gin.H{
				"eligible":  trialEligible,
				"remaining": trialRemaining,
				"total":     trialTotal,
			},
			"limits": gin.H{
				"max_target":                  maxTarget,
				"max_threads":                 maxThreads,
				"daily_reg_limit":             dailyLimit,
				"daily_used":                  int(dailyUsed),
				"daily_remaining":             dailyRemaining,
				"platform_grok_enabled":       h.settingSvc.Get("platform_grok_enabled", "true") == "true",
				"platform_openai_enabled":     h.settingSvc.Get("platform_openai_enabled", "true") == "true",
				"platform_kiro_enabled":       h.settingSvc.Get("platform_kiro_enabled", "true") == "true",
				"platform_gemini_enabled":     h.settingSvc.Get("platform_gemini_enabled", "true") == "true",
				"thread_tiers":                h.settingSvc.Get("thread_tiers", ""),
				"platform_grok_free_until":    h.settingSvc.Get("platform_grok_free_until", ""),
				"platform_openai_free_until":  h.settingSvc.Get("platform_openai_free_until", ""),
				"platform_kiro_free_until":    h.settingSvc.Get("platform_kiro_free_until", ""),
				"platform_gemini_free_until":  h.settingSvc.Get("platform_gemini_free_until", ""),
				"free_mode":                   buildFreeModeStatus(h.db, h.settingSvc, user.ID),
				"platform_limits":             buildPlatformLimits(h.db, h.settingSvc, user.ID),
			},
		}
		mu.Unlock()
	}()

	// 3. 当前任务（并行）
	wg.Add(1)
	go func() {
		defer wg.Done()
		var task model.Task
		err := h.db.Where("user_id = ? AND status IN ?", user.ID, []string{"running", "queued", "pending"}).
			Order("created_at DESC").First(&task).Error
		if err != nil {
			// 没有活跃任务，尝试取最近一条已结束的任务（保留顶栏计数）
			err2 := h.db.Where("user_id = ?", user.ID).
				Order("created_at DESC").First(&task).Error
			if err2 != nil {
				mu.Lock()
				resp["current_task"] = nil
				mu.Unlock()
				return
			}
		}
		// 检查孤儿任务（事务保护，防止并发双重退款）
		if task.Status == "running" && h.engine.GetStatus(user.ID, task.Platform) == nil {
			h.db.Transaction(func(tx *gorm.DB) error {
				var fresh model.Task
				if err := tx.Where("id = ? AND status = ?", task.ID, "running").First(&fresh).Error; err != nil {
					return err
				}
				now := time.Now()
				fresh.Status = "stopped"
				fresh.StoppedAt = &now
				tx.Save(&fresh)
				refund := fresh.CreditsReserved - fresh.SuccessCount
				if refund > 0 {
					h.creditSvc.RefundCredits(tx, user.ID, fresh.ID, refund)
				}
				task = fresh
				return nil
			})
		}
		// 清理孤儿排队任务：超过 10 分钟仍未调度的 queued/pending 视为孤儿
		if (task.Status == "queued" || task.Status == "pending") && time.Since(task.CreatedAt) > 10*time.Minute {
			h.db.Transaction(func(tx *gorm.DB) error {
				var fresh model.Task
				if err := tx.Where("id = ? AND status IN ?", task.ID, []string{"queued", "pending"}).First(&fresh).Error; err != nil {
					return err
				}
				now := time.Now()
				fresh.Status = "stopped"
				fresh.StoppedAt = &now
				tx.Save(&fresh)
				refund := fresh.CreditsReserved - fresh.SuccessCount
				if refund > 0 {
					h.creditSvc.RefundCredits(tx, user.ID, fresh.ID, refund)
				}
				task = fresh
				return nil
			})
		}
		mu.Lock()
		resp["current_task"] = gin.H{
			"task_id":          task.ID,
			"platform":         task.Platform,
			"target":           task.TargetCount,
			"threads":          task.ThreadCount,
			"credits_reserved": task.CreditsReserved,
			"success_count":    task.SuccessCount,
			"fail_count":       task.FailCount,
			"status":           task.Status,
			"is_done":          task.IsDone(),
		}
		mu.Unlock()
	}()

	// 4. 注册结果（并行）
	wg.Add(1)
	go func() {
		defer wg.Done()
		var results []model.TaskResult
		// 最多加载 200 条未归档结果，避免全量拉取阻塞首屏
		h.db.Where("user_id = ? AND is_archived = ?", user.ID, false).
			Order("created_at DESC").Limit(200).Find(&results)

		type resultItem struct {
			ID             uint            `json:"id"`
			Email          string          `json:"email"`
			Platform       string          `json:"platform"`
			CredentialData json.RawMessage `json:"credential_data"`
			SSOToken       string          `json:"auth_token,omitempty"`
			NsfwEnabled    bool            `json:"feature_enabled"`
			CreatedAt      string          `json:"created_at"`
		}
		items := make([]resultItem, 0, len(results))
		for _, r := range results {
			items = append(items, resultItem{
				ID:             r.ID,
				Email:          r.Email,
				Platform:       r.Platform,
				CredentialData: json.RawMessage(r.CredentialData),
				SSOToken:       r.SSOToken,
				NsfwEnabled:    r.NsfwEnabled,
				CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		}
		mu.Lock()
		resp["results"] = items
		mu.Unlock()
	}()

	// 5. 交易记录（并行）
	wg.Add(1)
	go func() {
		defer wg.Done()
		var txs []model.CreditTransaction
		h.db.Where("user_id = ?", user.ID).
			Order("created_at DESC").Limit(50).Find(&txs)
		mu.Lock()
		resp["tx_history"] = txs
		mu.Unlock()
	}()

	// 6. 公告（并行）
	wg.Add(1)
	go func() {
		defer wg.Done()
		var list []model.Announcement
		h.db.Order("created_at DESC").Limit(20).Find(&list)
		mu.Lock()
		resp["announcements"] = list
		mu.Unlock()
	}()

	// 7. 全局统计（并行）
	wg.Add(1)
	go func() {
		defer wg.Done()
		stats := h.buildGlobalStats()
		mu.Lock()
		resp["global_stats"] = stats
		mu.Unlock()
	}()

	// 8. 用户总成功/失败统计 + 各平台平均耗时（并行）
	wg.Add(1)
	go func() {
		defer wg.Done()
		var totalSuccess, totalFail int64
		h.db.Model(&model.Task{}).Where("user_id = ?", user.ID).
			Select("COALESCE(SUM(success_count), 0)").Scan(&totalSuccess)
		h.db.Model(&model.Task{}).Where("user_id = ?", user.ID).
			Select("COALESCE(SUM(fail_count), 0)").Scan(&totalFail)

		// 各平台成功/失败统计
		type platformStatRow struct {
			Platform string `json:"platform"`
			Success  int64  `json:"success"`
			Fail     int64  `json:"fail"`
		}
		var platformRows []platformStatRow
		h.db.Model(&model.Task{}).Where("user_id = ?", user.ID).
			Select("platform, COALESCE(SUM(success_count), 0) as success, COALESCE(SUM(fail_count), 0) as fail").
			Group("platform").Scan(&platformRows)
		platformMap := map[string]gin.H{}
		for _, pr := range platformRows {
			platformMap[pr.Platform] = gin.H{"success": pr.Success, "fail": pr.Fail}
		}

		// 各平台平均每个注册耗时（秒），取最近30个已完成任务
		type avgRow struct {
			Platform   string  `json:"platform"`
			AvgSecPer  float64 `json:"avg_sec_per"`
		}
		var rows []avgRow
		h.db.Raw(`
			SELECT platform,
				AVG(
					EXTRACT(EPOCH FROM stopped_at - started_at)
					/ NULLIF(success_count, 0)
				) AS avg_sec_per
			FROM tasks
			WHERE success_count > 0 AND started_at IS NOT NULL AND stopped_at IS NOT NULL
				AND status IN ('completed','stopped')
			GROUP BY platform
		`).Scan(&rows)
		avgMap := map[string]float64{}
		for _, r := range rows {
			if r.AvgSecPer > 0 {
				avgMap[r.Platform] = r.AvgSecPer
			}
		}

		mu.Lock()
		resp["user_stats"] = gin.H{
			"total_success":   totalSuccess,
			"total_fail":      totalFail,
			"avg_sec_per_reg": avgMap,
			"by_platform":     platformMap,
		}
		mu.Unlock()
	}()

	// 9. 最近完成的注册（公共展示，并行）
	wg.Add(1)
	go func() {
		defer wg.Done()
		items := h.buildRecentCompletions()
		mu.Lock()
		resp["recent_completions"] = items
		mu.Unlock()
	}()

	wg.Wait()
	c.JSON(http.StatusOK, resp)
}

// GlobalStats 全局统计（GET /api/stats/global）
// 面向所有已登录用户，用于顶栏全局大屏
func (h *InitHandler) GlobalStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.buildGlobalStats())
}

// LatestCompletions 最新完成的任务（GET /api/stats/latest-completions?after=<unix>）
// 供首页 toast 轮询，返回 after 之后完成的任务（全局，所有用户）
func (h *InitHandler) LatestCompletions(c *gin.Context) {
	afterStr := c.Query("after")
	after := time.Now().Add(-10 * time.Second) // 默认最近 10 秒
	if afterStr != "" {
		if ts, err := strconv.ParseInt(afterStr, 10, 64); err == nil {
			after = time.Unix(ts, 0)
		}
	}
	// 上下界保护：最多回溯 5 分钟，不接受未来时间戳
	minAfter := time.Now().Add(-5 * time.Minute)
	if after.Before(minAfter) {
		after = minAfter
	}
	if after.After(time.Now()) {
		after = time.Now().Add(-10 * time.Second)
	}

	var tasks []model.Task
	h.db.Where("status IN ? AND stopped_at > ? AND success_count > 0",
		[]string{"completed", "stopped"}, after).
		Order("stopped_at DESC").Limit(5).Find(&tasks)

	// 批量查用户信息（昵称 + 头像）
	userIDs := make([]uint, 0, len(tasks))
	for _, t := range tasks {
		userIDs = append(userIDs, t.UserID)
	}
	type userBrief struct {
		ID        uint
		Username  string
		Name      string
		AvatarURL string
	}
	userBriefMap := make(map[uint]userBrief)
	if len(userIDs) > 0 {
		var users []model.User
		h.db.Where("id IN ?", userIDs).Select("id, username, name, avatar_url").Find(&users)
		for _, u := range users {
			userBriefMap[u.ID] = userBrief{ID: u.ID, Username: u.Username, Name: u.Name, AvatarURL: u.AvatarURL}
		}
	}

	type completionItem struct {
		TaskID    uint   `json:"task_id"`
		Username  string `json:"username"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Platform  string `json:"platform"`
		Success   int    `json:"success_count"`
		StoppedAt int64  `json:"stopped_at"`
	}
	items := make([]completionItem, 0, len(tasks))
	for _, t := range tasks {
		if t.StoppedAt == nil {
			continue
		}
		ub := userBriefMap[t.UserID]
		items = append(items, completionItem{
			TaskID:    t.ID,
			Username:  ub.Username,
			Name:      ub.Name,
			AvatarURL: ub.AvatarURL,
			Platform:  t.Platform,
			Success:   t.SuccessCount,
			StoppedAt: t.StoppedAt.Unix(),
		})
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

// buildGlobalStats 构建全局统计数据
func (h *InitHandler) buildGlobalStats() gin.H {
	resetTime := dailyResetTime()

	// 运行中任务数
	var runningTasks int64
	h.db.Model(&model.Task{}).Where("status = ?", "running").Count(&runningTasks)

	// 排队中任务数
	var queuedTasks int64
	h.db.Model(&model.Task{}).Where("status = ?", "queued").Count(&queuedTasks)

	// 活跃用户数（有运行中/排队中任务的独立用户）
	var activeUsers int64
	h.db.Model(&model.Task{}).Where("status IN ?", []string{"running", "queued"}).Distinct("user_id").Count(&activeUsers)

	// 今日各平台注册数
	type platformStat struct {
		Platform string `json:"platform"`
		Count    int64  `json:"count"`
	}
	var todayStats []platformStat
	h.db.Model(&model.TaskResult{}).
		Where("created_at >= ?", resetTime).
		Select("platform, COUNT(*) as count").
		Group("platform").
		Scan(&todayStats)
	todayTotal := int64(0)
	todayPlatforms := map[string]int64{}
	for _, ps := range todayStats {
		todayPlatforms[ps.Platform] = ps.Count
		todayTotal += ps.Count
	}

	// 今日成功率
	var todaySuccess, todayFail int64
	h.db.Model(&model.Task{}).Where("created_at >= ?", resetTime).
		Select("COALESCE(SUM(success_count), 0)").Scan(&todaySuccess)
	h.db.Model(&model.Task{}).Where("created_at >= ?", resetTime).
		Select("COALESCE(SUM(fail_count), 0)").Scan(&todayFail)
	var successRate float64
	if todaySuccess+todayFail > 0 {
		successRate = float64(todaySuccess) / float64(todaySuccess+todayFail) * 100
	}

	// 总注册数
	var totalResults int64
	h.db.Model(&model.TaskResult{}).Count(&totalResults)

	return gin.H{
		"running_tasks":   runningTasks,
		"queued_tasks":    queuedTasks,
		"active_users":    activeUsers,
		"today_total":     todayTotal,
		"today_platforms": todayPlatforms,
		"today_success":   todaySuccess,
		"today_fail":      todayFail,
		"success_rate":    successRate,
		"total_results":   totalResults,
	}
}

// buildRecentCompletions 构建最近完成的注册列表（全站公共展示）
// 每用户只展示最新一条，按完成时间倒序，最多 10 条
func (h *InitHandler) buildRecentCompletions() []gin.H {
	var tasks []model.Task
	h.db.Raw(`
		SELECT * FROM (
			SELECT DISTINCT ON (user_id) *
			FROM tasks
			WHERE status IN ('completed', 'stopped')
			  AND success_count > 0
			  AND stopped_at IS NOT NULL
			ORDER BY user_id, stopped_at DESC
		) sub
		ORDER BY stopped_at DESC
		LIMIT 10
	`).Scan(&tasks)

	userIDs := make([]uint, 0, len(tasks))
	for _, t := range tasks {
		userIDs = append(userIDs, t.UserID)
	}
	type userInfo struct {
		ID        uint
		Username  string
		Name      string
		AvatarURL string
	}
	userInfoMap := make(map[uint]userInfo)
	if len(userIDs) > 0 {
		var users []model.User
		h.db.Where("id IN ?", userIDs).Select("id, username, name, avatar_url").Find(&users)
		for _, u := range users {
			userInfoMap[u.ID] = userInfo{ID: u.ID, Username: u.Username, Name: u.Name, AvatarURL: u.AvatarURL}
		}
	}

	items := make([]gin.H, 0, len(tasks))
	for _, t := range tasks {
		ts := int64(0)
		if t.StoppedAt != nil {
			ts = t.StoppedAt.Unix()
		}
		durationSec := 0
		if t.StartedAt != nil && t.StoppedAt != nil {
			durationSec = int(t.StoppedAt.Sub(*t.StartedAt).Seconds())
		}
		ui := userInfoMap[t.UserID]
		items = append(items, gin.H{
			"task_id":       t.ID,
			"username":      ui.Username,
			"name":          ui.Name,
			"avatar_url":    ui.AvatarURL,
			"platform":      t.Platform,
			"success_count": t.SuccessCount,
			"target_count":  t.TargetCount,
			"duration_sec":  durationSec,
			"stopped_at":    ts,
		})
	}
	return items
}

// RecentCompletions 最近完成的注册（GET /api/stats/recent-completions）
// 全站公共展示，返回最新 10 条成功的注册任务
func (h *InitHandler) RecentCompletions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": h.buildRecentCompletions()})
}

// buildInitPlatformPrices 构建各平台单价映射（Init 接口用）
func buildInitPlatformPrices(settingSvc *service.SettingService, globalCost float64) map[string]float64 {
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
