package model

import (
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConnectDB 连接 PostgreSQL 并自动迁移
func ConnectDB(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatal().Err(err).Msg("数据库连接失败")
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal().Err(err).Msg("获取底层数据库连接失败")
	}
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 自动迁移（开发环境用，生产用 golang-migrate）
	if err := db.AutoMigrate(
		&User{},
		&Task{},
		&TaskResult{},
		&CreditTransaction{},
		&SystemSetting{},
		&RedemptionCode{},
		&Announcement{},
		&Notification{},
		&NotificationRead{},
		&HFToken{},
		&HFSpace{},
		&UserProxy{},
		&ProxyPoolEntry{},
		&CardPoolEntry{},
	); err != nil {
		log.Fatal().Err(err).Msg("数据库迁移失败")
	}

	log.Info().Msg("数据库连接成功")
	return db
}
