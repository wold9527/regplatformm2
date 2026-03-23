package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/xiaolajiaoyyds/regplatformm/internal/config"
	"github.com/xiaolajiaoyyds/regplatformm/internal/model"
	"github.com/xiaolajiaoyyds/regplatformm/internal/pkg/cache"
	"gorm.io/gorm"
)

// cachedUser 用户缓存条目
type cachedUser struct {
	user      *model.User
	expiresAt time.Time
}

// AuthService 认证服务（L1 内存 + L2 Redis 两层缓存）
type AuthService struct {
	db         *gorm.DB
	cfg        *config.Config
	settingSvc *SettingService
	redis      *cache.RedisCache
	ucache     map[uint]cachedUser
	mu         sync.RWMutex
	l1TTL      time.Duration // L1 内存缓存 TTL
	l2TTL      time.Duration // L2 Redis 缓存 TTL
}

// NewAuthService 创建认证服务
func NewAuthService(db *gorm.DB, cfg *config.Config, settingSvc *SettingService, rc *cache.RedisCache) *AuthService {
	return &AuthService{
		db:         db,
		cfg:        cfg,
		settingSvc: settingSvc,
		redis:      rc,
		ucache:     make(map[uint]cachedUser),
		l1TTL:      10 * time.Second, // L1 内存：10 秒
		l2TTL:      2 * time.Minute,  // L2 Redis：2 分钟
	}
}

// CreateJWT 签发 JWT
func (s *AuthService) CreateJWT(userID uint) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Duration(s.cfg.JWTExpireHours) * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

// VerifyJWT 验证 JWT，返回 userID
func (s *AuthService) VerifyJWT(tokenStr string) (uint, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("签名算法不匹配: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, errors.New("无效 token")
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, errors.New("token 中缺少 user_id")
	}
	return uint(userIDFloat), nil
}

// ── 注册 / 登录 ─────────────────────────────────────────────────────────────

