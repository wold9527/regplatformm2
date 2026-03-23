package service

import (
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"gorm.io/gorm"
)

// isQueuedMsg 判断 StartTask 返回的消息是否表示任务仍在排队
func isQueuedMsg(msg string) bool {
	return msg == "已加入队列" || strings.HasPrefix(msg, "排队中")
}

// QueueScheduler 任务队列调度器，负责从 DB 队列中取出任务并启动
type QueueScheduler struct {
	engine     *TaskEngine
	db         *gorm.DB
	creditSvc  *CreditService
	dispatchMu sync.Mutex // 防止并发 TryDispatch 重复调度同一任务
}

// NewQueueScheduler 创建调度器
func NewQueueScheduler(db *gorm.DB, engine *TaskEngine, creditSvc *CreditService) *QueueScheduler {
	return &QueueScheduler{engine: engine, db: db, creditSvc: creditSvc}
}

// TryDispatch 尝试从队列中调度排队任务。
// 按创建时间扫描排队任务，跳过平台已满的任务，直到成功启动一个或全部扫描完毕。
func (q *QueueScheduler) TryDispatch() {
	q.dispatchMu.Lock()
	defer q.dispatchMu.Unlock()

	var tasks []model.Task
	q.db.Where("status = ?", "queued").
		Order("created_at ASC").
		Limit(20).
		Find(&tasks)

	for _, task := range tasks {
		ok, msg := q.engine.StartTask(task.UserID, task.ID, task.Platform, task.TargetCount, task.ThreadCount)
		if ok && !isQueuedMsg(msg) {
			log.Info().Uint("task_id", task.ID).Str("platform", task.Platform).Msg("队列任务已调度")
			return // 每次释放一个槽位，调度一个即可
		}
		if !ok {
			log.Warn().Uint("task_id", task.ID).Str("msg", msg).Msg("队列任务调度失败")
		}
		// 仍在排队（平台满或用户槽位满），尝试下一个排队任务
	}
}

// RecoverOnBoot 服务启动时清理残留状态并恢复队列。
// 应在 DB ready 后、HTTP server 启动前调用。
func (q *QueueScheduler) RecoverOnBoot() {
	now := time.Now()

	// running/stopping → stopped（服务重启导致的中断），同时退还未消费积分
	var interrupted []model.Task
	q.db.Where("status IN ?", []string{"running", "stopping"}).Find(&interrupted)
	for _, task := range interrupted {
		refund := task.CreditsReserved - task.SuccessCount
		if refund > 0 {
			q.creditSvc.RefundCredits(q.db, task.UserID, task.ID, refund)
			log.Info().Uint("task_id", task.ID).Int("refund", refund).Msg("重启恢复：退还中断任务积分")
		}
		q.db.Model(&task).Updates(map[string]interface{}{
			"status":     "stopped",
			"stopped_at": &now,
		})
	}
	if len(interrupted) > 0 {
		log.Warn().Int("count", len(interrupted)).Msg("重启恢复：中断任务已标记为 stopped 并退款")
	}

	// queued → 依次尝试调度（按平台独立限流，某平台满不影响其他平台）
	var queuedTasks []model.Task
	q.db.Where("status = ?", "queued").Order("created_at ASC").Find(&queuedTasks)
	for _, task := range queuedTasks {
		ok, msg := q.engine.StartTask(task.UserID, task.ID, task.Platform, task.TargetCount, task.ThreadCount)
		if ok && !isQueuedMsg(msg) {
			log.Info().Uint("task_id", task.ID).Str("platform", task.Platform).Msg("重启恢复：队列任务已调度")
		}
		// 不 break：某平台满或用户槽位满不影响其他排队任务
	}

	log.Info().Msg("任务队列恢复完成")
}
