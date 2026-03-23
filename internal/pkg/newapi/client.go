package newapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client 封装 New-API 管理员 API
type Client struct {
	baseURL      string
	adminToken   string
	adminUserID  int // 管理员自身的 New-API 用户 ID（用于 New-Api-User 鉴权头）
	http         *http.Client
}

// NewClient 创建 New-API 客户端
// adminUserID: 管理员在 New-API 中的用户 ID，所有请求的 New-Api-User 头必须设为此值
func NewClient(baseURL, adminToken string, adminUserID int) *Client {
	return &Client{
		baseURL:     baseURL,
		adminToken:  adminToken,
		adminUserID: adminUserID,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// setAdminAuth 设置管理员鉴权头
// New-API 的 authHelper 中间件要求 New-Api-User 与 token 对应的用户 ID 一致
func (c *Client) setAdminAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.adminToken)
	req.Header.Set("New-Api-User", fmt.Sprintf("%d", c.adminUserID))
}

// userResponse New-API 用户接口响应
type userResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ID    int `json:"id"`
		Quota int `json:"quota"`
	} `json:"data"`
	Message string `json:"message"`
}

// genericResponse New-API 通用响应
type genericResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// GetUserQuota 实时查询用户 quota（GET /api/user/{id}）
func (c *Client) GetUserQuota(newapiUserID int) (int, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/user/%d", c.baseURL, newapiUserID), nil)
	if err != nil {
		return 0, fmt.Errorf("创建请求失败: %w", err)
	}
	c.setAdminAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("请求 New-API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("New-API 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var result userResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("解析响应失败: %w", err)
	}

	if !result.Success {
		return 0, fmt.Errorf("New-API 错误: %s", result.Message)
	}

	return result.Data.Quota, nil
}

// DeductQuota 原子扣减 quota（POST /api/user/{id}/quota-deduct）
// 由 New-API 保证原子性，无需客户端加锁
func (c *Client) DeductQuota(newapiUserID int, amount int) error {
	payload := map[string]interface{}{
		"amount": amount,
		"remark": "regplatform 购买注册次数",
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/user/%d/quota-deduct", c.baseURL, newapiUserID), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	c.setAdminAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("请求 New-API 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("New-API 扣减失败 %d: %s", resp.StatusCode, string(respBody))
	}

	var result genericResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("New-API 扣减失败: %s", result.Message)
	}

	return nil
}
