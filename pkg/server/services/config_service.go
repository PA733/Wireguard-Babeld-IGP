package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"mesh-backend/pkg/config"
	"mesh-backend/pkg/types"

	"github.com/rs/zerolog"
)

// ConfigService 实现配置管理服务
type ConfigService struct {
	config *config.ServerConfig
	logger zerolog.Logger

	// 配置模板
	wgTemplate    *template.Template
	babelTemplate *template.Template
	templateMu    sync.RWMutex

	// 服务依赖
	nodeService *NodeService
	taskService *TaskService
}

// NewConfigService 创建配置服务实例
func NewConfigService(cfg *config.ServerConfig, logger zerolog.Logger, nodeService *NodeService, taskService *TaskService) *ConfigService {
	return &ConfigService{
		config:      cfg,
		logger:      logger.With().Str("service", "config").Logger(),
		nodeService: nodeService,
		taskService: taskService,
	}
}

// InitTemplates 初始化配置模板
func (s *ConfigService) InitTemplates() error {
	s.templateMu.Lock()
	defer s.templateMu.Unlock()

	// WireGuard配置模板
	wgTmpl, err := template.New("wireguard").Parse(`
[Interface]
PrivateKey = {{.PrivateKey}}
ListenPort = {{.ListenPort}}
Address = {{.Address}}

{{range .Peers}}
[Peer]
PublicKey = {{.PublicKey}}
AllowedIPs = {{.AllowedIPs}}
Endpoint = {{.Endpoint}}
{{end}}
`)
	if err != nil {
		return fmt.Errorf("parsing wireguard template: %w", err)
	}
	s.wgTemplate = wgTmpl

	// Babeld配置模板
	babelTmpl, err := template.New("babel").Parse(`
# Babeld configuration for node {{.NodeID}}
local-port {{.Port}}
random-id true
link-detect true

{{range .Interfaces}}
interface {{.Name}} type tunnel
{{end}}

# IPv4 routes
{{range .IPv4Routes}}
redistribute ip {{.Network}} ge {{.PrefixLen}} metric {{.Metric}}
{{end}}

# IPv6 routes
{{range .IPv6Routes}}
redistribute ipv6 {{.Network}} ge {{.PrefixLen}} metric {{.Metric}}
{{end}}
`)
	if err != nil {
		return fmt.Errorf("parsing babel template: %w", err)
	}
	s.babelTemplate = babelTmpl

	return nil
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
		WireGuard: wgConfig,
		Babel:     babelConfig,
		NodeInfo:  node.NodeInfo,
		Network:   node.Network,
	}

	return config, nil
}

