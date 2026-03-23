package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"gorm.io/gorm"
)

// CardPool 虚拟卡池（线程安全轮询，健康验证）
type CardPool struct {
	mu    sync.RWMutex
	cards []model.CardPoolEntry
	index uint64
	db    *gorm.DB
}

// NewCardPool 创建卡池
func NewCardPool(db *gorm.DB) *CardPool {
	p := &CardPool{db: db}
	p.Reload()
	return p
}

// Reload 从 DB 加载有效卡到内存
func (p *CardPool) Reload() {
	if p.db == nil {
		return
	}
	var entries []model.CardPoolEntry
	if err := p.db.Where("is_valid = ? AND fail_count < ?", true, 5).
		Order("use_count ASC, created_at ASC").Find(&entries).Error; err != nil {
		log.Warn().Err(err).Msg("卡池加载失败")
		return
	}

	p.mu.Lock()
	p.cards = entries
	atomic.StoreUint64(&p.index, 0)
	p.mu.Unlock()

	log.Info().Int("count", len(entries)).Msg("卡池已加载")
}

// GetNext 轮询获取下一张有效卡
func (p *CardPool) GetNext() *model.CardPoolEntry {
	p.mu.RLock()
	n := len(p.cards)
	if n == 0 {
		p.mu.RUnlock()
		return nil
	}
	idx := atomic.AddUint64(&p.index, 1)
	card := p.cards[idx%uint64(n)]
	p.mu.RUnlock()
	return &card
}

// Count 返回可用卡数量
func (p *CardPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.cards)
}

// MarkUsed 标记卡已使用
func (p *CardPool) MarkUsed(id uint) {
	if p.db == nil {
		return
	}
	now := time.Now()
	p.db.Model(&model.CardPoolEntry{}).Where("id = ?", id).Updates(map[string]interface{}{
		"use_count":    gorm.Expr("use_count + 1"),
		"last_used_at": now,
	})
}

// MarkFailed 标记卡使用失败
func (p *CardPool) MarkFailed(id uint) {
	if p.db == nil {
		return
	}
	p.db.Model(&model.CardPoolEntry{}).Where("id = ?", id).Updates(map[string]interface{}{
		"fail_count": gorm.Expr("fail_count + 1"),
	})
	// 失败次数过多自动标记无效
	p.db.Model(&model.CardPoolEntry{}).Where("id = ? AND fail_count >= ?", id, 5).
		Update("is_valid", false)
	p.Reload()
}

// PoolStats 返回卡池统计
func (p *CardPool) PoolStats() map[string]interface{} {
	if p.db == nil {
		return map[string]interface{}{"total": 0, "valid": 0, "invalid": 0}
	}
	var total, validCount int64
	p.db.Model(&model.CardPoolEntry{}).Count(&total)
	p.db.Model(&model.CardPoolEntry{}).Where("is_valid = ?", true).Count(&validCount)
	return map[string]interface{}{
		"total":   total,
		"valid":   validCount,
		"invalid": total - validCount,
		"active":  p.Count(),
	}
}

// StartValidator 启动后台定期验证协程（每 30 分钟验证一次）
func (p *CardPool) StartValidator(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		log.Info().Msg("卡池定时验证已启动")
		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("卡池定时验证已停止")
				return
			case <-ticker.C:
				p.validateExpired()
			}
		}
	}()
}

// validateExpired 标记已过期的卡为无效
func (p *CardPool) validateExpired() {
	if p.db == nil {
		return
	}
	now := time.Now()
	currentMonth := int(now.Month())
	currentYear := now.Year()

	result := p.db.Model(&model.CardPoolEntry{}).
		Where("is_valid = ? AND (exp_year < ? OR (exp_year = ? AND exp_month < ?))",
			true, currentYear, currentYear, currentMonth).
		Update("is_valid", false)
	if result.RowsAffected > 0 {
		log.Info().Int64("expired", result.RowsAffected).Msg("已标记过期卡为无效")
		p.Reload()
	}
}

// ValidateAll 验证所有卡（过期检查），返回 (总数, 有效, 无效)
func (p *CardPool) ValidateAll() (total, valid, invalid int) {
	if p.db == nil {
		return 0, 0, 0
	}
	p.validateExpired()

	var entries []model.CardPoolEntry
	if err := p.db.Find(&entries).Error; err != nil {
		return 0, 0, 0
	}
	total = len(entries)
	for _, e := range entries {
		if e.IsValid {
			valid++
		} else {
			invalid++
		}
	}
	p.Reload()
	return total, valid, invalid
}

// ValidateByIDs 验证指定 ID 的卡
func (p *CardPool) ValidateByIDs(ids []uint) (total, valid, invalid int) {
	if p.db == nil || len(ids) == 0 {
		return 0, 0, 0
	}
	p.validateExpired()

	var entries []model.CardPoolEntry
	if err := p.db.Where("id IN ?", ids).Find(&entries).Error; err != nil {
		return 0, 0, 0
	}
	total = len(entries)
	for _, e := range entries {
		if e.IsValid {
			valid++
		} else {
			invalid++
		}
	}
	p.Reload()
	return total, valid, invalid
}
