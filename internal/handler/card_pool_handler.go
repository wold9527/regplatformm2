package handler

import (
	"bufio"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"github.com/xiaolajiaoyyds/regplatformm/internal/service"
	"gorm.io/gorm"
)

// CardPoolHandler 虚拟卡池管理（管理员）
type CardPoolHandler struct {
	db       *gorm.DB
	cardPool *service.CardPool
}

// NewCardPoolHandler 创建卡池管理处理器
func NewCardPoolHandler(db *gorm.DB, cardPool *service.CardPool) *CardPoolHandler {
	return &CardPoolHandler{db: db, cardPool: cardPool}
}

// ── 请求结构体 ──

type createCardReq struct {
	Name           string `json:"name" binding:"max=100"`
	CardNumber     string `json:"card_number" binding:"required,min=13,max=19"`
	ExpMonth       int    `json:"exp_month" binding:"required,min=1,max=12"`
	ExpYear        int    `json:"exp_year" binding:"required,min=2024,max=2099"`
	CVC            string `json:"cvc" binding:"required,min=3,max=4"`
	BillingName    string `json:"billing_name" binding:"max=200"`
	BillingEmail   string `json:"billing_email" binding:"max=200"`
	BillingCountry string `json:"billing_country" binding:"max=10"`
	BillingCity    string `json:"billing_city" binding:"max=100"`
	BillingLine1   string `json:"billing_line1" binding:"max=300"`
	BillingZip     string `json:"billing_zip" binding:"max=20"`
	Provider       string `json:"provider" binding:"max=50"`
}

// List 获取卡池列表（GET /api/admin/card-pool，卡号脱敏）
func (h *CardPoolHandler) List(c *gin.Context) {
	query := h.db.Model(&model.CardPoolEntry{}).Order("created_at DESC")

	if filter := c.Query("filter"); filter == "valid" {
		query = query.Where("is_valid = ?", true)
	} else if filter == "invalid" {
		query = query.Where("is_valid = ?", false)
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 50
	}

	var total int64
	query.Session(&gorm.Session{}).Count(&total)

	var entries []model.CardPoolEntry
	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&entries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "查询失败"})
		return
	}

	// 脱敏：卡号和 CVC
	for i := range entries {
		entries[i].CardNumber = entries[i].MaskedNumber()
		entries[i].CVC = "***"
	}
	c.JSON(http.StatusOK, gin.H{
		"items": entries, "total": total, "page": page, "page_size": pageSize,
	})
}

// Create 添加卡（POST /api/admin/card-pool）
func (h *CardPoolHandler) Create(c *gin.Context) {
	var req createCardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	// 去重
	var count int64
	h.db.Model(&model.CardPoolEntry{}).Where("card_number = ?", req.CardNumber).Count(&count)
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"detail": "该卡号已存在"})
		return
	}

	country := req.BillingCountry
	if country == "" {
		country = "US"
	}
	provider := req.Provider
	if provider == "" {
		provider = "manual"
	}

	entry := model.CardPoolEntry{
		Name: req.Name, CardNumber: req.CardNumber,
		ExpMonth: req.ExpMonth, ExpYear: req.ExpYear, CVC: req.CVC,
		BillingName: req.BillingName, BillingEmail: req.BillingEmail,
		BillingCountry: country, BillingCity: req.BillingCity,
		BillingLine1: req.BillingLine1, BillingZip: req.BillingZip,
		Provider: provider, Source: "manual",
	}
	if err := h.db.Create(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "创建失败"})
		return
	}
	h.cardPool.Reload()
	entry.CardNumber = entry.MaskedNumber()
	entry.CVC = "***"
	c.JSON(http.StatusOK, entry)
}

// Delete 删除卡（DELETE /api/admin/card-pool/:id）
func (h *CardPoolHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "无效的 ID"})
		return
	}
	result := h.db.Where("id = ?", id).Delete(&model.CardPoolEntry{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "删除失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"detail": "卡不存在"})
		return
	}
	h.cardPool.Reload()
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// BatchDelete 批量删除（POST /api/admin/card-pool/batch-delete）
func (h *CardPoolHandler) BatchDelete(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	if len(req.IDs) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "单次最多删除 500 条"})
		return
	}
	result := h.db.Where("id IN ?", req.IDs).Delete(&model.CardPoolEntry{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "删除失败"})
		return
	}
	h.cardPool.Reload()
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("已删除 %d 条", result.RowsAffected)})
}

