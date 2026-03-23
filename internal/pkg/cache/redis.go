package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// RedisCache Redis 缓存层（L2）
// 设计为可选组件：REDIS_URL 为空时所有方法静默返回 miss，不影响业务
type RedisCache struct {
	client *redis.Client
	prefix string // key 前缀，如 "regp:"
}

// NewRedisCache 创建 Redis 缓存
// redisURL 为空则返回空壳实例（所有操作静默降级）
func NewRedisCache(redisURL, prefix string) *RedisCache {
	rc := &RedisCache{prefix: prefix}
	if redisURL == "" {
		log.Info().Msg("REDIS_URL 未配置，L2 缓存已禁用（纯内存模式）")
		return rc
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Error().Err(err).Str("url", redisURL).Msg("Redis URL 解析失败，L2 缓存已禁用")
		return rc
	}

	client := redis.NewClient(opts)

	// 连接测试（3 秒超时）
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Error().Err(err).Msg("Redis 连接失败，L2 缓存已禁用")
		return rc
	}

	rc.client = client
	log.Info().Str("addr", opts.Addr).Msg("Redis L2 缓存已连接")
	return rc
}

// Available 检查 Redis 是否可用
func (c *RedisCache) Available() bool {
	return c.client != nil
}

// key 拼接完整 key
func (c *RedisCache) key(k string) string {
	return c.prefix + k
}

// GetString 获取字符串值
func (c *RedisCache) GetString(ctx context.Context, key string) (string, bool) {
	if c.client == nil {
		return "", false
	}
	val, err := c.client.Get(ctx, c.key(key)).Result()
	if err != nil {
		return "", false
	}
	return val, true
}

// SetString 设置字符串值
func (c *RedisCache) SetString(ctx context.Context, key, value string, ttl time.Duration) {
	if c.client == nil {
		return
	}
	if err := c.client.Set(ctx, c.key(key), value, ttl).Err(); err != nil {
		log.Warn().Err(err).Str("key", key).Msg("Redis SET 失败")
	}
}

// GetJSON 获取 JSON 序列化的值
func (c *RedisCache) GetJSON(ctx context.Context, key string, dest interface{}) bool {
	if c.client == nil {
		return false
	}
	val, err := c.client.Get(ctx, c.key(key)).Bytes()
	if err != nil {
		return false
	}
	if err := json.Unmarshal(val, dest); err != nil {
		log.Warn().Err(err).Str("key", key).Msg("Redis JSON 反序列化失败")
		return false
	}
	return true
}

// SetJSON 设置 JSON 序列化的值
func (c *RedisCache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) {
	if c.client == nil {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		log.Warn().Err(err).Str("key", key).Msg("Redis JSON 序列化失败")
		return
	}
	if err := c.client.Set(ctx, c.key(key), data, ttl).Err(); err != nil {
		log.Warn().Err(err).Str("key", key).Msg("Redis SET 失败")
	}
}

// Del 删除 key
func (c *RedisCache) Del(ctx context.Context, keys ...string) {
	if c.client == nil {
		return
	}
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = c.key(k)
	}
	if err := c.client.Del(ctx, fullKeys...).Err(); err != nil {
		log.Warn().Err(err).Msg("Redis DEL 失败")
	}
}

// Close 关闭连接
func (c *RedisCache) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}
