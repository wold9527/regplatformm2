package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"github.com/xiaolajiaoyyds/regplatformm/internal/worker"
	"gorm.io/gorm"
)

// taskKey 运行中任务的唯一标识
type taskKey struct {
	UserID   uint
	Platform string
}

// RunningTask 运行中的任务
type RunningTask struct {
	TaskID       uint
	UserID       uint
	Platform     string
	Target       int
	Threads      int
	SuccessCount atomic.Int64
	FailCount    atomic.Int64
	ClaimedCount atomic.Int64 // 已认领的注册名额（防止并发超发）
	Stopping     atomic.Bool  // 用户已请求停止（允许立即启动新任务）
	Cancel       context.CancelFunc
	LogCh        chan string
	StatusCh     chan struct{} // 状态变更通知（成功/失败计数变化时触发）
	Done         chan struct{}
	StartedAt    time.Time // 任务启动时间（用于耗时统计）
	subscribers  []chan string
	subMu        sync.RWMutex
	logBuffer    []string   // 日志缓冲区（供刷新页面后回放）
	logBufMu     sync.RWMutex
}

// NotifyStatus 通知前端状态已变更（非阻塞）
func (rt *RunningTask) NotifyStatus() {
	select {
	case rt.StatusCh <- struct{}{}:
	default: // 已有待处理的通知，跳过
	}
}

// Subscribe 订阅日志
func (rt *RunningTask) Subscribe() <-chan string {
	ch := make(chan string, 200)
	rt.subMu.Lock()
	rt.subscribers = append(rt.subscribers, ch)
	rt.subMu.Unlock()
	return ch
}

// Unsubscribe 取消订阅
func (rt *RunningTask) Unsubscribe(ch <-chan string) {
	rt.subMu.Lock()
	defer rt.subMu.Unlock()
	for i, sub := range rt.subscribers {
		if sub == ch {
			rt.subscribers = append(rt.subscribers[:i], rt.subscribers[i+1:]...)
			close(sub)
			return
		}
	}
}

// IsDone 任务是否完成
func (rt *RunningTask) IsDone() bool {
	select {
	case <-rt.Done:
		return true
	default:
		return false
	}
}

// Log 写入日志
func (rt *RunningTask) Log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	// 防止往已关闭的 LogCh 写入导致 panic
	defer func() { recover() }()
	select {
	case rt.LogCh <- msg:
	default:
		// 通道满了丢弃
	}
}

// isInternalBrowserLog 判断 [~] 前缀的浏览器日志是否包含敏感/内部信息
// 返回 true 表示该日志不应展示给用户（URL、Token、IP、内部选择器等）
func isInternalBrowserLog(msg string) bool {
	// 完整 URL（泄漏内部服务地址）
	if strings.Contains(msg, "https://") || strings.Contains(msg, "http://") {
		return true
	}
	// Token / 密钥值
	if strings.Contains(msg, "Token:") || strings.Contains(msg, "token=") ||
		strings.Contains(msg, "xsrf") || strings.Contains(msg, "cookie") {
		return true
	}
	// OIDC 客户端凭据 / 设备码
	if strings.Contains(msg, "clientId") || strings.Contains(msg, "user_code=") ||
		strings.Contains(msg, "客户端已注册") {
		return true
	}
	// IP / 地理位置
	if strings.Contains(msg, "IP 地理位置") || strings.Contains(msg, "tz=") {
		return true
	}
	// 内部页面状态（含 URL 片段）
	if strings.Contains(msg, "页面:") || strings.Contains(msg, "URL:") ||
		strings.Contains(msg, "注册链接:") || strings.Contains(msg, "页面已自动跳转") {
		return true
	}
	// 邮箱明文 + 页面地址
	if strings.Contains(msg, "填写邮箱:") {
		return true
	}
	// Playwright CSS 选择器细节
	if strings.Contains(msg, "点击按钮 (") || strings.Contains(msg, "尝试点击按钮") {
		return true
	}
	// OIDC 内部步骤
	if strings.Contains(msg, "OIDC Step") || strings.Contains(msg, "OIDC:") {
		return true
	}
	// Playwright 内部状态
	if strings.Contains(msg, "networkidle") {
		return true
	}
	return false
}

// shouldShowToUser 判断日志是否应推送到前端
// 只保留用户关心的结果/进度/状态信息，过滤掉注册流程内部细节
func shouldShowToUser(msg string) bool {
	// 远程浏览器服务的流式日志（Kiro/Gemini 注册过程）
	// 需脱敏：隐藏 token/URL/cookie/内部技术细节，只展示进度
	if strings.Contains(msg, "[~]") {
		return !isInternalBrowserLog(msg)
	}
	// 成功结果（只保留 [✓] 最终汇总行，过滤掉中间步骤的 "注册成功" / "远程注册成功"）
	if strings.Contains(msg, "[✓]") {
		return true
	}
	// 分隔线
	if strings.Contains(msg, "════") {
		return true
	}
	// 任务汇总
	if strings.Contains(msg, "任务完成") || strings.Contains(msg, "任务已加入") {
		return true
	}
	// 统计信息（仅匹配任务汇总行，如 "成功: 5  失败: 3"，不匹配 "注册失败: xxx"）
	if strings.Contains(msg, "成功:") && strings.Contains(msg, "失败:") {
		return true
	}
	// 耗时/预计（不含宽泛的"等待"，避免排队轮询日志刷屏）
	if strings.Contains(msg, "耗时") || strings.Contains(msg, "预计") {
		return true
	}
	// 排队状态（仅匹配明确的排队反馈，不匹配内部 pollMailCode 等待日志）
	if strings.Contains(msg, "排队等待完成") || strings.Contains(msg, "排队中") {
		return true
	}
	// 进度指示
	if strings.Contains(msg, "第 ") || strings.Contains(msg, "进度") {
		return true
	}
	// 用户操作反馈
	if strings.Contains(msg, "手动停止") || strings.Contains(msg, "强制停止") {
		return true
	}
	return false
}

