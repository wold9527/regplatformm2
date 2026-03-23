package gptmail

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Client GPTMail 邮箱服务客户端（支持多 key 均匀轮询 + 429 冷却跳过）
type Client struct {
	baseURL    string
	keys       []string       // 多个 API Key
	keyIdx     atomic.Int64   // 轮询索引（每次请求 +1，均匀分配）
	cooldowns  map[int]time.Time // key 索引 → 冷却截止时间
	cdMu       sync.RWMutex     // cooldowns 读写锁
	httpClient *http.Client
	mu         sync.Mutex    // 全局请求节流锁
	lastCall   time.Time     // 上次请求时间
	minDelay   time.Duration // 最小请求间隔
}

// NewClient 创建 GPTMail 客户端，apiKey 支持逗号分隔多个 key 自动轮询
func NewClient(baseURL, apiKey string) *Client {
	keys := make([]string, 0)
	for _, k := range strings.Split(apiKey, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		keys = append(keys, "")
	}
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		keys:      keys,
		cooldowns: make(map[int]time.Time),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		minDelay: 1 * time.Second,
	}
}

// nextKey 获取下一个可用 key（均匀轮询 + 跳过冷却中的 key）
// 返回 key 字符串、对应索引、是否所有 key 都在冷却
func (c *Client) nextKey() (string, int, bool) {
	n := len(c.keys)
	base := int(c.keyIdx.Add(1) - 1) // 先取再加，每次请求递增

	c.cdMu.RLock()
	defer c.cdMu.RUnlock()

	// 第一轮：找未冷却的 key
	now := time.Now()
	for i := 0; i < n; i++ {
		idx := (base + i) % n
		if until, ok := c.cooldowns[idx]; ok && now.Before(until) {
			continue // 冷却中，跳过
		}
		return c.keys[idx], idx, false
	}

	// 所有 key 都在冷却 → 返回标记，让调用方快速失败
	return "", -1, true
}

// markCooldown 标记某个 key 进入冷却期
func (c *Client) markCooldown(idx int, duration time.Duration) {
	c.cdMu.Lock()
	defer c.cdMu.Unlock()
	c.cooldowns[idx] = time.Now().Add(duration)
}

// throttle 全局节流，确保请求间隔 >= minDelay
func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.lastCall)
	if elapsed < c.minDelay {
		time.Sleep(c.minDelay - elapsed)
	}
	c.lastCall = time.Now()
}

// nextDailyReset 计算到下一个早 8 点的时长（GPTMail 每日额度重置时间）
func nextDailyReset() time.Duration {
	now := time.Now()
	reset := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, now.Location())
	if now.After(reset) {
		reset = reset.AddDate(0, 0, 1) // 已过今天 8 点，算明天
	}
	return time.Until(reset)
}

// ErrAllKeysDead 所有 API Key 均不可用（401/403/429）
var ErrAllKeysDead = fmt.Errorf("GPTMail 所有 API Key 均不可用")

// doGet 执行 GET 请求（带节流 + 重试 + 401 快速失败 + 429 自动换 key）
// maxRetries 是网络重试次数；401/429 换 key 不消耗名额（额外 len(keys) 次预算）
func (c *Client) doGet(url string, maxRetries int) ([]byte, error) {
	var lastErr error
	totalBudget := maxRetries + len(c.keys)
	for i := 0; i < totalBudget; i++ {
		c.throttle()

		key, keyIdx, allDead := c.nextKey()
		if allDead {
			// 所有 key 都在冷却（通常是全部 401），立即放弃不浪费时间
			if lastErr == nil {
				lastErr = ErrAllKeysDead
			}
			return nil, fmt.Errorf("GPTMail 请求失败 (所有 key 不可用): %w", lastErr)
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-API-Key", key)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * 2 * time.Second)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			keyPreview := key
			if len(keyPreview) > 8 {
				keyPreview = keyPreview[:8]
			}
			lastErr = fmt.Errorf("GPTMail HTTP %d: key %s... 认证失败/额度耗尽", resp.StatusCode, keyPreview)
			// 401 可能是 key 无效，也可能是当日额度用完，冷却 15 分钟
			c.markCooldown(keyIdx, 15*time.Minute)
			continue // 立即尝试下一个 key（不 sleep）
		}
		if resp.StatusCode == 429 {
			keyPreview := key
			if len(keyPreview) > 8 {
				keyPreview = keyPreview[:8]
			}
			lastErr = fmt.Errorf("GPTMail 429 每日限额 (key: %s...)", keyPreview)
			// 每日限额，冷却到明天早 8 点重置
			c.markCooldown(keyIdx, nextDailyReset())
			continue // 不 sleep，立即尝试下一个 key
		}
		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("GPTMail HTTP %d: %s", resp.StatusCode, string(body))
			continue
		}
		return body, nil
	}
	return nil, fmt.Errorf("GPTMail 请求失败 (重试 %d 次): %w", maxRetries, lastErr)
}

