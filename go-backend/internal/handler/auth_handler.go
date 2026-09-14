package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"dbawork/internal/config"
)

// AuthHandler 登录占位（admin/admin，可配置）。
type AuthHandler struct {
	cfg *config.Config
}

// NewAuthHandler 构造。
func NewAuthHandler(cfg *config.Config) *AuthHandler { return &AuthHandler{cfg: cfg} }

// Login 校验用户名密码并签发 JWT。
func (h *AuthHandler) Login(c *gin.Context) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "请求参数不合法")
		return
	}
	if in.Username != h.cfg.AuthUsername || in.Password != h.cfg.AuthPassword {
		Fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	now := time.Now()
	expires := now.Add(24 * time.Hour)
	claims := jwt.MapClaims{
		"sub": in.Username,
		"iat": now.Unix(),
		"exp": expires.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		Fail(c, http.StatusInternalServerError, "生成令牌失败")
		return
	}
	OK(c, gin.H{"token": signed, "username": in.Username, "expires_at": expires.UTC().Format(time.RFC3339)})
}

// HealthHandler 健康检查。
type HealthHandler struct {
	cfg *config.Config
}

// NewHealthHandler 构造。
func NewHealthHandler(cfg *config.Config) *HealthHandler { return &HealthHandler{cfg: cfg} }

// Health 健康检查。
func (h *HealthHandler) Health(c *gin.Context) {
	OK(c, gin.H{
		"status":       "ok",
		"service":      "go-backend",
		"time":         time.Now().UTC().Format(time.RFC3339),
		"auth_enabled": h.cfg.AuthEnabled,
	})
}
