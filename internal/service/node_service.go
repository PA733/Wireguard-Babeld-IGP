package service

import (
	"context"
	"fmt"
	"time"

	"mesh-backend/internal/models"
	"mesh-backend/internal/store/types"
)

type NodeService struct {
	store      types.Store
	keyService *KeyService
}

func NewNodeService(store types.Store) *NodeService {
	return &NodeService{
		store:      store,
		keyService: NewKeyService(),
	}
}

func (s *NodeService) CreateNode(name, endpoint string) (*models.Node, error) {
	ctx := context.Background()

	// 生成 WireGuard 密钥对
	privateKey, publicKey, err := s.keyService.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	node := &models.Node{
		Name:       name,
		Endpoint:   endpoint,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		Status:     models.NodeStatusOffline,
		Token:      fmt.Sprintf("token-%d", time.Now().UnixNano()),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Store.CreateNode 会设置ID
	if err := s.store.CreateNode(ctx, node); err != nil {
		return nil, err
	}

	// 使用分配的ID设置网络配置
	node.IPv4Prefix = fmt.Sprintf("10.42.%d.0", node.ID)
	node.IPv6Prefix = fmt.Sprintf("2a13:a5c7:21ff:276:%d::", node.ID)
	node.LinkLocalAddr = fmt.Sprintf("fe80::%d", node.ID)

	// 更新节点信息
	if err := s.store.UpdateNode(ctx, node); err != nil {
		return nil, err
	}

	return node, nil
}

func (s *NodeService) GetNode(id int) (*models.Node, error) {
	ctx := context.Background()
	return s.store.GetNode(ctx, id)
}

func (s *NodeService) ListNodes() ([]*models.Node, error) {
	ctx := context.Background()
	return s.store.ListNodes(ctx)
}

func (s *NodeService) DeleteNode(id int) error {
	ctx := context.Background()
	return s.store.DeleteNode(ctx, id)
}
