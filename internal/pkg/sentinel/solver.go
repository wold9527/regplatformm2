// Package sentinel 实现 OpenAI Sentinel PoW 算法
// 逆向来源：https://sentinel.openai.com/sentinel/20260124ceb8/sdk.js
//
// 核心算法：
//  1. 构造 19 元素浏览器环境数组 (config)
//  2. PoW: FNV-1a 32bit hash + xorshift finalizer，暴力搜索 nonce
//  3. 调用 sentinel API 获取 c 字段和 seed/difficulty
//  4. 返回最终 openai-sentinel-token JSON 字符串
package sentinel

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	sentinelAPIURL = "https://sentinel.openai.com/backend-api/sentinel/req"
	sentinelUA     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36"
	scriptSrc      = "https://sentinel.openai.com/sentinel/20260124ceb8/sdk.js"
	jsHeapLimit    = 4294705152 // Chrome 典型值
	maxAttempts    = 500000

	// SDK 错误前缀常量
	errPrefix = "wQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D"
)

var (
	navProps = []string{
		"vendorSub", "productSub", "vendor", "maxTouchPoints",
		"scheduling", "userActivation", "doNotTrack", "geolocation",
		"connection", "plugins", "mimeTypes", "pdfViewerEnabled",
		"webkitTemporaryStorage", "webkitPersistentStorage",
		"hardwareConcurrency", "cookieEnabled", "credentials",
		"mediaDevices", "permissions", "locks", "ink",
	}
	docKeys = []string{"location", "implementation", "URL", "documentURI", "compatMode"}
	winKeys = []string{"Object", "Function", "Array", "Number", "parseFloat", "undefined"}
	cpuCores = []int{4, 8, 12, 16}
)

// Solver Sentinel PoW 求解器
type Solver struct {
	DeviceID string // UUID v4，整个 session 固定
	sid      string // 会话标识 UUID
	rng      *rand.Rand
}

