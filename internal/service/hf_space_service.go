package service

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	mathrand "math/rand"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	regplatform "github.com/xiaolajiaoyyds/regplatformm"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

// ── 常量：服务映射 + 随机词库 ──

// ServiceConfig HF Space 部署服务配置
type ServiceConfig struct {
	Dir       string // 模板目录（相对项目根）
	Script    string // 入口脚本文件名
	URLSecret string // Release URL 写入的 Secret key
}

// ServiceMap 支持的服务类型
var ServiceMap = map[string]ServiceConfig{
	"openai": {Dir: "HFNP", Script: "init.sh", URLSecret: "ARTIFACT_URL"},
	"grok":   {Dir: "HFGS", Script: "bootstrap.sh", URLSecret: "PKG_URL"},
	"kiro":   {Dir: "HFKR", Script: "start.sh", URLSecret: "MODEL_URL"},
	"gemini": {Dir: "HFGM", Script: "launch.sh", URLSecret: "GEMINI_URL"},
	"ts":     {Dir: "HFTS", Script: "run.sh", URLSecret: "DATA_URL"},
}

// CFEnvMap CF Worker 环境变量名映射
var CFEnvMap = map[string]string{
	"openai": "OPENAI_SPACES",
	"grok":   "GROK_SPACES",
	"kiro":   "KIRO_SPACES",
	"gemini": "GEMINI_SPACES",
	"ts":     "TS_SPACES",
}

// 随机化词库（ML/AI 风格，让 Space 名看起来像正经模型项目）
var adjectives = []string{
	// 模型规模/精度
	"tiny", "small", "base", "large", "mini", "nano", "micro", "lite",
	"slim", "fast", "quick", "swift", "lean", "dense", "deep", "wide",
	// ML 术语
	"linear", "focal", "modal", "latent", "sparse", "robust", "stable", "smooth",
	"neural", "causal", "masked", "frozen", "fused", "mixed", "multi", "cross",
	// 品质/性能
	"sharp", "clean", "clear", "smart", "bright", "vivid", "lucid", "crisp",
	"prime", "ultra", "hyper", "super", "turbo", "rapid", "agile", "steady",
	// 科学/数学
	"sigma", "theta", "omega", "delta", "alpha", "gamma", "beta", "zeta",
	"polar", "axial", "radial", "cubic", "quant", "logic", "optic", "sonic",
	// 自然/色调（保留部分通用词）
	"azure", "coral", "amber", "jade", "ivory", "slate", "olive", "cedar",
	"solar", "lunar", "frost", "storm", "misty", "boreal", "arctic", "ember",
}

var nouns = []string{
	// ML 核心概念
	"bert", "gpt", "llm", "vit", "clip", "lora", "rlhf", "sft",
	"embed", "token", "layer", "head", "block", "cell", "node", "unit",
	// 模型组件
	"encoder", "decoder", "adapter", "probe", "tuner", "mixer", "fuser", "pool",
	"gate", "lens", "core", "hub", "net", "lab", "bench", "forge",
	// 数据/训练
	"batch", "epoch", "loss", "grad", "norm", "step", "seed", "split",
	"cache", "index", "shard", "queue", "pipe", "flow", "stream", "relay",
	// 任务类型
	"chat", "gen", "cls", "ner", "qa", "sum", "seg", "det",
	"rank", "search", "match", "align", "parse", "tag", "scan", "eval",
	// 基础设施
	"serve", "infer", "train", "deploy", "scale", "sync", "route", "mesh",
	"edge", "grid", "rack", "dock", "bay", "pod", "slot", "depot",
	// 自然/通用（保留部分，让名字不全是术语）
	"bloom", "wave", "spark", "ridge", "peak", "vale", "drift", "cloud",
	"atlas", "nexus", "helix", "orbit", "prism", "pulse", "surge", "trace",
}

// ── SpaceHealthResult 健康检查结果 ──

// SpaceHealthResult 单个 Space 的健康检查结果
type SpaceHealthResult struct {
	URL        string `json:"url"`
	Healthy    bool   `json:"healthy"`
	Status     string `json:"status"`      // healthy/banned/sleeping/dead/unknown
	StatusCode int    `json:"status_code"`
	Reason     string `json:"reason"`
}

// OverviewItem 服务概览统计
type OverviewItem struct {
	Service  string `json:"service"`
	Total    int64  `json:"total"`
	Healthy  int64  `json:"healthy"`
	Building int64  `json:"building"`
	Banned   int64  `json:"banned"`
	Sleeping int64  `json:"sleeping"`
	Dead     int64  `json:"dead"`
	Unknown  int64  `json:"unknown"`
}

// AutoscaleLog 弹性管理操作日志
type AutoscaleLog struct {
	Step    string `json:"step"`
	Message string `json:"message"`
}

// AutoscaleResult 弹性管理结果
type AutoscaleResult struct {
	Service     string         `json:"service"`
	DryRun      bool           `json:"dry_run"`
	Before      int            `json:"before"`
	After       int            `json:"after"`
	Created     int            `json:"created"`
	Deleted     int            `json:"deleted"`
	HealthyNow  int            `json:"healthy_now"`
	Logs        []AutoscaleLog `json:"logs"`
}

// ── HFSpaceService ──

// HFSpaceService HF Space 管理服务
type HFSpaceService struct {
	db         *gorm.DB
	settingSvc *SettingService
	httpClient *http.Client
}

// NewHFSpaceService 创建 HF Space 管理服务
func NewHFSpaceService(db *gorm.DB, settingSvc *SettingService) *HFSpaceService {
	return &HFSpaceService{
		db:         db,
		settingSvc: settingSvc,
		httpClient: &http.Client{Timeout: 60 * time.Second}, // HF Commit API 上传文件耗时较长
	}
}

// ── Token 管理 ──

// CreateToken 创建 HF Token，调用 whoami 验证并填充 username
func (s *HFSpaceService) CreateToken(label, token string) (*model.HFToken, error) {
	// 调 HF whoami 验证 Token
	username, err := s.hfWhoami(token)
	if err != nil {
		return nil, fmt.Errorf("Token 验证失败: %w", err)
	}

	t := &model.HFToken{
		Label:    label,
		Token:    token,
		Username: username,
		IsValid:  true,
	}
	if err := s.db.Create(t).Error; err != nil {
		return nil, fmt.Errorf("创建 Token 失败: %w", err)
	}
	return t, nil
}

// ListTokens 列出所有 Token（token 字段脱敏）
func (s *HFSpaceService) ListTokens() ([]model.HFToken, error) {
	var tokens []model.HFToken
	if err := s.db.Order("id ASC").Find(&tokens).Error; err != nil {
		return nil, err
	}
	// 统计每个 Token 关联的 Space 数量
	for i := range tokens {
		var count int64
		s.db.Model(&model.HFSpace{}).Where("token_id = ?", tokens[i].ID).Count(&count)
		tokens[i].SpaceUsed = int(count)
		// 脱敏
		if len(tokens[i].Token) > 8 {
			tokens[i].Token = tokens[i].Token[:4] + "****" + tokens[i].Token[len(tokens[i].Token)-4:]
		}
	}
	return tokens, nil
}

// DeleteToken 删除 Token，关联 Space 的 token_id 置 0
func (s *HFSpaceService) DeleteToken(id uint) error {
	var token model.HFToken
	if err := s.db.First(&token, id).Error; err != nil {
		return fmt.Errorf("Token 不存在")
	}
	// 关联 Space 标记为 orphan
	s.db.Model(&model.HFSpace{}).Where("token_id = ?", id).Update("token_id", 0)
	return s.db.Delete(&token).Error
}

// ValidateToken 重新验证 Token
func (s *HFSpaceService) ValidateToken(id uint) (*model.HFToken, error) {
	var token model.HFToken
	if err := s.db.First(&token, id).Error; err != nil {
		return nil, fmt.Errorf("Token 不存在")
	}
	username, err := s.hfWhoami(token.Token)
	if err != nil {
		s.db.Model(&token).Updates(map[string]interface{}{"is_valid": false})
		token.IsValid = false
		return &token, nil
	}
	s.db.Model(&token).Updates(map[string]interface{}{"is_valid": true, "username": username})
	token.IsValid = true
	token.Username = username
	return &token, nil
}

// ValidateAllTokens 批量验证所有 Token
func (s *HFSpaceService) ValidateAllTokens() ([]model.HFToken, error) {
	var tokens []model.HFToken
	if err := s.db.Find(&tokens).Error; err != nil {
		return nil, err
	}
	for i := range tokens {
		username, err := s.hfWhoami(tokens[i].Token)
		if err != nil {
			s.db.Model(&tokens[i]).Update("is_valid", false)
			tokens[i].IsValid = false
		} else {
			s.db.Model(&tokens[i]).Updates(map[string]interface{}{"is_valid": true, "username": username})
			tokens[i].IsValid = true
			tokens[i].Username = username
		}
	}
	return tokens, nil
}

