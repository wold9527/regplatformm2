package config

import (
	"log"

	"github.com/spf13/viper"
)

// Config 基础设施配置（仅含启动必须项）
// 应用级配置（API Key、代理、验证码等）统一在管理后台系统设置中配置，持久化到数据库
type Config struct {
	// 服务器
	Port    int    `mapstructure:"PORT"`
	GinMode string `mapstructure:"GIN_MODE"`

	// 数据库
	DatabaseURL string `mapstructure:"DATABASE_URL"`

	// JWT
	JWTSecret      string `mapstructure:"JWT_SECRET"`
	JWTExpireHours int    `mapstructure:"JWT_EXPIRE_HOURS"`

	// SSO（可选，用于外部系统对接）
	SSOSecret string `mapstructure:"SSO_SECRET"`

	// Redis（可选，为空则纯内存缓存）
	RedisURL string `mapstructure:"REDIS_URL"`

	// 开发模式
	DevMode bool `mapstructure:"DEV_MODE"`

	// 管理员用户名（可选，该用户注册时自动成为管理员；为空则第一个注册的用户为管理员）
	AdminUsername string `mapstructure:"ADMIN_USERNAME"`
}

// Load 加载配置：环境变量 > .env 文件 > 默认值
func Load() *Config {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	// 默认值
	viper.SetDefault("PORT", 8000)
	viper.SetDefault("GIN_MODE", "release")
	viper.SetDefault("DATABASE_URL", "postgres://localhost:5432/regplatform?sslmode=disable")
	viper.SetDefault("JWT_SECRET", "change-me-in-production")
	viper.SetDefault("JWT_EXPIRE_HOURS", 72)
	viper.SetDefault("SSO_SECRET", "")
	viper.SetDefault("REDIS_URL", "")
	viper.SetDefault("DEV_MODE", false)
	viper.SetDefault("ADMIN_USERNAME", "")

	_ = viper.ReadInConfig() // .env 不存在不报错

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("配置解析失败: %v", err)
	}

	// 生产模式强制检查 JWT 密钥强度
	if cfg.GinMode == "release" && (cfg.JWTSecret == "change-me-in-production" || len(cfg.JWTSecret) < 32) {
		log.Fatalf("生产环境 JWT_SECRET 必须设置为 32 字符以上的强密钥")
	}

	return &cfg
}
