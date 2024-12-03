package service

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"mesh-backend/internal/models"
	"mesh-backend/internal/store/types"
	"mesh-backend/pkg/config"
)

type ConfigService struct {
	store       types.Store
	taskService *TaskService
	config      *config.Config
}

func NewConfigService(store types.Store, taskService *TaskService, cfg *config.Config) *ConfigService {
	return &ConfigService{
		store:       store,
		taskService: taskService,
		config:      cfg,
	}
}

func (s *ConfigService) GetNodeConfig(nodeID int) (*models.NodeConfig, error) {
	ctx := context.Background()

	node, err := s.store.GetNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	allNodes, err := s.store.ListNodes(ctx)
	if err != nil {
		return nil, err
	}

	// WireGuard 配置模板（每个peer一个配置文件）
	wgTmpl := `# WireGuard config for node {{.LocalName}} to peer {{.PeerName}}
[Interface]
# Link Local address for Babel
Address = {{.LocalLinkLocal}}
# Node addresses
Address = {{.LocalIPv4}}, {{.LocalIPv6}}
PrivateKey = {{.LocalPrivateKey}}
ListenPort = {{.ListenPort}}
Table = off

[Peer]
# Node {{.PeerName}}
PublicKey = {{.PeerPublicKey}}
Endpoint = {{.PeerEndpoint}}
# Mesh routing allowed IPs
AllowedIPs = {{.MeshIPv4}}, {{.MeshIPv6}}
# Babel protocol addresses
AllowedIPs = {{.LinkLocalNet}}, {{.BabelMulticast}}
`

	// Babeld 配置模板
	babelTmpl := `# Babeld config for node {{.Name}}
local-port {{.Port}}
random-id true
link-detect true
{{range .Interfaces}}
interface {{.}}
{{end}}
`

	// 为每个peer生成独立的WireGuard配置
	wgConfigs := make(map[string]string)
	interfaces := make([]string, 0)

	for _, peer := range allNodes {
		if peer.ID == node.ID {
			continue
		}

		peerIDStr := fmt.Sprintf("%d", peer.ID)
		interfaceName := fmt.Sprintf("wg-%s", peerIDStr)
		interfaces = append(interfaces, interfaceName)

		// 计算端口号：基础端口 + 节点ID
		localPort := s.config.Network.BasePort +
			node.ID + peer.ID

		data := struct {
			LocalName       string
			PeerName        string
			LocalLinkLocal  string
			LocalIPv4       string
			LocalIPv6       string
			LocalPrivateKey string
			ListenPort      int
			PeerPublicKey   string
			PeerEndpoint    string
			MeshIPv4        string
			MeshIPv6        string
			LinkLocalNet    string
			BabelMulticast  string
		}{
			LocalName:       node.Name,
			PeerName:        peer.Name,
			LocalLinkLocal:  node.GetLinkLocalPeerAddr(peer.ID),
			LocalIPv4:       node.GetPeerIP(peer.ID, false),
			LocalIPv6:       node.GetPeerIP(peer.ID, true),
			LocalPrivateKey: node.PrivateKey,
			ListenPort:      localPort,
			PeerPublicKey:   peer.PublicKey,
			PeerEndpoint:    peer.Endpoint,
			MeshIPv4:        s.config.Network.IPv4Range,
			MeshIPv6:        s.config.Network.IPv6Range,
			LinkLocalNet:    s.config.Network.LinkLocalNet,
			BabelMulticast:  s.config.Network.BabelMulticast,
		}

		var wgConfig strings.Builder
		tmpl := template.Must(template.New("wireguard").Parse(wgTmpl))
		if err := tmpl.Execute(&wgConfig, data); err != nil {
			return nil, fmt.Errorf("rendering wireguard config for peer %s: %w", peer.Name, err)
		}

		wgConfigs[peerIDStr] = wgConfig.String()
	}

	// 生成Babeld配置
	babelData := struct {
		Name       string
		Port       int
		Interfaces []string
	}{
		Name:       node.Name,
		Port:       s.config.Network.BabelPort,
		Interfaces: interfaces,
	}

	var babelConfig strings.Builder
	tmpl := template.Must(template.New("babeld").Parse(babelTmpl))
	if err := tmpl.Execute(&babelConfig, babelData); err != nil {
		return nil, fmt.Errorf("rendering babeld config: %w", err)
	}

	return &models.NodeConfig{
		WireGuard: wgConfigs,
		Babeld:    babelConfig.String(),
	}, nil
}

func (s *ConfigService) UpdateConfigs(nodeIDs []string) error {
	for _, idStr := range nodeIDs {
		var nodeID int
		if _, err := fmt.Sscanf(idStr, "%d", &nodeID); err != nil {
			continue
		}

		// 为每个节点创建配置更新任务
		_, err := s.taskService.CreateTask(nodeID, models.TaskTypeUpdate)
		if err != nil {
			return err
		}
	}

	return nil
}