// PurgeBannedSpaces 清理不可用 Space 的完整流程：
// 1. 健康检查（含 Runtime API 二次确认）
// 2. banned/sleeping 直接删除（被封或休眠的启动了也是 503）
// 3. dead 先尝试 restart → 等待 → 再检查 → 还是挂才删
func (s *HFSpaceService) PurgeBannedSpaces(service string) (deleted int, checked int, err error) {
	// 第一轮：健康检查
	results, err := s.CheckHealth(service)
	if err != nil {
		return 0, 0, err
	}
	checked = len(results)

	// banned/sleeping 直接删，不浪费时间 restart
	var directDelSpaces []model.HFSpace
	directDelQuery := s.db.Where("status IN ?", []string{"banned", "sleeping"})
	if service != "" {
		directDelQuery = directDelQuery.Where("service = ?", service)
	}
	directDelQuery.Find(&directDelSpaces)
	for _, sp := range directDelSpaces {
		if delErr := s.DeleteSpace(sp.ID); delErr != nil {
			log.Warn().Uint("space_id", sp.ID).Str("repo", sp.RepoID).Str("status", sp.Status).Err(delErr).Msg("清理不可用 Space 失败")
			continue
		}
		log.Info().Uint("space_id", sp.ID).Str("repo", sp.RepoID).Str("status", sp.Status).Msg("已清理不可用 Space")
		deleted++
	}

	// dead 的先尝试 restart
	var deadSpaces []model.HFSpace
	deadQuery := s.db.Where("status = ?", "dead")
	if service != "" {
		deadQuery = deadQuery.Where("service = ?", service)
	}
	deadQuery.Find(&deadSpaces)

	if len(deadSpaces) == 0 {
		return deleted, checked, nil
	}

	// 预加载 token
	var allTokens []model.HFToken
	s.db.Where("is_valid = ?", true).Find(&allTokens)
	tokenByID := make(map[uint]string)
	tokenByUser := make(map[string]string)
	for _, t := range allTokens {
		tokenByID[t.ID] = t.Token
		tokenByUser[t.Username] = t.Token
	}

	// 尝试 restart 所有 dead Space
	restartedIDs := make([]uint, 0, len(deadSpaces))
	for _, sp := range deadSpaces {
		token := tokenByID[sp.TokenID]
		if token == "" {
			owner := strings.SplitN(sp.RepoID, "/", 2)[0]
			token = tokenByUser[owner]
		}
		if token == "" {
			log.Warn().Str("repo", sp.RepoID).Msg("无可用 Token，跳过 restart 直接删除")
			if delErr := s.DeleteSpace(sp.ID); delErr == nil {
				deleted++
			}
			continue
		}
		if err := s.hfRestartSpace(token, sp.RepoID, false); err != nil {
			log.Warn().Str("repo", sp.RepoID).Err(err).Msg("restart 失败，直接删除")
			if delErr := s.DeleteSpace(sp.ID); delErr == nil {
				deleted++
			}
			continue
		}
		log.Info().Str("repo", sp.RepoID).Msg("已发送 restart，等待确认")
		restartedIDs = append(restartedIDs, sp.ID)
	}

	if len(restartedIDs) == 0 {
		return deleted, checked, nil
	}

	// 等待 30 秒让 Space 有时间启动
	waitSec := s.settingSvc.GetInt("hf_restart_wait_sec", 30)
	log.Info().Int("wait_sec", waitSec).Int("restarted", len(restartedIDs)).Msg("等待 restart 生效...")
	time.Sleep(time.Duration(waitSec) * time.Second)

	// 第二轮：重新检查 restarted 的 Space
	for _, spID := range restartedIDs {
		var sp model.HFSpace
		if s.db.First(&sp, spID).Error != nil {
			continue
		}
		token := tokenByID[sp.TokenID]
		if token == "" {
			owner := strings.SplitN(sp.RepoID, "/", 2)[0]
			token = tokenByUser[owner]
		}

		stillDead := true
		if token != "" {
			if stage, err := s.hfGetSpaceRuntime(token, sp.RepoID); err == nil {
				upper := strings.ToUpper(stage)
				// RUNNING/BUILDING 说明 restart 成功了
				if upper == "RUNNING" || upper == "RUNNING_BUILDING" || upper == "BUILDING" {
					stillDead = false
					newStatus := "healthy"
					if upper == "BUILDING" {
						newStatus = "building"
					}
					now := time.Now()
					s.db.Model(&sp).Updates(map[string]interface{}{
						"status":      newStatus,
						"last_check_at": &now,
					})
					log.Info().Str("repo", sp.RepoID).Str("stage", stage).Msg("restart 成功，Space 已恢复")
				}
			}
		}

		if stillDead {
			log.Info().Str("repo", sp.RepoID).Msg("restart 后仍不可用，删除")
			if delErr := s.DeleteSpace(sp.ID); delErr == nil {
				deleted++
			}
		}
	}

	return deleted, checked, nil
}

// ── Space 管理 ──

// SpaceListResult 分页结果
type SpaceListResult struct {
	Items []model.HFSpace `json:"items"`
	Total int64           `json:"total"`
}

// ListSpaces 按服务列出 Space（支持分页，page/pageSize 为 0 则不分页）
func (s *HFSpaceService) ListSpaces(service string, page, pageSize int, status string) (*SpaceListResult, error) {
	query := s.db.Model(&model.HFSpace{})
	if service != "" {
		query = query.Where("service = ?", service)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	dataQuery := s.db.Order("id ASC")
	if service != "" {
		dataQuery = dataQuery.Where("service = ?", service)
	}
	if status != "" {
		dataQuery = dataQuery.Where("status = ?", status)
	}
	if page > 0 && pageSize > 0 {
		dataQuery = dataQuery.Offset((page - 1) * pageSize).Limit(pageSize)
	}

	var spaces []model.HFSpace
	if err := dataQuery.Find(&spaces).Error; err != nil {
		return nil, err
	}
	return &SpaceListResult{Items: spaces, Total: total}, nil
}

// AddSpaceManual 手动添加已有 Space
func (s *HFSpaceService) AddSpaceManual(service, url, repoID string, tokenID uint) (*model.HFSpace, error) {
	space := &model.HFSpace{
		Service: service,
		URL:     url,
		RepoID:  repoID,
		TokenID: tokenID,
		Status:  "unknown",
	}
	if err := s.db.Create(space).Error; err != nil {
		return nil, fmt.Errorf("添加 Space 失败: %w", err)
	}
	return space, nil
}

// DeleteSpace 删除 Space（可选调 HF API 删除远程 repo）
// 增强：关联 token 失效时，按 repo owner 查找其他可用 token 重试
func (s *HFSpaceService) DeleteSpace(id uint) error {
	var space model.HFSpace
	if err := s.db.First(&space, id).Error; err != nil {
		return fmt.Errorf("Space 不存在")
	}

	// 尝试通过 HF API 删除远程 repo
	if space.RepoID != "" {
		deleted := false

		// 优先用关联 token
		if space.TokenID > 0 {
			var token model.HFToken
			if s.db.First(&token, space.TokenID).Error == nil && token.IsValid {
				if err := s.hfDeleteRepo(token.Token, space.RepoID); err == nil {
					deleted = true
				} else {
					log.Warn().Str("repo", space.RepoID).Err(err).Msg("关联 Token 删除远程 repo 失败，尝试按 owner 查找")
				}
			}
		}

		// 关联 token 失败，按 repo owner 找其他可用 token
		if !deleted {
			owner := strings.SplitN(space.RepoID, "/", 2)[0]
			var fallbackToken model.HFToken
			if s.db.Where("username = ? AND is_valid = ?", owner, true).First(&fallbackToken).Error == nil {
				if err := s.hfDeleteRepo(fallbackToken.Token, space.RepoID); err != nil {
					log.Warn().Str("repo", space.RepoID).Err(err).Msg("备选 Token 删除远程 repo 也失败，仅删除 DB 记录")
				}
			}
		}
	}

	return s.db.Delete(&space).Error
}

// removeSpaceDBOnly 仅从 DB 删除 Space 记录，不调 HF API 删除远程 repo
// 用于弹性管理清理死节点——保留 HF 上的 repo，下次 discover 可重新导入
func (s *HFSpaceService) removeSpaceDBOnly(id uint) error {
	return s.db.Delete(&model.HFSpace{}, id).Error
}

// CheckHealth 并发健康检查指定服务的所有 Space
func (s *HFSpaceService) CheckHealth(service string) ([]SpaceHealthResult, error) {
	var spaces []model.HFSpace
	query := s.db
	if service != "" {
		query = query.Where("service = ?", service)
	}
	if err := query.Find(&spaces).Error; err != nil {
		return nil, err
	}

	timeout := s.settingSvc.GetInt("hf_health_timeout", 8)
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}

	results := make([]SpaceHealthResult, len(spaces))

	g := new(errgroup.Group)
	g.SetLimit(20) // 并发上限 20

	// 预加载 token map（token_id → token 明文 + username → token 明文）
	var allTokens []model.HFToken
	s.db.Where("is_valid = ?", true).Find(&allTokens)
	tokenByID := make(map[uint]string)
	tokenByUser := make(map[string]string)
	for _, t := range allTokens {
		tokenByID[t.ID] = t.Token
		tokenByUser[t.Username] = t.Token
	}

	for i, sp := range spaces {
		i, sp := i, sp
		g.Go(func() error {
			result := s.checkOneHealth(client, sp.URL)

			// 对非 healthy 的 Space，调 HF Runtime API 二次确认真实 stage
			if !result.Healthy && sp.RepoID != "" {
				token := tokenByID[sp.TokenID]
				if token == "" {
					// token_id 对应的 token 失效，按 username 找
					owner := strings.SplitN(sp.RepoID, "/", 2)[0]
					token = tokenByUser[owner]
				}
				if token != "" {
					if stage, err := s.hfGetSpaceRuntime(token, sp.RepoID); err == nil && stage != "" {
						// 根据 HF 真实 stage 精确映射状态
						switch strings.ToUpper(stage) {
						case "RUNNING", "RUNNING_BUILDING":
							// Runtime API 说在跑，可能是 /health 端点还没就绪
							result.Status = "healthy"
							result.Healthy = true
							result.Reason = "runtime_running"
						case "BUILDING":
							result.Status = "building"
							result.Reason = "building"
						case "PAUSED":
							result.Status = "sleeping"
							result.Reason = "paused"
						case "STOPPED":
							result.Status = "dead"
							result.Reason = "stopped"
						case "RUNTIME_ERROR":
							result.Status = "dead"
							result.Reason = "runtime_error"
						case "BUILD_ERROR":
							result.Status = "dead"
							result.Reason = "build_error"
						case "CONFIG_ERROR":
							result.Status = "dead"
							result.Reason = "config_error"
						case "NO_APP_FILE":
							result.Status = "dead"
							result.Reason = "no_app_file"
						case "DELETING":
							result.Status = "dead"
							result.Reason = "deleting"
						}
						log.Debug().Str("repo", sp.RepoID).Str("stage", stage).Str("status", result.Status).Msg("Runtime API 二次确认")
					}
				}
			}

			results[i] = result

			// 更新 DB 状态
			now := time.Now()
			s.db.Model(&sp).Updates(map[string]interface{}{
				"status":        result.Status,
				"status_code":   result.StatusCode,
				"last_check_at": &now,
			})
			return nil
		})
	}
	_ = g.Wait()

	return results, nil
}