// GenerateEmail 创建临时邮箱，返回邮箱地址
func (c *Client) GenerateEmail() (string, error) {
	body, err := c.doGet(c.baseURL+"/api/generate-email", 3)
	if err != nil {
		return "", err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析邮箱响应失败: %w", err)
	}
	// 兼容两种格式：
	// 旧格式：{"email": "xxx"}
	// 新格式：{"success": true, "data": {"email": "xxx"}}
	email, _ := result["email"].(string)
	if email == "" {
		if data, ok := result["data"].(map[string]interface{}); ok {
			email, _ = data["email"].(string)
		}
	}
	if email == "" {
		return "", fmt.Errorf("邮箱响应中无 email 字段: %s", string(body))
	}
	return email, nil
}

// 验证码提取正则
var (
	reSubjectCode = regexp.MustCompile(`^([A-Z0-9]+-[A-Z0-9]+)`)
	reBodyCode    = regexp.MustCompile(`\b([A-Z0-9]{6,8})\b`)
)

// FetchVerificationCode 轮询获取验证码（最多 maxAttempts 次，每次间隔 interval）
func (c *Client) FetchVerificationCode(email string, maxAttempts int, interval time.Duration) (string, error) {
	url := fmt.Sprintf("%s/api/emails?email=%s", c.baseURL, email)

	for i := 0; i < maxAttempts; i++ {
		body, err := c.doGet(url, 1)
		if err != nil {
			time.Sleep(interval)
			continue
		}

		// API 统一格式：{"success":true,"data":{"emails":[...],"count":N},"error":""}
		var resp map[string]interface{}
		if err := json.Unmarshal(body, &resp); err != nil {
			time.Sleep(interval)
			continue
		}

		// 提取 data.emails 数组
		var emails []interface{}
		if data, ok := resp["data"].(map[string]interface{}); ok {
			if arr, ok := data["emails"].([]interface{}); ok {
				emails = arr
			}
		}

		for _, item := range emails {
			em, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			// 策略 1: 从 subject 提取
			if subject, ok := em["subject"].(string); ok {
				if m := reSubjectCode.FindStringSubmatch(subject); len(m) > 1 {
					code := strings.ReplaceAll(m[1], "-", "")
					return code, nil
				}
			}
			// 策略 2: 从 content/html_content 提取（API 字段名）
			for _, field := range []string{"content", "html_content"} {
				if content, ok := em[field].(string); ok && content != "" {
					if m := reBodyCode.FindStringSubmatch(content); len(m) > 1 {
						return m[1], nil
					}
				}
			}
		}
		time.Sleep(interval)
	}
	return "", fmt.Errorf("获取验证码超时 (%d 次轮询)", maxAttempts)
}

// DeleteEmail 删除临时邮箱
func (c *Client) DeleteEmail(email string) error {
	url := fmt.Sprintf("%s/api/emails/clear?email=%s", c.baseURL, email)
	c.throttle()
	key, _, allDead := c.nextKey()
	if allDead {
		return ErrAllKeysDead
	}
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", key)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
