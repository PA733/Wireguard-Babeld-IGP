package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"mesh-backend/pkg/config"
	"mesh-backend/pkg/types"

	"github.com/rs/zerolog"
)

// ConfigService 配置服务
type ConfigService struct {
	config        *config.ServerConfig
	nodeAuth      *NodeAuthenticator
	wgTemplate    *template.Template
	babelTemplate *template.Template
	templateMu    sync.RWMutex
	logger        zerolog.Logger

	// 服务依赖
	nodeService *NodeService
	taskService *TaskService
}

// NewConfigService 创建配置服务
func NewConfigService(cfg *config.ServerConfig, nodeService *NodeService, nodeAuth *NodeAuthenticator, logger zerolog.Logger, taskService *TaskService) (*ConfigService, error) {
	s := &ConfigService{
		config:      cfg,
		nodeService: nodeService,
		nodeAuth:    nodeAuth,
		logger:      logger.With().Str("component", "config_service").Logger(),
		taskService: taskService,
	}

	// 解析 WireGuard 模板
	wgTmpl, err := template.New("wireguard").Parse(cfg.Templates.WireGuard)
	if err != nil {
		return nil, fmt.Errorf("parsing wireguard template: %w", err)
	}
	s.wgTemplate = wgTmpl

	// 解析 Babeld 模板
	babelTmpl, err := template.New("babel").Parse(cfg.Templates.Babel)
	if err != nil {
		return nil, fmt.Errorf("parsing babel template: %w", err)
	}
	s.babelTemplate = babelTmpl

	return s, nil
}

// GenerateNodeConfig 生成节点配置
func (s *ConfigService) GenerateNodeConfig(nodeID int) (*types.NodeConfig, error) {
	// 获取节点信息
	node, err := s.nodeService.GetNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("getting node info: %w", err)
	}

	// 获取所有节点列表（用于生成peer配置）
	nodes, err := s.nodeService.ListNodes()
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	// 生成WireGuard配置
	wgConfig, err := s.generateWireGuardConfig(node, nodes)
	if err != nil {
		return nil, fmt.Errorf("generating wireguard config: %w", err)
	}

	// 生成Babeld配置
	babelConfig, err := s.generateBabeldConfig(node, nodes)
	if err != nil {
		return nil, fmt.Errorf("generating babel config: %w", err)
	}

	// 创建完整的节点配置
	config := &types.NodeConfig{
		ID:        node.ID,
		Name:      node.Name,
		Token:     node.Token,
		IPv4:      node.IPv4,
		IPv6:      node.IPv6,
		Peers:     node.Peers,
		Endpoints: node.Endpoints,
		PublicKey: node.PublicKey,
		WireGuard: wgConfig,
		Babel:     babelConfig,
		Network:   node.Network,
		CreatedAt: node.CreatedAt,
		UpdatedAt: time.Now(),
	}

	return config, nil
}

// UpdateConfig 更新节点配置
func (s *ConfigService) UpdateConfig(nodeID int, config *types.NodeConfig) error {
	// 更新节点配置
	if err := s.nodeService.UpdateNode(nodeID, config); err != nil {
		return fmt.Errorf("updating node config: %w", err)
	}

	// 创建配置更新任务
	_, err := s.taskService.CreateTask(types.TaskTypeUpdate, map[string]interface{}{
		"node_id": nodeID,
		"type":    "config_update",
	})
	if err != nil {
		return fmt.Errorf("creating update task: %w", err)
	}

	return nil
}