// Overview 各服务汇总统计（单次 GROUP BY 替代多次 COUNT）
func (s *HFSpaceService) Overview() ([]OverviewItem, error) {
	// 一次查出所有服务+状态的计数
	type row struct {
		Service string
		Status  string
		Cnt     int64
	}
	var rows []row
	if err := s.db.Model(&model.HFSpace{}).
		Select("service, status, COUNT(*) as cnt").
		Group("service, status").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	// 初始化各服务的统计
	serviceOrder := []string{"openai", "grok", "kiro", "gemini", "ts"}
	itemMap := make(map[string]*OverviewItem, len(serviceOrder))
	for _, svc := range serviceOrder {
		itemMap[svc] = &OverviewItem{Service: svc}
	}

	// 填充计数
	for _, r := range rows {
		item, ok := itemMap[r.Service]
		if !ok {
			continue // 跳过未知服务
		}
		item.Total += r.Cnt
		switch r.Status {
		case "healthy":
			item.Healthy = r.Cnt
		case "building":
			item.Building = r.Cnt
		case "banned":
			item.Banned = r.Cnt
		case "sleeping":
			item.Sleeping = r.Cnt
		case "dead":
			item.Dead = r.Cnt
		case "unknown":
			item.Unknown = r.Cnt
		}
	}

	items := make([]OverviewItem, 0, len(serviceOrder))
	for _, svc := range serviceOrder {
		items = append(items, *itemMap[svc])
	}
	return items, nil
}

// ── 弹性管理 ──

// Autoscale 弹性管理：健康检查 → 清理死节点（先 restart 再删） → 自动扩容 → 同步 CF → 同步 Pool
func (s *HFSpaceService) Autoscale(service string, target int, dryRun bool) (*AutoscaleResult, error) {
	if _, ok := ServiceMap[service]; !ok {
		return nil, fmt.Errorf("未知服务类型: %s", service)
	}
	if target <= 0 {
		target = s.settingSvc.GetInt("hf_default_target", 5)
	}

	result := &AutoscaleResult{
		Service: service,
		DryRun:  dryRun,
		Logs:    make([]AutoscaleLog, 0),
	}

	var spaces []model.HFSpace
	s.db.Where("service = ?", service).Find(&spaces)
	result.Before = len(spaces)

	addLog := func(step, msg string) {
		result.Logs = append(result.Logs, AutoscaleLog{Step: step, Message: msg})
		log.Info().Str("service", service).Str("step", step).Msg(msg)
	}

	// 1. 健康检查（含 Runtime API 二次确认）
	addLog("health_check", fmt.Sprintf("检查 %d 个 Space...", len(spaces)))
	s.CheckHealth(service)

	// 重新加载（CheckHealth 已更新 DB status）
	s.db.Where("service = ?", service).Find(&spaces)
	var healthyCnt, bannedCnt, sleepingCnt, deadCnt, buildingCnt int
	for _, sp := range spaces {
		switch sp.Status {
		case "healthy":
			healthyCnt++
		case "banned":
			bannedCnt++
		case "sleeping":
			sleepingCnt++
		case "building":
			buildingCnt++
		default:
			deadCnt++
		}
	}
	addLog("health_result", fmt.Sprintf("healthy=%d banned=%d sleeping=%d building=%d dead=%d",
		healthyCnt, bannedCnt, sleepingCnt, buildingCnt, deadCnt))

	// 2. 清理不可用节点
	// 预加载 token
	var allTokens []model.HFToken
	s.db.Where("is_valid = ?", true).Find(&allTokens)
	tokenByID := make(map[uint]string)
	tokenByUser := make(map[string]string)
	for _, t := range allTokens {
		tokenByID[t.ID] = t.Token
		tokenByUser[t.Username] = t.Token
	}

	// 2a. banned/sleeping 直接删（被封或休眠的启动了也是 503）
	var directDelSpaces []model.HFSpace
	s.db.Where("service = ? AND status IN ?", service, []string{"banned", "sleeping"}).Find(&directDelSpaces)
	if len(directDelSpaces) > 0 {
		addLog("cleanup", fmt.Sprintf("清理 %d 个 banned/sleeping 节点", len(directDelSpaces)))
		if !dryRun {
			for _, sp := range directDelSpaces {
				s.DeleteSpace(sp.ID)
				result.Deleted++
			}
		}
	}

	// 2b. dead 先尝试 restart → 等待 → 再检查 → 还是挂才删
	var deadSpaces []model.HFSpace
	s.db.Where("service = ? AND status = ?", service, "dead").Find(&deadSpaces)
	if len(deadSpaces) > 0 {
		addLog("cleanup", fmt.Sprintf("尝试 restart %d 个 dead 节点...", len(deadSpaces)))
		if !dryRun {
			restartedIDs := make([]uint, 0, len(deadSpaces))
			for _, sp := range deadSpaces {
				token := tokenByID[sp.TokenID]
				if token == "" {
					owner := strings.SplitN(sp.RepoID, "/", 2)[0]
					token = tokenByUser[owner]
				}
				if token == "" {
					s.DeleteSpace(sp.ID)
					result.Deleted++
					continue
				}
				if err := s.hfRestartSpace(token, sp.RepoID, false); err != nil {
					addLog("cleanup", fmt.Sprintf("%s restart 失败: %v，删除", sp.RepoID, err))
					s.DeleteSpace(sp.ID)
					result.Deleted++
					continue
				}
				restartedIDs = append(restartedIDs, sp.ID)
			}

			if len(restartedIDs) > 0 {
				waitSec := s.settingSvc.GetInt("hf_restart_wait_sec", 30)
				addLog("cleanup", fmt.Sprintf("等待 %ds 确认 %d 个 restart 结果...", waitSec, len(restartedIDs)))
				time.Sleep(time.Duration(waitSec) * time.Second)

				for _, spID := range restartedIDs {
					var sp model.HFSpace
					if s.db.First(&sp, spID).Error != nil {
						continue
					}
					token := tokenByID[sp.TokenID]
					if token == "" {
						owner := strings.SplitN(sp.RepoID, "/", 2)[0]
						token = tokenByUser[owner]
					}
					stillDead := true
					if token != "" {
						if stage, err := s.hfGetSpaceRuntime(token, sp.RepoID); err == nil {
							upper := strings.ToUpper(stage)
							if upper == "RUNNING" || upper == "RUNNING_BUILDING" || upper == "BUILDING" {
								stillDead = false
								newStatus := "healthy"
								if upper == "BUILDING" {
									newStatus = "building"
								}
								now := time.Now()
								s.db.Model(&sp).Updates(map[string]interface{}{
									"status":        newStatus,
									"last_check_at": &now,
								})
								addLog("cleanup", fmt.Sprintf("%s restart 成功 (stage=%s)", sp.RepoID, stage))
								if newStatus == "healthy" {
									healthyCnt++
								}
							}
						}
					}
					if stillDead {
						addLog("cleanup", fmt.Sprintf("%s restart 后仍不可用，删除", sp.RepoID))
						s.DeleteSpace(sp.ID)
						result.Deleted++
					}
				}
			}
		} else {
			result.Deleted = len(deadSpaces)
		}
	}

	// 3. 自动扩容：healthy < target 时自动部署补充
	if healthyCnt < target {
		needCreate := target - healthyCnt
		releaseURL := s.settingSvc.Get(fmt.Sprintf("hf_release_url_%s", service), "")
		if releaseURL == "" {
			addLog("scale_up", fmt.Sprintf("需补充 %d 个，但未配置 hf_release_url_%s，请先在系统设置填写 Release URL", needCreate, service))
		} else {
			addLog("scale_up", fmt.Sprintf("自动部署 %d 个新 Space", needCreate))
			if !dryRun {
				deployed, deployErrors, err := s.DeploySpaces(service, needCreate, releaseURL, 0, nil)
				if err != nil {
					addLog("scale_up", fmt.Sprintf("部署失败: %v", err))
				} else {
					result.Created = len(deployed)
					for _, e := range deployErrors {
						addLog("scale_up", e)
					}
					addLog("scale_up", fmt.Sprintf("成功 %d 个，失败 %d 个", len(deployed), len(deployErrors)))
				}
			} else {
				result.Created = needCreate
			}
		}
	} else {
		addLog("scale_up", fmt.Sprintf("healthy=%d >= target=%d，无需扩容", healthyCnt, target))
	}

	// 4. 同步 CF Worker
	if !dryRun {
		if _, err := s.SyncCFWorker(service); err != nil {
			addLog("sync_cf", fmt.Sprintf("CF 同步失败: %v", err))
		} else {
			addLog("sync_cf", "CF Worker 环境变量已更新")
		}
	}

	// 5. 同步 Pool 大小
	if !dryRun {
		s.SyncPoolSize()
		addLog("sync_pool", "调度池大小已同步")
	}

	// 最终统计
	var finalCount int64
	s.db.Model(&model.HFSpace{}).Where("service = ?", service).Count(&finalCount)
	result.After = int(finalCount)

	var finalHealthy int64
	s.db.Model(&model.HFSpace{}).Where("service = ? AND status = ?", service, "healthy").Count(&finalHealthy)
	result.HealthyNow = int(finalHealthy)

	return result, nil
}

// SyncCFResult 同步结果
type SyncCFResult struct {
	Service string `json:"service"`
	EnvKey  string `json:"env_key"`
	URLs    int    `json:"urls"`
}

