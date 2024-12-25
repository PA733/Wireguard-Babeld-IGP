package services

import (
	"mesh-backend/pkg/types"
)

// NodeService 实现节点管理服务
// type NodeService struct {
// 	config *config.ServerConfig
// 	logger zerolog.Logger
// 	store  store.Store

// 	// 节点管理
// 	nodeAuth *NodeAuthenticator
// 	nodes    map[int]*types.NodeConfig
// 	nodesMu  sync.RWMutex
// 	lastID   int32

// 	// 服务依赖
// 	taskService *TaskService
// }

// RegisterNode 注册新节点
// func (s *NodeService) RegisterNode(nodeID int, token string, config *types.NodeConfig) error {
// 	// 保存节点配置
// 	if err := s.store.CreateNode(config); err != nil {
// 		return fmt.Errorf("create node: %w", err)
// 	}

// 	// 注册节点令牌
// 	s.nodeAuth.RegisterNode(nodeID, token)

// 	return nil
// }

// UnregisterNode 注销节点
// func (s *NodeService) UnregisterNode(nodeID int) error {
// 	s.nodesMu.Lock()
// 	defer s.nodesMu.Unlock()

// 	// 检查节点是否存在
// 	if _, exists := s.nodes[nodeID]; !exists {
// 		return fmt.Errorf("node %d not found", nodeID)
// 	}

// 	// 注销节点认证信息
// 	s.nodeAuth.UnregisterNode(nodeID)

// 	// 删除节点配置
// 	delete(s.nodes, nodeID)

// 	s.logger.Info().
// 		Int("node_id", nodeID).
// 		Msg("Node unregistered")

// 	return nil
// }

// HandleListNodes 处理列出节点请求
// To-Do 修改接口名称
// func (s *NodeService) HandleListNodes(w http.ResponseWriter, r *http.Request) {
// 	switch r.Method {
// 	case http.MethodGet:
// 		nodes, err := s.ListNodes()
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusInternalServerError)
// 			return
// 		}
// 		w.Header().Set("Content-Type", "application/json")
// 		json.NewEncoder(w).Encode(nodes)

// 	case http.MethodPost:
// 		var req struct {
// 			Name     string `json:"name"`
// 			Endpoint string `json:"endpoint"`
// 		}
// 		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 			http.Error(w, "Invalid request body", http.StatusBadRequest)
// 			return
// 		}

// 		// 生成节点配置
// 		nodeID := s.nextNodeID()
// 		now := time.Now()

// 		// 生成认证令牌
// 		token, err := s.GenerateNodeToken(nodeID)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusInternalServerError)
// 			return
// 		}

// 		peersBytes, _ := json.Marshal([]string{})
// 		endpointBytes, _ := json.Marshal([]string{req.Endpoint})
// 		config := &types.NodeConfig{
// 			// 基本信息
// 			ID:        nodeID,
// 			Name:      req.Name,
// 			Token:     token,
// 			Peers:     string(peersBytes), // To-Do 添加预设节点
// 			Endpoints: string(endpointBytes),
// 			CreatedAt: now,
// 			UpdatedAt: now,
// 		}

// 		// 生成 WireGuard 密钥对
// 		privateKey, publicKey, err := generateWireGuardKeyPair()
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusInternalServerError)
// 			return
// 		}
// 		config.PrivateKey = privateKey
// 		config.PublicKey = publicKey

// 		// 创建节点
// 		if err := s.store.CreateNode(config); err != nil {
// 			http.Error(w, err.Error(), http.StatusInternalServerError)
// 			return
// 		}

// 		// 注册节点令牌
// 		s.nodeAuth.RegisterNode(config.ID, token)

// 		// 返回节点信息
// 		response := struct {
// 			ID        int    `json:"id"`
// 			Name      string `json:"name"`
// 			Token     string `json:"token"`
// 			PublicKey string `json:"public_key"`
// 		}{
// 			ID:        config.ID,
// 			Name:      config.Name,
// 			Token:     token,
// 			PublicKey: config.PublicKey,
// 		}

// 		w.Header().Set("Content-Type", "application/json")
// 		json.NewEncoder(w).Encode(response)

// 	default:
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 	}
// }

// HandleGetNode 处理获取节点请求
// func (s *NodeService) HandleGetNode(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodGet {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	// 从URL路径中提取节点ID
// 	parts := strings.Split(r.URL.Path, "/")
// 	if len(parts) < 3 {
// 		http.Error(w, "Invalid node ID", http.StatusBadRequest)
// 		return
// 	}

// 	nodeID, err := strconv.Atoi(parts[2])
// 	if err != nil {
// 		http.Error(w, "Invalid node ID", http.StatusBadRequest)
// 		return
// 	}

// 	node, err := s.GetNode(nodeID)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	if node == nil {
// 		http.Error(w, "Node not found", http.StatusNotFound)
// 		return
// 	}

// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(node)
// }

// StatusService 实现状态管理服务
// type StatusService struct {
// 	config *config.ServerConfig
// 	logger zerolog.Logger
// 	store  store.Store
// }

// UpdateNodeStatus 更新节点状态
func (s *StatusService) UpdateNodeStatus(status *types.NodeStatus) error {
	return s.store.UpdateNodeStatus(status.ID, status)
}

// GetNodeStatus 获取节点状态
func (s *StatusService) GetNodeStatus(nodeID int) (*types.NodeStatus, error) {
	return s.store.GetNodeStatus(nodeID)
}

// HandleGetStatus HTTP处理器：获取系统状态
// func (s *StatusService) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodGet {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	status := s.GetSystemStatus()
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(status)
// }

// GetMetrics 获取系统指标
func (s *StatusService) GetMetrics() map[string]interface{} {
	nodes, _ := s.store.ListNodeStatus()
	return map[string]interface{}{
		"nodes": nodes,
	}
}

// HandleTriggerConfigUpdate HTTP处理器：触发配置更新
// func (s *NodeService) HandleTriggerConfigUpdate(w http.ResponseWriter, r *http.Request) {
// 	if r.Method != http.MethodPost {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}

// 	// 从URL路径中提取节点ID
// 	parts := strings.Split(r.URL.Path, "/")
// 	if len(parts) < 4 {
// 		http.Error(w, "Invalid path", http.StatusBadRequest)
// 		return
// 	}

// 	nodeID, err := strconv.Atoi(parts[3])
// 	if err != nil {
// 		http.Error(w, "Invalid node ID", http.StatusBadRequest)
// 		return
// 	}

// 	// 触发配置更新
// 	if err := s.TriggerConfigUpdate(nodeID); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	w.WriteHeader(http.StatusOK)
// }