// Register 用户注册（用户名 + 密码）
// 第一个注册的用户自动成为管理员；或匹配 ADMIN_USERNAME 配置的用户名
func (s *AuthService) Register(username, password string) (*model.User, error) {
	if len(username) < 2 || len(username) > 50 {
		return nil, errors.New("用户名长度应为 2-50 个字符")
	}
	if len(password) < 8 {
		return nil, errors.New("密码长度不能少于 8 个字符")
	}
	// 密码复杂度：至少包含大写、小写、数字
	hasUpper, hasLower, hasDigit := false, false, false
	for _, ch := range password {
		switch {
		case ch >= 'A' && ch <= 'Z':
			hasUpper = true
		case ch >= 'a' && ch <= 'z':
			hasLower = true
		case ch >= '0' && ch <= '9':
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return nil, errors.New("密码必须包含大写字母、小写字母和数字")
	}

	// 检查用户名是否已存在
	var existing model.User
	if err := s.db.Where("username = ?", username).First(&existing).Error; err == nil {
		return nil, errors.New("用户名已存在")
	}

	// 密码加密
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	// 判断是否应设为管理员
	isAdmin := false
	role := 1

	// 规则 1：ADMIN_USERNAME 配置匹配
	if s.cfg.AdminUsername != "" && username == s.cfg.AdminUsername {
		isAdmin = true
		role = 100
	}

	// 规则 2：数据库中没有任何用户，第一个注册的自动成为管理员
	if !isAdmin {
		var count int64
		s.db.Model(&model.User{}).Count(&count)
		if count == 0 {
			isAdmin = true
			role = 100
		}
	}

	user := model.User{
		Username:     username,
		PasswordHash: string(hash),
		Name:         username,
		Role:         role,
		IsActive:     true,
		IsAdmin:      isAdmin,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("用户创建失败: %w", err)
	}

	// 新用户赠送积分
	if s.settingSvc != nil {
		bonus := s.settingSvc.GetInt("new_user_bonus", 0)
		if bonus > 0 {
			s.db.Model(&user).Update("credits", bonus)
			user.Credits = bonus
			tx := model.CreditTransaction{
				UserID:      user.ID,
				Amount:      bonus,
				Type:        model.TxTypeNewUserBonus,
				Description: fmt.Sprintf("新用户赠送 %d 积分", bonus),
			}
			s.db.Create(&tx)
		}
	}

	return &user, nil
}

// Login 用户登录（用户名 + 密码）
func (s *AuthService) Login(username, password string) (*model.User, error) {
	var user model.User
	if err := s.db.Where("username = ? AND is_active = ?", username, true).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, err
	}

	if user.PasswordHash == "" {
		return nil, errors.New("该账户未设置密码，请联系管理员")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	return &user, nil
}

// ── SSO 登录（可选，用于外部系统对接）────────────────────────────────────────

// SSOUserInfo 从外部 SSO JWT 解析出的用户信息
type SSOUserInfo struct {
	UID         int
	Username    string
	DisplayName string
	Email       string
	Role        int
	Quota       int
	Avatar      string
}

// VerifySSOToken 验证 SSO token（可选功能，需配置 SSO_SECRET）
func (s *AuthService) VerifySSOToken(tokenStr string) (*SSOUserInfo, error) {
	if s.cfg.SSOSecret == "" {
		return nil, errors.New("SSO 未配置")
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("SSO token 签名算法不匹配: %v", t.Header["alg"])
		}
		return []byte(s.cfg.SSOSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("SSO token 验证失败: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("无效 SSO token")
	}

	uid, _ := claims["uid"].(float64)
	username, _ := claims["username"].(string)
	displayName, _ := claims["display_name"].(string)
	email, _ := claims["email"].(string)
	role, _ := claims["role"].(float64)
	quota, _ := claims["quota"].(float64)
	avatar, _ := claims["avatar"].(string)

	return &SSOUserInfo{
		UID:         int(uid),
		Username:    username,
		DisplayName: displayName,
		Email:       email,
		Role:        int(role),
		Quota:       int(quota),
		Avatar:      avatar,
	}, nil
}

// FindOrCreateUser SSO 登录时查找或创建用户，并同步外部字段
func (s *AuthService) FindOrCreateUser(info *SSOUserInfo) (*model.User, error) {
	var user model.User
	err := s.db.Where("newapi_id = ?", info.UID).First(&user).Error
	if err == nil {
		updates := map[string]interface{}{
			"username":     info.Username,
			"newapi_quota": info.Quota,
		}
		if info.DisplayName != "" {
			updates["name"] = info.DisplayName
		}
		if info.Email != "" {
			updates["email"] = info.Email
		}
		if info.Avatar != "" {
			updates["avatar_url"] = info.Avatar
		}
		s.db.Model(&user).Updates(updates)
		s.InvalidateUserCache(user.ID)
		return &user, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	name := info.DisplayName
	if name == "" {
		name = info.Username
	}
	user = model.User{
		NewapiID:    info.UID,
		Username:    info.Username,
		Name:        name,
		Email:       info.Email,
		AvatarURL:   info.Avatar,
		NewapiQuota: info.Quota,
		Role:        1,
		IsActive:    true,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}

	// 新用户赠送积分
	if s.settingSvc != nil {
		bonus := s.settingSvc.GetInt("new_user_bonus", 0)
		if bonus > 0 {
			s.db.Model(&user).Update("credits", bonus)
			user.Credits = bonus
			tx := model.CreditTransaction{
				UserID:      user.ID,
				Amount:      bonus,
				Type:        model.TxTypeNewUserBonus,
				Description: fmt.Sprintf("新用户赠送 %d 积分", bonus),
			}
			s.db.Create(&tx)
		}
	}

	return &user, nil
}

// ── 缓存 ─────────────────────────────────────────────────────────────────────

// userRedisKey 生成用户缓存的 Redis key
func userRedisKey(userID uint) string {
	return "user:" + strconv.FormatUint(uint64(userID), 10)
}

// GetActiveUser 获取活跃用户（L1 内存 → L2 Redis → DB）
func (s *AuthService) GetActiveUser(userID uint) (*model.User, error) {
	// ── L1：内存缓存
	s.mu.RLock()
	if c, ok := s.ucache[userID]; ok && time.Now().Before(c.expiresAt) {
		s.mu.RUnlock()
		return c.user, nil
	}
	s.mu.RUnlock()

	// ── L2：Redis 缓存
	if s.redis != nil {
		var user model.User
		if s.redis.GetJSON(context.Background(), userRedisKey(userID), &user) {
			if user.IsActive {
				s.mu.Lock()
				s.ucache[userID] = cachedUser{user: &user, expiresAt: time.Now().Add(s.l1TTL)}
				s.mu.Unlock()
				return &user, nil
			}
		}
	}

	// ── L3：数据库
	var user model.User
	err := s.db.Where("id = ? AND is_active = ?", userID, true).First(&user).Error
	if err != nil {
		return nil, err
	}

	// 回填 L1 + L2
	s.mu.Lock()
	s.ucache[userID] = cachedUser{user: &user, expiresAt: time.Now().Add(s.l1TTL)}
	s.mu.Unlock()

	if s.redis != nil {
		s.redis.SetJSON(context.Background(), userRedisKey(userID), &user, s.l2TTL)
	}

	return &user, nil
}

// InvalidateUserCache 清除指定用户的 L1 + L2 缓存（余额变动时调用）
func (s *AuthService) InvalidateUserCache(userID uint) {
	s.mu.Lock()
	delete(s.ucache, userID)
	s.mu.Unlock()
	if s.redis != nil {
		s.redis.Del(context.Background(), userRedisKey(userID))
	}
}

// CreateDevUser 开发模式创建管理员用户
func (s *AuthService) CreateDevUser() (*model.User, error) {
	var user model.User
	err := s.db.Where("username = ?", "dev_admin").First(&user).Error
	if err == nil {
		return &user, nil
	}

	user = model.User{
		Username: "dev_admin",
		Name:     "开发管理员",
		Role:     100,
		IsActive: true,
		IsAdmin:  true,
		Credits:  9999,
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
