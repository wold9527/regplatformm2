// Package regplatform 提供 HF Space 模板文件的编译期嵌入。
// 模板文件在 DeploySpaces 时读取，无需运行时文件系统依赖。
package regplatform

import "embed"

// HFTemplateFS 嵌入 5 个服务的 Dockerfile 和入口脚本
//
//go:embed HFNP/Dockerfile HFNP/init.sh
//go:embed HFGS/Dockerfile HFGS/bootstrap.sh
//go:embed HFKR/Dockerfile HFKR/start.sh
//go:embed HFGM/Dockerfile HFGM/launch.sh
//go:embed HFTS/Dockerfile HFTS/run.sh
var HFTemplateFS embed.FS