// SyncCFWorker 读 DB 健康 Space URL → PATCH CF Worker env
func (s *HFSpaceService) SyncCFWorker(service string) (*SyncCFResult, error) {
	cfAccountID := s.settingSvc.Get("hf_cf_account_id", "")
	cfAPIToken := s.settingSvc.GetRaw("hf_cf_api_token")
	cfWorkerName := s.settingSvc.Get("hf_cf_worker_name", "hf-snow-worker")

	if cfAccountID == "" || cfAPIToken == "" {
		return nil, fmt.Errorf("CF 配置不完整（account_id 或 api_token 为空）")
	}

	envKey, ok := CFEnvMap[service]
	if !ok {
		return nil, fmt.Errorf("未知服务: %s", service)
	}

	// 获取健康 Space URL
	var spaces []model.HFSpace
	s.db.Where("service = ? AND status = ?", service, "healthy").Find(&spaces)
	urls := make([]string, 0, len(spaces))
	for _, sp := range spaces {
		urls = append(urls, sp.URL)
	}

	// GET 当前 Worker settings
	settingsURL := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/accounts/%s/workers/scripts/%s/settings",
		cfAccountID, cfWorkerName,
	)

	req, _ := http.NewRequest("GET", settingsURL, nil)
	req.Header.Set("Authorization", "Bearer "+cfAPIToken)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CF GET settings 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("CF GET settings 返回 %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}

	var cfResp struct {
		Result struct {
			Bindings []map[string]interface{} `json:"bindings"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return nil, fmt.Errorf("CF 响应解析失败: %w", err)
	}

	// 更新或添加目标 env var
	value := strings.Join(urls, " ")
	found := false
	for _, b := range cfResp.Result.Bindings {
		if b["type"] == "plain_text" && b["name"] == envKey {
			b["text"] = value
			found = true
			break
		}
	}
	if !found {
		cfResp.Result.Bindings = append(cfResp.Result.Bindings, map[string]interface{}{
			"type": "plain_text",
			"name": envKey,
			"text": value,
		})
	}

	// PATCH 回去（CF Workers Settings API 要求 multipart/form-data）
	patchBody, _ := json.Marshal(map[string]interface{}{
		"bindings": cfResp.Result.Bindings,
	})
	var mpBuf bytes.Buffer
	mpWriter := multipart.NewWriter(&mpBuf)
	settingsPart, _ := mpWriter.CreateFormField("settings")
	settingsPart.Write(patchBody)
	mpWriter.Close()

	patchReq, _ := http.NewRequest("PATCH", settingsURL, &mpBuf)
	patchReq.Header.Set("Authorization", "Bearer "+cfAPIToken)
	patchReq.Header.Set("Content-Type", mpWriter.FormDataContentType())
	patchResp, err := s.httpClient.Do(patchReq)
	if err != nil {
		return nil, fmt.Errorf("CF PATCH 失败: %w", err)
	}
	defer patchResp.Body.Close()

	if patchResp.StatusCode != 200 {
		body, _ := io.ReadAll(patchResp.Body)
		return nil, fmt.Errorf("CF PATCH 返回 %d: %s", patchResp.StatusCode, string(body[:min(len(body), 200)]))
	}

	log.Info().Str("service", service).Str("env_key", envKey).Int("urls", len(urls)).Msg("CF Worker env 已更新")
	return &SyncCFResult{Service: service, EnvKey: envKey, URLs: len(urls)}, nil
}

// SyncPoolSize 基于健康节点数自动调优全部调度参数
// 在 HF Space 同步、健康检查、CF Worker 同步后自动调用
//
// 算法：
//   - 每个 2C16G 节点承载 concurrencyPerNode 个并发注册
//   - 实际使用 utilizationRatio（0.9）比例，HF Space 自身 503 过载保护已兜底
//   - 单任务并发 = 总容量 / max(活跃用户数, 2)（动态分配，低用户时充分利用节点）
//   - 同时用户数 = 总容量 / 单任务并发（下限 3）
//   - 额外识别 reg_url 中的直连节点（非 HF CF Worker），计入容量
func (s *HFSpaceService) SyncPoolSize() {
	// ── 1. 统计各平台健康 HF Space 节点数 ──
	type svcCount struct {
		Service string
		Count   int64
	}
	var counts []svcCount
	s.db.Model(&model.HFSpace{}).
		Select("service, count(*) as count").
		Where("status = ?", "healthy").
		Group("service").
		Scan(&counts)

	healthyMap := make(map[string]int)
	var totalHealthy int
	for _, c := range counts {
		healthyMap[c.Service] = int(c.Count)
		totalHealthy += int(c.Count)
	}
	if totalHealthy <= 0 {
		return
	}

	// ── 2. 识别直连节点（Mac Mini 等，逗号分隔 URL 中非 hf- 开头的域名） ──
	registrationPlatforms := []string{"grok", "openai", "kiro", "gemini"}
	for _, p := range registrationPlatforms {
		regURL := s.settingSvc.Get(p+"_reg_url", "")
		if regURL == "" {
			continue
		}
		parts := strings.Split(regURL, ",")
		for _, u := range parts {
			u = strings.TrimSpace(u)
			// CF Worker 入口 URL（含 hf- 前缀）由 HF Space 提供容量，不重复计算
			// 其余 URL 视为独立直连节点
			if u != "" && !strings.Contains(u, "hf-") {
				healthyMap[p]++
			}
		}
	}

	// ── 3. 调优常量 ──
	const (
		concurrencyPerNode = 5    // 单节点（2C16G）并发注册数
		utilizationRatio   = 0.9  // 使用 90% 容量，HF Space 自身 503 + CF Worker 切节点已兜底
		minConcurrent      = 5    // 平台最低总并发
		maxParallelCap     = 40   // 单任务并发上限（单人场景可充分利用节点池）
		minParallel        = 3    // 单任务并发下限
		workersPerInst     = 5    // 每实例 Pool Worker 数
	)

	// ── 3.5 查询各平台当前活跃任务数（用于动态分配单任务并发） ──
	type platformActive struct {
		Platform string
		Count    int64
	}
	var activeCounts []platformActive
	s.db.Raw(`SELECT platform, COUNT(*) as count FROM tasks WHERE status = 'running' GROUP BY platform`).Scan(&activeCounts)
	activeMap := make(map[string]int)
	for _, a := range activeCounts {
		activeMap[a.Platform] = int(a.Count)
	}

	// ── 4. 按平台计算并写入 ──
	totalMaxConcurrent := 0
	maxPlatformUsers := 0

	for _, p := range registrationPlatforms {
		nodes := healthyMap[p]
		if nodes <= 0 {
			continue
		}

		// 总并发 = 节点 × 单节点并发 × 利用率
		maxConcurrent := int(float64(nodes*concurrencyPerNode) * utilizationRatio)
		if maxConcurrent < minConcurrent {
			maxConcurrent = minConcurrent
		}

		// 自适应单任务并发分配：
		//   0 个活跃用户 → 给 80% 容量（预留 20% 余量给随时可能进来的新用户）
		//   1 个活跃用户 → 给 80% 容量（同上，新用户进来后 10s 内 RefreshLimits 会自动收缩）
		//   2+ 个活跃用户 → 公平均分（总容量 / 活跃数）
		// HF Space 的 503 过载保护 + CF Worker 自动切节点 作为最终安全网
		activeUsers := activeMap[p]
		var maxParallel int
		if activeUsers <= 1 {
			// 单人或无人：给 80% 容量，不浪费节点
			maxParallel = maxConcurrent * 4 / 5
		} else {
			// 多人：公平均分
			maxParallel = maxConcurrent / activeUsers
		}
		if maxParallel > maxParallelCap {
			maxParallel = maxParallelCap
		}
		if maxParallel < minParallel {
			maxParallel = minParallel
		}

		// 同时用户数 = 总容量 / 单任务并发（下限 2）
		maxUsers := maxConcurrent / maxParallel
		if maxUsers < 2 {
			maxUsers = 2
		}

		_ = s.settingSvc.Set(p+"_max_concurrent", strconv.Itoa(maxConcurrent))
		_ = s.settingSvc.Set(p+"_max_parallel", strconv.Itoa(maxParallel))
		_ = s.settingSvc.Set(p+"_max_concurrent_users", strconv.Itoa(maxUsers))

		totalMaxConcurrent += maxConcurrent
		if maxUsers > maxPlatformUsers {
			maxPlatformUsers = maxUsers
		}

		log.Info().
			Str("platform", p).
			Int("healthy_nodes", nodes).
			Int("max_concurrent", maxConcurrent).
			Int("max_parallel", maxParallel).
			Int("max_concurrent_users", maxUsers).
			Msg("平台参数已自动调优")
	}

	// ── 5. 全局参数 ──
	_ = s.settingSvc.Set("hf_instance_count", strconv.Itoa(totalHealthy))
	_ = s.settingSvc.Set("workers_per_instance", strconv.Itoa(workersPerInst))

	// 全局最大并发任务 = 各平台并发总和 × 1.2
	maxTasks := int(float64(totalMaxConcurrent) * 1.2)
	if maxTasks < 30 {
		maxTasks = 30
	}
	_ = s.settingSvc.Set("max_concurrent_tasks", strconv.Itoa(maxTasks))

	// 全局最大同时用户 = 各平台最大值 + 2（余量）
	globalMaxUsers := maxPlatformUsers + 2
	_ = s.settingSvc.Set("max_concurrent_users", strconv.Itoa(globalMaxUsers))

	// ── 6. max_threads：取各平台 max_parallel 最大值 ──
	maxThreads := 0
	for _, p := range registrationPlatforms {
		v, _ := strconv.Atoi(s.settingSvc.Get(p+"_max_parallel", "3"))
		if v > maxThreads {
			maxThreads = v
		}
	}
	if maxThreads < 4 {
		maxThreads = 4
	}
	_ = s.settingSvc.Set("max_threads", strconv.Itoa(maxThreads))

	// ── 7. thread_tiers：按 max_threads 自动生成分段并发策略 ──
	// 小目标少线程（省资源），大目标逐步拉满
	// 策略：target ≤ 5 → 2线程, ≤ 15 → ceil(max/3), ≤ 30 → ceil(max/2),
	//       ≤ 60 → max-2, ≤ 100 → max-1, ≤ 200 → max, ≤ 500 → max
	type tier struct {
		Max     int `json:"max"`
		Threads int `json:"threads"`
	}
	mt := maxThreads
	tiers := []tier{
		{Max: 5, Threads: clampInt(2, 1, mt)},
		{Max: 15, Threads: clampInt((mt+2)/3, 2, mt)},
		{Max: 30, Threads: clampInt((mt+1)/2, 2, mt)},
		{Max: 60, Threads: clampInt(mt-2, 3, mt)},
		{Max: 100, Threads: clampInt(mt-1, 3, mt)},
		{Max: 200, Threads: mt},
		{Max: 500, Threads: mt},
	}
	tiersJSON, _ := json.Marshal(tiers)
	_ = s.settingSvc.Set("thread_tiers", string(tiersJSON))

	log.Info().
		Int("total_healthy", totalHealthy).
		Int("pool_workers", totalHealthy*workersPerInst).
		Int("max_concurrent_tasks", maxTasks).
		Int("max_concurrent_users", globalMaxUsers).
		Int("max_threads", maxThreads).
		Str("thread_tiers", string(tiersJSON)).
		Msg("全局调度参数已自动调优")
}

// clampInt 限制 v 在 [lo, hi] 范围内
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// UpdateSpaceResult 批量更新结果
type UpdateSpaceResult struct {
	Updated int      `json:"updated"`
	Failed  int      `json:"failed"`
	Logs    []string `json:"logs"`
}

// UpdateSpaces 对已有 Space 推送最新 Dockerfile + 入口脚本（触发重建），同步 Release URL Secret
// service="all" 时 4 种服务并行更新，避免串行超时
func (s *HFSpaceService) UpdateSpaces(service string) (*UpdateSpaceResult, error) {
	services := []string{service}
	if service == "all" {
		services = []string{"openai", "grok", "kiro", "gemini", "ts"}
	}

	result := &UpdateSpaceResult{Logs: []string{}}
	var mu sync.Mutex

	var wg sync.WaitGroup
	for _, svc := range services {
		wg.Add(1)
		go func(svc string) {
			defer wg.Done()
			s.updateOneService(svc, result, &mu)
		}(svc)
	}
	wg.Wait()

	return result, nil
}

// updateOneService 更新单个服务的所有 Space（供并行调用）
// 使用 errgroup 并发更新（上限 3），每次间隔 1s 避免触发 HF API 限流
// 单次 commit/secret 失败自动重试 2 次（指数退避）
func (s *HFSpaceService) updateOneService(svc string, result *UpdateSpaceResult, mu *sync.Mutex) {
	svcCfg, ok := ServiceMap[svc]
	if !ok {
		return
	}

	// 读取嵌入的模板文件
	dockerfileBytes, err := regplatform.HFTemplateFS.ReadFile(svcCfg.Dir + "/Dockerfile")
	if err != nil {
		mu.Lock()
		result.Logs = append(result.Logs, fmt.Sprintf("[%s] 读取 Dockerfile 失败: %v", svc, err))
		mu.Unlock()
		return
	}
	scriptBytes, err := regplatform.HFTemplateFS.ReadFile(svcCfg.Dir + "/" + svcCfg.Script)
	if err != nil {
		mu.Lock()
		result.Logs = append(result.Logs, fmt.Sprintf("[%s] 读取脚本失败: %v", svc, err))
		mu.Unlock()
		return
	}
	scriptContent := string(scriptBytes)

	releaseURL := s.settingSvc.GetRaw("hf_release_url_" + svc)
	ghPAT := s.settingSvc.GetRaw("hf_gh_pat")
	proxyURL := s.settingSvc.GetRaw("hf_proxy_url")

	var spaces []model.HFSpace
	s.db.Where("service = ?", svc).Find(&spaces)

	g := new(errgroup.Group)
	g.SetLimit(3) // 并发上限 3，避免 HF API 限流

	for idx, space := range spaces {
		idx, space := idx, space
		g.Go(func() error {
			// 错开请求，避免瞬间并发打满 HF API
			time.Sleep(time.Duration(idx%3) * time.Second)

			owner := strings.SplitN(space.RepoID, "/", 2)[0]
			var token model.HFToken
			if err := s.db.Where("username = ?", owner).First(&token).Error; err != nil {
				mu.Lock()
				result.Logs = append(result.Logs, fmt.Sprintf("[%s] %s — 找不到对应 Token (owner=%s): %v", svc, space.RepoID, owner, err))
				result.Failed++
				mu.Unlock()
				return nil
			}

			buildID := make([]byte, 4)
			_, _ = rand.Read(buildID)
			dockerfileFinal := randomizeDockerfile(string(dockerfileBytes))

			// 随机选择入口脚本名（更新时也随机化）
			deployScript := scriptNamePool[mathrand.Intn(len(scriptNamePool))]
			dockerfileFinal = strings.ReplaceAll(dockerfileFinal, svcCfg.Script, deployScript)

			// 查出现有文件，找到需要删除的旧 .sh 脚本
			var oldShells []string
			if existingFiles, err := s.hfListRepoFiles(token.Token, space.RepoID); err == nil {
				for _, f := range existingFiles {
					if strings.HasSuffix(f, ".sh") && f != deployScript {
						oldShells = append(oldShells, f)
					}
				}
			}

			// commit 文件 + 删除旧脚本（带重试）
			commitErr := s.hfRetry(3, func() error {
				return s.hfCommitFiles(token.Token, space.RepoID, map[string]string{
					deployScript:   scriptContent,
					"Dockerfile":   dockerfileFinal,
				}, "Update templates", oldShells...)
			})
			if commitErr != nil {
				mu.Lock()
				result.Logs = append(result.Logs, fmt.Sprintf("[%s] %s — commit 失败: %v", svc, space.RepoID, commitErr))
				result.Failed++
				mu.Unlock()
				return nil
			}

			if releaseURL != "" {
				if err := s.hfRetry(2, func() error {
					return s.hfSetSecret(token.Token, space.RepoID, svcCfg.URLSecret, releaseURL)
				}); err != nil {
					mu.Lock()
					result.Logs = append(result.Logs, fmt.Sprintf("[%s] %s — 更新 Release URL Secret 失败: %v", svc, space.RepoID, err))
					mu.Unlock()
				}
			}
			if ghPAT != "" {
				if err := s.hfRetry(2, func() error {
					return s.hfSetSecret(token.Token, space.RepoID, "GH_PAT", ghPAT)
				}); err != nil {
					mu.Lock()
					result.Logs = append(result.Logs, fmt.Sprintf("[%s] %s — 更新 GH_PAT 失败: %v", svc, space.RepoID, err))
					mu.Unlock()
				}
			}
			if proxyURL != "" {
				if err := s.hfRetry(2, func() error {
					return s.hfSetSecret(token.Token, space.RepoID, "PROXY_URL", proxyURL)
				}); err != nil {
					mu.Lock()
					result.Logs = append(result.Logs, fmt.Sprintf("[%s] %s — 更新 PROXY_URL 失败: %v", svc, space.RepoID, err))
					mu.Unlock()
				}
			}

			mu.Lock()
			result.Logs = append(result.Logs, fmt.Sprintf("[%s] %s — 更新完成，重建已触发", svc, space.RepoID))
			result.Updated++
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
}

// hfRetry 带指数退避的重试包装（HF API 限流/超时保护）
func (s *HFSpaceService) hfRetry(maxAttempts int, fn func() error) error {
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := fn(); err != nil {
			lastErr = err
			backoff := time.Duration(1<<uint(i)) * time.Second // 1s, 2s, 4s...
			log.Warn().Err(err).Int("attempt", i+1).Dur("backoff", backoff).Msg("HF API 调用失败，重试中")
			time.Sleep(backoff)
			continue
		}
		return nil
	}
	return lastErr
}

// DeploySpaces 批量部署新 Space
// 流程：创建 repo → 上传入口脚本 → 上传 README → 上传 Dockerfile（触发构建） → 设置 Secrets
// 上传顺序保证 Dockerfile 最后上传，此时入口脚本已存在，只触发一次有效构建
func (s *HFSpaceService) DeploySpaces(service string, count int, releaseURL string, tokenID uint, secrets map[string]string) ([]model.HFSpace, []string, error) {
	svcCfg, ok := ServiceMap[service]
	if !ok {
		return nil, nil, fmt.Errorf("未知服务类型: %s", service)
	}

	// 从嵌入的模板 FS 读取 Dockerfile 和入口脚本
	dockerfileBytes, err := regplatform.HFTemplateFS.ReadFile(svcCfg.Dir + "/Dockerfile")
	if err != nil {
		return nil, nil, fmt.Errorf("读取模板 Dockerfile 失败: %w", err)
	}
	scriptBytes, err := regplatform.HFTemplateFS.ReadFile(svcCfg.Dir + "/" + svcCfg.Script)
	if err != nil {
		return nil, nil, fmt.Errorf("读取模板脚本 %s 失败: %w", svcCfg.Script, err)
	}
	scriptContent := string(scriptBytes)

	ghPAT := s.settingSvc.GetRaw("hf_gh_pat")
	proxyURL := s.settingSvc.GetRaw("hf_proxy_url")

	// 获取可用 Token（指定 tokenID 时只用该 Token，否则轮询所有）
	var tokens []model.HFToken
	if tokenID > 0 {
		var t model.HFToken
		if err := s.db.First(&t, tokenID).Error; err != nil {
			return nil, nil, fmt.Errorf("指定的 Token (ID=%d) 不存在", tokenID)
		}
		tokens = []model.HFToken{t}
	} else {
		s.db.Where("is_valid = ?", true).Find(&tokens)
	}
	if len(tokens) == 0 {
		return nil, nil, fmt.Errorf("没有可用的 HF Token")
	}

	deployed := make([]model.HFSpace, 0, count)
	errors := make([]string, 0)

	for i := 0; i < count; i++ {
		token := tokens[i%len(tokens)]
		spaceName := generateSpaceName()
		repoID := token.Username + "/" + spaceName

		// 1. 创建 HF Space repo
		if err := s.hfCreateRepo(token.Token, repoID); err != nil {
			errors = append(errors, fmt.Sprintf("[%d] 创建 repo %s 失败: %v", i+1, repoID, err))
			continue
		}

		// 2. 一次性提交 README + 入口脚本 + Dockerfile（单次 commit，只触发一次构建）
		readme := generateReadme()
		dockerfileFinal := randomizeDockerfile(string(dockerfileBytes))

		// 随机选择入口脚本名（增加指纹多样性）
		deployScript := scriptNamePool[mathrand.Intn(len(scriptNamePool))]
		// Dockerfile 中替换原始脚本名为随机脚本名
		dockerfileFinal = strings.ReplaceAll(dockerfileFinal, svcCfg.Script, deployScript)

		if err := s.hfCommitFiles(token.Token, repoID, map[string]string{
			"README.md":    readme,
			deployScript:   scriptContent,
			"Dockerfile":   dockerfileFinal,
		}, "Initial deployment"); err != nil {
			errors = append(errors, fmt.Sprintf("[%d] commit 文件到 %s 失败: %v", i+1, repoID, err))
			_ = s.hfDeleteRepo(token.Token, repoID)
			continue
		}

		// 5. 设置 Secrets（Release URL + GH_PAT + 自定义）
		if err := s.hfSetSecret(token.Token, repoID, svcCfg.URLSecret, releaseURL); err != nil {
			errors = append(errors, fmt.Sprintf("[%d] 设置 Secret %s 失败: %v", i+1, svcCfg.URLSecret, err))
			continue
		}
		if ghPAT != "" {
			if err := s.hfSetSecret(token.Token, repoID, "GH_PAT", ghPAT); err != nil {
				log.Warn().Err(err).Str("repo", repoID).Msg("设置 GH_PAT 失败")
			}
		}
		if proxyURL != "" {
			if err := s.hfSetSecret(token.Token, repoID, "PROXY_URL", proxyURL); err != nil {
				log.Warn().Err(err).Str("repo", repoID).Msg("设置 PROXY_URL 失败")
			}
		}
		for k, v := range secrets {
			if err := s.hfSetSecret(token.Token, repoID, k, v); err != nil {
				log.Warn().Err(err).Str("repo", repoID).Str("key", k).Msg("设置自定义 Secret 失败")
			}
		}

		// 6. 构造 Space URL，写入 DB
		slug := strings.ToLower(token.Username + "-" + spaceName)
		spaceURL := fmt.Sprintf("https://%s.hf.space", slug)

		space := model.HFSpace{
			Service: service,
			RepoID:  repoID,
			URL:     spaceURL,
			TokenID: token.ID,
			Status:  "unknown",
		}
		s.db.Create(&space)
		deployed = append(deployed, space)

		log.Info().Str("repo", repoID).Str("url", spaceURL).Msg("Space 已部署")

		// 随机间隔 30~90 秒，模拟人工操作节奏，降低批量创建特征
		if i < count-1 {
			delay := time.Duration(30+mathrand.Intn(61)) * time.Second
			log.Info().Dur("delay", delay).Int("remaining", count-1-i).Msg("部署间隔等待中")
			time.Sleep(delay)
		}
	}

	return deployed, errors, nil
}

// ── Space 自动发现 ──

// DiscoverResult 发现结果
type DiscoverResult struct {
	Scanned  int      `json:"scanned"`  // 扫描的 Token 数
	Found    int      `json:"found"`    // HF API 返回的 Space 总数
	Imported int      `json:"imported"` // 新入库数
	Skipped  int      `json:"skipped"`  // 已存在跳过数
	Errors   []string `json:"errors,omitempty"`
}

// hfSpaceInfo HF API 返回的 Space 信息（只取需要的字段）
type hfSpaceInfo struct {
	ID     string `json:"id"`     // "username/space-name"
	Author string `json:"author"` // "username"
	SDK    string `json:"sdk"`    // "docker" / "gradio" 等
}

// DiscoverSpaces 遍历所有有效 Token，调 HF API 拉取该账号下所有 Space，自动入库
// defaultService: 无法自动判断服务类型时的默认值（openai/grok/kiro/gemini/ts），空则为 "unknown"
func (s *HFSpaceService) DiscoverSpaces(defaultService string) (*DiscoverResult, error) {
	result := &DiscoverResult{Errors: make([]string, 0)}

	if defaultService == "" {
		defaultService = "unknown"
	}

	// 构建脚本文件名 → 服务类型 反向映射
	scriptToService := make(map[string]string)
	for svc, cfg := range ServiceMap {
		scriptToService[cfg.Script] = svc
	}

	// 加载所有有效 Token
	var tokens []model.HFToken
	s.db.Where("is_valid = ?", true).Find(&tokens)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("没有可用的 HF Token，请先添加 Token")
	}
	result.Scanned = len(tokens)

	for _, token := range tokens {
		// 调 HF API 列出该用户的所有 Space
		spaces, err := s.hfListSpaces(token.Token, token.Username)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("[%s] 获取 Space 列表失败: %v", token.Username, err))
			continue
		}

		for _, info := range spaces {
			result.Found++

			// 只处理 docker SDK 的 Space（我们部署的都是 docker）
			if info.SDK != "docker" {
				result.Skipped++
				continue
			}

			// 构造 URL: username-spacename → https://username-spacename.hf.space
			slug := strings.ToLower(strings.ReplaceAll(info.ID, "/", "-"))
			spaceURL := fmt.Sprintf("https://%s.hf.space", slug)

			// 去重：按 repo_id 或 URL
			var count int64
			s.db.Model(&model.HFSpace{}).Where("repo_id = ? OR url = ?", info.ID, spaceURL).Count(&count)
			if count > 0 {
				result.Skipped++
				continue
			}

			// 通过文件树自动识别服务类型
			service := s.hfDetectService(token.Token, info.ID, scriptToService)
			if service == "" {
				service = defaultService
			}

			space := &model.HFSpace{
				Service: service,
				RepoID:  info.ID,
				URL:     spaceURL,
				TokenID: token.ID,
				Status:  "unknown",
			}
			if err := s.db.Create(space).Error; err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("[%s] 写入 DB 失败: %v", info.ID, err))
				continue
			}
			result.Imported++
			log.Info().Str("repo", info.ID).Str("service", service).Str("url", spaceURL).Msg("发现并导入 Space")
		}
	}

	log.Info().
		Int("scanned", result.Scanned).
		Int("found", result.Found).
		Int("imported", result.Imported).
		Int("skipped", result.Skipped).
		Msg("Space 自动发现完成")

	return result, nil
}

// hfListSpaces 调 HF API 列出指定用户的所有 Space
func (s *HFSpaceService) hfListSpaces(token, username string) ([]hfSpaceInfo, error) {
	url := fmt.Sprintf("https://huggingface.co/api/spaces?author=%s&limit=200", username)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	var spaces []hfSpaceInfo
	if err := json.NewDecoder(resp.Body).Decode(&spaces); err != nil {
		return nil, err
	}
	return spaces, nil
}

// RedetectService 对所有 service='unknown' 的 Space 重新通过文件树 API 识别服务类型
func (s *HFSpaceService) RedetectService() (*DiscoverResult, error) {
	result := &DiscoverResult{Errors: make([]string, 0)}

	scriptToService := make(map[string]string)
	for svc, cfg := range ServiceMap {
		scriptToService[cfg.Script] = svc
	}

	// 查出所有 unknown 的 Space
	var unknowns []model.HFSpace
	s.db.Where("service = ?", "unknown").Find(&unknowns)
	result.Found = len(unknowns)

	if len(unknowns) == 0 {
		return result, nil
	}

	// 预加载 token map（token_id → token 明文）
	var allTokens []model.HFToken
	s.db.Find(&allTokens)
	tokenMap := make(map[uint]string)
	for _, t := range allTokens {
		tokenMap[t.ID] = t.Token
	}

	for _, sp := range unknowns {
		result.Scanned++
		token := tokenMap[sp.TokenID]
		if token == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("[%s] 无关联 Token，跳过", sp.RepoID))
			result.Skipped++
			continue
		}

		detected := s.hfDetectService(token, sp.RepoID, scriptToService)
		if detected == "" {
			result.Skipped++
			continue
		}

		s.db.Model(&sp).Update("service", detected)
		result.Imported++ // 复用字段表示"已识别数"
		log.Info().Str("repo", sp.RepoID).Str("service", detected).Msg("重新识别 Space 服务类型")
	}

	return result, nil
}

// hfDetectService 通过 HF 文件树 API 自动识别 Space 的服务类型
// 优先匹配标准脚本文件名（init.sh/bootstrap.sh/start.sh/run.sh），
// 若匹配不到则读取 .sh 脚本内容，通过 URLSecret 关键字（ARTIFACT_URL/PKG_URL/MODEL_URL/DATA_URL）识别
func (s *HFSpaceService) hfDetectService(token, repoID string, scriptToService map[string]string) string {
	treeURL := fmt.Sprintf("https://huggingface.co/api/spaces/%s/tree/main", repoID)
	req, _ := http.NewRequest("GET", treeURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}

	var files []struct {
		RFilename string `json:"rfilename"`
		Path      string `json:"path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return ""
	}

	// 第一轮：标准脚本文件名精确匹配
	var shellScripts []string
	for _, f := range files {
		name := f.RFilename
		if name == "" {
			name = f.Path
		}
		if svc, ok := scriptToService[name]; ok {
			return svc
		}
		// 收集所有 .sh 文件，供第二轮 fallback 使用
		if strings.HasSuffix(name, ".sh") {
			shellScripts = append(shellScripts, name)
		}
	}

	// 第二轮 fallback：读取 .sh 脚本内容，通过 URLSecret 关键字识别
	secretToService := make(map[string]string)
	for svc, cfg := range ServiceMap {
		secretToService[cfg.URLSecret] = svc
	}
	for _, sh := range shellScripts {
		rawURL := fmt.Sprintf("https://huggingface.co/spaces/%s/raw/main/%s", repoID, sh)
		r, _ := http.NewRequest("GET", rawURL, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		rr, err := s.httpClient.Do(r)
		if err != nil || rr.StatusCode != 200 {
			if rr != nil {
				rr.Body.Close()
			}
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(rr.Body, 8192))
		rr.Body.Close()
		content := string(body)
		for secret, svc := range secretToService {
			if strings.Contains(content, secret) {
				return svc
			}
		}
	}

	return ""
}

// ── 内部方法：HF API 调用 ──

// hfWhoami 调用 HF whoami 接口验证 Token
func (s *HFSpaceService) hfWhoami(token string) (string, error) {
	req, _ := http.NewRequest("GET", "https://huggingface.co/api/whoami-v2", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var result struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Name == "" {
		return "", fmt.Errorf("whoami 返回空用户名")
	}
	return result.Name, nil
}

// hfCreateRepo 通过 HF API 创建 Space repo
func (s *HFSpaceService) hfCreateRepo(token, repoID string) error {
	// repoID 格式为 "username/spacename"，HF API 只需 "name" 传纯 repo 名
	parts := strings.SplitN(repoID, "/", 2)
	repoName := repoID
	if len(parts) == 2 {
		repoName = parts[1]
	}
	body, _ := json.Marshal(map[string]interface{}{
		"type":    "space",
		"name":    repoName,
		"sdk":     "docker",
		"private": false,
	})
	req, _ := http.NewRequest("POST", "https://huggingface.co/api/repos/create", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 200)]))
	}
	return nil
}

