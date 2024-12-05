package services

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mesh-backend/pkg/config"
	"mesh-backend/pkg/store"
	"mesh-backend/pkg/types"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/curve25519"
)

// NodeService 实现节点管理服务
type NodeService struct {
	config *config.ServerConfig
	logger zerolog.Logger
	store  store.Store

	// 节点管理
	nodeAuth *NodeAuthenticator
	nodes    map[int]*types.NodeConfig
	nodesMu  sync.RWMutex
	lastID   int32

	// 服务依赖
	taskService *TaskService
}

// NewNodeService 创建节点服务实例
func NewNodeService(cfg *config.ServerConfig, logger zerolog.Logger, store store.Store, taskService *TaskService, nodeAuth *NodeAuthenticator) *NodeService {
	srv := &NodeService{
		config:      cfg,
		logger:      logger.With().Str("service", "node").Logger(),
		store:       store,
		nodeAuth:    nodeAuth,
		nodes:       make(map[int]*types.NodeConfig),
		taskService: taskService,
	}

	// 初始化最大节点ID并加载 tokens
	if nodes, err := store.ListNodes(); err == nil {
		maxID := int32(0)
		for _, node := range nodes {
			if int32(node.ID) > maxID {
				maxID = int32(node.ID)
			}
			// 从数据库加载 token 到 nodeAuth
			srv.nodeAuth.RegisterNode(node.ID, node.Token)
			srv.logger.Debug().
				Int("node_id", node.ID).
				Str("token", node.Token).
				Msg("Loaded node token from database")
		}
		atomic.StoreInt32(&srv.lastID, maxID)
	}

	return srv
}

// RegisterNode 注册新节点
func (s *NodeService) RegisterNode(nodeID int, token string, config *types.NodeConfig) error {
	// 保存节点配置
	if err := s.store.CreateNode(config); err != nil {
		return fmt.Errorf("create node: %w", err)
	}

	// 注册节点令牌
	s.nodeAuth.RegisterNode(nodeID, token)

	return nil
}

// UnregisterNode 注销节点
func (s *NodeService) UnregisterNode(nodeID int) error {
	s.nodesMu.Lock()
	defer s.nodesMu.Unlock()

	// 检查节点是否存在
	if _, exists := s.nodes[nodeID]; !exists {
		return fmt.Errorf("node %d not found", nodeID)
	}

	// 注销节点认证信息
	s.nodeAuth.UnregisterNode(nodeID)

	// 删除节点配置
	delete(s.nodes, nodeID)

	s.logger.Info().
		Int("node_id", nodeID).
		Msg("Node unregistered")

	return nil
}

// GetNode 获取节点配置
func (s *NodeService) GetNode(nodeID int) (*types.NodeConfig, error) {
	return s.store.GetNode(nodeID)
}

// ListNodes 列出所有节点
func (s *NodeService) ListNodes() ([]*types.NodeConfig, error) {
	return s.store.ListNodes()
}

// UpdateNode 更新节点配置
func (s *NodeService) UpdateNode(nodeID int, config *types.NodeConfig) error {
	// 获取原有节点配置
	_, err := s.store.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("get old node: %w", err)
	}

	// 保留原有 token
	// config.Token = oldNode.Token

	// 更新节点配置
	if err := s.store.UpdateNode(nodeID, config); err != nil {
		return fmt.Errorf("update node: %w", err)
	}

	// 确保 token 在 nodeAuth 中注册
	// s.nodeAuth.RegisterNode(nodeID, config.Token)

	return nil
}

// DeleteNode 删除节点
func (s *NodeService) DeleteNode(nodeID int) error {
	s.nodeAuth.RemoveNode(nodeID)
	return s.store.DeleteNode(nodeID)
}