// New 创建新的 Solver，DeviceID 自动生成
func New() *Solver {
	return &Solver{
		DeviceID: uuid.New().String(),
		sid:      uuid.New().String(),
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// NewWithDeviceID 使用指定 DeviceID 创建 Solver
func NewWithDeviceID(deviceID string) *Solver {
	return &Solver{
		DeviceID: deviceID,
		sid:      uuid.New().String(),
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// fnv1a32 FNV-1a 32bit 哈希 + xorshift finalizer
// 逆向还原 SDK JS 中的匿名哈希函数：
//   e = 2166136261; e ^= t.charCodeAt(r); e = Math.imul(e, 16777619) >>> 0
//   然后 xorshift 混合（murmurhash3 风格 finalizer）
func fnv1a32(text string) string {
	h := uint32(2166136261) // FNV offset basis
	for _, ch := range text {
		h ^= uint32(ch)
		h *= 16777619 // uint32 自动截断，等价 Math.imul >>> 0
	}
	// xorshift finalizer
	h ^= h >> 16
	h *= 2246822507
	h ^= h >> 13
	h *= 3266489909
	h ^= h >> 16
	return fmt.Sprintf("%08x", h)
}

// base64Encode 模拟 SDK E() 函数：JSON.stringify（紧凑）→ UTF-8 → base64
func base64Encode(data []interface{}) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// getConfig 构造 19 元素浏览器环境数组
func (s *Solver) getConfig() []interface{} {
	now := time.Now().UTC()
	// JS Date.toString() 格式：Mon Feb 21 2026 06:47:00 GMT+0000 (Coordinated Universal Time)
	dateStr := now.Format("Mon Jan 02 2006 15:04:05 GMT+0000 (Coordinated Universal Time)")

	navProp := navProps[s.rng.Intn(len(navProps))]
	// SDK 使用 U+2212 Unicode 减号，而非 ASCII 连字符
	navVal := navProp + "\u2212undefined"

	perfNow := s.rng.Float64()*49000 + 1000 // [1000, 50000)
	timeOrigin := float64(now.UnixMilli()) - perfNow

	return []interface{}{
		"1920x1080",                              // [0] screen
		dateStr,                                  // [1] Date.toString()
		jsHeapLimit,                              // [2] performance.memory.jsHeapSizeLimit
		s.rng.Float64(),                          // [3] 占位，PoW 时被 nonce 替换
		sentinelUA,                               // [4] navigator.userAgent
		scriptSrc,                                // [5] script src
		nil,                                      // [6] script version
		nil,                                      // [7] data-build
		"en-US",                                  // [8] navigator.language
		"en-US,en",                               // [9] 占位，PoW 时被耗时替换
		s.rng.Float64(),                          // [10] Math.random()
		navVal,                                   // [11] navigator 属性（U+2212）
		docKeys[s.rng.Intn(len(docKeys))],        // [12] document key
		winKeys[s.rng.Intn(len(winKeys))],        // [13] window key
		perfNow,                                  // [14] performance.now()
		s.sid,                                    // [15] session UUID
		"",                                       // [16] URL 参数
		cpuCores[s.rng.Intn(len(cpuCores))],     // [17] navigator.hardwareConcurrency
		timeOrigin,                               // [18] performance.timeOrigin
	}
}

// GenerateRequirementsToken 生成 requirements token（prefix "gAAAAAC"，无需服务端参数）
// 对应 SDK 的 getRequirementsToken()
func (s *Solver) GenerateRequirementsToken() (string, error) {
	config := s.getConfig()
	config[3] = 1
	config[9] = s.rng.Intn(46) + 5 // [5, 50)

	data, err := base64Encode(config)
	if err != nil {
		return "", err
	}
	return "gAAAAAC" + data, nil
}

// SolvePoW 暴力搜索满足难度要求的 nonce，返回 PoW token（prefix "gAAAAAB"）
func (s *Solver) SolvePoW(seed, difficulty string) string {
	config := s.getConfig()
	startTime := time.Now()

	for i := 0; i < maxAttempts; i++ {
		config[3] = i
		config[9] = int(time.Since(startTime).Milliseconds())

		data, err := base64Encode(config)
		if err != nil {
			continue
		}

		hashHex := fnv1a32(seed + data)
		diffLen := len(difficulty)
		if len(hashHex) >= diffLen && hashHex[:diffLen] <= difficulty {
			return "gAAAAAB" + data + "~S"
		}
	}

	// 超过最大次数，返回错误 token
	errData, _ := base64Encode([]interface{}{nil})
	return "gAAAAAB" + errPrefix + errData
}

// sentinelReq Sentinel API 响应结构
type sentinelResp struct {
	Token      string `json:"token"`
	ProofOfWork struct {
		Required   bool   `json:"required"`
		Seed       string `json:"seed"`
		Difficulty string `json:"difficulty"`
	} `json:"proofofwork"`
}

// FetchChallenge 调用 sentinel API 获取 c 字段和 PoW 参数
func (s *Solver) FetchChallenge(ctx context.Context, client *http.Client, flow string) (*sentinelResp, error) {
	reqToken, err := s.GenerateRequirementsToken()
	if err != nil {
		return nil, fmt.Errorf("generate requirements token: %w", err)
	}

	body, _ := json.Marshal(map[string]string{
		"p":    reqToken,
		"id":   s.DeviceID,
		"flow": flow,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", sentinelAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	req.Header.Set("Referer", "https://sentinel.openai.com/backend-api/sentinel/frame.html")
	req.Header.Set("Origin", "https://sentinel.openai.com")
	req.Header.Set("User-Agent", sentinelUA)
	req.Header.Set("sec-ch-ua", `"Not:A-Brand";v="99", "Google Chrome";v="145", "Chromium";v="145"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sentinel API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return nil, fmt.Errorf("sentinel API returned %d: %s", resp.StatusCode, b)
	}

	var result sentinelResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode sentinel response: %w", err)
	}
	return &result, nil
}

// BuildToken 完整流程：获取 challenge → 求解 PoW → 返回最终 sentinel-token JSON 字符串
func (s *Solver) BuildToken(ctx context.Context, client *http.Client, flow string) (string, error) {
	challenge, err := s.FetchChallenge(ctx, client, flow)
	if err != nil {
		return "", fmt.Errorf("fetch challenge: %w", err)
	}

	cValue := challenge.Token
	pow := challenge.ProofOfWork

	var pValue string
	if pow.Required && pow.Seed != "" {
		pValue = s.SolvePoW(pow.Seed, pow.Difficulty)
	} else {
		pValue, err = s.GenerateRequirementsToken()
		if err != nil {
			return "", err
		}
	}

	token, _ := json.Marshal(map[string]string{
		"p":    pValue,
		"t":    "", // 服务端不校验此字段
		"c":    cValue,
		"id":   s.DeviceID,
		"flow": flow,
	})
	return strings.ReplaceAll(string(token), " ", ""), nil
}
