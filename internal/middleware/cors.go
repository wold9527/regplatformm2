package middleware

import (
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件
// 生产环境通过 CORS_ORIGINS 环境变量配置允许的域名（逗号分隔）
// 开发模式（GIN_MODE != release）允许所有来源
func CORS() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			// 开发模式允许所有来源
			if os.Getenv("GIN_MODE") != "release" {
				return true
			}
			// 生产环境：读取 CORS_ORIGINS 白名单
			allowed := os.Getenv("CORS_ORIGINS")
			if allowed == "" {
				// 未配置则允许同源请求（Origin 为空时浏览器不发送 CORS preflight）
				return origin == ""
			}
			for _, a := range strings.Split(allowed, ",") {
				if strings.TrimSpace(a) == origin {
					return true
				}
			}
			return false
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Auth-Token"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