// hfDeleteRepo 通过 HF API 删除 repo
func (s *HFSpaceService) hfDeleteRepo(token, repoID string) error {
	// HF API 要求拆分 repo_id 为 organization + name
	// 例如 "myuser/my-space" → organization="myuser", name="my-space"
	payload := map[string]interface{}{
		"type": "space",
	}
	if parts := strings.SplitN(repoID, "/", 2); len(parts) == 2 {
		payload["organization"] = parts[0]
		payload["name"] = parts[1]
	} else {
		payload["name"] = repoID
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("DELETE", "https://huggingface.co/api/repos/delete", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HF 删除 repo 返回 %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 200)]))
	}
	return nil
}

// hfGetSpaceRuntime 调用 HF Runtime API 获取 Space 真实运行状态
// 返回 stage 字符串（RUNNING/BUILDING/RUNTIME_ERROR/BUILD_ERROR/PAUSED/STOPPED/CONFIG_ERROR 等）
func (s *HFSpaceService) hfGetSpaceRuntime(token, repoID string) (string, error) {
	url := fmt.Sprintf("https://huggingface.co/api/spaces/%s/runtime", repoID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HF Runtime API 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HF Runtime API 返回 %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	var result struct {
		Stage string `json:"stage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("HF Runtime API 解析失败: %w", err)
	}
	return result.Stage, nil
}

// hfRestartSpace 调用 HF API 重启 Space
// factoryReboot=true 时会清除持久存储重建（通过 query param ?factory=true）
func (s *HFSpaceService) hfRestartSpace(token, repoID string, factoryReboot bool) error {
	restartURL := fmt.Sprintf("https://huggingface.co/api/spaces/%s/restart", repoID)
	if factoryReboot {
		restartURL += "?factory=true"
	}
	req, _ := http.NewRequest("POST", restartURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HF Restart API 请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HF Restart API 返回 %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 200)]))
	}
	return nil
}

// hfCommitFiles 在 HF Space repo 中提交多个文件（HF Hub NDJSON Commit API）
//
// 协议说明：POST /api/spaces/{repo}/commit/main，Content-Type: application/x-ndjson
// 每行一个独立 JSON 对象，统一 key/value 信封结构：
//   - 第 1 行（header）：{"key":"header","value":{"summary":"...","description":""}}
//   - 后续行（file）：{"key":"file","value":{"path":"...","encoding":"base64","content":"..."}}
func (s *HFSpaceService) hfCommitFiles(token, repoID string, files map[string]string, message string, deleteFiles ...string) error {
	var buf bytes.Buffer

	// ── 第 1 行：commit header（value 信封） ──
	headerLine, err := json.Marshal(map[string]interface{}{
		"key": "header",
		"value": map[string]interface{}{
			"summary":     message,
			"description": "",
		},
	})
	if err != nil {
		return err
	}
	buf.Write(headerLine)
	buf.WriteByte('\n')

	// ── 删除文件 ──
	for _, df := range deleteFiles {
		delLine, err := json.Marshal(map[string]interface{}{
			"key": "deletedFile",
			"value": map[string]interface{}{
				"path": df,
			},
		})
		if err != nil {
			return err
		}
		buf.Write(delLine)
		buf.WriteByte('\n')
	}

	// ── 后续行：每个文件一行（value 信封，与 header 一致） ──
	for path, content := range files {
		fileLine, err := json.Marshal(map[string]interface{}{
			"key": "file",
			"value": map[string]interface{}{
				"path":     path,
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte(content)),
			},
		})
		if err != nil {
			return err
		}
		buf.Write(fileLine)
		buf.WriteByte('\n')
	}

	commitURL := fmt.Sprintf("https://huggingface.co/api/spaces/%s/commit/main", repoID)
	req, err := http.NewRequest("POST", commitURL, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/x-ndjson")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HF commit 返回 %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 300)]))
	}
	return nil
}

// hfSetSecret 设置 Space Secret
func (s *HFSpaceService) hfSetSecret(token, repoID, key, value string) error {
	url := fmt.Sprintf("https://huggingface.co/api/spaces/%s/secrets", repoID)
	body, _ := json.Marshal(map[string]string{"key": key, "value": value})
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HF 设置 Secret 返回 %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 200)]))
	}
	return nil
}

// hfListRepoFiles 获取 Space 仓库的文件列表
func (s *HFSpaceService) hfListRepoFiles(token, repoID string) ([]string, error) {
	url := fmt.Sprintf("https://huggingface.co/api/spaces/%s", repoID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HF API 返回 %d", resp.StatusCode)
	}
	var data struct {
		Siblings []struct {
			RFilename string `json:"rfilename"`
		} `json:"siblings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	files := make([]string, 0, len(data.Siblings))
	for _, s := range data.Siblings {
		files = append(files, s.RFilename)
	}
	return files, nil
}

// checkOneHealth 检查单个 Space 健康状态
func (s *HFSpaceService) checkOneHealth(client *http.Client, spaceURL string) SpaceHealthResult {
	result := SpaceHealthResult{
		URL:    spaceURL,
		Status: "unknown",
	}

	healthURL := strings.TrimRight(spaceURL, "/") + "/health"
	resp, err := client.Get(healthURL)
	if err != nil {
		result.Status = "dead"
		result.Reason = "connection_error"
		return result
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode

	switch {
	case resp.StatusCode == 200:
		result.Healthy = true
		result.Status = "healthy"
	case resp.StatusCode == 403:
		result.Status = "banned"
		result.Reason = "banned_or_restricted"
	case resp.StatusCode == 404:
		result.Status = "dead"
		result.Reason = "not_found_or_deleted"
	case resp.StatusCode == 503:
		result.Status = "sleeping"
		result.Reason = "sleeping_or_building"
	default:
		result.Status = "dead"
		result.Reason = fmt.Sprintf("http_%d", resp.StatusCode)
	}
	return result
}

// ── 随机化工具 ──

// 入口脚本名池（部署时随机选择，增加指纹多样性）
var scriptNamePool = []string{
	"entrypoint.sh", "main.sh", "launch.sh", "setup.sh", "app.sh",
	"init.sh", "bootstrap.sh", "start.sh", "run.sh", "serve.sh",
}

// randomizeDockerfile 对 Dockerfile 进行指纹随机化
// 随机化 WORKDIR、注释位置、EXPOSE 注释、ENV 前缀等，避免所有 Space 指纹一致
func randomizeDockerfile(original string) string {
	lines := strings.Split(original, "\n")

	// 1. 随机化 WORKDIR 路径
	workdirs := []string{"/app", "/opt/svc", "/srv", "/home/app", "/opt/app", "/usr/src/app", "/workspace"}
	newWorkdir := workdirs[mathrand.Intn(len(workdirs))]
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "WORKDIR ") {
			lines[i] = "WORKDIR " + newWorkdir
		}
	}

	// 2. 随机添加 EXPOSE 端口注释（40% 概率）
	if mathrand.Intn(10) < 4 {
		ports := []string{"7860", "8080", "8000", "3000", "5000"}
		comment := fmt.Sprintf("# expose port %s for health checks", ports[mathrand.Intn(len(ports))])
		// 插入到中间随机位置
		pos := 1 + mathrand.Intn(max(len(lines)-1, 1))
		lines = append(lines[:pos], append([]string{comment}, lines[pos:]...)...)
	}

	// 3. 随机化 ENV 变量名前缀（如果有 ENV 行）
	prefixes := []string{"APP_", "SVC_", "SRV_", "NODE_", "INST_"}
	prefix := prefixes[mathrand.Intn(len(prefixes))]
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ENV ") && strings.Contains(trimmed, "BUILD_HASH=") {
			lines[i] = strings.Replace(line, "BUILD_HASH=", prefix+"BUILD_HASH=", 1)
		}
	}

	// 4. 在随机位置插入构建注释（替代原来只在末尾加 build-id）
	buildID := make([]byte, 4)
	rand.Read(buildID)
	comments := []string{
		fmt.Sprintf("# build-id: %s", hex.EncodeToString(buildID)),
		fmt.Sprintf("# revision: %s", hex.EncodeToString(buildID)),
		fmt.Sprintf("# checksum: %s", hex.EncodeToString(buildID)),
		fmt.Sprintf("# stamp: %s", hex.EncodeToString(buildID)),
	}
	buildComment := comments[mathrand.Intn(len(comments))]

	// 50% 概率插入中间，50% 概率追加末尾
	if mathrand.Intn(2) == 0 && len(lines) > 2 {
		pos := 1 + mathrand.Intn(len(lines)-1)
		lines = append(lines[:pos], append([]string{buildComment}, lines[pos:]...)...)
	} else {
		// 去掉尾部空行后追加
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, buildComment)
	}

	return strings.Join(lines, "\n") + "\n"
}