// broadcastLogs 广播日志到所有订阅者，同时缓存到 logBuffer
// 连续相同消息自动去重，避免大量并发注册时前端 DOM 爆炸
func (rt *RunningTask) broadcastLogs() {
	var (
		lastRaw  string // 上一条原始消息（不含时间戳）
		dupCount int    // 连续重复计数
	)

	// sendOne 发送单条日志到 buffer + 所有 subscriber
	// 日志始终写入 logBuffer（后端调试用），仅用户关心的日志推送到前端
	sendOne := func(line string) {
		rt.logBufMu.Lock()
		rt.logBuffer = append(rt.logBuffer, line)
		if len(rt.logBuffer) > 2000 {
			rt.logBuffer = rt.logBuffer[len(rt.logBuffer)-2000:]
		}
		rt.logBufMu.Unlock()

		if !shouldShowToUser(line) {
			return
		}

		rt.subMu.RLock()
		for _, sub := range rt.subscribers {
			select {
			case sub <- line:
			default:
			}
		}
		rt.subMu.RUnlock()
	}

	// flushDup 输出累积的重复计数
	flushDup := func() {
		if dupCount > 0 {
			sendOne(fmt.Sprintf("[%s] [~] ↑ 重复 ×%d", time.Now().Format("15:04:05"), dupCount))
			dupCount = 0
		}
	}

	for msg := range rt.LogCh {
		// 连续相同消息去重
		if msg == lastRaw {
			dupCount++
			// 每累积 100 条刷一次，避免长时间无输出
			if dupCount%100 == 0 {
				flushDup()
			}
			continue
		}
		flushDup()
		lastRaw = msg
		sendOne(fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg))
	}
	flushDup()
}

// GetLogBuffer 获取已缓存的历史日志（刷新页面后回放用）
func (rt *RunningTask) GetLogBuffer() []string {
	rt.logBufMu.RLock()
	defer rt.logBufMu.RUnlock()
	buf := make([]string, len(rt.logBuffer))
	copy(buf, rt.logBuffer)
	return buf
}

// TaskEngine 任务引擎
type TaskEngine struct {
	db            *gorm.DB
	proxyPool     *ProxyPool
	cardPool      *CardPool
	creditSvc     *CreditService
	settingSvc    *SettingService
	running       map[taskKey]*RunningTask
	mu            sync.RWMutex
	activeCount   atomic.Int32              // 全局活跃任务计数
	platformCount map[string]*atomic.Int32  // 按平台活跃任务计数
	scheduler     *QueueScheduler           // 队列调度器（启动后通过 SetScheduler 注入）
	dispatcher    *Dispatcher               // 全局 Round-Robin 调度器（所有任务共享 Worker Pool）
	workerCount   atomic.Int32              // 当前运行中的 Pool Worker 数
	targetWorkers atomic.Int32              // 目标 Pool Worker 数（从配置动态读取，热生效）
	configCache   map[string]*cachedConfig  // 按平台缓存的 ScanConfig 结果
	configCacheMu sync.RWMutex
}

// cachedConfig 平台扫描配置缓存
type cachedConfig struct {
	config    worker.Config
	scannedAt time.Time
}

// NewTaskEngine 创建任务引擎
func NewTaskEngine(db *gorm.DB, proxyPool *ProxyPool, cardPool *CardPool, creditSvc *CreditService, settingSvc *SettingService) *TaskEngine {
	// 注意：running/stopping 任务的清理和退款由 QueueScheduler.RecoverOnBoot() 统一处理
	// 此处不再重复清理，避免抢在退款之前把状态改掉

	// 初始化按平台并发计数器
	pc := make(map[string]*atomic.Int32)
	for name := range worker.Registry {
		pc[name] = &atomic.Int32{}
	}

	e := &TaskEngine{
		db:            db,
		proxyPool:     proxyPool,
		cardPool:      cardPool,
		creditSvc:     creditSvc,
		settingSvc:    settingSvc,
		running:       make(map[taskKey]*RunningTask),
		platformCount: pc,
		dispatcher:    NewDispatcher(),
		configCache:   make(map[string]*cachedConfig),
	}
	e.startWorkerPool()
	go e.startConfigRefresher()
	return e
}

// SetScheduler 注入队列调度器（解决循环依赖）
func (e *TaskEngine) SetScheduler(s *QueueScheduler) {
	e.scheduler = s
}

// countActiveUsers 统计当前有多少个不同用户在运行任务
func (e *TaskEngine) countActiveUsers() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	users := make(map[uint]struct{})
	for _, rt := range e.running {
		if !rt.IsDone() {
			users[rt.UserID] = struct{}{}
		}
	}
	return len(users)
}

