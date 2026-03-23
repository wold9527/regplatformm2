package worker

import (
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// 并发限流 + 排队等待中间件
//
// HF Space 免费容器资源有限（2 vCPU / 16GB RAM），每个注册请求 2-5 分钟。
// 设计：
//   - 处理槽（semaphore）：同时处理 MAX_CONCURRENT 个请求（默认 5）
//   - 等待队列：最多 MAX_QUEUE 个请求排队等待（默认 10）
//   - 队列满：返回 503，CF Worker 自动转发下一节点
//   - 排队中：阻塞等待，轮到后自动处理，响应中附带排队等待时长
//   - 平均耗时追踪：基于最近完成的请求计算 ETA

var (
	maxConcurrent int32 // 最大并发处理数
	maxQueue      int32 // 最大排队等待数
	activeCount   int64 // 当前正在处理的请求数
	waitingCount  int64 // 当前排队等待的请求数
	totalServed   int64 // 累计完成请求数（用于计算平均耗时）

	// 滑动窗口：最近 20 个请求的处理耗时（纳秒）
	durationWindow [20]int64
	durationIdx    int64
	durationMu     sync.Mutex
)

func init() {
	// 默认值适配 HF Space 免费实例（2 vCPU / 16GB）
	// Mac Mini 等高配环境通过 ENV 覆盖
	maxConcurrent = 5
	maxQueue = 10
	if v := os.Getenv("MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConcurrent = int32(n)
		}
	}
	if v := os.Getenv("MAX_QUEUE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxQueue = int32(n)
		}
	}
}

// recordDuration 记录一次请求的处理耗时（用于 ETA 估算）
func recordDuration(d time.Duration) {
	durationMu.Lock()
	idx := durationIdx % int64(len(durationWindow))
	durationWindow[idx] = int64(d)
	durationIdx++
	durationMu.Unlock()
	atomic.AddInt64(&totalServed, 1)
}

// avgDuration 返回最近请求的平均处理耗时
func avgDuration() time.Duration {
	durationMu.Lock()
	defer durationMu.Unlock()

	count := durationIdx
	if count == 0 {
		return 3 * time.Minute // 无历史数据，默认 3 分钟
	}
	if count > int64(len(durationWindow)) {
		count = int64(len(durationWindow))
	}
	var sum int64
	for i := int64(0); i < count; i++ {
		sum += durationWindow[i]
	}
	return time.Duration(sum / count)
}

// ConcurrencyLimiter 返回 Gin 中间件
//
// 流程：
//  1. 尝试立即获取处理槽 → 成功则直接处理
//  2. 处理槽满 → 检查排队是否有空位
//  3. 有空位 → 阻塞等待，轮到后处理（响应附带等待时长）
//  4. 队列也满 → 返回 503（CF Worker 转发下一节点）
func ConcurrencyLimiter() gin.HandlerFunc {
	sem := make(chan struct{}, maxConcurrent)

	return func(c *gin.Context) {
		start := time.Now()

		// 尝试立即获取处理槽
		select {
		case sem <- struct{}{}:
			atomic.AddInt64(&activeCount, 1)
			defer func() {
				<-sem
				atomic.AddInt64(&activeCount, -1)
				recordDuration(time.Since(start))
			}()
			c.Next()
			return
		default:
		}

		// 处理槽满，检查排队空位
		currentWaiting := atomic.LoadInt64(&waitingCount)
		if currentWaiting >= int64(maxQueue) {
			// 队列也满，返回 503（中性化响应，不暴露内部状态）
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"ok":    false,
				"error": "service temporarily unavailable",
				"retry": true,
			})
			c.Abort()
			return
		}

		// 进入排队
		pos := atomic.AddInt64(&waitingCount, 1)
		defer atomic.AddInt64(&waitingCount, -1)

		// 阻塞等待处理槽释放（受请求 context 超时控制）
		select {
		case sem <- struct{}{}:
			// 轮到了
			waitDuration := time.Since(start)
			atomic.AddInt64(&activeCount, 1)
			defer func() {
				<-sem
				atomic.AddInt64(&activeCount, -1)
				recordDuration(time.Since(start))
			}()
			// 在 context 中记录排队信息，handler 可以读取并写入响应日志
			c.Set("queue_waited", true)
			c.Set("queue_position", pos)
			c.Set("queue_wait_seconds", int(waitDuration.Seconds()))
			c.Next()

		case <-c.Request.Context().Done():
			// 请求超时或被取消
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"ok":    false,
				"error": "request timeout",
				"retry": true,
			})
			c.Abort()
		}
	}
}

// ConcurrencyStats 返回当前并发和排队状态（供 /health 端点使用，精简字段）
func ConcurrencyStats() gin.H {
	return gin.H{
		"load":  atomic.LoadInt64(&activeCount),
		"queue": atomic.LoadInt64(&waitingCount),
		"cap":   maxConcurrent,
		"avg":   int(avgDuration().Seconds()),
	}
}
