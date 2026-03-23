package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"github.com/xiaolajiaoyyds/regplatformm/internal/service"
	"gorm.io/gorm"
)

// geminiValidateMu 每用户验活并发锁，防止同一用户同时触发多个验活流
var geminiValidateMu sync.Map // key: userID (uint), value: *sync.Mutex

// GeminiValidateHandler Gemini Business 账号验活处理器
type GeminiValidateHandler struct {
	db        *gorm.DB
	authSvc   *service.AuthService
	proxyPool *service.ProxyPool
}

// NewGeminiValidateHandler 创建 Gemini 验活处理器
func NewGeminiValidateHandler(db *gorm.DB, authSvc *service.AuthService, proxyPool *service.ProxyPool) *GeminiValidateHandler {
	return &GeminiValidateHandler{db: db, authSvc: authSvc, proxyPool: proxyPool}
}

// ── Gemini Business 验活常量 ──────────────────────────────────────────────────

const (
	// geminiBusinessBase Gemini Business 的基础 URL
	geminiBusinessBase = "https://business.gemini.google"
)

// validateGeminiCredential 验证单个 Gemini Business 凭证
// 通过带 Cookie 请求 Gemini Business 页面，根据响应状态判断账号是否有效
// 返回 status: "valid" | "expired" | "invalid" | "error"
func validateGeminiCredential(ctx context.Context, proxyURL, cSes, cOses, csesidx, configID string) (string, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "error", fmt.Errorf("创建 CookieJar 失败: %w", err)
	}
	baseURL, _ := url.Parse(geminiBusinessBase)

	// 设置 Gemini Business 会话 Cookies
	cookies := []*http.Cookie{
		{Name: "C_SES", Value: cSes, Domain: ".gemini.google", Path: "/", Secure: true, HttpOnly: true},
	}
	if cOses != "" {
		cookies = append(cookies, &http.Cookie{
			Name: "C_OSES", Value: cOses, Domain: ".gemini.google", Path: "/", Secure: true, HttpOnly: true,
		})
	}
	jar.SetCookies(baseURL, cookies)

	// 构建验证 URL
	checkURL := geminiBusinessBase + "/"
	if csesidx != "" {
		checkURL += "?csesidx=" + url.QueryEscape(csesidx)
		if configID != "" {
			checkURL += "&cid=" + url.QueryEscape(configID)
		}
	}

	// 不跟随重定向，手动检测 Location
	// 构建 Transport（支持代理）
	var transport http.RoundTripper
	if proxyURL != "" {
		proxyClient, proxyErr := service.BuildProxyHTTPClient(proxyURL, 15*time.Second)
		if proxyErr == nil && proxyClient.Transport != nil {
			transport = proxyClient.Transport
		}
	}
	client := &http.Client{
		Timeout:   15 * time.Second,
		Jar:       jar,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 不自动重定向
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", checkURL, nil)
	if err != nil {
		return "error", fmt.Errorf("构建验证请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "error", fmt.Errorf("验证请求网络错误: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "error", fmt.Errorf("读取验证响应失败: %w", err)
	}
	bodyStr := string(bodyBytes)

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		// 429 速率限制 → 返回 error 而非 invalid，避免误删有效账号
		return "error", fmt.Errorf("Google 速率限制 HTTP 429")

	case resp.StatusCode == http.StatusOK:
		// 200 但可能是登录页面（检查页面内容）
		if strings.Contains(bodyStr, "accounts.google.com/signin") ||
			strings.Contains(bodyStr, "accounts.google.com/ServiceLogin") {
			return "expired", nil
		}
		return "valid", nil

	case resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently ||
		resp.StatusCode == http.StatusTemporaryRedirect:
		// 重定向到登录页 → 会话过期
		location := resp.Header.Get("Location")
		if strings.Contains(location, "accounts.google.com") ||
			strings.Contains(location, "auth.") ||
			strings.Contains(location, "login") ||
			strings.Contains(location, "signin") {
			return "expired", nil
		}
		// 重定向到其他 Gemini 页面（可能正常跳转）
		return "valid", nil

	case resp.StatusCode == http.StatusForbidden:
		// 403 → 账号被封禁或无权限
		if strings.Contains(bodyStr, "suspended") || strings.Contains(bodyStr, "banned") {
			return "suspended", nil
		}
		return "invalid", nil

	case resp.StatusCode == http.StatusUnauthorized:
		return "expired", nil

	case resp.StatusCode >= 500:
		return "error", fmt.Errorf("Gemini 服务端错误 HTTP %d", resp.StatusCode)

	default:
		return "invalid", fmt.Errorf("未知响应 HTTP %d: %s", resp.StatusCode, truncateStr(bodyStr, 120))
	}
}

