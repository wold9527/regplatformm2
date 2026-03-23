package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/xiaolajiaoyyds/regplatformm/internal/dto"
	"github.com/xiaolajiaoyyds/regplatformm/internal/service"
)

// HFSpaceHandler HF Space 管理处理器
type HFSpaceHandler struct {
	svc *service.HFSpaceService
}

// NewHFSpaceHandler 创建 HF Space 管理处理器
func NewHFSpaceHandler(svc *service.HFSpaceService) *HFSpaceHandler {
	return &HFSpaceHandler{svc: svc}
}

// ListTokens 列出所有 Token（GET /api/admin/hf/tokens）
func (h *HFSpaceHandler) ListTokens(c *gin.Context) {
	tokens, err := h.svc.ListTokens()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tokens)
}

// CreateToken 创建 Token（POST /api/admin/hf/tokens）
func (h *HFSpaceHandler) CreateToken(c *gin.Context) {
	var req dto.CreateHFTokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	token, err := h.svc.CreateToken(req.Label, req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	// 脱敏返回
	if len(token.Token) > 8 {
		token.Token = token.Token[:4] + "****" + token.Token[len(token.Token)-4:]
	}
	c.JSON(http.StatusOK, token)
}

// DeleteToken 删除 Token（DELETE /api/admin/hf/tokens/:id）
func (h *HFSpaceHandler) DeleteToken(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 Token ID"})
		return
	}
	if err := h.svc.DeleteToken(uint(id)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// ValidateToken 验证 Token（POST /api/admin/hf/tokens/:id/validate）
func (h *HFSpaceHandler) ValidateToken(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 Token ID"})
		return
	}
	token, err := h.svc.ValidateToken(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": err.Error()})
		return
	}
	// 脱敏返回
	if len(token.Token) > 8 {
		token.Token = token.Token[:4] + "****" + token.Token[len(token.Token)-4:]
	}
	c.JSON(http.StatusOK, token)
}

// ValidateAllTokens 批量验证所有 Token（POST /api/admin/hf/tokens/validate-all）
func (h *HFSpaceHandler) ValidateAllTokens(c *gin.Context) {
	tokens, err := h.svc.ValidateAllTokens()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	// 脱敏
	for i := range tokens {
		if len(tokens[i].Token) > 8 {
			tokens[i].Token = tokens[i].Token[:4] + "****" + tokens[i].Token[len(tokens[i].Token)-4:]
		}
	}
	valid := 0
	for _, t := range tokens {
		if t.IsValid {
			valid++
		}
	}
	c.JSON(http.StatusOK, gin.H{"tokens": tokens, "total": len(tokens), "valid": valid, "invalid": len(tokens) - valid})
}

// PurgeBannedSpaces 批量清理被封 Space（POST /api/admin/hf/spaces/purge?service=xxx）
func (h *HFSpaceHandler) PurgeBannedSpaces(c *gin.Context) {
	svc := c.Query("service")
	deleted, checked, err := h.svc.PurgeBannedSpaces(svc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("检查 %d 个 Space，删除 %d 个被封/失效的", checked, deleted),
		"checked": checked,
		"deleted": deleted,
	})
}

