package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ── Stripe / OpenAI 常量 ──

const (
	chatgptBaseURL   = "https://chatgpt.com"
	stripeBaseURL    = "https://api.stripe.com/v1"
	defaultPromo     = "team-1-month-free"
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	stripeJSOrigin   = "https://js.stripe.com"
	stripeJSReferer  = "https://js.stripe.com/"
	stripeJSVersion  = "2024-12-18.acacia"
)

// OpenAITeamWorker ChatGPT Team 全协议化开通
type OpenAITeamWorker struct{}

func init() {
	Register(&OpenAITeamWorker{})
}

func (w *OpenAITeamWorker) PlatformName() string { return "openai_team" }

func (w *OpenAITeamWorker) ScanConfig(ctx context.Context, proxy *ProxyEntry, cfg Config) (Config, error) {
	// openai_team 需要 stripe_pk（已内置）和卡池（通过 task engine 注入）
	return Config{}, nil
}

// RegisterOne 执行一次 Team 开通（双模式）
// 模式 1 — 有 session_token: 直接开通（已有账号）
// 模式 2 — 无 session_token: 先注册 OpenAI 账号，再串联开通 Team
// Config 中需包含卡信息：card_number, exp_month, exp_year, cvc, billing_* 等
func (w *OpenAITeamWorker) RegisterOne(ctx context.Context, opts RegisterOpts) {
	jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
	select {
	case <-ctx.Done():
		return
	case <-time.After(jitter):
	}

	logf := func(format string, args ...interface{}) {
		select {
		case opts.LogCh <- fmt.Sprintf(format, args...):
		default:
		}
	}

	cfg := opts.Config
	email := cfg["email"]
	sessionToken := cfg["session_token"]
	accessToken := cfg["access_token"]
	workspaceName := cfg["workspace_name"]
	if workspaceName == "" {
		workspaceName = "My Team"
	}
	seatQty := cfg["seat_quantity"]
	if seatQty == "" {
		seatQty = "2"
	}

	if cfg["card_number"] == "" {
		logf("[!] 缺少卡信息")
		opts.OnFail()
		return
	}

	// ── 模式 2: 先注册 OpenAI 账号 ──
	if sessionToken == "" {
		logf("[~] 未提供 session_token，先注册 OpenAI 账号...")
		openaiW, ok := Get("openai")
		if !ok {
			logf("[!] OpenAI 注册器未找到")
			opts.OnFail()
			return
		}
		regDone := make(chan bool, 1)
		var regEmail string
		var regData map[string]interface{}

		openaiW.RegisterOne(ctx, RegisterOpts{
			Proxy: opts.Proxy, Config: opts.Config, LogCh: opts.LogCh,
			OnSuccess: func(e string, cred map[string]interface{}) {
				regEmail = e
				regData = cred
				regDone <- true
			},
			OnFail: func() { regDone <- false },
		})

		if !<-regDone {
			logf("[!] OpenAI 注册失败，Team 开通中止")
			opts.OnFail()
			return
		}
		email = regEmail
		if st, ok := regData["session_token"].(string); ok && st != "" {
			sessionToken = st
		} else if at, ok := regData["access_token"].(string); ok && at != "" {
			accessToken = at
		} else {
			logf("[!] 注册成功但未获得 session_token 或 access_token，Team 开通中止")
			// 注册本身成功了，上报凭据但标记 team 失败
			opts.OnFail()
			return
		}
		logf("[~] 注册成功: %s，继续 Team 开通...", email)
	}

	logf("[~] 开始 Team 开通: %s → %s (%s 席位)", email, workspaceName, seatQty)

	// 构建 HTTP Client（可选代理）
	client := &http.Client{Timeout: 30 * time.Second}
	if opts.Proxy != nil && opts.Proxy.HTTPS != "" {
		proxyURL, err := url.Parse(opts.Proxy.HTTPS)
		if err == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		}
	}

	// Step 1: session_token → access_token（已有 access_token 则跳过）
	if accessToken == "" {
		if sessionToken == "" {
			logf("[!] 缺少 session_token 和 access_token")
			opts.OnFail()
			return
		}
		logf("[~] Step 1/4: 获取 access_token...")
		var err error
		accessToken, err = w.getAccessToken(ctx, client, sessionToken)
		if err != nil {
			logf("[!] 获取 access_token 失败: %v", err)
			opts.OnFail()
			return
		}
		logf("[~] access_token 获取成功")
	} else {
		logf("[~] Step 1/4: 使用已有 access_token")
	}

	// Step 2: 创建 Checkout Session
	billingCountry := cfg["billing_country"]
	if billingCountry == "" {
		billingCountry = "US"
	}
	logf("[~] Step 2/6: 创建 Checkout...")
	sessionID, pk, err := w.createCheckout(ctx, client, accessToken, workspaceName, seatQty, billingCountry)
	if err != nil {
		logf("[!] 创建 Checkout 失败: %v", err)
		opts.OnFail()
		return
	}
	logf("[~] Checkout: %s (PK: %s...)", sessionID[:24], pk[:20])

	// Step 3: Init Checkout Session
	logf("[~] Step 3/6: Init Session...")
	if err := w.initSession(ctx, client, sessionID, pk); err != nil {
		logf("[!] Init 失败: %v", err)
		opts.OnFail()
		return
	}

	// Step 4: 创建 Stripe PaymentMethod
	logf("[~] Step 4/6: 创建 PaymentMethod...")
	pmID, err := w.createPaymentMethod(ctx, client, cfg, pk)
	if err != nil {
		logf("[!] 创建 PaymentMethod 失败: %v", err)
		opts.OnFail()
		return
	}
	logf("[~] PaymentMethod: %s", pmID)

	// Step 5: 确认支付 + 3DS
	logf("[~] Step 5/6: 确认支付...")
	confirmResult, err := w.confirmPayment(ctx, client, sessionID, pmID, pk)
	if err != nil {
		logf("[!] 支付确认失败: %v", err)
		opts.OnFail()
		return
	}

	// 处理 3DS
	si, _ := confirmResult["setup_intent"].(map[string]interface{})
	if si != nil {
		siStatus, _ := si["status"].(string)
		if siStatus == "requires_action" {
			logf("[~] Step 5/6: 3DS 认证...")
			if err := w.handle3DS(ctx, client, si, pk); err != nil {
				logf("[!] 3DS 失败: %v", err)
				opts.OnFail()
				return
			}
		}
	}

	// Step 6: 验证最终结果
	logf("[~] Step 6/6: 验证结果...")
	time.Sleep(4 * time.Second)
	if err := w.verifyResult(ctx, client, sessionID, pk); err != nil {
		logf("[!] 验证失败: %v", err)
		opts.OnFail()
		return
	}

	logf("[✓] Team 开通成功: %s → %s", email, workspaceName)
	opts.OnSuccess(email, map[string]interface{}{
		"email":          email,
		"session_token":  sessionToken,
		"access_token":   accessToken,
		"workspace_name": workspaceName,
		"seat_quantity":  seatQty,
		"plan_type":      "team",
	})
}