// UpdateConfig 更新节点配置
func (s *ConfigService) UpdateConfig(nodeID int, config *types.NodeConfig) error {
	// 更新节点配置
	if err := s.nodeService.UpdateNodeConfig(nodeID, config); err != nil {
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

// generateWireGuardConfig 生成WireGuard配置
func (s *ConfigService) generateWireGuardConfig(node *types.NodeConfig, peers []*types.NodeConfig) (map[string]string, error) {
	s.templateMu.RLock()
	defer s.templateMu.RUnlock()

	// 为每个对等节点生成独立的配置
	configs := make(map[string]string)

	for _, peer := range peers {
		if peer.NodeInfo.ID == node.NodeInfo.ID {
			continue
		}

		// 准备当前对等节点的配置数据
		data := struct {
			PrivateKey string
			ListenPort int
			Address    string
			Peers      []struct {
				PublicKey  string
				AllowedIPs string
				Endpoint   string
			}
		}{
			// 本节点配置
			PrivateKey: node.NodeInfo.PrivateKey,
			ListenPort: s.config.Network.BasePort + node.NodeInfo.ID + peer.NodeInfo.ID,
			Address: fmt.Sprintf("%s, %s, %s",
				strings.Replace(
					strings.Replace(s.config.Network.IPv4Template, "{node}", fmt.Sprintf("%d", node.NodeInfo.ID), -1),
					"{peer}", fmt.Sprintf("%d", peer.NodeInfo.ID), -1,
				),
				strings.Replace(
					strings.Replace(s.config.Network.IPv6Template, "{node:x}", fmt.Sprintf("%x", node.NodeInfo.ID), -1),
					"{peer:x}", fmt.Sprintf("%x", peer.NodeInfo.ID), -1,
				),
				strings.Replace(
					strings.Replace(s.config.Network.LinkLocalTemplate, "{node}", fmt.Sprintf("%d", node.NodeInfo.ID), -1),
					"{peer}", fmt.Sprintf("%d", peer.NodeInfo.ID), -1,
				),
			),
			// 对等节点配置（只有一个）
			Peers: []struct {
				PublicKey  string
				AllowedIPs string
				Endpoint   string
			}{
				{
					PublicKey: peer.NodeInfo.PublicKey,
					AllowedIPs: fmt.Sprintf("%s, %s, %s",
						strings.Replace(
							strings.Replace(s.config.Network.IPv4Template, "{node}", fmt.Sprintf("%d", peer.NodeInfo.ID), -1),
							"{peer}", fmt.Sprintf("%d", node.NodeInfo.ID), -1,
						),
						strings.Replace(
							strings.Replace(s.config.Network.IPv6Template, "{node:x}", fmt.Sprintf("%x", peer.NodeInfo.ID), -1),
							"{peer:x}", fmt.Sprintf("%x", node.NodeInfo.ID), -1,
						),
						strings.Replace(
							strings.Replace(s.config.Network.LinkLocalTemplate, "{node}", fmt.Sprintf("%d", peer.NodeInfo.ID), -1),
							"{peer}", fmt.Sprintf("%d", node.NodeInfo.ID), -1,
						),
					),
					Endpoint: func() string {
						if len(peer.NodeInfo.Endpoints) > 0 {
							return peer.NodeInfo.Endpoints[0]
						}
						return ""
					}(),
				},
			},
		}

		// 执行模板
		var result strings.Builder
		if err := s.wgTemplate.Execute(&result, data); err != nil {
			return nil, fmt.Errorf("executing template for peer %d: %w", peer.NodeInfo.ID, err)
		}

		// 保存配置
		configs[fmt.Sprintf("%d", peer.NodeInfo.ID)] = result.String()
	}

	return configs, nil
}

// generateBabeldConfig 生成Babeld配置
func (s *ConfigService) generateBabeldConfig(node *types.NodeConfig, peers []*types.NodeConfig) (string, error) {
	s.templateMu.RLock()
	defer s.templateMu.RUnlock()

	// 准备模板数据
	data := struct {
		NodeID     int
		Port       int
		Interfaces []struct {
			Name string
		}
		IPv4Routes []struct {
			Network   string
			PrefixLen int
			Metric    int
		}
		IPv6Routes []struct {
			Network   string
			PrefixLen int
			Metric    int
		}
	}{
		NodeID: node.NodeInfo.ID,
		Port:   s.config.Network.BabelPort,
		Interfaces: make([]struct {
			Name string
		}, 0, len(peers)),
		IPv4Routes: []struct {
			Network   string
			PrefixLen int
			Metric    int
		}{
			{
				Network:   s.config.Network.IPv4Range,
				PrefixLen: 24,
				Metric:    128,
			},
		},
		IPv6Routes: []struct {
			Network   string
			PrefixLen int
			Metric    int
		}{
			{
				Network:   s.config.Network.IPv6Range,
				PrefixLen: 48,
				Metric:    128,
			},
		},
	}

	// 添加接口配置
	for _, peer := range peers {
		if peer.NodeInfo.ID == node.NodeInfo.ID {
			continue
		}
		data.Interfaces = append(data.Interfaces, struct {
			Name string
		}{
			Name: fmt.Sprintf("wg-%d", peer.NodeInfo.ID),
		})
	}

	// 执行模板
	var result strings.Builder
	if err := s.babelTemplate.Execute(&result, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return result.String(), nil
}
