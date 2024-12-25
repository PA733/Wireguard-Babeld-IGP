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

	"github.com/gin-gonic/gin"
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
	wgConfigBytes, _ := json.Marshal(wgConfig)
	config := &types.NodeConfig{
		ID:        node.ID,
		Name:      node.Name,
		Token:     node.Token,
		IPv4:      node.IPv4,
		IPv6:      node.IPv6,
		Peers:     node.Peers,
		Endpoints: node.Endpoints,
		PublicKey: node.PublicKey,
		WireGuard: string(wgConfigBytes),
		Babel:     babelConfig,
		// Network:   node.Network,
		MTU:           node.MTU,
		BasePort:      node.BasePort,
		LinkLocalNet:  node.LinkLocalNet,
		BabelPort:     node.BabelPort,
		BabelInterval: node.BabelInterval,
		CreatedAt:     node.CreatedAt,
		UpdatedAt:     time.Now(),
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
func (s *ConfigService) HandleGetConfig(c *gin.Context) {
	nodeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	config, err := s.GenerateNodeConfig(nodeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, config)
}

// HandleUpdateConfig HTTP处理器：更新节点配置
func (s *ConfigService) HandleUpdateConfig(c *gin.Context) {
	var req struct {
		Config *types.NodeConfig `json:"config" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	nodeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	if err := s.UpdateConfig(nodeID, req.Config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
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

		wgConn, err := s.nodeService.GenerateWireguardConnection(node.ID, peer.ID, s.config.Network.BasePort)
		if err != nil {
			return nil, fmt.Errorf("generating wireguard connection: %w", err)
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
			NodeID      int
			Peer        struct {
				PublicKey  string
				AllowedIPs string
				Endpoint   string
				ID         int
			}
		}{
			PrivateKey:  node.PrivateKey,
			ListenPort:  wgConn.Port,
			IPv4Address: IPv4Address,
			IPv6Address: IPv6Address,
			NodeID:      node.ID,
		}

		// 添加对等节点信息
		peerData := struct {
			PublicKey  string
			AllowedIPs string
			Endpoint   string
			ID         int
		}{
			PublicKey: peer.PublicKey,
			AllowedIPs: fmt.Sprintf("%s,%s",
				strings.Replace(s.config.Network.IPv4NodeTemplate, "{node}", fmt.Sprintf("%d", peer.ID), -1),
				strings.Replace(s.config.Network.IPv6NodeTemplate, "{node:x}", fmt.Sprintf("%x", peer.ID), -1)),
			Endpoint: fmt.Sprintf("%s:%d", peer.Endpoints[0], wgConn.Port),
			ID:       peer.ID,
		}
		data.Peer = peerData

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
		UpdateInterval: node.BabelInterval,
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

func (s *ConfigService) RegisterRoutes(r *gin.Engine) {
	r.GET("/config/:id", s.HandleGetConfig)
	r.POST("/config/:id", s.HandleUpdateConfig)
}

func (s *NodeService) GenerateWireguardConnection(nodeID int, peerID int, basePort int) (*types.WireguardConnection, error) {
	connection := &types.WireguardConnection{
		NodeID: nodeID,
		PeerID: peerID,
	}

	connection, err := s.store.GetOrCreateWireguardConnection(connection, basePort)
	if err != nil {
		return nil, fmt.Errorf("get or create wireguard connection: %w", err)
	}

	return connection, nil
}