// HandleGetConfig HTTP处理器：获取节点配置
func (s *ConfigService) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从URL路径中提取节点ID
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	nodeID, err := strconv.Atoi(parts[2])
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	config, err := s.GenerateNodeConfig(nodeID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// HandleUpdateConfig HTTP处理器：更新节点配置
func (s *ConfigService) HandleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		NodeID int               `json:"node_id"`
		Config *types.NodeConfig `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.UpdateConfig(req.NodeID, req.Config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// generateWireGuardConfig 生成 WireGuard 配置
func (s *ConfigService) generateWireGuardConfig(node *types.NodeConfig, peers []*types.NodeConfig) (map[string]string, error) {
	s.templateMu.RLock()
	defer s.templateMu.RUnlock()

	configs := make(map[string]string)
	for _, peer := range peers {
		if peer.ID == node.ID {
			continue
		}

		IPv4Address := strings.Replace(s.config.Network.IPv4Template, "{node}", fmt.Sprintf("%d", node.ID), -1)
		IPv4Address = strings.Replace(IPv4Address, "{peer}", fmt.Sprintf("%d", peer.ID), -1)
		IPv6Address := strings.Replace(s.config.Network.IPv6Template, "{node:x}", fmt.Sprintf("%x", node.ID), -1)
		IPv6Address = strings.Replace(IPv6Address, "{peer:x}", fmt.Sprintf("%x", peer.ID), -1)

		// 准备模板数据
		data := struct {
			PrivateKey  string
			ListenPort  int
			IPv4Address string
			IPv6Address string
			Peers       []struct {
				PublicKey  string
				AllowedIPs string
				Endpoint   string
			}
		}{
			PrivateKey:  node.PrivateKey,
			ListenPort:  s.config.Network.BasePort + peer.ID,
			IPv4Address: IPv4Address,
			IPv6Address: IPv6Address,
		}

		// 添加对等节点信息
		peerData := struct {
			PublicKey  string
			AllowedIPs string
			Endpoint   string
		}{
			PublicKey: peer.PublicKey,
			AllowedIPs: fmt.Sprintf("%s,%s",
				strings.Replace(s.config.Network.IPv4NodeTemplate, "{node}", fmt.Sprintf("%d", peer.ID), -1),
				strings.Replace(s.config.Network.IPv6NodeTemplate, "{node:x}", fmt.Sprintf("%x", peer.ID), -1)),
			Endpoint: fmt.Sprintf("%s:%d", peer.Endpoints[0], s.config.Network.BasePort+node.ID),
		}
		data.Peers = append(data.Peers, peerData)

		// 生成配置
		var buf strings.Builder
		if err := s.wgTemplate.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("executing wireguard template: %w", err)
		}

		configs[peer.Name] = buf.String()
	}

	return configs, nil
}

// generateBabeldConfig 生成 Babeld 配置
func (s *ConfigService) generateBabeldConfig(node *types.NodeConfig, peers []*types.NodeConfig) (string, error) {
	s.templateMu.RLock()
	defer s.templateMu.RUnlock()

	// 准备模板数据
	data := struct {
		NodeID         int
		Port           int
		UpdateInterval int
		Interfaces     []struct{ Name string }
		IPv4Routes     []struct{ Network, PrefixLen, Metric string }
		IPv6Routes     []struct{ Network, PrefixLen, Metric string }
	}{
		NodeID:         node.ID,
		Port:           s.config.Network.BabelPort,
		UpdateInterval: node.Network.BabelInterval,
	}

	// 添加接口配置
	for _, peer := range peers {
		if peer.ID == node.ID {
			continue
		}
		data.Interfaces = append(data.Interfaces, struct{ Name string }{
			Name: peer.Name,
		})
	}

	// 添加 IPv4 路由
	data.IPv4Routes = append(data.IPv4Routes, struct{ Network, PrefixLen, Metric string }{
		Network:   strings.Replace(s.config.Network.IPv4NodeTemplate, "{node}", fmt.Sprintf("%d", node.ID), -1),
		PrefixLen: "32",
		Metric:    "128",
	})

	// 添加 IPv6 路由
	data.IPv6Routes = append(data.IPv6Routes, struct{ Network, PrefixLen, Metric string }{
		Network:   strings.Replace(s.config.Network.IPv6NodeTemplate, "{node:x}", fmt.Sprintf("%x", node.ID), -1),
		PrefixLen: "80",
		Metric:    "128",
	})

	// 生成配置
	var buf strings.Builder
	if err := s.babelTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing babel template: %w", err)
	}

	return buf.String(), nil
}