// userHasRunningTask 检查该用户是否已有运行中任务（已占槽位）
func (e *TaskEngine) userHasRunningTask(userID uint) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, rt := range e.running {
		if rt.UserID == userID && !rt.IsDone() {
			return true
		}
	}
	return false
}

// countActiveUsersByPlatform 统计指定平台当前有多少个不同用户在运行任务
func (e *TaskEngine) countActiveUsersByPlatform(platform string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	users := make(map[uint]struct{})
	for _, rt := range e.running {
		if rt.Platform == platform && !rt.IsDone() {
			users[rt.UserID] = struct{}{}
		}
	}
	return len(users)
}

// computeMaxInflight 计算指定平台的单任务最大并发数（统一算法，供启动时 + RefreshLimits 复用）
// 自适应策略：单人 80% 容量，多人均分，上限不超过平台容量的 90%
func (e *TaskEngine) computeMaxInflight(platform string) int32 {
	defaultCap := 50
	if platform == "kiro" || platform == "gemini" {
		defaultCap = 15
	}
	maxConcurrent := e.settingSvc.GetInt(platform+"_max_concurrent", defaultCap)
	if maxConcurrent <= 0 {
		maxConcurrent = defaultCap
	}

	var activeCount int64
	e.db.Model(&model.Task{}).Where("platform = ? AND status = ?", platform, "running").Count(&activeCount)

	var maxParallel int
	if activeCount <= 1 {
		maxParallel = maxConcurrent * 4 / 5
	} else {
		maxParallel = maxConcurrent / int(activeCount)
	}

	// 上限：不超过平台总容量的 90%（预留缓冲给突发/新任务进入）
	parallelCap := maxConcurrent * 9 / 10
	if parallelCap < 5 {
		parallelCap = 5
	}
	if maxParallel > parallelCap {
		maxParallel = parallelCap
	}
	if maxParallel < 3 {
		maxParallel = 3
	}
	return int32(maxParallel)
}

// userHasRunningTaskOnPlatform 检查该用户在指定平台是否已有运行中任务
func (e *TaskEngine) userHasRunningTaskOnPlatform(userID uint, platform string) bool {
	key := taskKey{userID, platform}
	e.mu.RLock()
	defer e.mu.RUnlock()
	rt, ok := e.running[key]
	return ok && !rt.IsDone()
}

// EstimateWaitTime 估算排队用户的预计等待时间（秒）
// 基于该平台最近已完成任务的平均耗时 + 当前运行中任务的剩余时间
func (e *TaskEngine) EstimateWaitTime(queuePosition int, platform string) int {
	// 查询该平台最近 50 个已完成任务的平均耗时
	var avgDuration float64
	e.db.Model(&model.Task{}).
		Where("platform = ? AND status IN ? AND started_at IS NOT NULL AND stopped_at IS NOT NULL", platform, []string{"completed", "stopped"}).
		Order("stopped_at DESC").Limit(50).
		Select("COALESCE(AVG(EXTRACT(EPOCH FROM (stopped_at - started_at))), 120)").
		Scan(&avgDuration)
	if avgDuration < 10 {
		avgDuration = 120 // 兜底 2 分钟
	}

	// 找该平台当前运行中任务的最短剩余时间
	e.mu.RLock()
	minRemaining := avgDuration
	for _, rt := range e.running {
		if rt.IsDone() || rt.Platform != platform {
			continue
		}
		elapsed := time.Since(rt.StartedAt).Seconds()
		remaining := avgDuration - elapsed
		if remaining < 0 {
			remaining = 30 // 已超时的任务给 30 秒缓冲
		}
		if remaining < minRemaining {
			minRemaining = remaining
		}
	}
	e.mu.RUnlock()

	// 预计等待 = 最短剩余时间 × ceil(排队位置 / 槽位数)
	maxUsers, _ := strconv.Atoi(e.settingSvc.Get(platform+"_max_concurrent_users", "0"))
	if maxUsers <= 0 {
		maxUsers, _ = strconv.Atoi(e.settingSvc.Get("max_concurrent_users", "1"))
	}
	if maxUsers <= 0 {
		maxUsers = 1
	}
	rounds := (queuePosition + maxUsers - 1) / maxUsers
	return int(minRemaining) * rounds
}

// formatWaitTime 格式化等待秒数为可读字符串
func formatWaitTime(sec int) string {
	if sec < 60 {
		return fmt.Sprintf("约 %d 秒", sec)
	}
	min := sec / 60
	s := sec % 60
	if s > 0 {
		return fmt.Sprintf("约 %d 分 %d 秒", min, s)
	}
	return fmt.Sprintf("约 %d 分钟", min)
}

