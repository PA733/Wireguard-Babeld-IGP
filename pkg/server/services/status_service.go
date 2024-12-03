package services

import (
	"encoding/json"
	"net/http"

	"mesh-backend/pkg/config"
	"mesh-backend/pkg/store"
	"mesh-backend/pkg/types"

	"context"

	"github.com/rs/zerolog"
)

// StatusService 实现状态管理服务
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

// UpdateNodeStatus 更新节点状态
func (s *StatusService) UpdateNodeStatus(status *types.NodeStatus) error {
	return s.store.UpdateNodeStatus(context.Background(), status.ID, status)
}

// GetNodeStatus 获取节点状态
func (s *StatusService) GetNodeStatus(nodeID int) (*types.NodeStatus, error) {
	return s.store.GetNodeStatus(context.Background(), nodeID)
}

// GetSystemStatus 获取系统整体状态
func (s *StatusService) GetSystemStatus() map[string]interface{} {
	nodes, _ := s.store.ListNodeStatus(context.Background())
	return map[string]interface{}{
		"nodes": nodes,
	}
}

// HandleGetStatus HTTP处理器：获取系统状态
func (s *StatusService) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := s.GetSystemStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// GetMetrics 获取系统指标
func (s *StatusService) GetMetrics() map[string]interface{} {
	nodes, _ := s.store.ListNodeStatus(context.Background())
	return map[string]interface{}{
		"nodes": nodes,
	}
}
