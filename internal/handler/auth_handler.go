package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xiaolajiaoyyds/regplatformm/internal/config"
	"github.com/xiaolajiaoyyds/regplatformm/internal/middleware"
	"github.com/xiaolajiaoyyds/regplatformm/internal/service"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authSvc *service.AuthService
	cfg     *config.Config
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(authSvc *service.AuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, cfg: cfg}
}

// registerReq 注册请求
type registerReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// loginReq 登录请求
type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Register 用户注册（POST /api/auth/register）
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请提供用户名和密码"})
		return
	}

	user, err := h.authSvc.Register(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": err.Error()})
		return
	}

	jwt, err := h.authSvc.CreateJWT(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "JWT 签发失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": jwt,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"name":     user.Name,
			"role":     user.Role,
			"is_admin": user.IsAdmin,
		},
	})
}

// Login 用户登录（POST /api/auth/login）
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "请提供用户名和密码"})
		return
	}

	user, err := h.authSvc.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": err.Error()})
		return
	}

	jwt, err := h.authSvc.CreateJWT(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "JWT 签发失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": jwt,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"name":     user.Name,
			"role":     user.Role,
			"is_admin": user.IsAdmin,
		},
	})
}

// SSOLogin SSO 登录（GET /api/auth/sso?token=xxx，可选功能）
func (h *AuthHandler) SSOLogin(c *gin.Context) {
	tokenStr := c.Query("token")
	if tokenStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "缺少 token 参数"})
		return
	}

	ssoInfo, err := h.authSvc.VerifySSOToken(tokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": err.Error()})
		return
	}

	user, err := h.authSvc.FindOrCreateUser(ssoInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "用户创建失败"})
		return
	}

	jwt, err := h.authSvc.CreateJWT(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "JWT 签发失败"})
		return
	}

	c.Redirect(http.StatusFound, "/dashboard?sso_token="+jwt)
}

// DevLogin 开发模式登录（GET /api/auth/dev-login）
func (h *AuthHandler) DevLogin(c *gin.Context) {
	if !h.cfg.DevMode {
		c.JSON(http.StatusForbidden, gin.H{"detail": "非开发模式"})
		return
	}

	user, err := h.authSvc.CreateDevUser()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "创建开发用户失败"})
		return
	}

	jwt, err := h.authSvc.CreateJWT(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "JWT 签发失败"})
		return
	}

	c.Redirect(http.StatusFound, "/dashboard?sso_token="+jwt)
}

// Logout 登出（POST /api/auth/logout）
func (h *AuthHandler) Logout(c *gin.Context) {
	isSecure := h.cfg.GinMode == "release"
	c.SetCookie("token", "", -1, "/", "", isSecure, true)
	c.JSON(http.StatusOK, gin.H{"message": "已登出"})
}

// Me 获取当前用户信息（GET /api/auth/me）
func (h *AuthHandler) Me(c *gin.Context) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "未登录"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":                   user.ID,
		"username":             user.Username,
		"name":                 user.Name,
		"email":                user.Email,
		"avatar_url":           user.AvatarURL,
		"role":                 user.Role,
		"credits":              user.Credits,
		"free_trial_used":      user.FreeTrialUsed,
		"free_trial_remaining": user.FreeTrialRemaining,
		"is_admin":             user.IsAdmin,
	})
}
