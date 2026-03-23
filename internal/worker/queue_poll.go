package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// QueueStatus CF Worker /api/v1/status 聚合的远程节点池排队状态
type QueueStatus struct {
	TotalActive   int `json:"total_active"`
	TotalWaiting  int `json:"total_waiting"`
	MaxConcurrent int `json:"max_concurrent"`
	AvgSeconds    int `json:"avg_seconds"`
	HealthyNodes  int `json:"healthy_nodes"`
	TotalNodes    int `json:"total_nodes"`
}

// ETA 估算排队等待时间（秒）
// 公式：平均耗时 × 排队人数 ÷ 总处理槽位
func (qs *QueueStatus) ETA() int {
	if qs.TotalWaiting <= 0 {
		return 0
	}
	throughput := qs.MaxConcurrent
	if throughput < 1 {
		throughput = 1
	}
	return qs.AvgSeconds * qs.TotalWaiting / throughput
}

// queuePollClient 排队状态轮询专用 HTTP 客户端（短超时，不影响主请求）
var queuePollClient = &http.Client{Timeout: 5 * time.Second}

// PollQueueStatus 查询 CF Worker 聚合的排队状态
// statusURL 格式: https://cf-worker.xxx/api/v1/status?t=openai
func PollQueueStatus(ctx context.Context, statusURL string) (*QueueStatus, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", statusURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := queuePollClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var qs QueueStatus
	if err := json.NewDecoder(resp.Body).Decode(&qs); err != nil {
		return nil, err
	}
	return &qs, nil
}

// FormatQueueLog 将排队状态格式化为用户可读的日志行
func FormatQueueLog(qs *QueueStatus, elapsed int) string {
	if qs.TotalWaiting > 0 {
		eta := qs.ETA()
		return fmt.Sprintf("[…] 排队中 %ds | 处理中 %d · 排队 %d · 节点 %d | 预计还需 %d 秒",
			elapsed, qs.TotalActive, qs.TotalWaiting, qs.HealthyNodes, eta)
	}
	return fmt.Sprintf("[…] 远程注册进行中 (%ds)...", elapsed)
}

// BuildQueueStatusURL 构建排队状态查询 URL
// 多节点逗号分隔时，只取第一个（CF Worker），直连节点没有 /api/v1/status 路由
func BuildQueueStatusURL(serviceURL, platform string) string {
	if idx := strings.Index(serviceURL, ","); idx > 0 {
		serviceURL = serviceURL[:idx]
	}
	return strings.TrimRight(serviceURL, "/") + "/api/v1/status?t=" + platform
}