// ListSpaces 列出 Space（GET /api/admin/hf/spaces?service=xxx&status=xxx&page=1&page_size=20）
func (h *HFSpaceHandler) ListSpaces(c *gin.Context) {
	svc := c.Query("service")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	result, err := h.svc.ListSpaces(svc, page, pageSize, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// AddSpace 手动添加 Space（POST /api/admin/hf/spaces）
func (h *HFSpaceHandler) AddSpace(c *gin.Context) {
	var req dto.AddHFSpaceManualReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	space, err := h.svc.AddSpaceManual(req.Service, req.URL, req.RepoID, req.TokenID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, space)
}

// DeleteSpace 删除 Space（DELETE /api/admin/hf/spaces/:id）
func (h *HFSpaceHandler) DeleteSpace(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 Space ID"})
		return
	}
	if err := h.svc.DeleteSpace(uint(id)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// CheckHealth 触发健康检查（POST /api/admin/hf/spaces/health?service=xxx）
func (h *HFSpaceHandler) CheckHealth(c *gin.Context) {
	svc := c.Query("service")
	results, err := h.svc.CheckHealth(svc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": results, "total": len(results)})
}

// DeploySpaces 批量部署新 Space（POST /api/admin/hf/spaces/deploy）
func (h *HFSpaceHandler) DeploySpaces(c *gin.Context) {
	var req dto.DeployHFSpaceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	deployed, errors, err := h.svc.DeploySpaces(req.Service, req.Count, req.ReleaseURL, req.TokenID, req.Secrets)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"deployed": deployed,
		"errors":   errors,
		"success":  len(deployed),
		"failed":   len(errors),
	})
}

// Autoscale 触发弹性管理（POST /api/admin/hf/autoscale）
// service=all 时遍历所有服务，聚合结果为单个对象返回
func (h *HFSpaceHandler) Autoscale(c *gin.Context) {
	var req dto.HFAutoscaleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	services := []string{req.Service}
	if req.Service == "all" {
		services = []string{"openai", "grok", "kiro", "gemini", "ts"}
	}

	// 聚合结果
	merged := &service.AutoscaleResult{
		Service: req.Service,
		DryRun:  req.DryRun,
		Logs:    make([]service.AutoscaleLog, 0),
	}

	for _, svc := range services {
		result, err := h.svc.Autoscale(svc, req.Target, req.DryRun)
		if err != nil {
			merged.Logs = append(merged.Logs, service.AutoscaleLog{
				Step: svc, Message: "失败: " + err.Error(),
			})
			continue
		}
		merged.Before += result.Before
		merged.After += result.After
		merged.Created += result.Created
		merged.Deleted += result.Deleted
		merged.HealthyNow += result.HealthyNow
		// 各服务日志加前缀
		for _, l := range result.Logs {
			if len(services) > 1 {
				l.Step = svc + "/" + l.Step
			}
			merged.Logs = append(merged.Logs, l)
		}
	}

	c.JSON(http.StatusOK, merged)
}

// SyncCF 手动同步 CF Worker（POST /api/admin/hf/sync-cf?service=xxx）
func (h *HFSpaceHandler) SyncCF(c *gin.Context) {
	svc := c.Query("service")
	if svc == "" {
		// 同步所有服务
		errors := make([]string, 0)
		results := make([]service.SyncCFResult, 0)
		for _, s := range []string{"openai", "grok", "kiro", "gemini", "ts"} {
			r, err := h.svc.SyncCFWorker(s)
			if err != nil {
				errors = append(errors, s+": "+err.Error())
			} else if r != nil {
				results = append(results, *r)
			}
		}
		if len(errors) > 0 {
			c.JSON(http.StatusOK, gin.H{"message": "部分同步失败", "errors": errors, "results": results})
			return
		}
		// CF 同步完成后自动调优调度参数
		h.svc.SyncPoolSize()
		c.JSON(http.StatusOK, gin.H{"message": "所有服务 CF 环境变量已同步，调度参数已自动调优", "results": results})
		return
	}

	r, err := h.svc.SyncCFWorker(svc)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	// CF 同步完成后自动调优调度参数
	h.svc.SyncPoolSize()
	c.JSON(http.StatusOK, gin.H{"message": svc + " CF 环境变量已同步，调度参数已自动调优", "results": []service.SyncCFResult{*r}})
}

// Overview 各服务汇总统计（GET /api/admin/hf/overview）
func (h *HFSpaceHandler) Overview(c *gin.Context) {
	items, err := h.svc.Overview()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

// Discover 自动发现 Space（POST /api/admin/hf/discover?default_service=openai）
// 遍历所有有效 Token，调 HF API 拉取账号下的 Space，自动入库
func (h *HFSpaceHandler) Discover(c *gin.Context) {
	defaultService := c.DefaultQuery("default_service", "")
	result, err := h.svc.DiscoverSpaces(defaultService)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// UpdateSpaces 批量更新已有 Space（POST /api/admin/hf/spaces/update）
// 对所有 Space 推送最新模板文件，触发 HF 重建
func (h *HFSpaceHandler) UpdateSpaces(c *gin.Context) {
	var req dto.UpdateHFSpacesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	result, err := h.svc.UpdateSpaces(req.Service)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// Redetect 对 unknown 的 Space 重新识别服务类型（POST /api/admin/hf/redetect）
func (h *HFSpaceHandler) Redetect(c *gin.Context) {
	result, err := h.svc.RedetectService()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