// StartTask 启动任务
func (e *TaskEngine) StartTask(userID, taskID uint, platform string, target, threads int) (bool, string) {
	key := taskKey{userID, platform}

	w, ok := worker.Get(platform)
	if !ok {
		return false, fmt.Sprintf("不支持的平台: %s", platform)
	}

	// ── 按平台用户槽位并发控制 ──
	maxUsers, _ := strconv.Atoi(e.settingSvc.Get(platform+"_max_concurrent_users", "0"))
	if maxUsers <= 0 {
		// 向后兼容：平台专属未设置时回退到全局设置
		maxUsers, _ = strconv.Atoi(e.settingSvc.Get("max_concurrent_users", "0"))
	}
	if maxUsers > 0 && !e.userHasRunningTaskOnPlatform(userID, platform) {
		activeUsers := e.countActiveUsersByPlatform(platform)
		if activeUsers >= maxUsers {
			e.db.Model(&model.Task{}).Where("id = ? AND status != ?", taskID, "queued").Update("status", "queued")
			// 计算该平台排队位置和预计等待时间
			var queuePos int64
			e.db.Model(&model.Task{}).
				Where("status = ? AND platform = ? AND created_at < (SELECT created_at FROM tasks WHERE id = ?)", "queued", platform, taskID).
				Count(&queuePos)
			queuePos++ // 自己也算一个位置
			waitSec := e.EstimateWaitTime(int(queuePos), platform)
			log.Info().Uint("task_id", taskID).Uint("user_id", userID).Str("platform", platform).
				Int("active_users", activeUsers).Int("limit", maxUsers).Int64("queue_pos", queuePos).
				Msg("平台用户槽位已满，任务加入队列")
			return true, fmt.Sprintf("排队中，前方 %d 位用户，预计等待 %s", queuePos, formatWaitTime(waitSec))
		}
	}

	// ── 按平台并发控制 ──
	platformLimitKey := platform + "_max_concurrent"
	platformLimit, _ := strconv.Atoi(e.settingSvc.Get(platformLimitKey, "0"))
	if platformLimit <= 0 {
		// 浏览器类平台（kiro/gemini）并发能力远低于 HTTP 类（openai/grok），兜底值需区分
		switch platform {
		case "kiro", "gemini":
			platformLimit = 8
		default:
			platformLimit = 100
		}
	}
	pc := e.getPlatformCount(platform)
	newPlatformActive := pc.Add(1)
	if int(newPlatformActive) > platformLimit {
		pc.Add(-1)
		e.db.Model(&model.Task{}).Where("id = ? AND status != ?", taskID, "queued").Update("status", "queued")
		log.Info().Uint("task_id", taskID).Str("platform", platform).
			Int("active", int(newPlatformActive-1)).Int("limit", platformLimit).
			Msg("平台并发已满，任务加入队列")
		return true, "已加入队列"
	}

	// ── 全局并发控制（安全上限）──
	globalLimit, _ := strconv.Atoi(e.settingSvc.Get("max_concurrent_tasks", "100"))
	if globalLimit <= 0 {
		globalLimit = 100
	}
	newActive := e.activeCount.Add(1)
	if int(newActive) > globalLimit {
		e.activeCount.Add(-1)
		pc.Add(-1)
		e.db.Model(&model.Task{}).Where("id = ? AND status != ?", taskID, "queued").Update("status", "queued")
		log.Info().Uint("task_id", taskID).Int("active", int(newActive-1)).Int("limit", globalLimit).
			Msg("全局并发已满，任务加入队列")
		return true, "已加入队列"
	}

	ctx, cancel := context.WithCancel(context.Background())
	rt := &RunningTask{
		TaskID:    taskID,
		UserID:    userID,
		Platform:  platform,
		Target:    target,
		Threads:   threads,
		Cancel:    cancel,
		LogCh:     make(chan string, 500),
		StatusCh:  make(chan struct{}, 1),
		Done:      make(chan struct{}),
		StartedAt: time.Now(),
	}

	// 原子检查并设置，防止 TOCTOU 竞态
	e.mu.Lock()
	if existing, ok := e.running[key]; ok && !existing.IsDone() {
		e.mu.Unlock()
		cancel() // 清理刚创建的 context
		e.activeCount.Add(-1) // 回滚全局计数
		pc.Add(-1)            // 回滚平台计数
		return false, "该平台已有运行中的任务"
	}
	e.running[key] = rt
	e.mu.Unlock()

	// 更新任务状态为 running
	now := time.Now()
	e.db.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":     "running",
		"started_at": &now,
	})

	// 启动日志广播
	go rt.broadcastLogs()

	log.Info().Uint("task_id", taskID).Str("platform", platform).Int("target", target).Msg("任务已启动")

	// 异步执行重量级操作（ScanConfig + 调度 + 监控），避免阻塞调用方
	go e.executeStartedTask(ctx, cancel, rt, key, w, pc, taskID, userID, platform, target)

	return true, "任务已启动"
}

