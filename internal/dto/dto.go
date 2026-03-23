package dto

// ── Auth ──

// UserInfo 用户信息响应
type UserInfo struct {
	ID                 uint   `json:"id"`
	Username           string `json:"username"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	AvatarURL          string `json:"avatar_url"`
	TrustLevel         int    `json:"trust_level"`
	Role               int    `json:"role"`
	Credits            int    `json:"credits"`
	FreeTrialUsed      bool   `json:"free_trial_used"`
	FreeTrialRemaining int    `json:"free_trial_remaining"`
	IsAdmin            bool   `json:"is_admin"`
}

// ── Tasks ──

// CreateTaskReq 创建任务请求
type CreateTaskReq struct {
	Platform string `json:"platform" binding:"required,oneof=grok openai kiro gemini"`
	Target   int    `json:"target" binding:"required,min=1"`
	Threads  int    `json:"threads"` // 可选，0 或不传时后端自动计算
	Mode     string `json:"mode"`    // "free" 使用免费额度（受限制），"paid" 付费模式（无限制），空则自动判断
	ProxyID  uint   `json:"proxy_id"` // 0=使用系统默认代理，>0=使用用户保存的代理
}

// TaskStatus 任务状态响应
type TaskStatus struct {
	TaskID          uint   `json:"task_id"`
	Platform        string `json:"platform"`
	Target          int    `json:"target"`
	Threads         int    `json:"threads"`
	CreditsReserved int    `json:"credits_reserved"`
	SuccessCount    int    `json:"success_count"`
	FailCount       int    `json:"fail_count"`
	Status          string `json:"status"`
	IsDone          bool   `json:"is_done"`
	QueuePosition   int    `json:"queue_position,omitempty"`
	QueueWaitSec    int    `json:"queue_wait_sec,omitempty"`
}

// ── Credits ──

// BalanceResp 余额响应
type BalanceResp struct {
	Credits                int                `json:"credits"`
	FreeTrialRemaining     int                `json:"free_trial_remaining"`
	FreeTrialUsed          bool               `json:"free_trial_used"`
	CostPerReg             int                `json:"cost_per_reg"`
	Mode                   string             `json:"mode"` // "newapi" 或 "local"
	Display                string             `json:"display"`                  // 余额显示文字
	RegistrationsAvailable int                `json:"registrations_available"`  // 可用注册次数
	NewapiBalance          float64            `json:"newapi_balance"`           // New-API 余额（USD）
	NewapiBalanceDisplay   string             `json:"newapi_balance_display"`   // "$1.23"
	UnitPrice              float64            `json:"unit_price"`               // 单次注册价格（USD）
	UnitPriceDisplay       string             `json:"unit_price_display"`       // "$0.004"
	NewapiAvailable        int                `json:"newapi_available"`         // New-API 余额可购买次数
	PlatformPrices         map[string]float64 `json:"platform_prices"`          // 各平台单价（USD）
	FreeTrial              *FreeTrialResp     `json:"free_trial,omitempty"`
	Limits                 *LimitsResp        `json:"limits,omitempty"`
}

// FreeTrialResp 免费试用信息
type FreeTrialResp struct {
	Eligible  bool `json:"eligible"`  // 是否有资格领取
	Remaining int  `json:"remaining"` // 剩余次数
	Total     int  `json:"total"`     // 可领取总次数
}

// LimitsResp 系统限制（从管理后台设置读取）
type LimitsResp struct {
	MaxTarget      int `json:"max_target"`
	MaxThreads     int `json:"max_threads"`
	DailyRegLimit  int `json:"daily_reg_limit"`  // 0 = 不限制
	DailyUsed      int `json:"daily_used"`       // 今日已注册数量
	DailyRemaining int `json:"daily_remaining"`  // 今日剩余（-1 = 不限）
	// 平台开关
	PlatformGrokEnabled   bool `json:"platform_grok_enabled"`
	PlatformOpenaiEnabled bool `json:"platform_openai_enabled"`
	PlatformKiroEnabled   bool `json:"platform_kiro_enabled"`
	PlatformGeminiEnabled bool `json:"platform_gemini_enabled"`
	// 线程阶梯（JSON 字符串，前端解析）
	ThreadTiers string `json:"thread_tiers"`
	// 平台限时免费截止日期（YYYY-MM-DD，空=不免费）
	PlatformGrokFreeUntil   string `json:"platform_grok_free_until"`
	PlatformOpenaiFreeUntil string `json:"platform_openai_free_until"`
	PlatformKiroFreeUntil   string `json:"platform_kiro_free_until"`
	PlatformGeminiFreeUntil string `json:"platform_gemini_free_until"`
	// 免费模式状态（仅在平台处于免费期时有值）
	FreeMode map[string]*FreeModeInfo `json:"free_mode,omitempty"`
	// 各平台独立限制（付费/免费均生效）
	PlatformLimits map[string]*PlatformLimitInfo `json:"platform_limits,omitempty"`
}

// PlatformLimitInfo 单个平台的独立限制（付费/免费均生效）
type PlatformLimitInfo struct {
	TaskLimit      int `json:"task_limit"`       // 单任务上限（0=不限，回退全局 max_target）
	DailyLimit     int `json:"daily_limit"`      // 每日上限（0=不限）
	DailyUsed      int `json:"daily_used"`       // 今日该平台已注册
	DailyRemaining int `json:"daily_remaining"`  // 今日剩余（-1=不限）
}

// FreeModeInfo 单个平台的免费模式状态
type FreeModeInfo struct {
	Available          bool `json:"available"`            // 当前是否可以使用免费模式
	DailyUsed          int  `json:"daily_used"`           // 今日免费已用数量
	DailyLimit         int  `json:"daily_limit"`          // 免费每日上限（0=不限）
	DailyRemaining     int  `json:"daily_remaining"`      // 免费每日剩余（-1=不限）
	TaskLimit          int  `json:"task_limit"`           // 免费单任务上限
	CooldownSec        int  `json:"cooldown_sec"`         // 冷却配置（秒）
	CooldownRemaining  int  `json:"cooldown_remaining"`   // 冷却剩余秒数（0=可注册）
	Reason             string `json:"reason,omitempty"`    // 不可用原因（供前端提示）
}

// PurchaseReq 购买注册次数请求
type PurchaseReq struct {
	Amount   int    `json:"amount" binding:"required,min=1,max=10000"`
	Platform string `json:"platform"` // 可选，用于按平台定价
}

// RedeemReq 兑换请求
type RedeemReq struct {
	Code string `json:"code" binding:"required"`
}

// ── Admin ──

// RechargeReq 管理员充值/扣除请求（正数充值，负数扣除）
type RechargeReq struct {
	UserID  uint `json:"user_id" binding:"required"`
	Credits int  `json:"credits" binding:"required,ne=0"`
}

// GenerateCodesReq 生成兑换码请求
type GenerateCodesReq struct {
	Count     int    `json:"count" binding:"required,min=1,max=100"`
	Credits   int    `json:"credits" binding:"required,min=1"`
	BatchName string `json:"batch_name"`
}

// SaveSettingReq 保存设置请求
type SaveSettingReq struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value"`
}

