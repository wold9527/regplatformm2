package worker

import "context"

// ProxyEntry 代理配置
type ProxyEntry struct {
	HTTP  string `json:"http"`
	HTTPS string `json:"https"`
}

// Config 平台配置（各平台自定义字段）
type Config map[string]string

// RegisterOpts 注册循环参数
type RegisterOpts struct {
	Proxy     *ProxyEntry
	Config    Config
	LogCh     chan<- string
	OnSuccess func(email string, credential map[string]interface{})
	OnFail    func()
}

// Worker 平台注册器接口
type Worker interface {
	// PlatformName 返回平台标识
	PlatformName() string

	// ScanConfig 获取/验证平台配置（cfg 包含已注入的系统设置）
	ScanConfig(ctx context.Context, proxy *ProxyEntry, cfg Config) (Config, error)

	// RegisterOne 执行一次注册
	RegisterOne(ctx context.Context, opts RegisterOpts)
}

// 平台注册表
var Registry = map[string]Worker{}

// 平台显示名
var Labels = map[string]string{
	"grok":        "Grok",
	"openai":      "OpenAI",
	"openai_team": "OpenAI Team",
	"kiro":        "Kiro",
	"gemini":      "Gemini",
}

// Register 注册平台 Worker
func Register(w Worker) {
	Registry[w.PlatformName()] = w
}

// Get 获取平台 Worker
func Get(platform string) (Worker, bool) {
	w, ok := Registry[platform]
	return w, ok
}
