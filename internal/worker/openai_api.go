package worker

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xiaolajiaoyyds/regplatformm/internal/pkg/tempmail"
)

// openaiRegRequest 远程注册请求
type openaiRegRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	Proxy         string `json:"proxy"`
	YYDSMailURL   string `json:"yydsmail_url"`
	YYDSMailKey   string `json:"yydsmail_key"`
	EmailPriority string `json:"email_priority"` // 邮箱 Provider 优先级（如 "yydsmail"）
}

// OpenAIProtocolRegisterHandler 纯 HTTP 协议注册端点
// POST /api/v1/process
// 供远程调用：日本后端 → CF Workers → HF Space
func OpenAIProtocolRegisterHandler(c *gin.Context) {
	var req openaiRegRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "invalid request: " + err.Error()})
		return
	}

	// 构建 multi-provider 配置
	priority := req.EmailPriority
	if priority == "" {
		priority = "yydsmail"
	}
	cfg := map[string]string{
		"yydsmail_api_key":         req.YYDSMailKey,
		"yydsmail_base_url":        req.YYDSMailURL,
		"email_provider_priority":  priority,
	}
	mailProvider := tempmail.NewMultiProvider(cfg)

	// 日志收集（在邮箱创建之前定义，确保全流程都有日志）
	logs := make([]string, 0, 20)
	logf := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		logs = append(logs, msg)
		fmt.Printf("[LOG] %s\n", msg) // 同时打印到 stdout，方便调试
	}

	// 诊断日志：记录收到的配置
	logf("[*] 邮箱优先级: %s", priority)
	creds := ""
	if req.YYDSMailKey != "" {
		creds += "yydsmail "
	}
	if creds == "" {
		creds = "(未配置凭证)"
	}
	logf("[*] 可用凭证: %s", creds)
	if req.Proxy != "" {
		logf("[*] 代理: %s", req.Proxy)
	}

	// 创建邮箱（如果未提供）
	email := req.Email
	password := req.Password
	var mailMeta map[string]string

	if email == "" {
		var err error
		email, mailMeta, err = mailProvider.GenerateEmail(c.Request.Context())
		if err != nil {
			logf("[-] 创建邮箱失败: %s", err)
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "initialization failed: " + err.Error(), "logs": logs})
			return
		}
		logf("[+] 邮箱创建成功: %s (via %s)", email, mailMeta["provider"])
	} else {
		// 后端已生成邮箱，使用 yydsmail 轮询验证码
		mailMeta = map[string]string{"provider": "yydsmail"}
		logf("[*] 使用后端提供的邮箱: %s", email)
	}
	if password == "" {
		password = genOpenAIPassword(16)
	}

	// 排队等待信息（由 ConcurrencyLimiter 中间件注入 gin context）
	if waited, ok := c.Get("queue_waited"); ok && waited.(bool) {
		pos, _ := c.Get("queue_position")
		waitSec, _ := c.Get("queue_wait_seconds")
		logf("[*] 排队等待完成: 位置 #%d, 等待 %d 秒", pos, waitSec)
	}

	// 构建代理
	var proxy *ProxyEntry
	if req.Proxy != "" {
		proxy = &ProxyEntry{HTTPS: req.Proxy, HTTP: req.Proxy}
	}

	// 执行注册（最长 5 分钟）
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	var reg *openaiRegistrar
	firstName, lastName := randomOpenAIName()
	birthdate := randomBirthdate()

	// step0 + step2 带重试（最多 3 次），每次重建 registrar 避免脏 session 状态
	var step0_2OK bool
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			logf("[*] step0+step2 重试 (%d/3)...", attempt+1)
			ctxSleep(ctx, time.Duration(attempt)*time.Second)
		}

		reg = newOpenAIRegistrar(proxy, logf)
		logf("[*] 预热中...")
		reg.doRequest(ctx, "GET", "https://chatgpt.com/", nil, reg.navHeaders()) //nolint:errcheck
		ctxSleep(ctx, 300*time.Millisecond)

		logf("[*] step0: 初始化注册会话...")
		if err := reg.step0(ctx, email); err != nil {
			logf("[-] step0 失败: %s", err)
			continue
		}
		logf("[+] step0 完成")
		ctxSleep(ctx, time.Second)

		logf("[*] step2: 提交邮箱和密码...")
		if err := reg.step2(ctx, email, password); err != nil {
			logf("[-] step2 失败: %s", err)
			continue
		}
		logf("[+] step2 完成")
		ctxSleep(ctx, time.Second)

		step0_2OK = true
		break
	}
	if !step0_2OK {
		logf("[-] step0+step2 重试 3 次仍失败")
		go mailProvider.DeleteEmail(context.Background(), email, mailMeta)
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "session init failed after retries", "email": email, "logs": logs})
		return
	}

	logf("[*] step3: 请求发送验证码...")
	if err := reg.step3(ctx); err != nil {
		logf("[!] step3 发送验证码可能失败: %s（继续等待，可能仍会收到）", err)
	}

	// 等待验证码（45s 超时，超时后尝试换邮箱内联重试一次，与本地协议逻辑对齐）
	logf("[*] 等待验证码（最长 45s）...")
	code := pollMailCode(ctx, mailProvider, email, mailMeta, 45*time.Second, logf)
	if code == "" {
		logf("[-] 验证码超时，尝试换邮箱重试...")
		tempmail.RecordFailure(mailMeta["provider"])
		go mailProvider.DeleteEmail(context.Background(), email, mailMeta)

		// 内联重试：用同一个 MultiProvider 换邮箱（会自动降级到下一个 provider）
		email2, mailMeta2, err2 := mailProvider.GenerateEmail(ctx)
		if err2 != nil {
			logf("[-] 换邮箱失败: %s", err2)
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "verification timeout", "email": email, "logs": logs})
			return
		}
		logf("[*] 换邮箱重试: %s (via %s)", email2, mailMeta2["provider"])

		// 用新邮箱重走 step0-3
		reg2 := newOpenAIRegistrar(proxy, logf)
		logf("[*] 预热中...")
		reg2.doRequest(ctx, "GET", "https://chatgpt.com/", nil, reg2.navHeaders()) //nolint:errcheck
		ctxSleep(ctx, 300*time.Millisecond)

		if err := reg2.step0(ctx, email2); err != nil {
			logf("[-] 重试 step0: %s", err)
			go mailProvider.DeleteEmail(context.Background(), email2, mailMeta2)
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "retry init failed: " + err.Error(), "email": email, "logs": logs})
			return
		}
		if err := reg2.step2(ctx, email2, password); err != nil {
			logf("[-] 重试 step2: %s", err)
			go mailProvider.DeleteEmail(context.Background(), email2, mailMeta2)
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "retry verify failed: " + err.Error(), "email": email, "logs": logs})
			return
		}
		if err := reg2.step3(ctx); err != nil {
			logf("[!] 重试 step3 发送验证码可能失败: %s（继续等待）", err)
		}

		code = pollMailCode(ctx, mailProvider, email2, mailMeta2, 45*time.Second, logf)
		if code == "" {
			logf("[-] 换邮箱后验证码仍超时")
			tempmail.RecordFailure(mailMeta2["provider"])
			go mailProvider.DeleteEmail(context.Background(), email2, mailMeta2)
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "verification timeout after retry", "email": email2, "logs": logs})
			return
		}
		tempmail.RecordSuccess(mailMeta2["provider"])
		// 切换到新邮箱继续后续流程
		email = email2
		mailMeta = mailMeta2
		reg = reg2
		logf("[*] 验证码: %s", code)
	} else {
		tempmail.RecordSuccess(mailMeta["provider"])
		logf("[*] 验证码: %s", code)
	}

	// 步骤4（失败时重发验证码重试一次，与本地协议逻辑对齐）
	logf("[*] step4: 提交验证码...")
	if err := reg.step4(ctx, code); err != nil {
		logf("[-] %s，重发验证码重试...", err)
		if err := reg.step3(ctx); err != nil {
			logf("[!] 重发 step3 可能失败: %s（继续等待）", err)
		}
		retryCode := pollMailCode(ctx, mailProvider, email, mailMeta, 60*time.Second, logf)
		if retryCode == "" || retryCode == code {
			logf("[-] 重试验证码超时或未刷新")
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": "verification retry failed", "email": email, "logs": logs})
			go mailProvider.DeleteEmail(context.Background(), email, mailMeta)
			return
		}
		logf("[*] 重试验证码: %s", retryCode)
		if err2 := reg.step4(ctx, retryCode); err2 != nil {
			logf("[-] %s", err2)
			c.JSON(http.StatusOK, gin.H{"ok": false, "error": err2.Error(), "email": email, "logs": logs})
			go mailProvider.DeleteEmail(context.Background(), email, mailMeta)
			return
		}
	}
	logf("[+] step4 完成")
	ctxSleep(ctx, time.Second)

	logf("[*] step5: 填写个人信息...")
	step5Failed := false
	if err := reg.step5(ctx, firstName, lastName, birthdate); err != nil {
		// step5 只是填姓名生日，此时账号已注册成功（step4 验证码已通过），不删邮箱
		logf("[!] step5 失败: %s（账号已创建，继续尝试获取 token）", err)
		step5Failed = true
	}

	if step5Failed {
		logf("[+] 注册部分成功（step5 未完成）: %s", email)
	} else {
		logf("[+] 注册成功: %s", email)
	}

	// ── OAuth Token 获取（多级降级 + 重试，与 registerViaProtocol 逻辑对齐） ──
	logf("[*] 获取 Token...")

	result := gin.H{
		"ok":         true,
		"email":      email,
		"password":   password,
		"first_name": firstName,
		"last_name":  lastName,
		"birthdate":  birthdate,
	}
	tokenOK := false

	// 第一级：从注册 session 的 consent 流程获取 code（step5 失败则跳过）
	// 注意：OAuth 重试详情仅输出到 stdout 调试，不推送给前端用户
	if !step5Failed {
		// 等待 3-6s（随机抖动），让 OpenAI 后端状态充分传播
		jitter := time.Duration(3000+rand.Intn(3000)) * time.Millisecond
		ctxSleep(ctx, jitter)
		authCode, err := reg.step6(ctx)
		if err == nil {
			codex, tokenErr := reg.step7(ctx, authCode)
			if tokenErr == nil {
				logf("[+] Token 获取成功")
				for k, v := range codex {
					if k != "logs" {
						result[k] = v
					}
				}
				tokenOK = true
			} else {
				fmt.Printf("[DEBUG] Token 交换失败: %s\n", tokenErr)
			}
		} else {
			fmt.Printf("[DEBUG] consent 流程失败: %s\n", err)
		}
	} else {
		fmt.Printf("[DEBUG] step5 失败，跳过 consent，直接尝试 OAuth 重新登录\n")
	}

	// 第二级：全流程 OAuth 重新登录（最多重试 3 次，递增延迟 + 随机抖动避免并发踩踏 429）
	if !tokenOK {
		oauthBaseDelays := []time.Duration{5 * time.Second, 12 * time.Second, 25 * time.Second}
		for attempt, base := range oauthBaseDelays {
			if ctx.Err() != nil {
				break
			}
			jitter := time.Duration(rand.Intn(3000)) * time.Millisecond
			delay := base + jitter
			fmt.Printf("[DEBUG] OAuth 重新登录第 %d 次尝试，等待 %s...\n", attempt+1, delay)
			ctxSleep(ctx, delay)
			if ctx.Err() != nil {
				break
			}
			codex, loginErr := reg.oauthLogin(ctx, email, password, mailProvider, mailMeta)
			if loginErr == nil {
				logf("[+] Token 获取成功")
				for k, v := range codex {
					if k != "logs" {
						result[k] = v
					}
				}
				tokenOK = true
				break
			}
			fmt.Printf("[DEBUG] OAuth 重新登录第 %d 次失败: %s\n", attempt+1, loginErr)
		}
	}

	// Token 获取全部失败：返回失败，不保存不完整凭证
	if !tokenOK {
		fmt.Printf("[DEBUG] %s Token 获取失败，标记为失败\n", email)
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "token acquisition failed", "email": email, "logs": logs})
		return
	}

	// 注入邮箱元数据，供后续 OTP 查询使用
	result["mail_provider"] = mailMeta["provider"]
	emailMetaCopy := make(map[string]interface{}, len(mailMeta))
	for k, v := range mailMeta {
		emailMetaCopy[k] = v
	}
	result["email_meta"] = emailMetaCopy

	result["logs"] = logs
	c.JSON(http.StatusOK, result)
}
