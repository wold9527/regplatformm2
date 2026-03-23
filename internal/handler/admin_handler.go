package handler

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xiaolajiaoyyds/regplatformm/internal/dto"
	"github.com/xiaolajiaoyyds/regplatformm/internal/middleware"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"github.com/xiaolajiaoyyds/regplatformm/internal/pkg/tempmail"
	"github.com/xiaolajiaoyyds/regplatformm/internal/service"
	"gorm.io/gorm"
)

// AdminHandler 管理后台处理器
type AdminHandler struct {
	db         *gorm.DB
	creditSvc  *service.CreditService
	settingSvc *service.SettingService
	proxyPool  *service.ProxyPool
	taskEngine *service.TaskEngine
}

// NewAdminHandler 创建管理后台处理器
func NewAdminHandler(db *gorm.DB, creditSvc *service.CreditService, settingSvc *service.SettingService, proxyPool *service.ProxyPool, taskEngine *service.TaskEngine) *AdminHandler {
	return &AdminHandler{db: db, creditSvc: creditSvc, settingSvc: settingSvc, proxyPool: proxyPool, taskEngine: taskEngine}
}

// ListUsers 用户列表（GET /api/admin/users?page=&page_size=&search=）
// 支持分页和搜索，page_size=-1 返回全量
func (h *AdminHandler) ListUsers(c *gin.Context) {
	search := c.Query("search")

	base := h.db.Model(&model.User{})
	if search != "" {
		like := "%" + search + "%"
		base = base.Where("username ILIKE ? OR name ILIKE ? OR email ILIKE ?", like, like, like)
	}

	// Session 克隆防止 Count 污染后续 Find 查询
	var total int64
	base.Session(&gorm.Session{}).Count(&total)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}

	// 排序：支持 credits / newapi_quota / created_at，默认 created_at DESC
	sortBy := c.DefaultQuery("sort_by", "created_at")
	sortOrder := c.DefaultQuery("sort_order", "desc")
	allowedSort := map[string]bool{"credits": true, "newapi_quota": true, "created_at": true}
	if !allowedSort[sortBy] {
		sortBy = "created_at"
	}
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	query := base.Order(sortBy + " " + sortOrder)
	if pageSize > 0 {
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	var users []model.User
	query.Find(&users)

	c.JSON(http.StatusOK, gin.H{
		"items":     users,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Recharge 管理员充值/扣除（POST /api/admin/credits/recharge）
// Credits > 0 为充值，< 0 为扣除
func (h *AdminHandler) Recharge(c *gin.Context) {
	var req dto.RechargeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	// 扣除时检查余额是否足够
	if req.Credits < 0 {
		var user model.User
		if err := h.db.First(&user, req.UserID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"detail": "用户不存在"})
			return
		}
		if user.Credits+req.Credits < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": fmt.Sprintf("余额不足，当前 %d 积分，无法扣除 %d", user.Credits, -req.Credits)})
			return
		}
	}

	admin := middleware.GetUser(c)
	var desc string
	if req.Credits > 0 {
		desc = fmt.Sprintf("管理员 %s 充值 %d 积分", admin.Username, req.Credits)
	} else {
		desc = fmt.Sprintf("管理员 %s 扣除 %d 积分", admin.Username, -req.Credits)
	}
	if err := h.creditSvc.Recharge(h.db, req.UserID, req.Credits, desc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	msg := "充值成功"
	if req.Credits < 0 {
		msg = fmt.Sprintf("已扣除 %d 积分", -req.Credits)
	}
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

// ToggleAdmin 切换管理员状态（POST /api/admin/users/:id/toggle-admin）
func (h *AdminHandler) ToggleAdmin(c *gin.Context) {
	admin := middleware.GetUser(c)
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的用户 ID"})
		return
	}

	var user model.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "用户不存在"})
		return
	}

	// 不能操作 root 用户
	if user.IsRoot() {
		c.JSON(http.StatusForbidden, gin.H{"detail": "无法修改超级管理员"})
		return
	}
	// 非 root 不能操作其他管理员
	if !admin.IsRoot() && user.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"detail": "权限不足"})
		return
	}

	newIsAdmin := !user.IsAdmin
	newRole := 1
	if newIsAdmin {
		newRole = 10
	}
	h.db.Model(&user).Updates(map[string]interface{}{
		"is_admin": newIsAdmin,
		"role":     newRole,
	})

	c.JSON(http.StatusOK, gin.H{"message": "已更新", "is_admin": newIsAdmin})
}

// GenerateCodes 生成兑换码（POST /api/admin/codes）
func (h *AdminHandler) GenerateCodes(c *gin.Context) {
	admin := middleware.GetUser(c)
	var req dto.GenerateCodesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	codes := make([]model.RedemptionCode, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		code := generateCode()
		codes = append(codes, model.RedemptionCode{
			Code:      code,
			Credits:   req.Credits,
			BatchName: req.BatchName,
			CreatedBy: &admin.ID,
		})
	}
	h.db.Create(&codes)

	codeStrs := make([]string, len(codes))
	for i, c := range codes {
		codeStrs[i] = c.Code
	}
	c.JSON(http.StatusOK, gin.H{"codes": codeStrs, "count": len(codes)})
}

