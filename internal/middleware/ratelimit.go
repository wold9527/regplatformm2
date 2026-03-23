package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// rateBucket 令牌桶
type rateBucket struct {
	tokens    float64
	lastTime  time.Time
	maxTokens float64
	refillRate float64 // tokens per second
}

// RateLimiter 基于 IP 的内存限流器
type RateLimiter struct {
	buckets map[string]*rateBucket
	mu      sync.Mutex
	max     float64
	rate    float64
}

// NewRateLimiter 创建限流器
// max: 最大令牌数（突发容量）, perMinute: 每分钟补充令牌数
func NewRateLimiter(max float64, perMinute float64) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*rateBucket),
		max:     max,
		rate:    perMinute / 60.0,
	}
	// 定期清理过期桶
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			rl.mu.Lock()
			cutoff := time.Now().Add(-30 * time.Minute)
			for k, b := range rl.buckets {
				if b.lastTime.Before(cutoff) {
					delete(rl.buckets, k)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &rateBucket{tokens: rl.max, lastTime: now, maxTokens: rl.max, refillRate: rl.rate}
		rl.buckets[key] = b
	}

	// 补充令牌
	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastTime = now

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// RateLimit 限流中间件
func RateLimit(limiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if !limiter.Allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"detail": "请求过于频繁，请稍后再试"})
			return
		}
		c.Next()
	}
}