// executeStartedTask 执行已通过并发检查的任务（ScanConfig + 调度 + 监控）
// 在 goroutine 中运行，避免阻塞 StartTask 的调用方
func (e *TaskEngine) executeStartedTask(
	ctx context.Context, cancel context.CancelFunc,
	rt *RunningTask, key taskKey, w worker.Worker, pc *atomic.Int32,
	taskID, userID uint, platform string, target int,
) {
	// 获取共享基础配置（ScanConfig 只调一次，所有 Pool Worker 共享结果）
	baseCfg := worker.Config{}
	e.injectSettings(baseCfg)

	// 优先使用缓存配置（后台每 5 分钟自动刷新）
	scannedCfg, cached := e.getCachedConfig(platform)
	if !cached {
		rt.Log("[*] 正在获取页面配置（直连失败时会走浏览器过盾，约需 30-60 秒）...")
		var scanErr error
		scannedCfg, scanErr = w.ScanConfig(ctx, e.proxyPool.GetNext(), baseCfg)
		if scanErr != nil {
			rt.Log("[-] 配置获取失败: %s", scanErr)
			cancel()
			close(rt.Done)
			close(rt.LogCh) // 停止 broadcastLogs goroutine
			stopNow := time.Now()
			e.db.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]interface{}{
				"status":     "stopped",
				"stopped_at": &stopNow,
			})
			// 退还所有预扣积分
			var task model.Task
			if dbErr := e.db.First(&task, taskID).Error; dbErr == nil && task.CreditsReserved > 0 {
				e.creditSvc.RefundCredits(e.db, userID, taskID, task.CreditsReserved)
			}
			e.mu.Lock()
			delete(e.running, key)
			e.mu.Unlock()
			e.creditSvc.CleanupTask(taskID)
			e.activeCount.Add(-1)
			pc.Add(-1)
			if e.scheduler != nil {
				e.scheduler.TryDispatch()
			}
			return
		}
		// 扫描成功，更新缓存供后续任务使用
		e.updateConfigCache(platform, scannedCfg)
	} else {
		rt.Log("[*] 使用缓存配置（后台每 5 分钟自动刷新）")
	}
	// 合并扫描配置到基础配置
	for k, v := range scannedCfg {
		if baseCfg[k] == "" {
			baseCfg[k] = v
		}
	}
	// 将扫描到的动态配置回写数据库，避免 site_key / action_id 过期
	for _, k := range []string{"site_key", "action_id"} {
		if v := scannedCfg[k]; v != "" {
			dbKey := platform + "_" + k
			if setErr := e.settingSvc.Set(dbKey, v); setErr != nil {
				rt.Log("[!] 配置持久化失败 (%s): %s", dbKey, setErr)
			} else {
				rt.Log("[*] 动态配置已更新: %s = %s", dbKey, v)
			}
		}
	}

	// 注入用户指定的代理（任务创建时保存在 task.proxy_url）
	{
		var task model.Task
		if err := e.db.Select("proxy_url").First(&task, taskID).Error; err == nil && task.ProxyURL != "" {
			baseCfg["user_proxy"] = task.ProxyURL
			rt.Log("[*] 使用用户指定代理")
		}
	}

	// 解析多节点 URL 配置（逗号分隔 → 轮询分配到各 Pool Worker）
	multiNodeKeys := []string{"kiro_reg_url", "gemini_reg_url", "openai_reg_url", "grok_reg_url", "camoufox_reg_url", "turnstile_solver_url", "cf_bypass_solver_url"}
	nodeURLs := make(map[string][]string)
	for _, nk := range multiNodeKeys {
		if val := baseCfg[nk]; val != "" && strings.Contains(val, ",") {
			parts := strings.Split(val, ",")
			trimmed := make([]string, 0, len(parts))
			for _, p := range parts {
				if t := strings.TrimSpace(p); t != "" {
					trimmed = append(trimmed, t)
				}
			}
			if len(trimmed) > 1 {
				nodeURLs[nk] = trimmed
				rt.Log("[*] %s → %d 个节点轮询", nk, len(trimmed))
			}
		}
	}

	// 计算单任务最大并发数（与 RefreshLimits 相同的自适应算法，避免前 10 秒使用过时值）
	maxParallel := int(e.computeMaxInflight(platform))

	// 构建调度条目并加入全局 Dispatcher（Pool Worker 会自动拾取并执行）
	entry := &DispatchEntry{
		Key:      key,
		RT:       rt,
		Worker:   w,
		Ctx:      ctx,
		Cancel:   cancel,
		Config:   baseCfg,
		NodeURLs: nodeURLs,
		Platform: platform,
		done:     make(chan struct{}),
	}
	entry.maxInflight.Store(int32(maxParallel))
	e.dispatcher.Add(entry)
	// 新任务加入后立即刷新所有任务的 maxInflight（不等 10 秒定时器，公平分配即时生效）
	e.dispatcher.RefreshLimits(func(p string) int32 {
		return e.computeMaxInflight(p)
	})

	rt.Log("════════════════════════════════════")
	rt.Log("[*] 配置扫描完成，任务已加入调度池（当前 %d 个 Pool Worker，单任务并发上限 %d）", e.workerCount.Load(), maxParallel)
	rt.Log("════════════════════════════════════")

	// 监控 goroutine：等待调度条目完成后清理资源
	go func() {
		<-entry.done

		success := int(rt.SuccessCount.Load())
		fail := int(rt.FailCount.Load())
		elapsed := time.Since(rt.StartedAt).Round(time.Second)
		rt.Log("════════════════════════════════════")
		rt.Log("[✓] 任务完成  成功: %d  失败: %d  耗时: %s", success, fail, elapsed)
		rt.Log("════════════════════════════════════")

		// 从调度器移除（停止分配新名额给此任务）
		e.dispatcher.Remove(key)
		// 立即刷新剩余任务的 maxInflight（任务退出后释放的容量即时分配给剩余任务）
		e.dispatcher.RefreshLimits(func(p string) int32 {
			return e.computeMaxInflight(p)
		})

		// 更新任务状态（区分用户停止 vs 自然完成）
		stopNow := time.Now()
		finalStatus := "completed"
		if ctx.Err() != nil {
			// context 被 cancel，检查是否用户主动停止（非达标取消）
			total := rt.SuccessCount.Load() + rt.FailCount.Load()
			if total < int64(target) {
				finalStatus = "stopped"
			}
		}
		e.db.Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]interface{}{
			"status":     finalStatus,
			"stopped_at": &stopNow,
		})

		// 退还未消费积分（必须在 close(rt.Done) 之前，确保前端收到 complete 时退款已入库）
		// 使用 DB 中的 success_count 作为退款依据（内存计数器可能因 DB 写入失败而偏高）
		var task model.Task
		if err := e.db.First(&task, taskID).Error; err == nil {
			refund := task.CreditsReserved - task.SuccessCount
			if refund > 0 {
				e.creditSvc.RefundCredits(e.db, userID, taskID, refund)
			}
		}

		// 关闭 Done channel → 触发 SSE complete 事件（此时退款已到账）
		close(rt.Done)
		close(rt.LogCh)

		// 清理运行中任务 + 退款锁内存
		e.mu.Lock()
		delete(e.running, key)
		e.mu.Unlock()
		e.creditSvc.CleanupTask(taskID)

		// 释放全局并发槽位 + 平台槽位，并尝试调度下一个排队任务
		e.activeCount.Add(-1)
		pc.Add(-1)
		if e.scheduler != nil {
			e.scheduler.TryDispatch()
		}
	}()
}

