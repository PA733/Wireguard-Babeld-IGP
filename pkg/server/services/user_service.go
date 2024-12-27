package services

import (
	"errors"
	"net/http"

	"mesh-backend/pkg/config"
	"mesh-backend/pkg/server/middleware"
	"mesh-backend/pkg/store"
	"mesh-backend/pkg/types"
	"mesh-backend/pkg/utils/password"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// UserService 用户服务
type UserService struct {
	config *config.ServerConfig
	logger zerolog.Logger
	store  store.Store
}

// NewUserService 创建用户服务实例
func NewUserService(cfg *config.ServerConfig, logger zerolog.Logger, store store.Store) *UserService {
	return &UserService{
		config: cfg,
		logger: logger.With().Str("service", "user").Logger(),
		store:  store,
	}
}

// RegisterRoutes 注册路由
func (s *UserService) RegisterRoutes(r *gin.Engine) {
	auth := r.Group("/auth")
	{
		auth.POST("/register", s.HandleRegister)
		auth.POST("/login", s.HandleLogin)
	}
}

// HandleRegister 处理用户注册
func (s *UserService) HandleRegister(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// 检查用户名是否已存在
	exists, err := s.store.CheckUserExists(req.Username)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to check user existence")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	// 使用 Argon2id 哈希密码
	hashedPassword, err := password.HashPassword(req.Password)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to hash password")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// 创建用户
	user := &types.User{
		Username: req.Username,
		Password: hashedPassword,
	}

	if err := s.store.CreateUser(user); err != nil {
		s.logger.Error().Err(err).Msg("Failed to create user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}

// HandleLogin 处理用户登录
func (s *UserService) HandleLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// 获取用户
	user, err := s.store.GetUserByUsername(req.Username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		}
		s.logger.Error().Err(err).Msg("Failed to get user")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// 验证密码
	valid, err := password.VerifyPassword(req.Password, user.Password)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to verify password")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	// 生成 JWT token
	token, err := middleware.GenerateToken(user.ID, user.Username)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to generate token")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}
