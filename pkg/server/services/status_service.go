package services

import (
	"mesh-backend/pkg/config"
	"mesh-backend/pkg/store"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

type StatusService struct {
	config *config.ServerConfig
	logger zerolog.Logger
	store  store.Store
}

// NewStatusService 创建状态服务实例
func NewStatusService(cfg *config.ServerConfig, logger zerolog.Logger, store store.Store) *StatusService {
	return &StatusService{
		config: cfg,
		logger: logger.With().Str("service", "status").Logger(),
		store:  store,
	}
}

func (s *StatusService) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/status", s.HandleGetStatus)
}

func (s *StatusService) HandleGetStatus(c *gin.Context) {
	status := s.GetSystemStatus()
	c.JSON(http.StatusOK, status)
}

// GetSystemStatus 获取系统整体状态
func (s *StatusService) GetSystemStatus() map[string]interface{} {
	nodes, _ := s.store.ListNodeStatus()
	return map[string]interface{}{
		"nodes": nodes,
	}
}