// SSEValidateGemini 验活 Gemini 账号 SSE 端点（GET /ws/gemini/validate?token=&action=archive|export_archive|validate&scope=active|archived）
func (h *GeminiValidateHandler) SSEValidateGemini(c *gin.Context) {
	// 1. JWT 鉴权
	token := c.Query("token")
	userID, err := h.authSvc.VerifyJWT(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "未登录"})
		return
	}

	// 2. 每用户并发锁——同一用户只能同时运行一个验活流
	userMu, _ := geminiValidateMu.LoadOrStore(userID, &sync.Mutex{})
	mu := userMu.(*sync.Mutex)
	if !mu.TryLock() {
		c.JSON(http.StatusConflict, gin.H{"detail": "已有验活任务进行中，请等待完成"})
		return
	}
	defer mu.Unlock()

	// 3. action 白名单校验
	action := c.DefaultQuery("action", "archive")
	if action != "validate" && action != "archive" && action != "export_archive" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 action 参数"})
		return
	}

	// 4. scope 参数
	scope := c.DefaultQuery("scope", "active")
	if scope != "active" && scope != "archived" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 scope 参数"})
		return
	}
	isArchived := scope == "archived"

	// 5. 查询当前用户的 Gemini 账号（排除已禁用的）
	var results []model.TaskResult
	if err := h.db.Where("user_id = ? AND platform = ? AND is_archived = ? AND disabled = ?", userID, "gemini", isArchived, false).
		Order("created_at ASC").
		Find(&results).Error; err != nil {
		log.Error().Err(err).Uint("user_id", userID).Msg("Gemini 验活查询账号失败")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "查询账号失败"})
		return
	}

	if len(results) == 0 {
		c.JSON(http.StatusOK, gin.H{"detail": "没有 Gemini 账号需要验证"})
		return
	}

	// 6. SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ctx := c.Request.Context()
	total := len(results)
	var validCount, invalidCount, errorCount int
	var validResults []model.TaskResult

	// 构建验活用代理 URL（每批在循环内轮换代理）
	// 此处仅做初始化检查，实际代理在每批循环中获取

	sseWrite(c, gin.H{"type": "log", "message": fmt.Sprintf("[*] 开始验证 %d 个 Gemini 账号...", total)})
	sseWrite(c, gin.H{"type": "progress", "validated": 0, "total": total, "valid": 0, "invalid": 0})
	c.Writer.Flush()

	// 7. 分批并发验证（每批 5 个，批次间 500ms，每批轮换代理）
	const batchSize = 5
	for i := 0; i < total; i += batchSize {
		select {
		case <-ctx.Done():
			return
		default:
		}

		end := i + batchSize
		if end > total {
			end = total
		}
		batch := results[i:end]

		// 每批轮换代理，避免所有请求走同一个出口
		var batchProxyURL string
		if p := h.proxyPool.GetNext(); p != nil {
			batchProxyURL = p.HTTP
		}

		type batchResult struct {
			idx    int
			result model.TaskResult
			status string
		}
		var mu sync.Mutex
		var batchResults []batchResult
		var wg sync.WaitGroup

		for j, r := range batch {
			wg.Add(1)
			go func(idx int, r model.TaskResult) {
				defer wg.Done()

				// 解析 credential_data 提取 c_ses/c_oses/csesidx/config_id
				var cred map[string]interface{}
				if err := json.Unmarshal(r.CredentialData, &cred); err != nil {
					mu.Lock()
					batchResults = append(batchResults, batchResult{idx: idx, result: r, status: "error"})
					mu.Unlock()
					return
				}

				cSes, _ := cred["c_ses"].(string)
				cOses, _ := cred["c_oses"].(string)
				csesidx, _ := cred["csesidx"].(string)
				configID, _ := cred["config_id"].(string)

				if cSes == "" {
					mu.Lock()
					batchResults = append(batchResults, batchResult{idx: idx, result: r, status: "invalid"})
					mu.Unlock()
					return
				}

				status, validateErr := validateGeminiCredential(ctx, batchProxyURL, cSes, cOses, csesidx, configID)
				if validateErr != nil {
					log.Warn().Err(validateErr).Str("email", r.Email).Str("status", status).Msg("Gemini 验活异常")
				}

				mu.Lock()
				batchResults = append(batchResults, batchResult{idx: idx, result: r, status: status})
				mu.Unlock()
			}(j, r)
		}
		wg.Wait()

		// 处理本批结果
		now := time.Now()
		for _, br := range batchResults {
			switch br.status {
			case "valid":
				validCount++
				validResults = append(validResults, br.result)
				// 更新 last_validated_at
				if err := h.db.Model(&model.TaskResult{}).Where("id = ? AND user_id = ?", br.result.ID, userID).
					Update("last_validated_at", now).Error; err != nil {
					log.Warn().Err(err).Uint("id", br.result.ID).Msg("更新 last_validated_at 失败")
				}
				sseWrite(c, gin.H{"type": "log", "message": fmt.Sprintf("[+] %s → 正常可用", br.result.Email)})
			case "suspended":
				invalidCount++
				if err := h.db.Model(&model.TaskResult{}).Where("id = ? AND user_id = ?", br.result.ID, userID).
					Updates(map[string]interface{}{"disabled": true, "disabled_reason": "已封禁", "disabled_at": now}).Error; err != nil {
					log.Warn().Err(err).Uint("id", br.result.ID).Msg("禁用封禁账号失败")
				}
				sseWrite(c, gin.H{"type": "log", "message": fmt.Sprintf("[-] %s → 已封禁，已禁用", br.result.Email)})
			case "expired":
				invalidCount++
				if err := h.db.Model(&model.TaskResult{}).Where("id = ? AND user_id = ?", br.result.ID, userID).
					Updates(map[string]interface{}{"disabled": true, "disabled_reason": "Cookie 过期", "disabled_at": now}).Error; err != nil {
					log.Warn().Err(err).Uint("id", br.result.ID).Msg("禁用过期账号失败")
				}
				sseWrite(c, gin.H{"type": "log", "message": fmt.Sprintf("[-] %s → Cookie 过期，已禁用", br.result.Email)})
			case "invalid":
				invalidCount++
				if err := h.db.Model(&model.TaskResult{}).Where("id = ? AND user_id = ?", br.result.ID, userID).
					Updates(map[string]interface{}{"disabled": true, "disabled_reason": "账号无效", "disabled_at": now}).Error; err != nil {
					log.Warn().Err(err).Uint("id", br.result.ID).Msg("禁用无效账号失败")
				}
				sseWrite(c, gin.H{"type": "log", "message": fmt.Sprintf("[-] %s → 账号无效，已禁用", br.result.Email)})
			case "error":
				errorCount++
				sseWrite(c, gin.H{"type": "log", "message": fmt.Sprintf("[!] %s → 检测异常(跳过)，可能是 Google 临时故障", br.result.Email)})
			}
		}

		validated := validCount + invalidCount + errorCount
		sseWrite(c, gin.H{"type": "progress", "validated": validated, "total": total, "valid": validCount, "invalid": invalidCount})
		c.Writer.Flush()

		if end < total {
			time.Sleep(500 * time.Millisecond)
		}
	}

	// 8. 归档有效账号（仅 scope=active 且 action=archive/export_archive 时）
	archivedCount := 0
	if !isArchived && action != "validate" && len(validResults) > 0 {
		var validIDs []uint
		for _, r := range validResults {
			validIDs = append(validIDs, r.ID)
		}
		if err := h.db.Model(&model.TaskResult{}).
			Where("id IN ? AND user_id = ?", validIDs, userID).
			Update("is_archived", true).Error; err != nil {
			log.Error().Err(err).Msg("Gemini 验活归档更新失败")
		} else {
			archivedCount = len(validResults)
		}
	}

	// 9. complete 事件
	completeData := gin.H{
		"type":     "complete",
		"total":    total,
		"valid":    validCount,
		"invalid":  invalidCount,
		"error":    errorCount,
		"archived": archivedCount,
	}

	if (action == "export_archive" || isArchived) && len(validResults) > 0 {
		type exportItem struct {
			ID             uint            `json:"id"`
			Email          string          `json:"email"`
			CredentialData json.RawMessage `json:"credential_data"`
		}
		creds := make([]exportItem, 0, len(validResults))
		for _, r := range validResults {
			creds = append(creds, exportItem{
				ID:             r.ID,
				Email:          r.Email,
				CredentialData: json.RawMessage(r.CredentialData),
			})
		}
		completeData["credentials"] = creds
	}

	logMsg := fmt.Sprintf(
		"[+] 验证完成: 共 %d 个，正常 %d 个，失效 %d 个已禁用，异常跳过 %d 个",
		total, validCount, invalidCount, errorCount,
	)
	if archivedCount > 0 {
		logMsg += fmt.Sprintf("，已归档 %d 个", archivedCount)
	}
	sseWrite(c, gin.H{"type": "log", "message": logMsg})
	sseWrite(c, completeData)
	c.Writer.Flush()
}