// StopTask 停止任务
func (e *TaskEngine) StopTask(userID uint, platform string) bool {
	key := taskKey{userID, platform}
	e.mu.RLock()
	rt, ok := e.running[key]
	e.mu.RUnlock()
	if !ok || rt.IsDone() {
		return false
	}

	rt.Log("[!] 用户手动停止任务")
	rt.Stopping.Store(true)
	rt.Cancel()
	e.dispatcher.ForceComplete(key) // 立即标记完成，不等 inflight worker

	// 更新状态
	e.db.Model(&model.Task{}).Where("id = ?", rt.TaskID).Update("status", "stopping")
	return true
}

// GetStatus 获取运行中任务的状态
func (e *TaskEngine) GetStatus(userID uint, platform string) *RunningTask {
	key := taskKey{userID, platform}
	e.mu.RLock()
	defer e.mu.RUnlock()
	rt, ok := e.running[key]
	if !ok || rt.IsDone() || rt.Stopping.Load() {
		return nil
	}
	return rt
}

// FindRunningTaskByID 通过 taskID 查找运行中任务
func (e *TaskEngine) FindRunningTaskByID(taskID uint) *RunningTask {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, rt := range e.running {
		if rt.TaskID == taskID {
			return rt
		}
	}
	return nil
}

// RunningTaskInfo 运行中任务的安全快照（不暴露 channel/cancel 等内部字段）
type RunningTaskInfo struct {
	TaskID       uint      `json:"task_id"`
	UserID       uint      `json:"user_id"`
	Platform     string    `json:"platform"`
	Target       int       `json:"target"`
	Threads      int       `json:"threads"`
	SuccessCount int64     `json:"success_count"`
	FailCount    int64     `json:"fail_count"`
	StartedAt    time.Time `json:"started_at"`
	Stopping     bool      `json:"stopping"`
}

// ListAllRunning 返回所有运行中任务的快照（管理员用）
func (e *TaskEngine) ListAllRunning() []RunningTaskInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]RunningTaskInfo, 0, len(e.running))
	for _, rt := range e.running {
		if rt.IsDone() {
			continue
		}
		result = append(result, RunningTaskInfo{
			TaskID:       rt.TaskID,
			UserID:       rt.UserID,
			Platform:     rt.Platform,
			Target:       rt.Target,
			Threads:      rt.Threads,
			SuccessCount: rt.SuccessCount.Load(),
			FailCount:    rt.FailCount.Load(),
			StartedAt:    rt.StartedAt,
			Stopping:     rt.Stopping.Load(),
		})
	}
	return result
}

// AdminStopTask 管理员强制停止任务（通过 taskID 查找）
func (e *TaskEngine) AdminStopTask(taskID uint) bool {
	e.mu.RLock()
	var found *RunningTask
	var foundKey taskKey
	for k, rt := range e.running {
		if rt.TaskID == taskID && !rt.IsDone() {
			found = rt
			foundKey = k
			break
		}
	}
	e.mu.RUnlock()

	if found == nil {
		return false
	}

	log.Info().Uint("task_id", taskID).Msg("管理员强制停止任务")
	found.Log("[!] 管理员强制停止任务")
	found.Stopping.Store(true)
	found.Cancel()
	e.dispatcher.ForceComplete(foundKey) // 立即标记完成，不等 inflight worker

	e.db.Model(&model.Task{}).Where("id = ?", taskID).Update("status", "stopping")
	return true
}

// getPlatformCount 获取平台并发计数器（惰性初始化）
func (e *TaskEngine) getPlatformCount(platform string) *atomic.Int32 {
	e.mu.RLock()
	if pc, ok := e.platformCount[platform]; ok {
		e.mu.RUnlock()
		return pc
	}
	e.mu.RUnlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	if pc, ok := e.platformCount[platform]; ok {
		return pc
	}
	pc := &atomic.Int32{}
	e.platformCount[platform] = pc
	return pc
}

