package handler

import (
	"encoding/json"
	"fmt"
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

// TaskHandler 任务处理器
type TaskHandler struct {
	db         *gorm.DB
	engine     *service.TaskEngine
	creditSvc  *service.CreditService
	settingSvc *service.SettingService
}

// NewTaskHandler 创建任务处理器
func NewTaskHandler(db *gorm.DB, engine *service.TaskEngine, creditSvc *service.CreditService, settingSvc *service.SettingService) *TaskHandler {
	return &TaskHandler{db: db, engine: engine, creditSvc: creditSvc, settingSvc: settingSvc}
}

// Create 创建任务（POST /api/tasks）
func (h *TaskHandler) Create(c *gin.Context) {
	user := middleware.GetUser(c)
	var req dto.CreateTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	// ── 校验系统限制 ──────────────────────────────────────────────
	// 平台开关
	platformKey := "platform_" + req.Platform + "_enabled"
	if h.settingSvc.Get(platformKey, "true") != "true" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": fmt.Sprintf("%s 平台注册已关闭", req.Platform)})
		return
	}

	maxTarget, _ := strconv.Atoi(h.settingSvc.Get("max_target", "1000"))
	maxThreads, _ := strconv.Atoi(h.settingSvc.Get("max_threads", "16"))
	if maxTarget > 0 && req.Target > maxTarget {
		c.JSON(http.StatusBadRequest, gin.H{"detail": fmt.Sprintf("注册数量不能超过 %d", maxTarget)})
		return
	}

	// 自动计算并发线程数（用户不传或传 0 时，从配置读取阶梯）
	if req.Threads <= 0 {
		req.Threads = calcThreadsByTiers(h.settingSvc, req.Target)
	}
	if maxThreads > 0 && req.Threads > maxThreads {
		req.Threads = maxThreads
	}

	// ── 校验每日注册上限（凌晨 6 点重置）──────────────────────────
	dailyLimit, _ := strconv.Atoi(h.settingSvc.Get("daily_reg_limit", "0"))
	if dailyLimit > 0 {
		resetTime := dailyResetTime()
		var dailyUsed int64
		h.db.Model(&model.Task{}).
			Where("user_id = ? AND created_at >= ?", user.ID, resetTime).
			Select("COALESCE(SUM(success_count), 0)").Scan(&dailyUsed)
		if int(dailyUsed)+req.Target > dailyLimit {
			remaining := dailyLimit - int(dailyUsed)
			if remaining < 0 {
				remaining = 0
			}
			c.JSON(http.StatusBadRequest, gin.H{"detail": fmt.Sprintf("今日注册已达上限（%d/%d），凌晨 6 点重置，剩余 %d 次", dailyUsed, dailyLimit, remaining)})
			return
		}
	}

	// ── 校验平台级注册上限（付费/免费均生效）──────────────────────
	platformTaskLimit := h.settingSvc.GetInt("platform_"+req.Platform+"_task_limit", 0)
	if platformTaskLimit > 0 && req.Target > platformTaskLimit {
		c.JSON(http.StatusBadRequest, gin.H{"detail": fmt.Sprintf("%s 平台单任务最多注册 %d 个", req.Platform, platformTaskLimit)})
		return
	}
	platformDailyLimit := h.settingSvc.GetInt("platform_"+req.Platform+"_daily_limit", 0)
	if platformDailyLimit > 0 {
		resetTime := dailyResetTime()
		var platformDailyUsed int64
		h.db.Model(&model.Task{}).
			Where("user_id = ? AND platform = ? AND created_at >= ?", user.ID, req.Platform, resetTime).
			Select("COALESCE(SUM(success_count), 0)").Scan(&platformDailyUsed)
		if int(platformDailyUsed)+req.Target > platformDailyLimit {
			remaining := platformDailyLimit - int(platformDailyUsed)
			if remaining < 0 {
				remaining = 0
			}
			c.JSON(http.StatusBadRequest, gin.H{"detail": fmt.Sprintf("%s 今日注册已达上限（%d/%d），凌晨 6 点重置，剩余 %d 次", req.Platform, platformDailyUsed, platformDailyLimit, remaining)})
			return
		}
	}

	// 判断平台是否限时免费
	platformFree := isPlatformFree(h.settingSvc, req.Platform)

	// ── 免费模式 vs 付费模式判定 ──────────────────────────────────────
	// mode: "free" = 用户选择免费（受限），"paid" = 用户选择付费（无限制），空 = 自动
	useFreeTier := false
	if platformFree {
		if req.Mode == "paid" {
			// 用户主动选择付费，走正常积分流程
			useFreeTier = false
		} else {
			// mode=="free" 或空值：尝试使用免费额度
			if err := h.validateFreeMode(h.db, user.ID, req.Platform, req.Target); err != nil {
				if req.Mode == "free" {
					// 用户明确要求免费但不满足条件，拒绝
					c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
					return
				}
				// mode 为空（自动模式）：免费不够用，降级到付费
				useFreeTier = false
			} else {
				useFreeTier = true
			}
		}
	}

	// 创建任务记录
	creditsReserved := req.Target
	if useFreeTier {
		creditsReserved = 0
	}

	// 查询用户指定的代理
	var proxyURL string
	if req.ProxyID > 0 {
		var userProxy model.UserProxy
		if err := h.db.Where("id = ? AND user_id = ?", req.ProxyID, user.ID).First(&userProxy).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "指定的代理不存在"})
			return
		}
		proxyURL = userProxy.URL()
	}

	task := model.Task{
		UserID:          user.ID,
		Platform:        req.Platform,
		Status:          "pending",
		TargetCount:     req.Target,
		ThreadCount:     req.Threads,
		CreditsReserved: creditsReserved,
		ProxyURL:        proxyURL,
	}

	tx := h.db.Begin()
	if err := tx.Create(&task).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "任务创建失败"})
		return
	}

	// 预扣积分（免费模式跳过）
	if !useFreeTier {
		if err := h.creditSvc.ReserveCredits(tx, user.ID, task.ID, req.Target); err != nil {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "任务提交失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id":  task.ID,
		"platform": task.Platform,
		"target":   task.TargetCount,
		"threads":  task.ThreadCount,
		"status":   task.Status,
	})
}

