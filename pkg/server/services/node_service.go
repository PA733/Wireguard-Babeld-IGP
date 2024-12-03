package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

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
func NewNodeService(cfg *config.ServerConfig, logger zerolog.Logger, store store.Store, taskService *TaskService) *NodeService {
	srv := &NodeService{
		config:      cfg,
		logger:      logger.With().Str("service", "node").Logger(),
		store:       store,
		nodeAuth:    NewNodeAuthenticator(),
		nodes:       make(map[int]*types.NodeConfig),
		taskService: taskService,
	}

	// 初始化最大节点ID
	if nodes, err := store.ListNodes(context.Background()); err == nil {
		maxID := int32(0)
		for _, node := range nodes {
			if int32(node.NodeInfo.ID) > maxID {
				maxID = int32(node.NodeInfo.ID)
			}
		}
		atomic.StoreInt32(&srv.lastID, maxID)
	}

	return srv
}

// RegisterNode 注册新节点
func (s *NodeService) RegisterNode(nodeID int, token string, config *types.NodeConfig) error {
	// 验证节点令牌
	if !s.nodeAuth.ValidateToken(nodeID, token) {
		return fmt.Errorf("invalid node token")
	}

	// 保存节点配置
	ctx := context.Background()
	if err := s.store.CreateNode(ctx, config); err != nil {
		return fmt.Errorf("create node: %w", err)
	}

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
	ctx := context.Background()
	return s.store.GetNode(ctx, nodeID)
}

// ListNodes 列出所有节点
func (s *NodeService) ListNodes() ([]*types.NodeConfig, error) {
	ctx := context.Background()
	return s.store.ListNodes(ctx)
}

// UpdateNode 更新节点配置
func (s *NodeService) UpdateNode(nodeID int, config *types.NodeConfig) error {
	ctx := context.Background()
	return s.store.UpdateNode(ctx, nodeID, config)
}

// DeleteNode 删除节点
func (s *NodeService) DeleteNode(nodeID int) error {
	ctx := context.Background()
	s.nodeAuth.RemoveNode(nodeID)
	return s.store.DeleteNode(ctx, nodeID)
}

// HandleListNodes 处理列出节点请求
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
		config := &types.NodeConfig{}
		config.NodeInfo.Name = req.Name
		config.NodeInfo.ID = s.nextNodeID()
		config.NodeInfo.Endpoints = []string{req.Endpoint}

		// 分配 IP 地址
		config.NodeInfo.IPv4 = strings.Replace(
			strings.Replace(s.config.Network.IPv4NodeTemplate, "{node}", fmt.Sprintf("%d", config.NodeInfo.ID), -1),
			"{peer}", "0", -1,
		)
		config.NodeInfo.IPv6 = strings.Replace(
			strings.Replace(s.config.Network.IPv6NodeTemplate, "{node:x}", fmt.Sprintf("%x", config.NodeInfo.ID), -1),
			"{peer:x}", "0", -1,
		)

		// 生成密钥对
		privateKey, publicKey, err := generateWireGuardKeyPair()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		config.NodeInfo.PrivateKey = privateKey
		config.NodeInfo.PublicKey = publicKey

		// 设置网络配置
		config.Network.MTU = 1420
		config.Network.BasePort = s.config.Network.BasePort
		config.Network.LinkLocalNet = s.config.Network.LinkLocalNet
		config.Network.BabelPort = s.config.Network.BabelPort
		config.Network.BabelInterval = 5000 // 5秒

		// 生成认证令牌
		token, err := s.GenerateNodeToken(config.NodeInfo.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// 创建节点
		if err := s.CreateNode(config); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// 返回节点信息
		response := struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Token     string `json:"token"`
			PublicKey string `json:"public_key"`
		}{
			ID:        config.NodeInfo.ID,
			Name:      config.NodeInfo.Name,
			Token:     token,
			PublicKey: config.NodeInfo.PublicKey,
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

// GenerateNodeToken 生成新的节点令牌
func (s *NodeService) GenerateNodeToken(nodeID int) (string, error) {
	token, err := s.nodeAuth.GenerateToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	s.nodeAuth.RegisterNode(nodeID, token)
	return token, nil
}

// generateWireGuardKeyPair 生成WireGuard密钥对
func generateWireGuardKeyPair() (privateKey, publicKey string, err error) {
	// 生成私钥
	privateKeyBytes := make([]byte, 32)
	if _, err := rand.Read(privateKeyBytes); err != nil {
		return "", "", fmt.Errorf("generating private key: %w", err)
	}

	// 调整私钥
	privateKeyBytes[0] &= 248
	privateKeyBytes[31] &= 127
	privateKeyBytes[31] |= 64

	// 生成公钥
	var publicKeyBytes [32]byte
	curve25519.ScalarBaseMult(&publicKeyBytes, (*[32]byte)(privateKeyBytes))

	// 编码为Base64
	privateKey = base64.StdEncoding.EncodeToString(privateKeyBytes)
	publicKey = base64.StdEncoding.EncodeToString(publicKeyBytes[:])

	return privateKey, publicKey, nil
}

// UpdateNodeConfig 更新节点配置
func (s *NodeService) UpdateNodeConfig(nodeID int, config *types.NodeConfig) error {
	s.nodesMu.Lock()
	defer s.nodesMu.Unlock()

	s.nodes[nodeID] = config

	// 保存到存储
	ctx := context.Background()
	if err := s.store.UpdateNode(ctx, nodeID, config); err != nil {
		return fmt.Errorf("update node in store: %w", err)
	}

	return nil
}

// nextNodeID 生成下一个节点ID
func (s *NodeService) nextNodeID() int {
	return int(atomic.AddInt32(&s.lastID, 1))
}

// CreateNode 创建新节点
func (s *NodeService) CreateNode(config *types.NodeConfig) error {
	ctx := context.Background()
	return s.store.CreateNode(ctx, config)
}