// generateSpaceName 生成随机 Space 名称（ML 项目风格）
func generateSpaceName() string {
	adj := func() string { return adjectives[mathrand.Intn(len(adjectives))] }
	noun := func() string { return nouns[mathrand.Intn(len(nouns))] }
	hexN := func(n int) string {
		b := make([]byte, n)
		rand.Read(b)
		return hex.EncodeToString(b)
	}
	// 版本号风格后缀
	ver := func() string {
		return fmt.Sprintf("v%d", mathrand.Intn(3)+1)
	}

	switch mathrand.Intn(7) {
	case 0: // adj-noun-hex4（经典 HF 风格）
		return fmt.Sprintf("%s-%s-%s", adj(), noun(), hexN(2))
	case 1: // noun-adj-hex4
		return fmt.Sprintf("%s-%s-%s", noun(), adj(), hexN(2))
	case 2: // adj-noun-v1（版本号风格，像模型发布）
		return fmt.Sprintf("%s-%s-%s", adj(), noun(), ver())
	case 3: // noun-noun-hex4（双名词，像 bert-base）
		return fmt.Sprintf("%s-%s-%s", noun(), noun(), hexN(2))
	case 4: // adj-noun（简短，像 tiny-llm）
		return fmt.Sprintf("%s-%s", adj(), noun())
	case 5: // noun-adj-noun（三段式，像 bert-base-encoder）
		return fmt.Sprintf("%s-%s-%s", noun(), adj(), noun())
	default: // adj-adj-noun（双形容词，像 fast-slim-bert）
		return fmt.Sprintf("%s-%s-%s", adj(), adj(), noun())
	}
}