// Start 启动任务（POST /api/tasks/:id/start）
func (h *TaskHandler) Start(c *gin.Context) {
	user := middleware.GetUser(c)
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的任务 ID"})
		return
	}

	var task model.Task
	if err := h.db.Where("id = ? AND user_id = ?", taskID, user.ID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "任务不存在"})
		return
	}

	if task.Status != "pending" && task.Status != "stopped" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "任务状态不允许启动"})
		return
	}

	// 快速同步检查：该平台是否已有运行中任务（避免长阻塞后才报错）
	if rt := h.engine.GetStatus(user.ID, task.Platform); rt != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "该平台已有运行中的任务"})
		return
	}

	// StartTask 快速返回排队/启动状态，重量级操作在内部 goroutine 执行
	ok, msg := h.engine.StartTask(user.ID, task.ID, task.Platform, task.TargetCount, task.ThreadCount)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"detail": msg})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": msg, "task_id": task.ID})
}

// Stop 停止任务（POST /api/tasks/:id/stop）
// 支持停止运行中 (running) 和排队中 (queued/pending) 的任务
func (h *TaskHandler) Stop(c *gin.Context) {
	user := middleware.GetUser(c)
	taskID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的任务 ID"})
		return
	}

	var task model.Task
	if err := h.db.Where("id = ? AND user_id = ?", taskID, user.ID).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "任务不存在"})
		return
	}

	// 运行中任务：走引擎停止流程
	if h.engine.StopTask(user.ID, task.Platform) {
		c.JSON(http.StatusOK, gin.H{"message": "任务停止中"})
		return
	}

	// 排队中 / pending 任务：直接在 DB 取消 + 退款
	if task.Status == "queued" || task.Status == "pending" {
		if err := h.db.Transaction(func(tx *gorm.DB) error {
			var fresh model.Task
			if err := tx.Where("id = ? AND status IN ?", task.ID, []string{"queued", "pending"}).First(&fresh).Error; err != nil {
				return err // 状态已变（可能刚被调度器拉起），跳过
			}
			now := time.Now()
			fresh.Status = "stopped"
			fresh.StoppedAt = &now
			tx.Save(&fresh)
			// 退还预扣积分
			if fresh.CreditsReserved > 0 {
				h.creditSvc.RefundCredits(tx, user.ID, fresh.ID, int(fresh.CreditsReserved))
			}
			return nil
		}); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "任务状态已变更，请刷新页面"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "排队已取消，积分已退还"})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"detail": "没有运行中的任务"})
}