// ── HTTP 步骤 ──

// getAccessToken 用 session_token 换 access_token
func (w *OpenAITeamWorker) getAccessToken(ctx context.Context, client *http.Client, sessionToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", chatgptBaseURL+"/api/auth/session", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept", "*/*")
	req.AddCookie(&http.Cookie{Name: "__Secure-next-auth.session-token", Value: sessionToken})
	req.AddCookie(&http.Cookie{Name: "__Secure-next-auth.callback-url", Value: chatgptBaseURL + "/"})

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("JSON 解析失败: %w", err)
	}
	token, ok := result["accessToken"].(string)
	if !ok || token == "" {
		return "", fmt.Errorf("accessToken 为空")
	}
	return token, nil
}

// createCheckout 创建 Stripe Checkout Session, 返回 (sessionID, publishableKey)
func (w *OpenAITeamWorker) createCheckout(ctx context.Context, client *http.Client, accessToken, workspace, seats, country string) (string, string, error) {
	currency := "USD"
	if isEURCountry(country) {
		currency = "EUR"
	}
	payload := map[string]interface{}{
		"plan_name": "chatgptteamplan",
		"team_plan_data": map[string]interface{}{
			"workspace_name": workspace,
			"price_interval": "month",
			"seat_quantity":  json.Number(seats),
		},
		"billing_details": map[string]interface{}{
			"country":  country,
			"currency": currency,
		},
		"promo_campaign": map[string]interface{}{
			"promo_campaign_id":          defaultPromo,
			"is_coupon_from_query_param": true,
		},
		"checkout_ui_mode": "redirect",
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", chatgptBaseURL+"/backend-api/payments/checkout", strings.NewReader(string(body)))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Origin", chatgptBaseURL)

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 200)]))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", "", err
	}
	sid, _ := result["checkout_session_id"].(string)
	if sid == "" {
		sid, _ = result["session_id"].(string)
	}
	pk, _ := result["publishable_key"].(string)
	if sid == "" {
		return "", "", fmt.Errorf("未获得 session_id, keys=%v", mapKeys(result))
	}
	return sid, pk, nil
}