// generateReadme 生成随机 README（ML/AI 主题）
func generateReadme() string {
	themes := []struct{ title, emoji, desc string }{
		{"Text Classification Demo", "📝", "Fine-tuned transformer for multi-label text classification"},
		{"Sentiment Analysis API", "😊", "Real-time sentiment scoring with DistilBERT"},
		{"NER Extraction Service", "🏷️", "Named entity recognition powered by SpaCy and transformers"},
		{"Text Summarization", "📄", "Abstractive summarization using BART-base"},
		{"Question Answering", "❓", "Extractive QA with RoBERTa fine-tuned on SQuAD"},
		{"Image Classification", "🖼️", "ViT-based image classifier with top-5 predictions"},
		{"Object Detection API", "🔍", "YOLOv8-nano real-time object detection endpoint"},
		{"Text Generation", "✍️", "Causal language model inference with sampling controls"},
		{"Embedding Service", "🧮", "Sentence embeddings via all-MiniLM-L6-v2"},
		{"Translation API", "🌐", "Multi-language translation with MarianMT"},
		{"Speech Recognition", "🎙️", "Whisper-tiny automatic speech recognition"},
		{"Zero-Shot Classifier", "🎯", "Zero-shot text classification with MNLI"},
		{"Semantic Search", "🔎", "Dense retrieval with bi-encoder embeddings"},
		{"Token Classification", "🔤", "POS tagging and chunking with BERT-base"},
		{"Text2Text Generation", "🔄", "Flan-T5 instruction-following text generation"},
		{"Feature Extraction", "📊", "Hidden state extraction for downstream tasks"},
		{"Fill Mask Demo", "🎭", "Masked language modeling with BERT"},
		{"Paraphrase Detection", "🔁", "Sentence similarity scoring with cross-encoder"},
		{"Document QA", "📑", "Layout-aware document question answering"},
		{"Code Generation", "💻", "Code completion with CodeGen-350M-mono"},
		{"Chatbot Demo", "💬", "Conversational AI with DialoGPT-medium"},
		{"Image Segmentation", "🎨", "Semantic segmentation with SegFormer-B0"},
		{"Audio Classification", "🔊", "Environmental sound classification with AST"},
		{"Table QA", "📋", "Table question answering with TAPAS-base"},
	}

	colors := []string{"red", "yellow", "green", "blue", "indigo", "purple", "pink", "gray"}
	licenses := []string{"mit", "apache-2.0", "bsd-3-clause", "bsd-2-clause", "isc", "mpl-2.0", "unlicense"}

	// 随机特性池（每次抽 3 条，ML/AI 风格）
	featurePool := []string{
		"Optimized inference with ONNX Runtime",
		"Dynamic batching for throughput optimization",
		"Model quantization support (INT8/FP16)",
		"Automatic mixed precision inference",
		"Health check endpoint at /health",
		"Configurable max sequence length",
		"GPU and CPU inference support",
		"Streaming token generation",
		"Built-in tokenizer with padding and truncation",
		"Model warm-up on startup for low latency",
		"Prometheus metrics at /metrics",
		"Graceful shutdown with request draining",
		"Environment-based model configuration",
		"Docker-optimized multi-stage build",
		"Horizontal scaling with stateless design",
		"Request queuing with configurable concurrency",
		"Automatic model caching and versioning",
		"Input validation and sanitization",
	}

	// 随机 usage 片段池（ML 风格）
	usagePool := []string{
		"curl -X POST http://localhost:7860/predict -H 'Content-Type: application/json' -d '{\"text\": \"Hello world\"}'",
		"curl http://localhost:7860/health",
		"docker run -p 7860:7860 -e MODEL_NAME=distilbert-base IMAGE_NAME",
		"python -c \"import requests; print(requests.post('http://localhost:7860/predict', json={'text':'test'}).json())\"",
	}

	// 随机 API 端点池（ML 风格）
	apiPool := []string{
		"GET  /health       — Liveness and readiness probe",
		"POST /predict      — Run model inference on input",
		"POST /embed        — Generate embeddings for text",
		"GET  /model/info   — Model metadata and config",
		"POST /tokenize     — Tokenize input text",
		"GET  /metrics      — Prometheus-compatible metrics",
	}

	theme := themes[mathrand.Intn(len(themes))]
	c1 := colors[mathrand.Intn(len(colors))]
	c2 := colors[mathrand.Intn(len(colors))]
	lic := licenses[mathrand.Intn(len(licenses))]

	var sb strings.Builder

	// YAML front matter
	fmt.Fprintf(&sb, `---
title: %s
emoji: %s
colorFrom: %s
colorTo: %s
sdk: docker
pinned: false
license: %s
short_description: %s
---

`, theme.title, theme.emoji, c1, c2, lic, theme.desc)

	// 随机添加 badge（50% 概率）
	if mathrand.Intn(2) == 0 {
		badges := []string{
			fmt.Sprintf("![Build](https://img.shields.io/badge/build-passing-brightgreen)"),
			fmt.Sprintf("![License](https://img.shields.io/badge/license-%s-blue)", lic),
			fmt.Sprintf("![Docker](https://img.shields.io/badge/docker-ready-blue)"),
			fmt.Sprintf("![Version](https://img.shields.io/badge/version-1.%d.%d-orange)", mathrand.Intn(10), mathrand.Intn(20)),
		}
		n := 1 + mathrand.Intn(3) // 1~3 个 badge
		for i := 0; i < n && i < len(badges); i++ {
			sb.WriteString(badges[i])
			sb.WriteString(" ")
		}
		sb.WriteString("\n\n")
	}

	// 标题 + 描述（ML 风格）
	descs := []string{
		"Optimized for low-latency inference on CPU.",
		"Powered by Hugging Face Transformers.",
		"Lightweight model serving with dynamic batching.",
		"Built with ONNX Runtime for production deployment.",
		"Fine-tuned on domain-specific data for best accuracy.",
	}
	fmt.Fprintf(&sb, "# %s\n\n%s.\n%s\n", theme.title, theme.desc, descs[mathrand.Intn(len(descs))])

	// Features section（70% 概率）
	if mathrand.Intn(10) < 7 {
		sb.WriteString("\n## Features\n\n")
		used := map[int]bool{}
		for len(used) < 3 {
			idx := mathrand.Intn(len(featurePool))
			if !used[idx] {
				used[idx] = true
				fmt.Fprintf(&sb, "- %s\n", featurePool[idx])
			}
		}
	}

	// Usage section（60% 概率）
	if mathrand.Intn(10) < 6 {
		sb.WriteString("\n## Usage\n\n```bash\n")
		sb.WriteString(usagePool[mathrand.Intn(len(usagePool))])
		sb.WriteString("\n```\n")
	}

	// API section（50% 概率）
	if mathrand.Intn(2) == 0 {
		sb.WriteString("\n## API\n\n```\n")
		used := map[int]bool{}
		n := 2 + mathrand.Intn(3) // 2~4 个端点
		for len(used) < n && len(used) < len(apiPool) {
			idx := mathrand.Intn(len(apiPool))
			if !used[idx] {
				used[idx] = true
				sb.WriteString(apiPool[idx])
				sb.WriteString("\n")
			}
		}
		sb.WriteString("```\n")
	}

	return sb.String()
}