// ListCodes 兑换码列表（GET /api/admin/codes?batch=&page=1&page_size=20）
func (h *AdminHandler) ListCodes(c *gin.Context) {
	batch := c.Query("batch")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	query := h.db.Model(&model.RedemptionCode{})
	if batch != "" {
		query = query.Where("batch_name = ?", batch)
	}

	var total int64
	query.Count(&total)

	var codes []model.RedemptionCode
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&codes)
	c.JSON(http.StatusOK, gin.H{
		"items": codes,
		"total": total,
		"page":  page,
		"pages": (int(total) + pageSize - 1) / pageSize,
	})
}

// Stats 全局统计（GET /api/admin/stats）
func (h *AdminHandler) Stats(c *gin.Context) {
	var userCount, taskCount, resultCount int64
	h.db.Model(&model.User{}).Count(&userCount)
	h.db.Model(&model.Task{}).Count(&taskCount)
	h.db.Model(&model.TaskResult{}).Count(&resultCount)

	// 今日统计（凌晨 6 点重置）
	resetTime := dailyResetTime()
	var todayTasks, todayResults int64
	h.db.Model(&model.Task{}).Where("created_at >= ?", resetTime).Count(&todayTasks)
	h.db.Model(&model.TaskResult{}).Where("created_at >= ?", resetTime).Count(&todayResults)

	// 运行中 / 排队中任务分别统计
	var runningTasks, queuedTasks int64
	h.db.Model(&model.Task{}).Where("status = ?", "running").Count(&runningTasks)
	h.db.Model(&model.Task{}).Where("status IN ?", []string{"queued", "pending"}).Count(&queuedTasks)

	// 购买统计（New-API 余额购买）
	var purchaseCount int64
	var purchaseCredits int64
	h.db.Model(&model.CreditTransaction{}).Where("type = ?", model.TxTypePurchase).Count(&purchaseCount)
	h.db.Model(&model.CreditTransaction{}).Where("type = ?", model.TxTypePurchase).
		Select("COALESCE(SUM(amount), 0)").Scan(&purchaseCredits)

	// 今日购买
	var todayPurchaseCount int64
	var todayPurchaseCredits int64
	h.db.Model(&model.CreditTransaction{}).Where("type = ? AND created_at >= ?", model.TxTypePurchase, resetTime).Count(&todayPurchaseCount)
	h.db.Model(&model.CreditTransaction{}).Where("type = ? AND created_at >= ?", model.TxTypePurchase, resetTime).
		Select("COALESCE(SUM(amount), 0)").Scan(&todayPurchaseCredits)

	// 兑换码统计
	var redeemCount int64
	var redeemCredits int64
	h.db.Model(&model.CreditTransaction{}).Where("type = ?", model.TxTypeRedeem).Count(&redeemCount)
	h.db.Model(&model.CreditTransaction{}).Where("type = ?", model.TxTypeRedeem).
		Select("COALESCE(SUM(amount), 0)").Scan(&redeemCredits)

	// 各平台注册数
	type platformStat struct {
		Platform string `json:"platform"`
		Count    int64  `json:"count"`
	}
	var platformStats []platformStat
	h.db.Model(&model.TaskResult{}).
		Select("platform, COUNT(*) as count").
		Group("platform").
		Scan(&platformStats)
	platformMap := make(map[string]int64)
	for _, ps := range platformStats {
		platformMap[ps.Platform] = ps.Count
	}

	// 今日各平台注册数
	var todayPlatformStats []platformStat
	h.db.Model(&model.TaskResult{}).
		Where("created_at >= ?", resetTime).
		Select("platform, COUNT(*) as count").
		Group("platform").
		Scan(&todayPlatformStats)
	todayPlatformMap := make(map[string]int64)
	for _, ps := range todayPlatformStats {
		todayPlatformMap[ps.Platform] = ps.Count
	}

	// 总消费积分（预扣）
	var totalConsumed int64
	h.db.Model(&model.CreditTransaction{}).Where("type = ?", model.TxTypeConsume).
		Select("COALESCE(SUM(ABS(amount)), 0)").Scan(&totalConsumed)

	// 总退还积分
	var totalRefunded int64
	h.db.Model(&model.CreditTransaction{}).Where("type = ?", model.TxTypeRefund).
		Select("COALESCE(SUM(amount), 0)").Scan(&totalRefunded)

	// 平台内积分总量（所有用户当前余额之和）
	var totalCreditsInSystem int64
	h.db.Model(&model.User{}).Select("COALESCE(SUM(credits), 0)").Scan(&totalCreditsInSystem)

	// 成功率：总成功数 / (总成功 + 总失败)
	var totalSuccess, totalFail int64
	h.db.Model(&model.Task{}).Select("COALESCE(SUM(success_count), 0)").Scan(&totalSuccess)
	h.db.Model(&model.Task{}).Select("COALESCE(SUM(fail_count), 0)").Scan(&totalFail)
	var successRate float64
	if totalSuccess+totalFail > 0 {
		successRate = float64(totalSuccess) / float64(totalSuccess+totalFail) * 100
	}

	// 各平台成功率（汇总历史任务）
	type platformSuccessStat struct {
		Platform string  `json:"platform"`
		Success  int64   `json:"success"`
		Fail     int64   `json:"fail"`
	}
	var platformSuccessStats []platformSuccessStat
	h.db.Model(&model.Task{}).
		Select("platform, COALESCE(SUM(success_count), 0) as success, COALESCE(SUM(fail_count), 0) as fail").
		Where("platform != ''").
		Group("platform").
		Scan(&platformSuccessStats)
	type platformRateItem struct {
		Success     int64   `json:"success"`
		Fail        int64   `json:"fail"`
		SuccessRate float64 `json:"success_rate"`
	}
	platformRateMap := make(map[string]platformRateItem)
	for _, ps := range platformSuccessStats {
		rate := 0.0
		if ps.Success+ps.Fail > 0 {
			rate = float64(ps.Success) / float64(ps.Success+ps.Fail) * 100
		}
		platformRateMap[ps.Platform] = platformRateItem{
			Success:     ps.Success,
			Fail:        ps.Fail,
			SuccessRate: rate,
		}
	}

	// 今日各平台成功率
	var todayPlatformSuccessStats []platformSuccessStat
	h.db.Model(&model.Task{}).
		Select("platform, COALESCE(SUM(success_count), 0) as success, COALESCE(SUM(fail_count), 0) as fail").
		Where("platform != '' AND created_at >= ?", resetTime).
		Group("platform").
		Scan(&todayPlatformSuccessStats)
	todayPlatformRateMap := make(map[string]platformRateItem)
	for _, ps := range todayPlatformSuccessStats {
		rate := 0.0
		if ps.Success+ps.Fail > 0 {
			rate = float64(ps.Success) / float64(ps.Success+ps.Fail) * 100
		}
		todayPlatformRateMap[ps.Platform] = platformRateItem{
			Success:     ps.Success,
			Fail:        ps.Fail,
			SuccessRate: rate,
		}
	}

	// 各平台平均任务耗时（秒）：只统计已完成且有 started_at/stopped_at 的任务
	type platformAvgDuration struct {
		Platform    string  `json:"platform"`
		AvgSeconds  float64 `json:"avg_seconds"`
		TaskCount   int64   `json:"task_count"`
	}
	var platformDurations []platformAvgDuration
	h.db.Model(&model.Task{}).
		Select("platform, AVG(EXTRACT(EPOCH FROM (stopped_at - started_at))) as avg_seconds, COUNT(*) as task_count").
		Where("platform != '' AND status IN ? AND started_at IS NOT NULL AND stopped_at IS NOT NULL",
			[]string{"completed", "stopped", "failed"}).
		Group("platform").
		Scan(&platformDurations)
	platformDurationMap := make(map[string]platformAvgDuration)
	for _, pd := range platformDurations {
		platformDurationMap[pd.Platform] = pd
	}

	// 活跃用户（7天 / 30天内有任务的独立用户数）
	now := time.Now()
	day7 := now.AddDate(0, 0, -7)
	day30 := now.AddDate(0, 0, -30)
	var active7d, active30d int64
	h.db.Model(&model.Task{}).Where("created_at >= ?", day7).Distinct("user_id").Count(&active7d)
	h.db.Model(&model.Task{}).Where("created_at >= ?", day30).Distinct("user_id").Count(&active30d)

	// 7 日新增用户趋势（按天统计注册数，最近 7 天）
	type dailyNewUser struct {
		Day   string `json:"day"`
		Count int64  `json:"count"`
	}
	var dailyNewUsers []dailyNewUser
	h.db.Model(&model.User{}).
		Select("DATE(created_at) as day, COUNT(*) as count").
		Where("created_at >= ?", day7).
		Group("DATE(created_at)").
		Order("day ASC").
		Scan(&dailyNewUsers)
	// 补全 7 天，确保前端有完整序列
	dailyMap := make(map[string]int64)
	for _, d := range dailyNewUsers {
		dailyMap[d.Day] = d.Count
	}
	newUsers7d := make([]dailyNewUser, 7)
	for i := 6; i >= 0; i-- {
		dayStr := now.AddDate(0, 0, -i).Format("2006-01-02")
		label := now.AddDate(0, 0, -i).Format("01/02")
		newUsers7d[6-i] = dailyNewUser{Day: label, Count: dailyMap[dayStr]}
	}

	// 最近交易动态（最新 10 笔购买/兑换）
	var recentTxs []model.CreditTransaction
	h.db.Where("type IN ?", []string{model.TxTypePurchase, model.TxTypeRedeem, model.TxTypeRecharge}).
		Order("created_at DESC").Limit(10).
		Preload("User").
		Find(&recentTxs)
	type recentTxItem struct {
		ID          uint   `json:"id"`
		UserID      uint   `json:"user_id"`
		Username    string `json:"username"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		Amount      int    `json:"amount"`
		Description string `json:"description"`
		CreatedAt   string `json:"created_at"`
	}
	recentItems := make([]recentTxItem, 0, len(recentTxs))
	for _, tx := range recentTxs {
		recentItems = append(recentItems, recentTxItem{
			ID:          tx.ID,
			UserID:      tx.UserID,
			Username:    tx.User.Username,
			Name:        tx.User.Name,
			Type:        tx.Type,
			Amount:      tx.Amount,
			Description: tx.Description,
			CreatedAt:   tx.CreatedAt.Format("01-02 15:04"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total_users":              userCount,
		"total_tasks":              taskCount,
		"total_results":            resultCount,
		"today_tasks":              todayTasks,
		"today_results":            todayResults,
		"running_tasks":            runningTasks,
		"queued_tasks":             queuedTasks,
		"purchase_count":           purchaseCount,
		"purchase_credits":         purchaseCredits,
		"today_purchase_count":     todayPurchaseCount,
		"today_purchase_credits":   todayPurchaseCredits,
		"redeem_count":             redeemCount,
		"redeem_credits":           redeemCredits,
		"total_consumed":           totalConsumed,
		"total_refunded":           totalRefunded,
		"total_credits_in_system":  totalCreditsInSystem,
		"total_success":            totalSuccess,
		"total_fail":               totalFail,
		"success_rate":             successRate,
		"active_7d":                active7d,
		"active_30d":               active30d,
		"platforms":                platformMap,
		"today_platforms":          todayPlatformMap,
		"platform_rates":           platformRateMap,
		"today_platform_rates":     todayPlatformRateMap,
		"platform_avg_duration":    platformDurationMap,
		"new_users_7d":             newUsers7d,
		"recent_txs":               recentItems,
	})
}

// GetSettings 获取系统设置（GET /api/admin/settings）
func (h *AdminHandler) GetSettings(c *gin.Context) {
	c.JSON(http.StatusOK, h.settingSvc.GetAll())
}

// SaveSetting 保存设置（POST /api/admin/settings）
func (h *AdminHandler) SaveSetting(c *gin.Context) {
	var req dto.SaveSettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	if err := h.settingSvc.Set(req.Key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	// 代理相关设置变更时刷新代理池
	if req.Key == "default_proxy" && h.proxyPool != nil {
		h.proxyPool.Reload()
	}
	c.JSON(http.StatusOK, gin.H{"message": "已保存"})
}

// GetSettingRaw 获取原始设置值（GET /api/admin/settings/raw?key=）
func (h *AdminHandler) GetSettingRaw(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "缺少 key"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "value": h.settingSvc.GetRaw(key)})
}

// PublicAnnouncements 公告列表 - 面向普通用户（GET /api/announcements）
func (h *AdminHandler) PublicAnnouncements(c *gin.Context) {
	var list []model.Announcement
	h.db.Order("created_at DESC").Limit(20).Find(&list)
	c.JSON(http.StatusOK, list)
}

// CreateAnnouncement 发布公告（POST /api/admin/announcements）
func (h *AdminHandler) CreateAnnouncement(c *gin.Context) {
	admin := middleware.GetUser(c)
	var req struct {
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "标题和内容不能为空"})
		return
	}
	ann := model.Announcement{Title: req.Title, Content: req.Content, CreatedBy: admin.ID}
	h.db.Create(&ann)
	c.JSON(http.StatusOK, ann)
}

// ListAnnouncements 公告列表（GET /api/admin/announcements）
func (h *AdminHandler) ListAnnouncements(c *gin.Context) {
	var list []model.Announcement
	h.db.Order("created_at DESC").Limit(50).Find(&list)
	c.JSON(http.StatusOK, list)
}

// DeleteAnnouncement 删除公告（DELETE /api/admin/announcements/:id）
func (h *AdminHandler) DeleteAnnouncement(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的公告 ID"})
		return
	}
	if err := h.db.Where("id = ?", id).Delete(&model.Announcement{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// UserDetail 用户详情（GET /api/admin/users/:id/detail）
// 返回用户信息、各平台注册账号、任务摘要、最近交易
func (h *AdminHandler) UserDetail(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的用户 ID"})
		return
	}

	var user model.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "用户不存在"})
		return
	}

	// 各平台注册结果（含凭据）
	// 每个平台最多展示一部分，限制总量避免全量拉取
	var results []model.TaskResult
	h.db.Where("user_id = ?", user.ID).Order("created_at DESC").Limit(150).Find(&results)

	type resultItem struct {
		ID             uint            `json:"id"`
		Platform       string          `json:"platform"`
		Email          string          `json:"email"`
		CredentialData json.RawMessage `json:"credential_data"`
		IsArchived     bool            `json:"is_archived"`
		CreatedAt      string          `json:"created_at"`
	}
	platformResults := map[string][]resultItem{
		"grok": {}, "openai": {}, "kiro": {}, "gemini": {},
	}
	for _, r := range results {
		item := resultItem{
			ID:             r.ID,
			Platform:       r.Platform,
			Email:          r.Email,
			CredentialData: json.RawMessage(r.CredentialData),
			IsArchived:     r.IsArchived,
			CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04"),
		}
		p := r.Platform
		if p == "" {
			p = "grok"
		}
		platformResults[p] = append(platformResults[p], item)
	}

	// 任务统计
	var taskCount int64
	var totalSuccess, totalFail int64
	h.db.Model(&model.Task{}).Where("user_id = ?", user.ID).Count(&taskCount)
	h.db.Model(&model.Task{}).Where("user_id = ?", user.ID).
		Select("COALESCE(SUM(success_count), 0)").Scan(&totalSuccess)
	h.db.Model(&model.Task{}).Where("user_id = ?", user.ID).
		Select("COALESCE(SUM(fail_count), 0)").Scan(&totalFail)

	// 最近 20 条交易记录
	var txs []model.CreditTransaction
	h.db.Where("user_id = ?", user.ID).
		Order("created_at DESC").Limit(20).Find(&txs)
	type txItem struct {
		ID          uint   `json:"id"`
		Type        string `json:"type"`
		Amount      int    `json:"amount"`
		Description string `json:"description"`
		CreatedAt   string `json:"created_at"`
	}
	txItems := make([]txItem, 0, len(txs))
	for _, tx := range txs {
		txItems = append(txItems, txItem{
			ID:          tx.ID,
			Type:        tx.Type,
			Amount:      tx.Amount,
			Description: tx.Description,
			CreatedAt:   tx.CreatedAt.Format("01-02 15:04"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"user":             user,
		"platform_results": platformResults,
		"task_summary": gin.H{
			"total_tasks":   taskCount,
			"total_success": totalSuccess,
			"total_fail":    totalFail,
		},
		"recent_txs": txItems,
	})
}

// CleanupData 清理旧数据（POST /api/admin/cleanup）
// 删除指定天数之前的已完成任务、注册结果、交易记录
func (h *AdminHandler) CleanupData(c *gin.Context) {
	var req struct {
		Days            int  `json:"days" binding:"required,min=1"`
		CleanResults    bool `json:"clean_results"`
		CleanTasks      bool `json:"clean_tasks"`
		CleanTx         bool `json:"clean_tx"`
		CleanArchivedOnly bool `json:"clean_archived_only"` // 仅清理已归档的结果
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请输入有效的天数（≥1）"})
		return
	}

	cutoff := time.Now().AddDate(0, 0, -req.Days)
	cleaned := make(map[string]int64)

	// 清理注册结果
	if req.CleanResults {
		query := h.db.Where("created_at < ?", cutoff)
		if req.CleanArchivedOnly {
			query = query.Where("is_archived = ?", true)
		}
		result := query.Delete(&model.TaskResult{})
		cleaned["results"] = result.RowsAffected
	}

	// 清理已完成的任务（不清理运行中/排队中的）
	if req.CleanTasks {
		result := h.db.Where("created_at < ? AND status IN ?", cutoff,
			[]string{"completed", "failed", "stopped"}).Delete(&model.Task{})
		cleaned["tasks"] = result.RowsAffected
	}

	// 清理交易记录
	if req.CleanTx {
		result := h.db.Where("created_at < ?", cutoff).Delete(&model.CreditTransaction{})
		cleaned["transactions"] = result.RowsAffected
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("已清理 %d 天前的数据", req.Days),
		"cleaned": cleaned,
	})
}

// DataStats 数据量统计（GET /api/admin/data-stats）
// 返回各表的行数，供清理页面参考
func (h *AdminHandler) DataStats(c *gin.Context) {
	var taskCount, resultCount, archivedCount, txCount, codeCount int64
	h.db.Model(&model.Task{}).Count(&taskCount)
	h.db.Model(&model.TaskResult{}).Count(&resultCount)
	h.db.Model(&model.TaskResult{}).Where("is_archived = ?", true).Count(&archivedCount)
	h.db.Model(&model.CreditTransaction{}).Count(&txCount)
	h.db.Model(&model.RedemptionCode{}).Count(&codeCount)

	// 各时间段分布
	now := time.Now()
	day30 := now.AddDate(0, 0, -30)
	day90 := now.AddDate(0, 0, -90)

	var results30, results90 int64
	h.db.Model(&model.TaskResult{}).Where("created_at < ?", day30).Count(&results30)
	h.db.Model(&model.TaskResult{}).Where("created_at < ?", day90).Count(&results90)

	var tasks30, tasks90 int64
	h.db.Model(&model.Task{}).Where("created_at < ? AND status IN ?", day30,
		[]string{"completed", "failed", "stopped"}).Count(&tasks30)
	h.db.Model(&model.Task{}).Where("created_at < ? AND status IN ?", day90,
		[]string{"completed", "failed", "stopped"}).Count(&tasks90)

	c.JSON(http.StatusOK, gin.H{
		"tasks":            taskCount,
		"results":          resultCount,
		"archived_results": archivedCount,
		"transactions":     txCount,
		"codes":            codeCount,
		"older_than_30d": gin.H{
			"results": results30,
			"tasks":   tasks30,
		},
		"older_than_90d": gin.H{
			"results": results90,
			"tasks":   tasks90,
		},
	})
}

// RunningTasks 获取所有运行中任务（GET /api/admin/running-tasks）
func (h *AdminHandler) RunningTasks(c *gin.Context) {
	infos := h.taskEngine.ListAllRunning()

	// 收集所有 userID 批量查用户信息
	userIDs := make([]uint, 0, len(infos))
	for _, info := range infos {
		userIDs = append(userIDs, info.UserID)
	}
	userMap := make(map[uint]model.User)
	if len(userIDs) > 0 {
		var users []model.User
		h.db.Where("id IN ?", userIDs).Find(&users)
		for _, u := range users {
			userMap[u.ID] = u
		}
	}

	// 批量查任务的 credits_reserved
	taskIDs := make([]uint, 0, len(infos))
	for _, info := range infos {
		taskIDs = append(taskIDs, info.TaskID)
	}
	taskMap := make(map[uint]model.Task)
	if len(taskIDs) > 0 {
		var tasks []model.Task
		h.db.Where("id IN ?", taskIDs).Find(&tasks)
		for _, t := range tasks {
			taskMap[t.ID] = t
		}
	}

	type runningTaskItem struct {
		TaskID          uint    `json:"task_id"`
		UserID          uint    `json:"user_id"`
		Username        string  `json:"username"`
		AvatarURL       string  `json:"avatar_url"`
		Platform        string  `json:"platform"`
		Target          int     `json:"target"`
		Threads         int     `json:"threads"`
		SuccessCount    int64   `json:"success_count"`
		FailCount       int64   `json:"fail_count"`
		CreditsReserved int     `json:"credits_reserved"`
		ElapsedSec      float64 `json:"elapsed_sec"`
		Stopping        bool    `json:"stopping"`
	}

	items := make([]runningTaskItem, 0, len(infos))
	for _, info := range infos {
		u := userMap[info.UserID]
		t := taskMap[info.TaskID]
		items = append(items, runningTaskItem{
			TaskID:          info.TaskID,
			UserID:          info.UserID,
			Username:        u.Username,
			AvatarURL:       u.AvatarURL,
			Platform:        info.Platform,
			Target:          info.Target,
			Threads:         info.Threads,
			SuccessCount:    info.SuccessCount,
			FailCount:       info.FailCount,
			CreditsReserved: t.CreditsReserved,
			ElapsedSec:      time.Since(info.StartedAt).Seconds(),
			Stopping:        info.Stopping,
		})
	}

	// 在线用户数 = 不同 UserID 的数量
	onlineSet := make(map[uint]struct{})
	for _, info := range infos {
		onlineSet[info.UserID] = struct{}{}
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks":        items,
		"online_users": len(onlineSet),
	})
}

// AdminStopTask 管理员强制停止任务（POST /api/admin/tasks/:id/stop）
// 支持停止运行中 (running) 和排队中 (queued/pending) 的任务
func (h *AdminHandler) AdminStopTask(c *gin.Context) {
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的任务 ID"})
		return
	}

	// 运行中任务：走引擎停止流程
	if h.taskEngine.AdminStopTask(uint(taskID)) {
		c.JSON(http.StatusOK, gin.H{"message": "任务停止中"})
		return
	}

	// 排队中 / pending 任务：直接在 DB 取消 + 退款
	var task model.Task
	if err := h.db.Where("id = ? AND status IN ?", taskID, []string{"queued", "pending"}).First(&task).Error; err == nil {
		now := time.Now()
		task.Status = "stopped"
		task.StoppedAt = &now
		h.db.Save(&task)
		if task.CreditsReserved > 0 {
			h.creditSvc.RefundCredits(h.db, task.UserID, task.ID, task.CreditsReserved)
		}
		c.JSON(http.StatusOK, gin.H{"message": "排队已取消，积分已退还"})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"detail": "没有运行中的任务"})
}

// AdminDeleteTask 管理员删除任务（DELETE /api/admin/tasks/:id）
func (h *AdminHandler) AdminDeleteTask(c *gin.Context) {
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的任务 ID"})
		return
	}

	var task model.Task
	if err := h.db.First(&task, taskID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "任务不存在"})
		return
	}

	// 运行中的任务不允许直接删除
	if task.Status == "running" || task.Status == "stopping" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "任务运行中，请先停止"})
		return
	}

	// 退还未消费积分
	refund := task.CreditsReserved - task.SuccessCount
	if refund > 0 {
		h.creditSvc.RefundCredits(h.db, task.UserID, task.ID, refund)
	}

	// 删除任务结果 + 任务记录
	h.db.Where("task_id = ?", taskID).Delete(&model.TaskResult{})
	h.db.Delete(&task)
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// SendNotification 发送通知（POST /api/admin/notifications）
func (h *AdminHandler) SendNotification(c *gin.Context) {
	admin := middleware.GetUser(c)
	var req dto.SendNotificationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "标题和内容不能为空"})
		return
	}
	notification := model.Notification{
		UserID:    req.UserID,
		Title:     req.Title,
		Content:   req.Content,
		CreatedBy: admin.ID,
	}
	if err := h.db.Create(&notification).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "发送失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "通知已发送", "id": notification.ID})
}

// ListAdminNotifications 管理员查看已发送通知列表（GET /api/admin/notifications）
func (h *AdminHandler) ListAdminNotifications(c *gin.Context) {
	var notifications []model.Notification
	h.db.Order("created_at DESC").Limit(100).Find(&notifications)

	// 附带发送者用户名
	creatorIDs := make([]uint, 0)
	for _, n := range notifications {
		if n.CreatedBy > 0 {
			creatorIDs = append(creatorIDs, n.CreatedBy)
		}
	}
	creatorMap := make(map[uint]string)
	if len(creatorIDs) > 0 {
		var users []model.User
		h.db.Where("id IN ?", creatorIDs).Select("id, username").Find(&users)
		for _, u := range users {
			creatorMap[u.ID] = u.Username
		}
	}

	type notifItem struct {
		ID         uint   `json:"id"`
		UserID     uint   `json:"user_id"`
		Title      string `json:"title"`
		Content    string `json:"content"`
		CreatedBy  uint   `json:"created_by"`
		CreatorName string `json:"creator_name"`
		CreatedAt  string `json:"created_at"`
	}
	items := make([]notifItem, 0, len(notifications))
	for _, n := range notifications {
		items = append(items, notifItem{
			ID:         n.ID,
			UserID:     n.UserID,
			Title:      n.Title,
			Content:    n.Content,
			CreatedBy:  n.CreatedBy,
			CreatorName: creatorMap[n.CreatedBy],
			CreatedAt:  n.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	c.JSON(http.StatusOK, items)
}

// DeleteNotification 管理员删除通知（DELETE /api/admin/notifications/:id）
func (h *AdminHandler) DeleteNotification(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的通知 ID"})
		return
	}
	// 同时删除广播通知的已读记录
	h.db.Where("notification_id = ?", id).Delete(&model.NotificationRead{})
	if err := h.db.Where("id = ?", id).Delete(&model.Notification{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// UserNotifications 用户获取通知列表（GET /api/notifications）
func (h *AdminHandler) UserNotifications(c *gin.Context) {
	user := middleware.GetUser(c)

	var notifications []model.Notification
	h.db.Where("user_id = ? OR user_id = 0", user.ID).
		Order("created_at DESC").
		Limit(50).
		Find(&notifications)

	// 查询该用户已读的广播通知 ID 集合
	var readIDs []uint
	h.db.Model(&model.NotificationRead{}).
		Where("user_id = ?", user.ID).
		Pluck("notification_id", &readIDs)
	readSet := make(map[uint]struct{}, len(readIDs))
	for _, id := range readIDs {
		readSet[id] = struct{}{}
	}

	// 构建响应：广播通知的已读状态从 notification_reads 表判断
	type notifItem struct {
		model.Notification
		IsRead bool `json:"is_read"`
	}
	items := make([]notifItem, 0, len(notifications))
	var unreadCount int
	for _, n := range notifications {
		read := n.IsRead // 个人通知用 DB 字段
		if n.UserID == 0 {
			_, read = readSet[n.ID] // 广播通知用关联表
		}
		if !read {
			unreadCount++
		}
		items = append(items, notifItem{Notification: n, IsRead: read})
	}

	c.JSON(http.StatusOK, gin.H{
		"notifications": items,
		"unread_count":  unreadCount,
	})
}

// MarkNotificationRead 标记通知已读（PATCH /api/notifications/:id/read）
func (h *AdminHandler) MarkNotificationRead(c *gin.Context) {
	user := middleware.GetUser(c)
	idParam := c.Param("id")

	if idParam == "all" {
		// 个人通知：直接更新 is_read
		h.db.Model(&model.Notification{}).
			Where("user_id = ? AND is_read = ?", user.ID, false).
			Update("is_read", true)

		// 广播通知：批量插入未读的广播通知已读记录（ON CONFLICT 幂等）
		h.db.Exec(`INSERT INTO notification_reads (notification_id, user_id, read_at)
			SELECT id, ?, NOW() FROM notifications WHERE user_id = 0
			AND id NOT IN (SELECT notification_id FROM notification_reads WHERE user_id = ?)
			ON CONFLICT DO NOTHING`, user.ID, user.ID)

		c.JSON(http.StatusOK, gin.H{"message": "已全部标记为已读"})
		return
	}

	notifID, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的通知 ID"})
		return
	}

	// 查询通知类型
	var notif model.Notification
	if err := h.db.First(&notif, notifID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "通知不存在"})
		return
	}

	if notif.UserID == 0 {
		// 广播通知：插入用户已读记录
		h.db.Where("notification_id = ? AND user_id = ?", notifID, user.ID).
			FirstOrCreate(&model.NotificationRead{NotificationID: uint(notifID), UserID: user.ID})
	} else if notif.UserID == user.ID {
		// 个人通知：更新 is_read
		h.db.Model(&notif).Update("is_read", true)
	} else {
		c.JSON(http.StatusForbidden, gin.H{"detail": "无权操作此通知"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已标记为已读"})
}

// ProviderStats 邮箱 provider 成功率统计（GET /api/admin/provider-stats）
func (h *AdminHandler) ProviderStats(c *gin.Context) {
	stats := tempmail.GetProviderStats()
	type item struct {
		Provider    string  `json:"provider"`
		Success     int64   `json:"success"`
		Fail        int64   `json:"fail"`
		Total       int64   `json:"total"`
		SuccessRate float64 `json:"success_rate"`
	}
	items := make([]item, 0, len(stats))
	for name, counts := range stats {
		total := counts[0] + counts[1]
		rate := 0.0
		if total > 0 {
			rate = float64(counts[0]) / float64(total) * 100
		}
		items = append(items, item{
			Provider:    name,
			Success:     counts[0],
			Fail:        counts[1],
			Total:       total,
			SuccessRate: rate,
		})
	}
	c.JSON(http.StatusOK, gin.H{"providers": items})
}

// generateCode 生成 XXXX-XXXX-XXXX-XXXX 格式兑换码
func generateCode() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%04X-%04X-%04X-%04X",
		uint16(b[0])<<8|uint16(b[1]),
		uint16(b[2])<<8|uint16(b[3]),
		uint16(b[4])<<8|uint16(b[5]),
		uint16(b[6])<<8|uint16(b[7]),
	)
}

// RecentActivity 最近注册活动（GET /api/admin/recent-activity?page=1&page_size=20）
// 返回最近的任务列表，含用户信息、平台、单次任务注册量（非累计），支持分页
func (h *AdminHandler) RecentActivity(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	base := h.db.Model(&model.Task{}).Where("status IN ?",
		[]string{"completed", "stopped", "failed"})

	var total int64
	base.Session(&gorm.Session{}).Count(&total)

	var tasks []model.Task
	base.Session(&gorm.Session{}).Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks)

	// 批量查用户
	userIDs := make([]uint, 0, len(tasks))
	for _, t := range tasks {
		userIDs = append(userIDs, t.UserID)
	}
	userMap := make(map[uint]model.User)
	if len(userIDs) > 0 {
		var users []model.User
		h.db.Where("id IN ?", userIDs).Find(&users)
		for _, u := range users {
			userMap[u.ID] = u
		}
	}

	type activityItem struct {
		TaskID      uint   `json:"task_id"`
		UserID      uint   `json:"user_id"`
		Username    string `json:"username"`
		AvatarURL   string `json:"avatar_url"`
		Platform    string `json:"platform"`
		Target      int    `json:"target"`
		Success     int    `json:"success_count"`
		Fail        int    `json:"fail_count"`
		Credits     int    `json:"credits_reserved"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
		CompletedAt string `json:"completed_at"`
	}

	items := make([]activityItem, 0, len(tasks))
	for _, t := range tasks {
		u := userMap[t.UserID]
		completedAt := ""
		if t.StoppedAt != nil {
			completedAt = t.StoppedAt.Format("01-02 15:04:05")
		}
		items = append(items, activityItem{
			TaskID:      t.ID,
			UserID:      t.UserID,
			Username:    u.Username,
			AvatarURL:   u.AvatarURL,
			Platform:    t.Platform,
			Target:      t.TargetCount,
			Success:     t.SuccessCount,
			Fail:        t.FailCount,
			Credits:     t.CreditsReserved,
			Status:      t.Status,
			CreatedAt:   t.CreatedAt.Format("01-02 15:04:05"),
			CompletedAt: completedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GPTMailKeyStatus 已废弃，保留路由兼容
func (h *AdminHandler) GPTMailKeyStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"keys": []interface{}{}, "message": "已迁移至 YYDS Mail"})
}
