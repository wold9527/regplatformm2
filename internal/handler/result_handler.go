package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xiaolajiaoyyds/regplatformm/internal/middleware"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"github.com/xiaolajiaoyyds/regplatformm/internal/pkg/tempmail"
	"github.com/xiaolajiaoyyds/regplatformm/internal/service"
	"gorm.io/gorm"
)

// resultItem 结果列表响应项（ListAll / ListArchived 共用）
type resultItem struct {
	ID              uint            `json:"id"`
	Email           string          `json:"email"`
	Platform        string          `json:"platform"`
	CredentialData  json.RawMessage `json:"credential_data"`
	SSOToken        string          `json:"auth_token,omitempty"`
	NsfwEnabled     bool            `json:"feature_enabled"`
	Disabled        bool            `json:"disabled"`
	DisabledReason  string          `json:"disabled_reason,omitempty"`
	DisabledAt      *string         `json:"disabled_at,omitempty"`
	LastValidatedAt *string         `json:"last_validated_at,omitempty"`
	CreatedAt       string          `json:"created_at"`
}

// toResultItems 将 model.TaskResult 切片转换为 API 响应项
func toResultItems(results []model.TaskResult) []resultItem {
	items := make([]resultItem, 0, len(results))
	for _, r := range results {
		item := resultItem{
			ID:             r.ID,
			Email:          r.Email,
			Platform:       r.Platform,
			CredentialData: json.RawMessage(r.CredentialData),
			SSOToken:       r.SSOToken,
			NsfwEnabled:    r.NsfwEnabled,
			Disabled:       r.Disabled,
			DisabledReason: r.DisabledReason,
			CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if r.DisabledAt != nil {
			s := r.DisabledAt.Format("2006-01-02 15:04:05")
			item.DisabledAt = &s
		}
		if r.LastValidatedAt != nil {
			s := r.LastValidatedAt.Format("2006-01-02 15:04:05")
			item.LastValidatedAt = &s
		}
		items = append(items, item)
	}
	return items
}

// ResultHandler 结果处理器
type ResultHandler struct {
	db         *gorm.DB
	settingSvc *service.SettingService
}

// NewResultHandler 创建结果处理器
func NewResultHandler(db *gorm.DB, settingSvc *service.SettingService) *ResultHandler {
	return &ResultHandler{db: db, settingSvc: settingSvc}
}

// GetResults 获取任务结果（GET /api/results/:taskId）
func (h *ResultHandler) GetResults(c *gin.Context) {
	user := middleware.GetUser(c)
	taskID, err := strconv.ParseUint(c.Param("taskId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的任务 ID"})
		return
	}

	var results []model.TaskResult
	if err := h.db.Where("task_id = ? AND user_id = ?", taskID, user.ID).
		Order("created_at DESC").
		Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, results)
}

// Export 导出结果（GET /api/results/:taskId/export）
func (h *ResultHandler) Export(c *gin.Context) {
	user := middleware.GetUser(c)
	taskID, err := strconv.ParseUint(c.Param("taskId"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的任务 ID"})
		return
	}

	var results []model.TaskResult
	if err := h.db.Where("task_id = ? AND user_id = ?", taskID, user.ID).Find(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "查询失败"})
		return
	}

	if len(results) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "没有结果可导出"})
		return
	}

	// 判断平台，选择导出格式
	platform := ""
	if len(results) > 0 {
		platform = results[0].Platform
	}

	var lines []string
	for _, r := range results {
		if platform == "grok" && r.SSOToken != "" {
			lines = append(lines, r.SSOToken)
		} else {
			lines = append(lines, string(r.CredentialData))
		}
	}

	content := strings.Join(lines, "\n")
	// 清理文件名中的特殊字符，防止 Content-Disposition 头注入
	safePlatform := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, platform)
	filename := fmt.Sprintf("task_%d_%s_results.txt", taskID, safePlatform)
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(content))
}

