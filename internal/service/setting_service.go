package service

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"github.com/xiaolajiaoyyds/regplatformm/internal/pkg/cache"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SettingService 系统设置服务（L1 内存 + L2 Redis 两层缓存）
// 读取优先级：L1 内存(5s) → L2 Redis(5min) → DB system_settings → 硬编码默认值
type SettingService struct {
	db    *gorm.DB
	redis *cache.RedisCache
	cache map[string]cachedSetting
	mu    sync.RWMutex
	l1TTL time.Duration // L1 内存缓存 TTL
	l2TTL time.Duration // L2 Redis 缓存 TTL
}

type cachedSetting struct {
	value     string
	expiresAt time.Time
}

// NewSettingService 创建设置服务
func NewSettingService(db *gorm.DB, rc *cache.RedisCache) *SettingService {
	return &SettingService{
		db:    db,
		redis: rc,
		cache: make(map[string]cachedSetting),
		l1TTL: 5 * time.Second,   // L1 内存：5 秒（高频读，快速感知变更）
		l2TTL: 5 * time.Minute,   // L2 Redis：5 分钟（跨实例共享，减少 DB 压力）
	}
}

// redisKey 生成 Redis key
func settingRedisKey(key string) string {
	return "setting:" + key
}

// Get 获取设置值（L1 → L2 → DB → 硬编码默认值）
func (s *SettingService) Get(key string, fallback string) string {
	// ── L1：内存缓存 ─────────────────────────────────────────────────────
	s.mu.RLock()
	if c, ok := s.cache[key]; ok && time.Now().Before(c.expiresAt) {
		s.mu.RUnlock()
		return resolveValue(c.value, fallback, key)
	}
	s.mu.RUnlock()

	// ── L2：Redis 缓存 ──────────────────────────────────────────────────
	if s.redis != nil {
		if val, ok := s.redis.GetString(context.Background(), settingRedisKey(key)); ok {
			// 回填 L1
			s.mu.Lock()
			s.cache[key] = cachedSetting{value: val, expiresAt: time.Now().Add(s.l1TTL)}
			s.mu.Unlock()
			return resolveValue(val, fallback, key)
		}
	}

	// ── L3：数据库（静默查询，record not found 不打日志） ────────────────
	var setting model.SystemSetting
	dbValue := ""
	if err := s.db.Session(&gorm.Session{Logger: s.db.Logger.LogMode(logger.Silent)}).Where("\"key\" = ?", key).First(&setting).Error; err == nil {
		dbValue = setting.Value
	}

	// 回填 L1 + L2（即使为空也缓存，避免反复查库）
	s.mu.Lock()
	s.cache[key] = cachedSetting{value: dbValue, expiresAt: time.Now().Add(s.l1TTL)}
	s.mu.Unlock()

	if s.redis != nil {
		s.redis.SetString(context.Background(), settingRedisKey(key), dbValue, s.l2TTL)
	}

	return resolveValue(dbValue, fallback, key)
}

// resolveValue 解析最终值：dbValue → fallback → 硬编码默认值
func resolveValue(value, fallback, key string) string {
	if value != "" {
		return value
	}
	if fallback != "" {
		return fallback
	}
	for _, def := range model.DefaultSettings {
		if def.Key == key {
			return def.DefaultValue
		}
	}
	return ""
}

// GetInt 获取整数设置
func (s *SettingService) GetInt(key string, fallback int) int {
	val := s.Get(key, "")
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}

// GetBool 获取布尔设置
func (s *SettingService) GetBool(key string, fallback bool) bool {
	val := s.Get(key, "")
	if val == "" {
		return fallback
	}
	return val == "true" || val == "1"
}

// Set 保存设置（同时清除 L1 + L2 缓存）
func (s *SettingService) Set(key, value string) error {
	err := s.db.Save(&model.SystemSetting{Key: key, Value: value}).Error
	if err == nil {
		// 清除 L1
		s.mu.Lock()
		delete(s.cache, key)
		s.mu.Unlock()
		// 清除 L2
		if s.redis != nil {
			s.redis.Del(context.Background(), settingRedisKey(key))
		}
	}
	return err
}

// GetAll 获取所有设置（含脱敏）
func (s *SettingService) GetAll() []map[string]interface{} {
	results := make([]map[string]interface{}, 0, len(model.DefaultSettings))
	for _, def := range model.DefaultSettings {
		val := s.Get(def.Key, def.DefaultValue)
		displayVal := val
		if def.IsSensitive && len(val) > 4 {
			displayVal = val[:2] + "****" + val[len(val)-2:]
		}
		results = append(results, map[string]interface{}{
			"key":         def.Key,
			"label":       def.Label,
			"group":       def.Group,
			"type":        def.Type,
			"value":       displayVal,
			"description": def.Description,
			"is_set":      val != "" && val != def.DefaultValue,
			"has_value":   val != "" && val != def.DefaultValue,
			"is_secret":   def.IsSensitive,
		})
	}
	return results
}

// GetRaw 获取原始值（管理员用）
func (s *SettingService) GetRaw(key string) string {
	return s.Get(key, "")
}