// Import 批量导入卡（POST /api/admin/card-pool/import）
// 格式：每行一张卡，管道分隔 number|exp_month|exp_year|cvc[|name|email|country|city|line1|zip]
func (h *CardPoolHandler) Import(c *gin.Context) {
	var req struct {
		Cards    string `json:"cards" binding:"required"`
		Provider string `json:"provider"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}
	provider := req.Provider
	if provider == "" {
		provider = "manual"
	}

	scanner := bufio.NewScanner(strings.NewReader(req.Cards))
	var imported, skipped, failed int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if imported+skipped+failed >= 1000 {
			failed++
			continue
		}
		entry, err := parseCardLine(line, provider)
		if err != nil {
			failed++
			continue
		}
		var count int64
		h.db.Model(&model.CardPoolEntry{}).Where("card_number = ?", entry.CardNumber).Count(&count)
		if count > 0 {
			skipped++
			continue
		}
		if err := h.db.Create(entry).Error; err != nil {
			failed++
			continue
		}
		imported++
	}
	h.cardPool.Reload()
	c.JSON(http.StatusOK, gin.H{
		"imported": imported, "skipped": skipped, "failed": failed,
		"message": fmt.Sprintf("导入完成：成功 %d，重复跳过 %d，失败 %d", imported, skipped, failed),
	})
}

// parseCardLine 解析单行卡信息（管道分隔）
func parseCardLine(line, provider string) (*model.CardPoolEntry, error) {
	parts := strings.Split(line, "|")
	if len(parts) < 4 {
		return nil, fmt.Errorf("格式错误，至少需要 number|month|year|cvc")
	}
	month, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || month < 1 || month > 12 {
		return nil, fmt.Errorf("月份无效")
	}
	year, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil || year < 2024 {
		return nil, fmt.Errorf("年份无效")
	}
	entry := &model.CardPoolEntry{
		CardNumber: strings.TrimSpace(parts[0]),
		ExpMonth:   month,
		ExpYear:    year,
		CVC:        strings.TrimSpace(parts[3]),
		Provider:   provider,
		Source:     "imported",
		BillingCountry: "US",
	}
	if len(parts) > 4 { entry.BillingName = strings.TrimSpace(parts[4]) }
	if len(parts) > 5 { entry.BillingEmail = strings.TrimSpace(parts[5]) }
	if len(parts) > 6 { entry.BillingCountry = strings.TrimSpace(parts[6]) }
	if len(parts) > 7 { entry.BillingCity = strings.TrimSpace(parts[7]) }
	if len(parts) > 8 { entry.BillingLine1 = strings.TrimSpace(parts[8]) }
	if len(parts) > 9 { entry.BillingZip = strings.TrimSpace(parts[9]) }
	return entry, nil
}

// Validate 触发卡池验证（POST /api/admin/card-pool/validate）
func (h *CardPoolHandler) Validate(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	_ = c.ShouldBindJSON(&req)

	if len(req.IDs) > 0 {
		if len(req.IDs) > 500 {
			c.JSON(http.StatusBadRequest, gin.H{"detail": "单次最多验证 500 张"})
			return
		}
		total, valid, invalid := h.cardPool.ValidateByIDs(req.IDs)
		c.JSON(http.StatusOK, gin.H{
			"total": total, "valid": valid, "invalid": invalid,
			"message": fmt.Sprintf("验证完成：共 %d 张，有效 %d，无效 %d", total, valid, invalid),
		})
		return
	}

	total, valid, invalid := h.cardPool.ValidateAll()
	c.JSON(http.StatusOK, gin.H{
		"total": total, "valid": valid, "invalid": invalid,
		"message": fmt.Sprintf("验证完成：共 %d 张，有效 %d，无效 %d", total, valid, invalid),
	})
}

// Stats 卡池统计（GET /api/admin/card-pool/stats）
func (h *CardPoolHandler) Stats(c *gin.Context) {
	stats := h.cardPool.PoolStats()
	c.JSON(http.StatusOK, stats)
}

// PurgeInvalid 清除所有无效卡（POST /api/admin/card-pool/purge）
func (h *CardPoolHandler) PurgeInvalid(c *gin.Context) {
	result := h.db.Where("is_valid = ?", false).Delete(&model.CardPoolEntry{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "清除失败"})
		return
	}
	h.cardPool.Reload()
	c.JSON(http.StatusOK, gin.H{
		"deleted": result.RowsAffected,
		"message": fmt.Sprintf("已清除 %d 张无效卡", result.RowsAffected),
	})
}