// HandleListNodes 处理列出节点请求
// To-Do 修改接口名称
func (s *NodeService) HandleListNodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		nodes, err := s.ListNodes()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nodes)

	case http.MethodPost:
		var req struct {
			Name     string `json:"name"`
			Endpoint string `json:"endpoint"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// 生成节点配置
		nodeID := s.nextNodeID()
		now := time.Now()

		// 生成认证令牌
		token, err := s.GenerateNodeToken(nodeID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		config := &types.NodeConfig{
			// 基本信息
			ID:        nodeID,
			Name:      req.Name,
			Token:     token,
			Peers:     []string{}, // To-Do 添加预设节点
			Endpoints: []string{req.Endpoint},
			CreatedAt: now,
			UpdatedAt: now,
		}

		// 生成 WireGuard 密钥对
		privateKey, publicKey, err := generateWireGuardKeyPair()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		config.PrivateKey = privateKey
		config.PublicKey = publicKey

		// 创建节点
		if err := s.store.CreateNode(config); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// 注册节点令牌
		s.nodeAuth.RegisterNode(config.ID, token)

		// 返回节点信息
		response := struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Token     string `json:"token"`
			PublicKey string `json:"public_key"`
		}{
			ID:        config.ID,
			Name:      config.Name,
			Token:     token,
			PublicKey: config.PublicKey,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleGetNode 处理获取节点请求
func (s *NodeService) HandleGetNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径中提取节点ID
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	nodeID, err := strconv.Atoi(parts[2])
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	node, err := s.GetNode(nodeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if node == nil {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// nextNodeID 生成下一个节点ID
func (s *NodeService) nextNodeID() int {
	return int(atomic.AddInt32(&s.lastID, 1))
}

// generateWireGuardKeyPair 生成WireGuard密钥对
func generateWireGuardKeyPair() (privateKey, publicKey string, err error) {
	var private, public [32]byte

	// 生成私钥
	if _, err := rand.Read(private[:]); err != nil {
		return "", "", fmt.Errorf("generating private key: %w", err)
	}

	// 生成公钥
	curve25519.ScalarBaseMult(&public, &private)

	// 编码为Base64
	privateKey = base64.StdEncoding.EncodeToString(private[:])
	publicKey = base64.StdEncoding.EncodeToString(public[:])

	return privateKey, publicKey, nil
}

// GenerateNodeToken 生成节点认证令牌
func (s *NodeService) GenerateNodeToken(nodeID int) (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}

	tokenStr := base64.URLEncoding.EncodeToString(token)
	s.nodeAuth.RegisterNode(nodeID, tokenStr)

	s.logger.Debug().
		Int("node_id", nodeID).
		Str("token", tokenStr).
		Msg("Generated and registered new token")

	return tokenStr, nil
}

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
	return s.store.UpdateNodeStatus(status.ID, status)
}

// GetNodeStatus 获取节点状态
func (s *StatusService) GetNodeStatus(nodeID int) (*types.NodeStatus, error) {
	return s.store.GetNodeStatus(nodeID)
}

// GetSystemStatus 获取系统整体状态
func (s *StatusService) GetSystemStatus() map[string]interface{} {
	nodes, _ := s.store.ListNodeStatus()
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
	nodes, _ := s.store.ListNodeStatus()
	return map[string]interface{}{
		"nodes": nodes,
	}
}

// TriggerConfigUpdate 触发节点配置更新任务
func (s *NodeService) TriggerConfigUpdate(nodeID int) error {
	task := &types.Task{
		ID:        fmt.Sprintf("config_update_%d_%d", nodeID, time.Now().Unix()),
		Type:      "config_update",
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Params: map[string]string{
			"node_id": strconv.Itoa(nodeID),
		},
	}

	// 保存任务
	if err := s.taskService.SaveTask(task); err != nil {
		return fmt.Errorf("saving task: %w", err)
	}

	s.logger.Info().
		Int("node_id", nodeID).
		Str("task_id", task.ID).
		Msg("Triggered config update task")

	return nil
}

// HandleTriggerConfigUpdate HTTP处理器：触发配置更新
func (s *NodeService) HandleTriggerConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径中提取节点ID
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	nodeID, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	// 触发配置更新
	if err := s.TriggerConfigUpdate(nodeID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