// saveResult 保存注册结果
func (e *TaskEngine) saveResult(taskID, userID uint, platform, email string, credential map[string]interface{}) {
	credJSON, err := json.Marshal(credential)
	if err != nil {
		log.Error().Err(err).Uint("task_id", taskID).Msg("credential 序列化失败")
	}
	result := model.TaskResult{
		TaskID:         taskID,
		UserID:         userID,
		Platform:       platform,
		Email:          email,
		CredentialData: credJSON,
	}
	// Grok 向后兼容：提取 auth_token（兼容旧数据 sso_token）
	if platform == "grok" {
		if token, ok := credential["auth_token"].(string); ok {
			result.SSOToken = token
		} else if token, ok := credential["sso_token"].(string); ok {
			result.SSOToken = token
		}
	}
	if err := e.db.Create(&result).Error; err != nil {
		log.Error().Err(err).Uint("task_id", taskID).Str("email", email).Msg("保存注册结果失败")
	}
}

// injectSettings 将系统设置注入到 Worker 配置中（不覆盖已有值）
func (e *TaskEngine) injectSettings(cfg worker.Config) {
	// 需要注入的系统设置键列表
	keys := []string{
		"yydsmail_api_key",
		"yydsmail_base_url",
		"email_provider_priority",
		"turnstile_solver_url",
		"turnstile_solver_proxy",
		"cf_bypass_solver_url",
		"capsolver_key",
		"yescaptcha_key",
		"grok_action_id",
		"grok_site_key",
		"camoufox_reg_url",
		"openai_reg_url",
		"kiro_reg_url",
		"grok_reg_url",
		"grok_email_providers",
		"openai_email_providers",
		"kiro_email_providers",
		"kiro_proxy",
		"gemini_reg_url",
		"gemini_email_providers",
		"gemini_proxy",
		"grok_proxy",
		"openai_proxy",
		"stripe_pk",
	}
	for _, k := range keys {
		if cfg[k] == "" {
			if v := e.settingSvc.Get(k, ""); v != "" {
				cfg[k] = v
			}
		}
	}
}

// startWorkerPool 启动 Worker Pool 并开启后台动态伸缩
// 每 10 秒检查 hf_instance_count × workers_per_instance，自动扩容/缩容
func (e *TaskEngine) startWorkerPool() {
	poolSize := e.calcPoolSize()
	e.targetWorkers.Store(int32(poolSize))
	e.workerCount.Store(int32(poolSize))
	for i := 0; i < poolSize; i++ {
		go e.runPoolWorker()
	}
	go e.poolResizer()
	log.Info().Int("pool_size", poolSize).Msg("Worker Pool 已启动")
}

// calcPoolSize 根据 HF 实例数 × 每实例并发数 计算 Pool 大小
func (e *TaskEngine) calcPoolSize() int {
	instances, _ := strconv.Atoi(e.settingSvc.Get("hf_instance_count", "20"))
	if instances <= 0 {
		instances = 20
	}
	perInstance, _ := strconv.Atoi(e.settingSvc.Get("workers_per_instance", "10"))
	if perInstance <= 0 {
		perInstance = 10
	}
	return instances * perInstance
}

// poolResizer 后台动态伸缩 Worker Pool + 刷新任务并发上限
// 每 10 秒读取配置，扩容立即生效，缩容等 Worker 干完手头活后退出
// 同时刷新所有活跃任务的 maxInflight，使 SyncPoolSize 的动态参数热生效
func (e *TaskEngine) poolResizer() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		desired := int32(e.calcPoolSize())
		e.targetWorkers.Store(desired)

		current := e.workerCount.Load()
		if desired > current {
			// 扩容：立即补充差额 Worker
			diff := desired - current
			for i := int32(0); i < diff; i++ {
				e.workerCount.Add(1)
				go e.runPoolWorker()
			}
			log.Info().Int32("old", current).Int32("new", desired).Int32("spawned", diff).Msg("Worker Pool 扩容")
		} else if desired < current {
			// 缩容：多余 Worker 完成当前任务后自动退出
			log.Info().Int32("current", current).Int32("target", desired).Msg("Worker Pool 缩容中")
			e.dispatcher.Wake() // 唤醒空闲 Worker 让它们检查退出条件
		}

		// 动态刷新活跃任务的 maxInflight（复用 computeMaxInflight 统一算法）
		e.dispatcher.RefreshLimits(func(p string) int32 {
			return e.computeMaxInflight(p)
		})
	}
}

// runPoolWorker 单个 Pool Worker 的主循环
// 不断从 Dispatcher 获取下一个注册任务并执行，空闲时阻塞等待
func (e *TaskEngine) runPoolWorker() {
	defer e.workerCount.Add(-1)
	for {
		// 缩容检查：超过目标数的 Worker 主动退出
		if e.workerCount.Load() > e.targetWorkers.Load() {
			return
		}
		job, ok := e.dispatcher.Next(e.proxyPool)
		if !ok {
			return // Dispatcher 已关闭（服务关停）
		}
		e.executeRegJob(job)
	}
}

