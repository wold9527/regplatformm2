// Package netutil 提供网络安全工具函数（SSRF 防护等）
package netutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// allowedSchemes 订阅/代理允许的 URL 协议白名单
var allowedSchemes = map[string]bool{
	"http":  true,
	"https": true,
}

// IsPrivateHost 检查主机名是否解析到私网/保留地址（SSRF 防护）
// 采用 fail-closed 策略：DNS 解析失败时返回 true（视为私网）
func IsPrivateHost(host string) bool {
	// 先尝试直接解析为 IP
	if ip := net.ParseIP(host); ip != nil {
		return isPrivateIP(ip)
	}

	// 域名解析（fail-closed：解析失败视为私网）
	resolved, err := net.LookupHost(host)
	if err != nil {
		return true
	}
	if len(resolved) == 0 {
		return true
	}

	for _, r := range resolved {
		if ip := net.ParseIP(r); ip != nil {
			if isPrivateIP(ip) {
				return true
			}
		}
	}
	return false
}

// isPrivateIP 判断 IP 是否属于私网/保留地址段
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	// 云元数据端点
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return true
	}
	return false
}

// ValidateURLScheme 校验 URL 协议是否在白名单内（仅允许 http/https）
func ValidateURLScheme(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URL 格式错误: %w", err)
	}
	if !allowedSchemes[parsed.Scheme] {
		return fmt.Errorf("不允许的协议 %q，仅支持 http/https", parsed.Scheme)
	}
	return nil
}

// ValidateURLHost 校验 URL 的 Host 是否为私网地址
func ValidateURLHost(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URL 格式错误: %w", err)
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("URL 缺少主机地址")
	}
	if IsPrivateHost(host) {
		return fmt.Errorf("不允许访问内网地址 %s", host)
	}
	return nil
}

// NewSSRFSafeClient 创建带 SSRF 防护的 HTTP Client
// - 连接时校验目标 IP（防 DNS rebinding / TOCTOU）
// - 跟随重定向时校验每个跳转目标
// - 限制最多 5 次重定向
func NewSSRFSafeClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// 解析实际连接的 IP（连接时校验，防 DNS rebinding）
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("地址格式错误: %w", err)
			}

			// 解析 host → IP
			ips, err := net.DefaultResolver.LookupHost(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("DNS 解析失败: %w", err)
			}

			// 分离 IPv4 和 IPv6，仅使用 IPv4（国内 IPv6 国际出口普遍不稳定）
			var ip4s []string
			for _, ipStr := range ips {
				ip := net.ParseIP(ipStr)
				if ip == nil {
					continue
				}
				if isPrivateIP(ip) {
					return nil, fmt.Errorf("SSRF 防护：拒绝连接到内网地址 %s", ip)
				}
				if ip.To4() != nil {
					ip4s = append(ip4s, ipStr)
				}
				// 跳过 IPv6：国内电信/联通 IPv6 到 Cloudflare 等 CDN 经常 connection reset
			}

			if len(ip4s) == 0 {
				return nil, fmt.Errorf("DNS 解析无可用 IPv4 地址: %s", host)
			}

			var lastErr error
			for _, ipStr := range ip4s {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ipStr, port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("重定向次数超限（最多 5 次）")
			}
			// 校验重定向目标协议
			if !allowedSchemes[req.URL.Scheme] {
				return fmt.Errorf("重定向到不允许的协议 %q", req.URL.Scheme)
			}
			return nil
		},
	}
}