// Current 获取当前运行中任务（GET /api/tasks/current?platform=）
func (h *TaskHandler) Current(c *gin.Context) {
	user := middleware.GetUser(c)
	platform := c.Query("platform")

	// 查询数据库中的运行中/排队任务
	query := h.db.Where("user_id = ? AND status IN ?", user.ID, []string{"running", "stopping", "queued", "pending"})
	if platform != "" {
		query = query.Where("platform = ?", platform)
	}

	var task model.Task
	if err := query.Order("created_at DESC").First(&task).Error; err != nil {
		c.JSON(http.StatusOK, nil)
		return
	}

	// 检查孤儿任务（DB 显示 running/stopping 但内存无对应任务）—— 事务保护防双倍退款
	if (task.Status == "running" || task.Status == "stopping") && h.engine.GetStatus(user.ID, task.Platform) == nil {
		_ = h.db.Transaction(func(tx *gorm.DB) error {
			var fresh model.Task
			if err := tx.Where("id = ? AND status IN ?", task.ID, []string{"running", "stopping"}).First(&fresh).Error; err != nil {
				return err // 已被其他请求处理，跳过
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

	resp := dto.TaskStatus{
		TaskID:          task.ID,
		Platform:        task.Platform,
		Target:          task.TargetCount,
		Threads:         task.ThreadCount,
		CreditsReserved: task.CreditsReserved,
		SuccessCount:    task.SuccessCount,
		FailCount:       task.FailCount,
		Status:          task.Status,
		IsDone:          task.IsDone(),
	}

	// 排队中任务：计算实时排队位置和预计等待时间
	if task.Status == "queued" || task.Status == "pending" {
		var queuePos int64
		h.db.Model(&model.Task{}).
			Where("status IN ? AND platform = ? AND created_at < ?", []string{"queued", "pending"}, task.Platform, task.CreatedAt).
			Count(&queuePos)
		queuePos++ // 自己也算一个位置
		resp.QueuePosition = int(queuePos)
		resp.QueueWaitSec = h.engine.EstimateWaitTime(int(queuePos), task.Platform)
	}

	c.JSON(http.StatusOK, resp)
}

// History 历史任务（GET /api/tasks/history）
func (h *TaskHandler) History(c *gin.Context) {
	user := middleware.GetUser(c)
	var tasks []model.Task
	h.db.Where("user_id = ?", user.ID).
		Order("created_at DESC").
		Limit(20).
		Find(&tasks)

	results := make([]dto.TaskStatus, 0, len(tasks))
	for _, t := range tasks {
		results = append(results, dto.TaskStatus{
			TaskID:          t.ID,
			Platform:        t.Platform,
			Target:          t.TargetCount,
			Threads:         t.ThreadCount,
			CreditsReserved: t.CreditsReserved,
			SuccessCount:    t.SuccessCount,
			FailCount:       t.FailCount,
			Status:          t.Status,
			IsDone:          t.IsDone(),
		})
	}
	c.JSON(http.StatusOK, results)
}

// calcThreadsByTiers 从配置读取线程阶梯，匹配目标数量返回线程数
func calcThreadsByTiers(settingSvc *service.SettingService, target int) int {
	tiersJSON := settingSvc.Get("thread_tiers", "")
	if tiersJSON != "" {
		var tiers []struct {
			Max     int `json:"max"`
			Threads int `json:"threads"`
		}
		if err := json.Unmarshal([]byte(tiersJSON), &tiers); err == nil && len(tiers) > 0 {
			for _, tier := range tiers {
				if target <= tier.Max {
					return tier.Threads
				}
			}
			// 超过所有阶梯，使用最后一档
			return tiers[len(tiers)-1].Threads
		}
	}
	// 降级：硬编码默认值
	switch {
	case target <= 5:
		return 1
	case target <= 20:
		return 2
	case target <= 100:
		return 3
	case target <= 300:
		return 5
	case target <= 500:
		return 8
	default:
		return 12
	}
}

// isPlatformFree 判断平台是否在限时免费期内
func isPlatformFree(settingSvc *service.SettingService, platform string) bool {
	key := "platform_" + platform + "_free_until"
	dateStr := settingSvc.Get(key, "")
	if dateStr == "" {
		return false
	}
	deadline, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}
	// 截止日当天全天有效（截止日+1天的零点之前）
	return time.Now().Before(deadline.AddDate(0, 0, 1))
}

// validateFreeMode 校验免费模式的三重限制：单任务上限、每日上限、冷却时间
// 冷却逻辑：上一轮免费任务用满 task_limit → 触发冷却；没用满 → 不冷却
// 返回 nil 表示可以使用免费模式，返回 error 说明原因
func (h *TaskHandler) validateFreeMode(db *gorm.DB, userID uint, platform string, target int) error {
	prefix := "platform_" + platform + "_free_"

	// ── 1. 单任务上限 ────────────────────────────────────────────────
	taskLimit := h.settingSvc.GetInt(prefix+"task_limit", 2)
	if taskLimit > 0 && target > taskLimit {
		return fmt.Errorf("免费模式单次最多注册 %d 个，如需更多请切换付费模式", taskLimit)
	}

	// ── 2. 每日上限（凌晨 6 点重置）────────────────────────────────────
	dailyLimit := h.settingSvc.GetInt(prefix+"daily_limit", 5)
	resetTime := dailyResetTime()
	var dailyFreeUsed int64
	db.Model(&model.Task{}).
		Where("user_id = ? AND platform = ? AND credits_reserved = 0 AND created_at >= ?", userID, platform, resetTime).
		Select("COALESCE(SUM(target_count), 0)").Scan(&dailyFreeUsed)

	if dailyLimit > 0 && int(dailyFreeUsed)+target > dailyLimit {
		remaining := dailyLimit - int(dailyFreeUsed)
		if remaining < 0 {
			remaining = 0
		}
		return fmt.Errorf("今日免费额度已用 %d/%d，剩余 %d 次，如需更多请切换付费模式", dailyFreeUsed, dailyLimit, remaining)
	}

	// ── 3. 冷却：上一轮用满 task_limit 才触发，没用满不冷却 ──────────────
	cooldownMin := h.settingSvc.GetInt(prefix+"cooldown", 30)
	if cooldownMin > 0 && taskLimit > 0 {
		var lastTask model.Task
		err := db.Where("user_id = ? AND platform = ? AND credits_reserved = 0 AND stopped_at IS NOT NULL", userID, platform).
			Order("stopped_at DESC").First(&lastTask).Error
		if err == nil && lastTask.StoppedAt != nil && lastTask.TargetCount >= taskLimit {
			// 上一轮用满了 task_limit → 检查冷却窗口
			cooldownEnd := lastTask.StoppedAt.Add(time.Duration(cooldownMin) * time.Minute)
			if time.Now().Before(cooldownEnd) {
				remainingMin := int(time.Until(cooldownEnd).Minutes()) + 1
				return fmt.Errorf("上一轮免费注册已用满 %d 个，冷却中还需 %d 分钟，或切换付费模式", taskLimit, remainingMin)
			}
		}
	}

	return nil
}