func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// initSession 初始化 Checkout Session (必须在 PM 创建前调用)
func (w *OpenAITeamWorker) initSession(ctx context.Context, client *http.Client, sessionID, pk string) error {
	initURL := fmt.Sprintf("%s/payment_pages/%s/init", stripeBaseURL, sessionID)
	form := url.Values{"key": {pk}, "browser_locale": {"en-US"}}
	req, err := http.NewRequestWithContext(ctx, "POST", initURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("init HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	return nil
}

// handle3DS 处理 3DS 认证 (frictionless)
func (w *OpenAITeamWorker) handle3DS(ctx context.Context, client *http.Client, si map[string]interface{}, pk string) error {
	na, _ := si["next_action"].(map[string]interface{})
	if na == nil {
		return fmt.Errorf("无 next_action")
	}
	sdk, _ := na["use_stripe_sdk"].(map[string]interface{})
	if sdk == nil {
		return fmt.Errorf("无 use_stripe_sdk")
	}
	src, _ := sdk["three_d_secure_2_source"].(string)
	if src == "" {
		return fmt.Errorf("无 3DS source")
	}
	browserJSON, _ := json.Marshal(map[string]interface{}{
		"fingerprintAttempted":      false,
		"challengeWindowSize":       nil,
		"threeDSCompInd":            "Y",
		"browserJavaEnabled":        false,
		"browserJavascriptEnabled":  true,
		"browserLanguage":           "en-US",
		"browserColorDepth":         "24",
		"browserScreenHeight":       "900",
		"browserScreenWidth":        "1440",
		"browserTZ":                 "-480",
		"browserUserAgent":          defaultUserAgent,
	})
	form := url.Values{
		"source":  {src},
		"browser": {string(browserJSON)},
		"one_click_authn_device_support[hosted]":           {"false"},
		"one_click_authn_device_support[spc_eligible]":     {"false"},
		"one_click_authn_device_support[webauthn_eligible]": {"false"},
		"key": {pk},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", stripeBaseURL+"/3ds2/authenticate", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", stripeJSOrigin)
	req.Header.Set("Referer", stripeJSReferer)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("3DS 响应解析失败: %w", err)
	}
	state, _ := result["state"].(string)
	if state != "succeeded" {
		errMsg, _ := result["error"].(map[string]interface{})
		if errMsg != nil {
			msg, _ := errMsg["message"].(string)
			return fmt.Errorf("3DS state=%s: %s", state, msg)
		}
		return fmt.Errorf("3DS state=%s", state)
	}
	return nil
}

// verifyResult 通过 re-init 验证 SetupIntent 最终状态
func (w *OpenAITeamWorker) verifyResult(ctx context.Context, client *http.Client, sessionID, pk string) error {
	initURL := fmt.Sprintf("%s/payment_pages/%s/init", stripeBaseURL, sessionID)
	form := url.Values{"key": {pk}, "browser_locale": {"en-US"}}
	req, err := http.NewRequestWithContext(ctx, "POST", initURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("verify 解析失败: %w", err)
	}
	si, _ := result["setup_intent"].(map[string]interface{})
	if si == nil {
		return nil // 无 SI 说明可能已完成
	}
	st, _ := si["status"].(string)
	if st == "succeeded" {
		return nil
	}
	lpe, _ := si["last_setup_error"].(map[string]interface{})
	if lpe != nil {
		dc, _ := lpe["decline_code"].(string)
		msg, _ := lpe["message"].(string)
		return fmt.Errorf("card_declined:%s %s", dc, msg)
	}
	return fmt.Errorf("SI status=%s", st)
}

// isEURCountry 判断是否为欧元区国家
func isEURCountry(country string) bool {
	eurCountries := map[string]bool{
		"FR": true, "DE": true, "NL": true, "BE": true, "AT": true,
		"IT": true, "ES": true, "PT": true, "IE": true, "FI": true,
		"GR": true, "LU": true, "SK": true, "SI": true, "EE": true,
		"LV": true, "LT": true, "CY": true, "MT": true,
	}
	return eurCountries[country]
}

// createPaymentMethod 创建 Stripe PaymentMethod（需 Stripe.js Origin + email）
func (w *OpenAITeamWorker) createPaymentMethod(ctx context.Context, client *http.Client, cfg Config, pk string) (string, error) {
	form := url.Values{
		"type":                                  {"card"},
		"card[number]":                          {cfg["card_number"]},
		"card[exp_month]":                       {cfg["exp_month"]},
		"card[exp_year]":                        {cfg["exp_year"]},
		"card[cvc]":                             {cfg["cvc"]},
		"billing_details[name]":                 {cfg["billing_name"]},
		"billing_details[email]":                {cfg["billing_email"]},
		"billing_details[address][country]":     {cfg["billing_country"]},
		"billing_details[address][city]":        {cfg["billing_city"]},
		"billing_details[address][line1]":       {cfg["billing_line1"]},
		"billing_details[address][postal_code]": {cfg["billing_zip"]},
		"key":                                   {pk},
		"payment_user_agent":                    {"stripe.js/3c18316ee2; stripe-js-v3/3c18316ee2; checkout"},
		"pasted_fields":                         {"number"},
		"_stripe_version":                       {stripeJSVersion},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", stripeBaseURL+"/payment_methods", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", stripeJSOrigin)
	req.Header.Set("Referer", stripeJSReferer)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	pmID, _ := result["id"].(string)
	if pmID == "" {
		return "", fmt.Errorf("未获得 payment_method id")
	}
	return pmID, nil
}

// confirmPayment 确认 Stripe 支付, 返回完整响应 (含 setup_intent)
func (w *OpenAITeamWorker) confirmPayment(ctx context.Context, client *http.Client, sessionID, pmID, pk string) (map[string]interface{}, error) {
	confirmURL := fmt.Sprintf("%s/payment_pages/%s/confirm", stripeBaseURL, sessionID)
	form := url.Values{
		"payment_method":               {pmID},
		"expected_amount":              {"0"},
		"expected_payment_method_type": {"card"},
		"consent[terms_of_service]":    {"accepted"},
		"return_url":                   {fmt.Sprintf("https://pay.openai.com/c/pay/%s", sessionID)},
		"key":                          {pk},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", confirmURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", stripeJSOrigin)
	req.Header.Set("Referer", stripeJSReferer)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}
	return result, nil
}
