package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"github.com/xiaolajiaoyyds/regplatformm/internal/service"
)

// Auth JWT 认证中间件
// token 提取优先级：Cookie("token") > X-Auth-Token > Authorization Bearer
func Auth(authSvc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Cookie
		token, _ := c.Cookie("token")

		// 2. X-Auth-Token header
		if token == "" {
			token = c.GetHeader("X-Auth-Token")
		}

		// 3. Authorization: Bearer xxx
		if token == "" {
			auth := c.GetHeader("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				token = auth[7:]
			}
		}

		// 4. Query parameter（SSE 用）
		if token == "" {
			token = c.Query("token")
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "未登录"})
			return
		}

		userID, err := authSvc.VerifyJWT(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "登录已过期"})
			return
		}

		user, err := authSvc.GetActiveUser(userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"detail": "账号不可用"})
			return
		}

		c.Set("user", user)
		c.Set("userID", user.ID)
		c.Next()
	}
}

// GetUser 从 gin.Context 中提取当前用户
func GetUser(c *gin.Context) *model.User {
	u, exists := c.Get("user")
	if !exists {
		return nil
	}
	return u.(*model.User)
}

// Admin 管理员权限中间件（需在 Auth 之后使用）
func Admin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetUser(c)
		if user == nil || !user.IsAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"detail": "需要管理员权限"})
			return
		}
		c.Next()
	}
}
