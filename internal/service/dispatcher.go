package service

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/xiaolajiaoyyds/regplatformm/internal/worker"
)

// DispatchEntry 调度器中的任务条目（一个活跃任务 = 一个 Entry）
type DispatchEntry struct {
	Key         taskKey
	RT          *RunningTask
	Worker      worker.Worker
	Ctx         context.Context
	Cancel      context.CancelFunc
	Config      worker.Config       // 合并后的完整配置（base + scanned）
	NodeURLs    map[string][]string // 多节点 URL 轮询表
	nodeIdx     atomic.Int64        // 轮询索引
	inflight    atomic.Int32        // 正在执行中的注册数
	maxInflight atomic.Int32        // 单任务最大并发数（动态更新，由 poolResizer 定期刷新）
	Platform    string              // 所属平台（用于动态刷新 maxInflight）
	done        chan struct{}        // 所有注册完成信号
	doneOnce    sync.Once
}

// markDone 安全地标记任务完成（仅触发一次）
func (de *DispatchEntry) markDone() {
	de.doneOnce.Do(func() { close(de.done) })
}

// checkCompletion 在每次注册完成后检查任务是否全部结束。
//
// N+1 超发策略：OpenAI 平台会多认领 1 个名额，所以这里优先判断
// 成功数是否已达 Target，达标则立即 markDone，不等剩余 inflight。
// 剩余那个 inflight 注册会在后台跑完并将结果写入数据库，对用户是免费 bonus。
func (de *DispatchEntry) checkCompletion() {
	rt := de.RT
	// 成功数已达目标 → 立即完成，不等剩余 inflight（N+1 超发的关键路径）
	if rt.SuccessCount.Load() >= int64(rt.Target) {
		de.markDone()
		return
	}
	// 用户停止 → 立即完成，不等 inflight（浏览器会随 ctx 取消自行退出）
	if de.Ctx.Err() != nil {
		de.markDone()
		return
	}
	if de.inflight.Load() > 0 {
		return // 仍有执行中的注册，继续等待
	}
	// 安全阀：失败数达到目标数，所有名额都失败了
	if rt.FailCount.Load() >= int64(rt.Target) {
		de.markDone()
		return
	}
}

// RegJob 单次注册任务（从 Dispatcher 分发给 Pool Worker）
type RegJob struct {
	Entry *DispatchEntry
	Proxy *worker.ProxyEntry
	Cfg   worker.Config
}

// Dispatcher 全局 Round-Robin 公平调度器
// 所有活跃任务共享 Worker Pool，按轮询顺序分配注册名额
type Dispatcher struct {
	mu     sync.Mutex
	cond   *sync.Cond
	tasks  []*DispatchEntry
	rrIdx  int
	closed bool
}

// NewDispatcher 创建调度器
func NewDispatcher() *Dispatcher {
	d := &Dispatcher{tasks: make([]*DispatchEntry, 0)}
	d.cond = sync.NewCond(&d.mu)
	return d
}

// Add 注册新任务到调度器
func (d *Dispatcher) Add(entry *DispatchEntry) {
	d.mu.Lock()
	d.tasks = append(d.tasks, entry)
	d.mu.Unlock()
	d.cond.Broadcast()
}

// Remove 移除已完成的任务
func (d *Dispatcher) Remove(key taskKey) {
	d.mu.Lock()
	for i, e := range d.tasks {
		if e.Key == key {
			d.tasks = append(d.tasks[:i], d.tasks[i+1:]...)
			if d.rrIdx >= len(d.tasks) && d.rrIdx > 0 {
				d.rrIdx = 0
			}
			break
		}
	}
	d.mu.Unlock()
	d.cond.Broadcast()
}

// Next 获取下一个待执行的注册任务（阻塞直到有工作或调度器关闭）
// 使用 Round-Robin 保证用户间公平：A→B→C→A→B→C...
func (d *Dispatcher) Next(proxyPool *ProxyPool) (*RegJob, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for {
		if d.closed {
			return nil, false
		}

		n := len(d.tasks)
		if n == 0 {
			d.cond.Wait()
			continue
		}

		// Round-Robin 遍历所有任务，找到一个可分配名额的
		for i := 0; i < n; i++ {
			idx := (d.rrIdx + i) % n
			entry := d.tasks[idx]

			// 跳过已取消/已完成的任务
			if entry.Ctx.Err() != nil {
				continue
			}

			// 失败安全阀：失败数已达目标，不再分配新名额（防止无限重试循环）
			if entry.RT.FailCount.Load() >= int64(entry.RT.Target) {
				continue
			}

			// 并发上限：inflight 达到 maxInflight 时跳过，防止打爆 Space 池
			limit := entry.maxInflight.Load()
			if limit > 0 && entry.inflight.Load() >= limit {
				continue
			}

			// 原子认领一个注册名额
			newClaimed := entry.RT.ClaimedCount.Add(1)
			maxClaim := int64(entry.RT.Target)
			// N+1 超发策略：OpenAI 平台多分配 1 个名额，让第 N+1 个注册提前启动。
			// 当前 N 个成功后 checkCompletion 会立即 markDone，第 N+1 个在后台
			// 继续跑完并写库，结果对用户是免费 bonus，积分不额外扣除。
			if entry.Worker.PlatformName() == "openai" {
				maxClaim += 1
			}
			if newClaimed > maxClaim {
				entry.RT.ClaimedCount.Add(-1)
				continue // 已全部认领
			}

			// 成功认领，推进 Round-Robin 指针（下次从下一个任务开始）
			d.rrIdx = (idx + 1) % n
			entry.inflight.Add(1)

			// 构建配置副本（含 node URL 轮询分配）
			cfg := make(worker.Config, len(entry.Config))
			for k, v := range entry.Config {
				cfg[k] = v
			}
			nodeN := int(entry.nodeIdx.Add(1) - 1)
			for key, urls := range entry.NodeURLs {
				cfg[key] = urls[nodeN%len(urls)]
			}

			return &RegJob{
				Entry: entry,
				Proxy: proxyPool.GetNextForPlatform(entry.Key.Platform),
				Cfg:   cfg,
			}, true
		}

		// 所有任务都无法认领（已满或已取消），等待变化
		d.cond.Wait()
	}
}

// Wake 唤醒等待中的 Worker（名额释放、任务变化时调用）
func (d *Dispatcher) Wake() {
	d.cond.Broadcast()
}

// ForceComplete 强制标记指定任务完成（用户停止时调用，不等 inflight）
func (d *Dispatcher) ForceComplete(key taskKey) {
	d.mu.Lock()
	for _, e := range d.tasks {
		if e.Key == key {
			e.markDone()
			break
		}
	}
	d.mu.Unlock()
	d.cond.Broadcast()
}

// Close 关闭调度器（服务关停时调用）
func (d *Dispatcher) Close() {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
	d.cond.Broadcast()
}

// ActiveTaskCount 返回调度器中的活跃任务数
func (d *Dispatcher) ActiveTaskCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.tasks)
}

// RefreshLimits 动态刷新所有活跃任务的 maxInflight
// 由 poolResizer 每 10 秒调用，确保 SyncPoolSize 更新的参数能热生效到正在运行的任务
func (d *Dispatcher) RefreshLimits(getLimitFn func(platform string) int32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, entry := range d.tasks {
		if entry.Ctx.Err() != nil {
			continue
		}
		newLimit := getLimitFn(entry.Platform)
		if newLimit > 0 {
			entry.maxInflight.Store(newLimit)
		}
	}
	d.cond.Broadcast() // 唤醒可能因 maxInflight 被阻塞的 Worker
}
