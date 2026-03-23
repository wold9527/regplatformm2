package model

import "time"

// SystemSetting 系统设置（键值对）
type SystemSetting struct {
	Key       string    `gorm:"primaryKey;size:100" json:"key"`
	Value     string    `gorm:"type:text;default:''" json:"value"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// SettingDefinition 设置项定义
type SettingDefinition struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Group        string `json:"group"`
	Type         string `json:"type"` // text, number, boolean, url, password
	DefaultValue string `json:"default_value"`
	Description  string `json:"description"`
	IsSensitive  bool   `json:"is_sensitive"`
	ReadOnly     bool   `json:"read_only,omitempty"`
}

// DefaultSettings 系统设置项定义（9 组：infra / email / captcha / platform_grok / platform_openai / platform_kiro / platform_gemini / user_policy / hfspace）
var DefaultSettings = []SettingDefinition{

	// ── 基础设施（infra）── 原 newapi + proxy + crs 合并
	{Key: "newapi_base_url", Label: "New-API 地址", Group: "infra", Type: "url", DefaultValue: "", Description: "New-API 系统的基础 URL"},
	{Key: "newapi_admin_token", Label: "New-API 管理令牌", Group: "infra", Type: "password", DefaultValue: "", Description: "New-API 管理员 API Token", IsSensitive: true},
	{Key: "newapi_admin_id", Label: "New-API 管理员 ID", Group: "infra", Type: "number", DefaultValue: "0", Description: "New-API 管理员用户 ID"},
	{Key: "newapi_cost_per_reg", Label: "单次注册成本(USD)", Group: "infra", Type: "number", DefaultValue: "0.004", Description: "每次注册扣减的 USD 金额"},
	{Key: "default_proxy", Label: "默认代理", Group: "infra", Type: "text", DefaultValue: "", Description: "默认 HTTP/SOCKS5 代理地址"},
	{Key: "kookeey_api", Label: "Kookeey API", Group: "infra", Type: "url", DefaultValue: "", Description: "Kookeey 动态住宅代理 API 地址"},
	{Key: "crs_endpoint", Label: "CRS 端点", Group: "infra", Type: "url", DefaultValue: "", Description: "Codex 注册系统上报端点"},
	{Key: "crs_token", Label: "CRS Token", Group: "infra", Type: "password", DefaultValue: "", Description: "CRS 认证令牌", IsSensitive: true},

	// ── 邮箱服务（email）── 不变
	{Key: "yydsmail_api_key", Label: "YYDS Mail API Key", Group: "email", Type: "password", DefaultValue: "", Description: "YYDS Mail 临时邮箱 API 密钥", IsSensitive: true},
	{Key: "yydsmail_base_url", Label: "YYDS Mail 服务地址", Group: "email", Type: "url", DefaultValue: "", Description: "YYDS Mail API 基础 URL"},
	{Key: "email_provider_priority", Label: "邮箱优先级", Group: "email", Type: "text", DefaultValue: "yydsmail", Description: "邮箱服务优先级（逗号分隔，如 yydsmail）"},

	// ── 验证码（captcha）── 不变
	{Key: "turnstile_solver_url", Label: "Turnstile Solver URL", Group: "captcha", Type: "url", DefaultValue: "http://127.0.0.1:5072", Description: "Turnstile 验证码解决器地址"},
	{Key: "turnstile_solver_proxy", Label: "Solver 专用代理", Group: "captcha", Type: "text", DefaultValue: "", Description: "Turnstile solver 浏览器专用代理（socks5://user:pass@host:port），仅求解使用，不影响注册流程"},
	{Key: "capsolver_key", Label: "CapSolver Key", Group: "captcha", Type: "password", DefaultValue: "", Description: "CapSolver 云端验证码求解 API Key", IsSensitive: true},
	{Key: "yescaptcha_key", Label: "YesCaptcha Key", Group: "captcha", Type: "password", DefaultValue: "", Description: "YesCaptcha 云端解码 API Key", IsSensitive: true},
	{Key: "cf_bypass_solver_url", Label: "CF-Bypass Solver URL", Group: "captcha", Type: "url", DefaultValue: "http://127.0.0.1:5073", Description: "cloudflare-bypass-2026 求解器地址（级联备用）"},

	// ── Grok 平台（platform_grok）── 原 grok + platforms/limits 中 grok 相关
	{Key: "platform_grok_enabled", Label: "Grok 注册开关", Group: "platform_grok", Type: "boolean", DefaultValue: "true", Description: "是否开放 Grok 平台注册"},
	{Key: "grok_action_id", Label: "Grok Action ID", Group: "platform_grok", Type: "text", DefaultValue: "", Description: "Grok Turnstile action ID"},
	{Key: "grok_site_key", Label: "Grok Site Key", Group: "platform_grok", Type: "text", DefaultValue: "0x4AAAAAAAhr9JGVDZbrZOo0", Description: "Grok Turnstile site key"},
	{Key: "camoufox_reg_url", Label: "Camoufox 注册服务", Group: "platform_grok", Type: "url", DefaultValue: "", Description: "Camoufox 浏览器注册服务地址，留空则使用 HTTP 模式"},
	{Key: "grok_reg_url", Label: "Grok 远程注册服务", Group: "platform_grok", Type: "url", DefaultValue: "", Description: "Grok HF 远程注册服务地址（支持逗号分隔多节点轮询），留空则走本地协议"},
	{Key: "grok_proxy", Label: "Grok 专用代理", Group: "platform_grok", Type: "text", DefaultValue: "", Description: "Grok 注册专用固定代理（http/socks5://user:pass@host:port），proxy_mode 为 fixed 时生效，留空则回退到代理池"},
	{Key: "grok_proxy_mode", Label: "Grok 代理模式", Group: "platform_grok", Type: "text", DefaultValue: "pool", Description: "代理策略：pool=代理池轮询, fixed=使用 grok_proxy 固定代理, direct=不使用代理, smart=按延迟智能选择"},
	{Key: "grok_email_providers", Label: "Grok 邮箱选择", Group: "platform_grok", Type: "text", DefaultValue: "", Description: "Grok 使用的邮箱服务（逗号分隔多选），留空使用全局优先级"},
	{Key: "grok_max_concurrent", Label: "Grok 最大并发", Group: "platform_grok", Type: "number", DefaultValue: "500", Description: "Grok 平台最大同时运行任务数（纯 HTTP，可设较高值）"},
	{Key: "grok_max_concurrent_users", Label: "Grok 同时注册用户数", Group: "platform_grok", Type: "number", DefaultValue: "0", Description: "Grok 平台同时允许多少个不同用户注册，0 表示不限制（回退到全局设置）"},
	{Key: "platform_grok_unit_price", Label: "Grok 单价(USD)", Group: "platform_grok", Type: "text", DefaultValue: "", Description: "留空使用全局 newapi_cost_per_reg"},
	{Key: "platform_grok_free_until", Label: "Grok 限时免费截止", Group: "platform_grok", Type: "text", DefaultValue: "", Description: "YYYY-MM-DD，留空表示不免费"},
	{Key: "platform_grok_free_daily_limit", Label: "Grok 免费每日上限", Group: "platform_grok", Type: "number", DefaultValue: "5", Description: "免费模式下单用户每天最多注册数量，0 表示不限制"},
	{Key: "platform_grok_free_task_limit", Label: "Grok 免费单任务上限", Group: "platform_grok", Type: "number", DefaultValue: "2", Description: "免费模式下单次任务最大注册数量"},
	{Key: "platform_grok_free_cooldown", Label: "Grok 免费冷却(分钟)", Group: "platform_grok", Type: "number", DefaultValue: "30", Description: "免费模式下两次任务之间的冷却时间（分钟），0 表示不冷却"},
	{Key: "platform_grok_daily_limit", Label: "Grok 每日注册上限", Group: "platform_grok", Type: "number", DefaultValue: "0", Description: "该平台单用户每日最大注册总量（凌晨6点重置），0 表示不限制，付费/免费均生效"},
	{Key: "platform_grok_task_limit", Label: "Grok 单任务注册上限", Group: "platform_grok", Type: "number", DefaultValue: "0", Description: "该平台单次任务最大注册数量，0 表示不限制（回退到全局 max_target），付费/免费均生效"},

	// ── OpenAI 平台（platform_openai）── 原 openai + platforms/limits 中 openai 相关
	{Key: "platform_openai_enabled", Label: "OpenAI 注册开关", Group: "platform_openai", Type: "boolean", DefaultValue: "true", Description: "是否开放 OpenAI 平台注册"},
	{Key: "openai_reg_url", Label: "OpenAI 注册服务", Group: "platform_openai", Type: "url", DefaultValue: "", Description: "OpenAI 浏览器注册服务地址（openai-reg 容器），留空则走本机脚本降级"},
	{Key: "openai_proxy", Label: "OpenAI 专用代理", Group: "platform_openai", Type: "text", DefaultValue: "", Description: "OpenAI 注册专用固定代理（http/socks5://user:pass@host:port），proxy_mode 为 fixed 时生效"},
	{Key: "openai_proxy_mode", Label: "OpenAI 代理模式", Group: "platform_openai", Type: "text", DefaultValue: "pool", Description: "代理策略：pool=代理池轮询, fixed=使用 openai_proxy 固定代理, direct=不使用代理, smart=按延迟智能选择"},
	{Key: "openai_email_providers", Label: "OpenAI 邮箱选择", Group: "platform_openai", Type: "text", DefaultValue: "", Description: "OpenAI 使用的邮箱服务（逗号分隔多选），留空使用全局优先级"},
	{Key: "openai_max_concurrent", Label: "OpenAI 最大并发", Group: "platform_openai", Type: "number", DefaultValue: "500", Description: "OpenAI 平台最大同时运行任务数"},
	{Key: "openai_max_concurrent_users", Label: "OpenAI 同时注册用户数", Group: "platform_openai", Type: "number", DefaultValue: "0", Description: "OpenAI 平台同时允许多少个不同用户注册，0 表示不限制（回退到全局设置）"},
	{Key: "platform_openai_unit_price", Label: "OpenAI 单价(USD)", Group: "platform_openai", Type: "text", DefaultValue: "", Description: "留空使用全局 newapi_cost_per_reg"},
	{Key: "platform_openai_free_until", Label: "OpenAI 限时免费截止", Group: "platform_openai", Type: "text", DefaultValue: "", Description: "YYYY-MM-DD，留空表示不免费"},
	{Key: "platform_openai_free_daily_limit", Label: "OpenAI 免费每日上限", Group: "platform_openai", Type: "number", DefaultValue: "5", Description: "免费模式下单用户每天最多注册数量，0 表示不限制"},
	{Key: "platform_openai_free_task_limit", Label: "OpenAI 免费单任务上限", Group: "platform_openai", Type: "number", DefaultValue: "2", Description: "免费模式下单次任务最大注册数量"},
	{Key: "platform_openai_free_cooldown", Label: "OpenAI 免费冷却(分钟)", Group: "platform_openai", Type: "number", DefaultValue: "30", Description: "免费模式下两次任务之间的冷却时间（分钟），0 表示不冷却"},
	{Key: "platform_openai_daily_limit", Label: "OpenAI 每日注册上限", Group: "platform_openai", Type: "number", DefaultValue: "0", Description: "该平台单用户每日最大注册总量（凌晨6点重置），0 表示不限制，付费/免费均生效"},
	{Key: "platform_openai_task_limit", Label: "OpenAI 单任务注册上限", Group: "platform_openai", Type: "number", DefaultValue: "0", Description: "该平台单次任务最大注册数量，0 表示不限制（回退到全局 max_target），付费/免费均生效"},

	// ── Kiro 平台（platform_kiro）── 原 kiro + platforms/limits 中 kiro 相关
	{Key: "platform_kiro_enabled", Label: "Kiro 注册开关", Group: "platform_kiro", Type: "boolean", DefaultValue: "true", Description: "是否开放 Kiro 平台注册"},
	{Key: "kiro_reg_url", Label: "Kiro 注册服务", Group: "platform_kiro", Type: "url", DefaultValue: "http://127.0.0.1:5076", Description: "AWS Builder ID 浏览器注册服务地址（aws-builder-id-reg 容器）"},
	{Key: "kiro_email_providers", Label: "Kiro 邮箱选择", Group: "platform_kiro", Type: "text", DefaultValue: "yydsmail", Description: "Kiro 使用的邮箱服务（逗号分隔多选），留空使用全局优先级"},
	{Key: "kiro_proxy", Label: "Kiro 专用代理", Group: "platform_kiro", Type: "text", DefaultValue: "", Description: "Kiro 注册专用代理地址（http://user:pass@host:port），proxy_mode 为 fixed 时生效，留空使用默认代理"},
	{Key: "kiro_proxy_mode", Label: "Kiro 代理模式", Group: "platform_kiro", Type: "text", DefaultValue: "pool", Description: "代理策略：pool=代理池轮询, fixed=使用 kiro_proxy 固定代理, direct=不使用代理, smart=按延迟智能选择"},
	{Key: "kiro_max_concurrent", Label: "Kiro 最大并发", Group: "platform_kiro", Type: "number", DefaultValue: "8", Description: "Kiro 平台最大同时运行任务数（受浏览器节点内存限制）"},
	{Key: "kiro_max_concurrent_users", Label: "Kiro 同时注册用户数", Group: "platform_kiro", Type: "number", DefaultValue: "0", Description: "Kiro 平台同时允许多少个不同用户注册，0 表示不限制（回退到全局设置）"},
	{Key: "platform_kiro_unit_price", Label: "Kiro 单价(USD)", Group: "platform_kiro", Type: "text", DefaultValue: "", Description: "留空使用全局 newapi_cost_per_reg"},
	{Key: "platform_kiro_free_until", Label: "Kiro 限时免费截止", Group: "platform_kiro", Type: "text", DefaultValue: "", Description: "YYYY-MM-DD，留空表示不免费"},
	{Key: "platform_kiro_free_daily_limit", Label: "Kiro 免费每日上限", Group: "platform_kiro", Type: "number", DefaultValue: "5", Description: "免费模式下单用户每天最多注册数量，0 表示不限制"},
	{Key: "platform_kiro_free_task_limit", Label: "Kiro 免费单任务上限", Group: "platform_kiro", Type: "number", DefaultValue: "2", Description: "免费模式下单次任务最大注册数量"},
	{Key: "platform_kiro_free_cooldown", Label: "Kiro 免费冷却(分钟)", Group: "platform_kiro", Type: "number", DefaultValue: "30", Description: "免费模式下两次任务之间的冷却时间（分钟），0 表示不冷却"},
	{Key: "platform_kiro_daily_limit", Label: "Kiro 每日注册上限", Group: "platform_kiro", Type: "number", DefaultValue: "0", Description: "该平台单用户每日最大注册总量（凌晨6点重置），0 表示不限制，付费/免费均生效"},
	{Key: "platform_kiro_task_limit", Label: "Kiro 单任务注册上限", Group: "platform_kiro", Type: "number", DefaultValue: "0", Description: "该平台单次任务最大注册数量，0 表示不限制（回退到全局 max_target），付费/免费均生效"},

	// ── Gemini 平台（platform_gemini）── Gemini Business 注册
	{Key: "platform_gemini_enabled", Label: "Gemini 注册开关", Group: "platform_gemini", Type: "boolean", DefaultValue: "true", Description: "是否开放 Gemini Business 平台注册"},
	{Key: "gemini_reg_url", Label: "Gemini 注册服务", Group: "platform_gemini", Type: "url", DefaultValue: "", Description: "Gemini 浏览器注册服务地址（复用 aws-builder-id-reg 的 /gemini/process），留空则使用 kiro_reg_url"},
	{Key: "gemini_email_providers", Label: "Gemini 邮箱选择", Group: "platform_gemini", Type: "text", DefaultValue: "yydsmail", Description: "Gemini 使用的邮箱服务（逗号分隔多选），留空使用全局优先级"},
	{Key: "gemini_proxy", Label: "Gemini 专用代理", Group: "platform_gemini", Type: "text", DefaultValue: "", Description: "Gemini 注册专用代理地址（http://user:pass@host:port），proxy_mode 为 fixed 时生效，留空使用默认代理"},
	{Key: "gemini_proxy_mode", Label: "Gemini 代理模式", Group: "platform_gemini", Type: "text", DefaultValue: "pool", Description: "代理策略：pool=代理池轮询, fixed=使用 gemini_proxy 固定代理, direct=不使用代理, smart=按延迟智能选择"},
	{Key: "gemini_max_concurrent", Label: "Gemini 最大并发", Group: "platform_gemini", Type: "number", DefaultValue: "8", Description: "Gemini 平台最大同时运行任务数（受浏览器节点内存限制）"},
	{Key: "gemini_max_concurrent_users", Label: "Gemini 同时注册用户数", Group: "platform_gemini", Type: "number", DefaultValue: "0", Description: "Gemini 平台同时允许多少个不同用户注册，0 表示不限制"},
	{Key: "platform_gemini_unit_price", Label: "Gemini 单价(USD)", Group: "platform_gemini", Type: "text", DefaultValue: "", Description: "留空使用全局 newapi_cost_per_reg"},
	{Key: "platform_gemini_free_until", Label: "Gemini 限时免费截止", Group: "platform_gemini", Type: "text", DefaultValue: "", Description: "YYYY-MM-DD，留空表示不免费"},
	{Key: "platform_gemini_free_daily_limit", Label: "Gemini 免费每日上限", Group: "platform_gemini", Type: "number", DefaultValue: "5", Description: "免费模式下单用户每天最多注册数量，0 表示不限制"},
	{Key: "platform_gemini_free_task_limit", Label: "Gemini 免费单任务上限", Group: "platform_gemini", Type: "number", DefaultValue: "2", Description: "免费模式下单次任务最大注册数量"},
	{Key: "platform_gemini_free_cooldown", Label: "Gemini 免费冷却(分钟)", Group: "platform_gemini", Type: "number", DefaultValue: "30", Description: "免费模式下两次任务之间的冷却时间（分钟），0 表示不冷却"},
	{Key: "platform_gemini_daily_limit", Label: "Gemini 每日注册上限", Group: "platform_gemini", Type: "number", DefaultValue: "0", Description: "该平台单用户每日最大注册总量（凌晨6点重置），0 表示不限制，付费/免费均生效"},
	{Key: "platform_gemini_task_limit", Label: "Gemini 单任务注册上限", Group: "platform_gemini", Type: "number", DefaultValue: "0", Description: "该平台单次任务最大注册数量，0 表示不限制（回退到全局 max_target），付费/免费均生效"},

	// ── 用户策略（user_policy）── 原 trial + limits 中全局项
	{Key: "free_trial_enabled", Label: "免费试用开关", Group: "user_policy", Type: "boolean", DefaultValue: "true", Description: "是否允许新用户领取免费试用"},
	{Key: "free_trial_count", Label: "免费试用次数", Group: "user_policy", Type: "number", DefaultValue: "2", Description: "新用户可领取的免费注册次数"},
	{Key: "new_user_bonus", Label: "新用户赠送积分", Group: "user_policy", Type: "number", DefaultValue: "0", Description: "新用户首次登录自动赠送积分数，0 表示不赠送"},
	{Key: "max_threads", Label: "最大线程（前端显示）", Group: "user_policy", Type: "number", DefaultValue: "16", Description: "前端线程选择器上限，实际并发由 Worker Pool 统一管理"},
	{Key: "max_target", Label: "最大注册数", Group: "user_policy", Type: "number", DefaultValue: "1000", Description: "单任务最大注册数量"},
	{Key: "max_concurrent_tasks", Label: "全局最大并发", Group: "user_policy", Type: "number", DefaultValue: "1000", Description: "系统全局最大同时运行任务数（安全上限）"},
	{Key: "max_concurrent_users", Label: "同时注册用户数", Group: "user_policy", Type: "number", DefaultValue: "0", Description: "同时允许多少个不同用户执行注册任务，0 表示不限制。超出的用户自动排队，前方用户完成后自动开始"},
	{Key: "daily_reg_limit", Label: "每日注册上限", Group: "user_policy", Type: "number", DefaultValue: "0", Description: "单用户每天最多注册数量（凌晨6点重置），0 表示不限制"},
	{Key: "thread_tiers", Label: "线程阶梯配置", Group: "user_policy", Type: "text",
		DefaultValue: `[{"max":5,"threads":1},{"max":20,"threads":2},{"max":100,"threads":3},{"max":300,"threads":5},{"max":500,"threads":8},{"max":99999,"threads":12}]`,
		Description: "前端线程建议值（仅显示用），实际并发由 Worker Pool 统一调度。JSON 数组，按 max 升序"},

	// ── HF 空间（hfspace）── Pool 配置 + 运维 + CF 同步 + 部署
	{Key: "hf_instance_count", Label: "HF 实例数量", Group: "hfspace", Type: "number", DefaultValue: "20", Description: "当前部署的 HF Space 实例数量，弹性管理后自动更新，10 秒生效", ReadOnly: true},
	{Key: "workers_per_instance", Label: "每实例并发数", Group: "hfspace", Type: "number", DefaultValue: "10", Description: "每个实例承载的并发注册数（纯 HTTP 建议 10），Pool 总大小 = 实例数 × 此值"},
	{Key: "openai_max_parallel", Label: "OpenAI 单任务并发上限", Group: "hfspace", Type: "number", DefaultValue: "50", Description: "单个 OpenAI 注册任务最多同时执行的注册数，防止打爆 Space 池"},
	{Key: "grok_max_parallel", Label: "Grok 单任务并发上限", Group: "hfspace", Type: "number", DefaultValue: "50", Description: "单个 Grok 注册任务最多同时执行的注册数，防止打爆 Space 池"},
	{Key: "kiro_max_parallel", Label: "Kiro 单任务并发上限", Group: "hfspace", Type: "number", DefaultValue: "15", Description: "单个 Kiro 注册任务最多同时执行的注册数（Python 服务并发低）"},
	{Key: "gemini_max_parallel", Label: "Gemini 单任务并发上限", Group: "hfspace", Type: "number", DefaultValue: "15", Description: "单个 Gemini 注册任务最多同时执行的注册数（Python 服务并发低）"},
	{Key: "hf_default_target", Label: "默认目标节点数", Group: "hfspace", Type: "number", DefaultValue: "5", Description: "每服务默认的目标健康 Space 数量（弹性管理用）"},
	{Key: "hf_health_timeout", Label: "健康检查超时(秒)", Group: "hfspace", Type: "number", DefaultValue: "8", Description: "单个 Space /health 请求超时时间"},
	{Key: "hf_release_url_openai", Label: "OpenAI Release URL", Group: "hfspace", Type: "url", DefaultValue: "https://github.com/xiaolajiaoyyds/regplatformm/releases/download/inference-runtime-latest/inference-runtime.zip", Description: "OpenAI Space 部署用的 GitHub Release URL（弹性管理自动扩容用）"},
	{Key: "hf_release_url_grok", Label: "Grok Release URL", Group: "hfspace", Type: "url", DefaultValue: "https://github.com/xiaolajiaoyyds/regplatformm/releases/download/stream-worker-latest/stream-worker.zip", Description: "Grok Space 部署用的 GitHub Release URL"},
	{Key: "hf_release_url_kiro", Label: "Kiro Release URL", Group: "hfspace", Type: "url", DefaultValue: "https://github.com/xiaolajiaoyyds/regplatformm/releases/download/browser-agent-latest/browser-agent.zip", Description: "Kiro Space 部署用的 GitHub Release URL"},
	{Key: "hf_release_url_gemini", Label: "Gemini Release URL", Group: "hfspace", Type: "url", DefaultValue: "https://github.com/xiaolajiaoyyds/regplatformm/releases/download/gemini-agent-latest/gemini-agent.zip", Description: "Gemini Space 部署用的 GitHub Release URL"},
	{Key: "hf_release_url_ts", Label: "TS Release URL", Group: "hfspace", Type: "url", DefaultValue: "https://github.com/xiaolajiaoyyds/regplatformm/releases/download/net-toolkit-latest/net-toolkit.zip", Description: "TS Space 部署用的 GitHub Release URL"},
	{Key: "hf_cf_account_id", Label: "CF Account ID", Group: "hfspace", Type: "text", DefaultValue: "", Description: "Cloudflare Account ID（用于更新 Worker 环境变量）"},
	{Key: "hf_cf_api_token", Label: "CF API Token", Group: "hfspace", Type: "password", DefaultValue: "", Description: "Cloudflare API Token", IsSensitive: true},
	{Key: "hf_cf_worker_name", Label: "CF Worker 名称", Group: "hfspace", Type: "text", DefaultValue: "", Description: "CF Worker 脚本名称"},
	{Key: "hf_gh_pat", Label: "GitHub PAT", Group: "hfspace", Type: "password", DefaultValue: "", Description: "GitHub Personal Access Token（部署 Space 用）", IsSensitive: true},
	{Key: "hf_proxy_url", Label: "Space 出口代理", Group: "hfspace", Type: "text", DefaultValue: "", Description: "HF Space 出站代理地址（如 socks5://ip:port），部署/更新时注入为 PROXY_URL secret，留空则不使用代理"},
}
