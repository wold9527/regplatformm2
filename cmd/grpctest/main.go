package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"

	"github.com/xiaolajiaoyyds/regplatformm/internal/pkg/gptmail"
	"github.com/xiaolajiaoyyds/regplatformm/internal/pkg/grpcweb"
)

// 独立测试脚本：验证 gRPC-web 注册流程是否正确
func main() {
	// 配置（从环境变量或硬编码）
	gptmailURL := envOrDefault("GPTMAIL_URL", "https://mail.chatgpt.org.uk")
	gptmailKey := os.Getenv("GPTMAIL_KEY")

	fmt.Println("=== gRPC-web 注册流程测试 ===")
	fmt.Printf("gptmail URL: %s\n", gptmailURL)

	// 创建 HTTP 客户端
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	// Phase 0: 会话预热
	fmt.Println("\n[Phase 0] 会话预热...")
	warmup, _ := http.NewRequest("GET", "https://accounts.x.ai", nil)
	warmup.Header.Set("User-Agent", ua)
	if resp, err := client.Do(warmup); err == nil {
		fmt.Printf("  预热响应: %d\n", resp.StatusCode)
		resp.Body.Close()
	} else {
		fmt.Printf("  预热失败: %s\n", err)
	}

	// Phase 1: 创建邮箱
	fmt.Println("\n[Phase 1] 创建临时邮箱...")
	mailClient := gptmail.NewClient(gptmailURL, gptmailKey)
	email, err := mailClient.GenerateEmail()
	if err != nil {
		fmt.Printf("  创建邮箱失败: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("  邮箱: %s\n", email)
	defer func() { go mailClient.DeleteEmail(email) }()

	password := "Test12345abcde"
	firstName := "Testfn"
	lastName := "Testln"

	// Phase 2: 发送验证码
	fmt.Println("\n[Phase 2] 发送验证码 (gRPC-web)...")
	codeBody := grpcweb.EncodeEmailCode(email)
	fmt.Printf("  请求体长度: %d bytes\n", len(codeBody))
	fmt.Printf("  请求体 hex: %x\n", codeBody)

	sendResp, err := doGRPCWeb(client, ua,
		"https://accounts.x.ai/auth_mgmt.AuthManagement/CreateEmailValidationCode",
		codeBody)
	if err != nil {
		fmt.Printf("  发送失败: %s\n", err)
		os.Exit(1)
	}
	sendBody, _ := io.ReadAll(sendResp.Body)
	sendResp.Body.Close()
	fmt.Printf("  响应: HTTP %d, body_len=%d\n", sendResp.StatusCode, len(sendBody))
	fmt.Printf("  grpc-status: %s\n", sendResp.Header.Get("Grpc-Status"))
	fmt.Printf("  grpc-message: %s\n", sendResp.Header.Get("Grpc-Message"))
	if sendResp.StatusCode != 200 {
		fmt.Printf("  响应体: %s\n", string(sendBody))
		os.Exit(1)
	}
	fmt.Println("  验证码发送成功!")

	// Phase 3: 获取验证码
	fmt.Println("\n[Phase 3] 等待验证码...")
	code, err := mailClient.FetchVerificationCode(email, 30, 1*time.Second)
	if err != nil {
		fmt.Printf("  获取验证码失败: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("  验证码: %s\n", code)

	// Phase 4: 验证邮箱
	fmt.Println("\n[Phase 4] 验证邮箱 (gRPC-web)...")
	verifyBody := grpcweb.EncodeVerifyCode(email, code)
	verifyResp, err := doGRPCWeb(client, ua,
		"https://accounts.x.ai/auth_mgmt.AuthManagement/VerifyEmailValidationCode",
		verifyBody)
	if err != nil {
		fmt.Printf("  验证失败: %s\n", err)
		os.Exit(1)
	}
	verifyRespBody, _ := io.ReadAll(verifyResp.Body)
	verifyResp.Body.Close()
	fmt.Printf("  响应: HTTP %d, body_len=%d\n", verifyResp.StatusCode, len(verifyRespBody))
	fmt.Printf("  grpc-status: %s\n", verifyResp.Header.Get("Grpc-Status"))
	if verifyResp.StatusCode != 200 {
		fmt.Printf("  响应体: %s\n", string(verifyRespBody))
		os.Exit(1)
	}
	fmt.Println("  邮箱验证成功!")

	// Phase 5: 注册 (gRPC-web)
	fmt.Println("\n[Phase 5] 注册账号 (gRPC-web)...")

	// Pass 1: 无 Turnstile token
	regBody := grpcweb.EncodeCreateUserAndSession(email, firstName, lastName, password, code, "")
	fmt.Printf("  请求体长度: %d bytes (无 turnstile token)\n", len(regBody))

	for _, endpoint := range []string{
		"https://accounts.x.ai/auth_mgmt.AuthManagement/CreateUserAndSessionV2",
		"https://accounts.x.ai/auth_mgmt.AuthManagement/CreateUserAndSession",
	} {
		isV2 := strings.Contains(endpoint, "V2")
		fmt.Printf("\n  尝试 %s (V2=%v)...\n", endpoint[strings.LastIndex(endpoint, "/")+1:], isV2)

		regResp, err := doGRPCWeb(client, ua, endpoint, regBody)
		if err != nil {
			fmt.Printf("  请求失败: %s\n", err)
			continue
		}
		respBody, _ := io.ReadAll(regResp.Body)
		regResp.Body.Close()

		fmt.Printf("  响应: HTTP %d, body_len=%d\n", regResp.StatusCode, len(respBody))
		fmt.Printf("  grpc-status: %s\n", regResp.Header.Get("Grpc-Status"))
		fmt.Printf("  grpc-message: %s\n", regResp.Header.Get("Grpc-Message"))
		fmt.Printf("  响应体 hex: %x\n", respBody)

		// 打印所有 response headers
		fmt.Println("  --- Response Headers ---")
		for k, v := range regResp.Header {
			fmt.Printf("    %s: %s\n", k, strings.Join(v, ", "))
		}

		// 打印 Set-Cookie
		for _, c := range regResp.Cookies() {
			fmt.Printf("  Set-Cookie: %s=%s\n", c.Name, c.Value[:min(30, len(c.Value))])
		}

		grpcStatus := regResp.Header.Get("Grpc-Status")
		if regResp.StatusCode != 200 || (grpcStatus != "" && grpcStatus != "0") {
			fmt.Printf("  该端点注册失败\n")
			continue
		}

		// 尝试解析 SSO token
		var cookie string
		if isV2 {
			cookie, err = grpcweb.DecodeSessionCookie(respBody)
		} else {
			cookie, err = grpcweb.DecodeSessionCookieV1(respBody)
		}
		if err != nil {
			fmt.Printf("  解析 protobuf 响应失败: %s\n", err)
			// 检查 HTTP cookies
			for _, c := range regResp.Cookies() {
				if c.Name == "sso" {
					cookie = c.Value
					fmt.Printf("  从 Set-Cookie 获取到 sso token\n")
					break
				}
			}
		}

		if cookie != "" {
			fmt.Printf("\n  === 注册成功! ===\n")
			fmt.Printf("  SSO Token: %s...\n", cookie[:min(30, len(cookie))])
			fmt.Printf("  Token 长度: %d\n", len(cookie))

			// Phase 6: TOS
			fmt.Println("\n[Phase 6] 接受 TOS...")
			tosBody := grpcweb.EncodeTosAccepted()
			tosResp, err := doGRPCWebWithCookies(client, ua,
				"https://accounts.x.ai/auth_mgmt.AuthManagement/SetTosAcceptedVersion",
				"https://accounts.x.ai", "https://accounts.x.ai/accept-tos",
				tosBody, map[string]string{"sso": cookie, "sso-rw": cookie})
			if err != nil {
				fmt.Printf("  TOS 失败: %s\n", err)
			} else {
				tosRespBody, _ := io.ReadAll(tosResp.Body)
				tosResp.Body.Close()
				fmt.Printf("  TOS 响应: HTTP %d, grpc-status=%s, body_len=%d\n",
					tosResp.StatusCode, tosResp.Header.Get("Grpc-Status"), len(tosRespBody))
			}

			fmt.Printf("\n=== 测试完成 ===\n")
			fmt.Printf("Email: %s\n", email)
			fmt.Printf("Password: %s\n", password)
			fmt.Printf("SSO Token: %s\n", cookie)
			os.Exit(0)
		}
	}

	fmt.Println("\n[-] 所有端点均失败")
	os.Exit(1)
}

func doGRPCWeb(client *http.Client, ua, endpoint string, body []byte) (*http.Response, error) {
	return doGRPCWebWithCookies(client, ua, endpoint,
		"https://accounts.x.ai", "https://accounts.x.ai/sign-up?redirect=grok-com",
		body, nil)
}

func doGRPCWebWithCookies(client *http.Client, ua, endpoint, origin, referer string, body []byte, cookies map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/grpc-web+proto")
	req.Header.Set("X-Grpc-Web", "1")
	req.Header.Set("X-User-Agent", "connect-es/2.1.1")
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", referer)
	req.Header.Set("User-Agent", ua)

	if len(cookies) > 0 {
		var parts []string
		for k, v := range cookies {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
		req.Header.Set("Cookie", strings.Join(parts, "; "))
	}

	return client.Do(req)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