// ListAll 所有未归档结果（GET /api/results?platform=&page=&page_size=）
// 支持分页：page 从 1 开始，page_size 默认 100，page_size=-1 返回全量（向后兼容）
func (h *ResultHandler) ListAll(c *gin.Context) {
	user := middleware.GetUser(c)
	platform := c.Query("platform")

	base := h.db.Where("user_id = ? AND is_archived = ?", user.ID, false)
	if platform != "" {
		base = base.Where("platform = ?", platform)
	}

	// 总数（必须用 Session 克隆，避免 Count 污染后续查询的内部状态）
	var total int64
	base.Session(&gorm.Session{}).Model(&model.TaskResult{}).Count(&total)

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "100"))
	if page < 1 {
		page = 1
	}

	query := base.Order("created_at DESC")
	// page_size=-1 或不传 page 参数时向后兼容，返回全量
	if pageSize > 0 {
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	var results []model.TaskResult
	query.Find(&results)

	c.JSON(http.StatusOK, gin.H{
		"items":     toResultItems(results),
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ArchiveAll 归档结果（POST /api/results/archive）
// 支持 body {"platform":"grok"} 按平台归档，不传则归档全部
// 返回本次归档数量 archived_count
func (h *ResultHandler) ArchiveAll(c *gin.Context) {
	user := middleware.GetUser(c)

	var req struct {
		Platform string `json:"platform"`
	}
	_ = c.ShouldBindJSON(&req)

	query := h.db.Model(&model.TaskResult{}).
		Where("user_id = ? AND is_archived = ?", user.ID, false)
	if req.Platform != "" {
		query = query.Where("platform = ?", req.Platform)
	}
	result := query.Update("is_archived", true)
	c.JSON(http.StatusOK, gin.H{
		"message":        "已归档",
		"archived_count": result.RowsAffected,
	})
}

// ListArchived 已归档结果（GET /api/results/archived?platform=&page=&page_size=&count_only=true）
// count_only=true 时仅返回总数（不查具体数据，极快）
// 支持分页：page 从 1 开始，page_size 默认 50，page_size=-1 返回全量（导出用）
func (h *ResultHandler) ListArchived(c *gin.Context) {
	user := middleware.GetUser(c)
	platform := c.Query("platform")

	base := h.db.Where("user_id = ? AND is_archived = ?", user.ID, true)
	if platform != "" {
		base = base.Where("platform = ?", platform)
	}

	// 总数（必须用 Session 克隆，避免 Count 污染后续查询的内部状态）
	var total int64
	base.Session(&gorm.Session{}).Model(&model.TaskResult{}).Count(&total)

	// count_only 模式：只返回总数
	if c.Query("count_only") == "true" {
		c.JSON(http.StatusOK, gin.H{"total": total})
		return
	}

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}

	query := base.Order("created_at DESC")
	if pageSize > 0 {
		query = query.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	var results []model.TaskResult
	query.Find(&results)

	c.JSON(http.StatusOK, gin.H{
		"items":     toResultItems(results),
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// FetchOTP 获取邮箱验证码（GET /api/email/otp?email=xxx）
// 自动从数据库反查 provider 信息，前端只需传 email
func (h *ResultHandler) FetchOTP(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "缺少 email 参数"})
		return
	}

	// 优先从 query 参数获取 provider（向后兼容），否则从数据库反查
	provider := c.Query("provider")
	meta := map[string]string{"provider": provider}

	if provider == "" {
		// 从数据库查找该邮箱的最新注册结果，提取 email_provider 和 email_meta
		var result model.TaskResult
		if err := h.db.Where("email = ?", email).Order("created_at DESC").First(&result).Error; err == nil {
			var credData map[string]interface{}
			if err := json.Unmarshal(result.CredentialData, &credData); err == nil {
				if ep, ok := credData["mail_provider"].(string); ok && ep != "" {
					provider = ep
					meta["provider"] = provider
				} else if ep, ok := credData["provider"].(string); ok && ep != "" {
					provider = ep
					meta["provider"] = provider
				} else if ep, ok := credData["email_provider"].(string); ok && ep != "" {
					provider = ep
					meta["provider"] = provider
				}
				if em, ok := credData["email_meta"].(map[string]interface{}); ok {
					for k, v := range em {
						if s, ok := v.(string); ok {
							meta[k] = s
						}
					}
				}
			}
		}
		// 数据库也没有，按域名猜测
		if provider == "" {
			provider = detectProviderByDomain(email)
			meta["provider"] = provider
		}
	} else {
		// query 参数模式（向后兼容）：从 URL 读取 meta 字段
		meta["token"] = c.Query("token")
		meta["sid_token"] = c.Query("sid_token")
		meta["account_id"] = c.Query("account_id")
		meta["base_url"] = c.Query("base_url")
		meta["admin_token"] = c.Query("admin_token")
	}

	cfg := map[string]string{
		"yydsmail_base_url": h.settingSvc.Get("yydsmail_base_url", ""),
		"yydsmail_api_key":  h.settingSvc.Get("yydsmail_api_key", ""),
	}

	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无法识别邮箱 provider"})
		return
	}
	if tempmail.NormalizeProviderNameForAPI(provider) == "yydsmail" && cfg["yydsmail_api_key"] == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "YYDS Mail API Key 未配置"})
		return
	}

	// 单次尝试获取验证码（不轮询，前端按需点击重试）
	code, err := tempmail.FetchVerificationCodeByProvider(c.Request.Context(), provider, cfg, email, meta)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": "", "error": "暂无验证码"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": code})
}

// detectProviderByDomain 根据邮箱域名自动识别 provider
func detectProviderByDomain(email string) string {
	return "yydsmail"
}

// ReEnable 恢复被禁用的账号（POST /api/results/:id/re-enable）
func (h *ResultHandler) ReEnable(c *gin.Context) {
	user := middleware.GetUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 ID"})
		return
	}

	result := h.db.Model(&model.TaskResult{}).
		Where("id = ? AND user_id = ? AND disabled = ?", id, user.ID, true).
		Updates(map[string]interface{}{
			"disabled":        false,
			"disabled_reason": "",
			"disabled_at":     nil,
		})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "账号不存在或未被禁用"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "账号已恢复"})
}