// executeRegJob 执行单次注册任务（含重试逻辑）
// 由 Pool Worker 调用，完成后通知 DispatchEntry 检查任务是否全部结束
func (e *TaskEngine) executeRegJob(job *RegJob) {
	entry := job.Entry
	rt := entry.RT

	defer func() {
		entry.inflight.Add(-1)
		entry.checkCompletion()
		e.dispatcher.Wake() // 释放并发槽位，唤醒等待中的 Worker
	}()

	const maxRetries = 3

	// openai_team: 从卡池分配卡并注入配置
	var cardID uint
	if rt.Platform == "openai_team" && e.cardPool != nil {
		card := e.cardPool.GetNext()
		if card == nil {
			rt.Log("[-] 卡池无可用卡，跳过")
			rt.ClaimedCount.Add(-1)
			rt.FailCount.Add(1)
			rt.NotifyStatus()
			e.db.Model(&model.Task{}).Where("id = ?", rt.TaskID).
				Update("fail_count", gorm.Expr("fail_count + 1"))
			e.dispatcher.Wake()
			return
		}
		cardID = card.ID
		job.Cfg["card_number"] = card.CardNumber
		job.Cfg["exp_month"] = strconv.Itoa(card.ExpMonth)
		job.Cfg["exp_year"] = strconv.Itoa(card.ExpYear)
		job.Cfg["cvc"] = card.CVC
		job.Cfg["billing_name"] = card.BillingName
		job.Cfg["billing_email"] = card.BillingEmail
		job.Cfg["billing_country"] = card.BillingCountry
		job.Cfg["billing_city"] = card.BillingCity
		job.Cfg["billing_line1"] = card.BillingLine1
		job.Cfg["billing_zip"] = card.BillingZip
	}

	for retryN := 1; retryN <= maxRetries; retryN++ {
		// 用户已停止 → 立即退出，不再尝试注册
		if entry.Ctx.Err() != nil {
			break
		}
		var succeeded, failed bool
		entry.Worker.RegisterOne(entry.Ctx, worker.RegisterOpts{
			Proxy:  job.Proxy,
			Config: job.Cfg,
			LogCh:  rt.LogCh,
			OnSuccess: func(email string, credential map[string]interface{}) {
				succeeded = true
				newCount := rt.SuccessCount.Add(1)
				rt.NotifyStatus()
				// 实时推送成功账号到前端日志（序号+邮箱）
				rt.Log("[✓] #%d %s", newCount, email)
				e.saveResult(rt.TaskID, rt.UserID, rt.Platform, email, credential)
				// 原子递增防止并发 worker 互相覆盖
				e.db.Model(&model.Task{}).Where("id = ?", rt.TaskID).
					Update("success_count", gorm.Expr("success_count + 1"))
				// 卡池生命周期：标记卡已使用
				if cardID > 0 && e.cardPool != nil {
					e.cardPool.MarkUsed(cardID)
				}
				if newCount >= int64(rt.Target) {
					entry.Cancel()
				}
			},
			OnFail: func() {
				failed = true
			},
		})
		if succeeded || !failed {
			break // 成功或 ctx 已取消，退出重试
		}
		if retryN < maxRetries {
			// 中间重试不推送前端，静默重试
		} else {
			rt.Log("[-] 已重试 %d 次，计入失败", maxRetries)
			rt.ClaimedCount.Add(-1) // 释放名额，让 Pool Worker 可以重新认领
			// 卡池生命周期：标记卡失败
			if cardID > 0 && e.cardPool != nil {
				e.cardPool.MarkFailed(cardID)
			}
			newCount := rt.FailCount.Add(1)
			rt.NotifyStatus()
			// 原子递增防止并发 worker 互相覆盖
			e.db.Model(&model.Task{}).Where("id = ?", rt.TaskID).
				Update("fail_count", gorm.Expr("fail_count + 1"))
			if newCount >= int64(rt.Target) {
				entry.Cancel()
			}
			e.dispatcher.Wake() // 唤醒等待中的 Worker，重新认领释放的名额
		}
	}
}

// ─── ScanConfig 全局缓存 ───

// startConfigRefresher 后台定时刷新所有平台的扫描配置
// 首次延迟 10 秒（等服务就绪），之后每 5 分钟刷新一次
func (e *TaskEngine) startConfigRefresher() {
	time.Sleep(10 * time.Second)
	e.refreshAllConfigs()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		e.refreshAllConfigs()
	}
}

// refreshAllConfigs 刷新所有已注册平台的配置缓存
func (e *TaskEngine) refreshAllConfigs() {
	for name, w := range worker.Registry {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		baseCfg := worker.Config{}
		e.injectSettings(baseCfg)

		scanned, err := w.ScanConfig(ctx, e.proxyPool.GetNext(), baseCfg)
		cancel()
		if err != nil {
			log.Warn().Str("platform", name).Err(err).Msg("配置缓存刷新失败（保留旧缓存）")
			continue
		}

		e.updateConfigCache(name, scanned)
		log.Info().Str("platform", name).Msg("配置缓存已刷新")
	}
}

// getCachedConfig 获取平台的缓存配置（10 分钟内有效）
func (e *TaskEngine) getCachedConfig(platform string) (worker.Config, bool) {
	e.configCacheMu.RLock()
	defer e.configCacheMu.RUnlock()
	cc, ok := e.configCache[platform]
	if !ok || time.Since(cc.scannedAt) > 10*time.Minute {
		return nil, false
	}
	// 返回副本，避免并发修改
	cp := make(worker.Config, len(cc.config))
	for k, v := range cc.config {
		cp[k] = v
	}
	return cp, true
}

// updateConfigCache 更新平台配置缓存
func (e *TaskEngine) updateConfigCache(platform string, cfg worker.Config) {
	e.configCacheMu.Lock()
	defer e.configCacheMu.Unlock()
	e.configCache[platform] = &cachedConfig{
		config:    cfg,
		scannedAt: time.Now(),
	}
}
