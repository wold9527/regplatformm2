package handler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xiaolajiaoyyds/regplatformm/internal/service"
)

// WSHandler SSE/WebSocket 日志处理器
type WSHandler struct {
	engine  *service.TaskEngine
	authSvc *service.AuthService
}

// NewWSHandler 创建日志处理器
func NewWSHandler(engine *service.TaskEngine, authSvc *service.AuthService) *WSHandler {
	return &WSHandler{engine: engine, authSvc: authSvc}
}

// SSELogs SSE 日志流（GET /ws/logs/:taskId/stream?token=）
func (h *WSHandler) SSELogs(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.Param("taskId"), 10, 32)
	token := c.Query("token")

	userID, err := h.authSvc.VerifyJWT(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "未登录"})
		return
	}

	// 等待任务出现（最多 10 秒，感知客户端断开）
	var rt *service.RunningTask
	timeout := time.After(10 * time.Second)
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-timeout:
			c.JSON(http.StatusNotFound, gin.H{"detail": "任务不存在或已结束"})
			return
		default:
			rt = h.engine.FindRunningTaskByID(uint(taskID))
			if rt != nil && rt.UserID == userID {
				goto found
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

found:

	if rt == nil || rt.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"detail": "任务不存在或已结束"})
		return
	}

	// 订阅日志
	sub := rt.Subscribe()
	defer rt.Unsubscribe(sub)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// 先发送历史日志（页面刷新后回放）
	for _, msg := range rt.GetLogBuffer() {
		c.SSEvent("", gin.H{"type": "log", "message": msg})
	}
	// 发送当前状态快照
	c.SSEvent("", gin.H{
		"type":    "status",
		"success": rt.SuccessCount.Load(),
		"fail":    rt.FailCount.Load(),
		"target":  rt.Target,
	})
	c.Writer.Flush()

	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-sub:
			if !ok {
				return false
			}
			c.SSEvent("", gin.H{"type": "log", "message": msg})
			return true
		case <-rt.StatusCh:
			// 成功/失败计数变化，立即推送状态
			c.SSEvent("", gin.H{
				"type":    "status",
				"success": rt.SuccessCount.Load(),
				"fail":    rt.FailCount.Load(),
				"target":  rt.Target,
			})
			// StatusCh 与 Done 可能同时就绪，select 随机选中 StatusCh 时需补发 complete
			if rt.IsDone() {
				elapsed := time.Since(rt.StartedAt).Round(time.Second).String()
				c.SSEvent("", gin.H{
					"type":    "complete",
					"success": rt.SuccessCount.Load(),
					"fail":    rt.FailCount.Load(),
					"target":  rt.Target,
					"elapsed": elapsed,
				})
				return false
			}
			return true
		case <-time.After(2 * time.Second):
			// 心跳 + 状态
			c.SSEvent("", gin.H{
				"type":    "status",
				"success": rt.SuccessCount.Load(),
				"fail":    rt.FailCount.Load(),
				"target":  rt.Target,
			})
			return !rt.IsDone()
		case <-rt.Done:
			elapsed := time.Since(rt.StartedAt).Round(time.Second).String()
			c.SSEvent("", gin.H{
				"type":    "complete",
				"success": rt.SuccessCount.Load(),
				"fail":    rt.FailCount.Load(),
				"target":  rt.Target,
				"elapsed": elapsed,
			})
			return false
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// WebSocketLogs WebSocket 日志（GET /ws/logs/:taskId?token=）
func (h *WSHandler) WebSocketLogs(c *gin.Context) {
	// WebSocket 作为 SSE 的降级备选，暂用 SSE 替代
	c.JSON(http.StatusOK, gin.H{
		"message": "请使用 SSE 端点: /ws/logs/{taskId}/stream?token=",
		"sse_url": fmt.Sprintf("/ws/logs/%s/stream", c.Param("taskId")),
	})
}