// ── Notifications ──

// SendNotificationReq 发送通知请求
type SendNotificationReq struct {
	UserID  uint   `json:"user_id"`                                // 0 = 广播
	Title   string `json:"title" binding:"required,max=200"`
	Content string `json:"content" binding:"required,max=5000"`
}

// ── HF Space ──

// CreateHFTokenReq 创建 HF Token 请求
type CreateHFTokenReq struct {
	Label string `json:"label" binding:"required,max=100"`
	Token string `json:"token" binding:"required,max=200"`
}

// AddHFSpaceManualReq 手动添加 Space 请求
type AddHFSpaceManualReq struct {
	Service string `json:"service" binding:"required,oneof=openai grok kiro gemini ts"`
	URL     string `json:"url" binding:"required,url"`
	RepoID  string `json:"repo_id" binding:"required,max=200"`
	TokenID uint   `json:"token_id"`
}

// DeployHFSpaceReq 批量部署 Space 请求
type DeployHFSpaceReq struct {
	Service    string            `json:"service" binding:"required,oneof=openai grok kiro gemini ts"`
	Count      int               `json:"count" binding:"required,min=1,max=50"`
	ReleaseURL string            `json:"release_url" binding:"required"`
	TokenID    uint              `json:"token_id"`             // 指定 Token（0 = 轮询所有可用 Token）
	Secrets    map[string]string `json:"secrets"`              // 额外 Secrets（可选）
}

// UpdateHFSpacesReq 批量更新已有 Space 请求
type UpdateHFSpacesReq struct {
	Service string `json:"service" binding:"required,oneof=openai grok kiro gemini ts all"`
}

// HFAutoscaleReq 弹性管理请求
type HFAutoscaleReq struct {
	Service string `json:"service" binding:"required,oneof=openai grok kiro gemini ts all"`
	Target  int    `json:"target" binding:"min=0"`
	DryRun  bool   `json:"dry_run"`
}
