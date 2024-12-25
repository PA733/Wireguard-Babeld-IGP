package store

import (
	"fmt"
	"time"

	"mesh-backend/pkg/types"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GormStore 通用GORM存储实现
type GormStore struct {
	db *gorm.DB
}

// NewGormStore 创建GORM存储实例
func NewGormStore(dialector gorm.Dialector) (*GormStore, error) {
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	store := &GormStore{db: db}

	if err := store.initialize(); err != nil {
		return nil, fmt.Errorf("initializing database: %w", err)
	}

	return store, nil
}

// initialize 初始化数据库
func (s *GormStore) initialize() error {
	err := s.db.AutoMigrate(&types.NodeConfig{}, &types.NodeStatus{}, &types.Task{}, &types.WireguardConnection{})
	if err != nil {
		return fmt.Errorf("auto migrating tables: %w", err)
	}
	return nil
}

// SaveTask 保存任务
func (s *GormStore) SaveTask(task *types.Task) error {
	result := s.db.Create(&task)
	if result.Error != nil {
		return fmt.Errorf("inserting task: %w", result.Error)
	}

	return nil
}

// GetTask 获取任务
func (s *GormStore) GetTask(id string) (*types.Task, error) {
	var task types.Task
	result := s.db.First(&task, "id = ?", id)
	if result.Error != nil {
		return nil, fmt.Errorf("querying task: %w", result.Error)
	}

	return &task, nil
}

// ListTasks 列出任务
func (s *GormStore) ListTasks(filter TaskFilter) ([]*types.Task, error) {
	// Unimplemented
	return nil, nil
}

// DeleteTask 删除任务
func (s *GormStore) DeleteTask(id string) error {
	result := s.db.Delete(&types.Task{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("deleting task: %w", result.Error)
	}

	return nil
}

// CleanupTasks 清理过期任务
func (s *GormStore) CleanupTasks() error {
	// cutoff := time.Now().Add(-24 * time.Hour)
	// _, err := s.db.Exec("DELETE FROM tasks WHERE completed_at < ?", cutoff)
	result := s.db.Delete(&types.Task{}, "completed_at < ?", time.Now().Add(-24*time.Hour))
	if result.Error != nil {
		return fmt.Errorf("deleting tasks: %w", result.Error)
	}
	return nil
}

// Close 关闭数据库连接
func (s *GormStore) Close() error {
	// return s.db.Close()
	return nil
}

// CreateNode 创建节点
func (s *GormStore) CreateNode(node *types.NodeConfig) error {
	result := s.db.Create(node)
	if result.Error != nil {
		return fmt.Errorf("creating node: %w", result.Error)
	}
	return nil
}

// GetNode 获取节点
func (s *GormStore) GetNode(nodeID int) (*types.NodeConfig, error) {
	var node types.NodeConfig
	result := s.db.First(&node, nodeID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("node %d not found", nodeID)
		}
		return nil, fmt.Errorf("querying node: %w", result.Error)
	}
	return &node, nil
}

// UpdateNode 更新节点
func (s *GormStore) UpdateNode(nodeID int, node *types.NodeConfig) error {
	result := s.db.Model(&types.NodeConfig{}).Where("id = ?", nodeID).Updates(node)
	if result.Error != nil {
		return fmt.Errorf("updating node: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("node %d not found", nodeID)
	}
	return nil
}

// DeleteNode 删除节点
func (s *GormStore) DeleteNode(nodeID int) error {
	result := s.db.Delete(&types.NodeConfig{}, nodeID)
	if result.Error != nil {
		return fmt.Errorf("deleting node: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("node %d not found", nodeID)
	}
	return nil
}

// ListNodes 列出所有节点
func (s *GormStore) ListNodes() ([]*types.NodeConfig, error) {
	var nodes []*types.NodeConfig
	result := s.db.Find(&nodes)
	if result.Error != nil {
		return nil, fmt.Errorf("querying nodes: %w", result.Error)
	}
	return nodes, nil
}

// UpdateNodeStatus 更新节点状态
func (s *GormStore) UpdateNodeStatus(nodeID int, status *types.NodeStatus) error {
	status.ID = nodeID
	result := s.db.Save(status)
	if result.Error != nil {
		return fmt.Errorf("upserting node status: %w", result.Error)
	}
	return nil
}

// GetNodeStatus 获取节点状态
func (s *GormStore) GetNodeStatus(nodeID int) (*types.NodeStatus, error) {
	var status types.NodeStatus
	result := s.db.First(&status, nodeID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("status for node %d not found", nodeID)
		}
		return nil, fmt.Errorf("querying node status: %w", result.Error)
	}
	return &status, nil
}

// ListNodeStatus 列出所有节点状态
func (s *GormStore) ListNodeStatus() ([]*types.NodeStatus, error) {
	var statuses []*types.NodeStatus
	result := s.db.Find(&statuses)
	if result.Error != nil {
		return nil, fmt.Errorf("querying node status: %w", result.Error)
	}
	return statuses, nil
}

// GetOrCreateWireguardConnection 获取或创建Wireguard连接
func (s *GormStore) GetOrCreateWireguardConnection(connection *types.WireguardConnection, basePort int) (*types.WireguardConnection, error) {
	if connection == nil {
		return nil, fmt.Errorf("connection cannot be nil")
	}

	var conn types.WireguardConnection

	// 情况1：如果提供了Port，则根据Port查询连接
	if connection.Port != 0 {
		result := s.db.Where("port = ?", connection.Port).First(&conn)
		if result.Error == nil {
			return &conn, nil
		} else if result.Error != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("querying wireguard connection by port: %w", result.Error)
		}
		return nil, fmt.Errorf("wireguard connection not found with port %d", connection.Port)
	}

	// 情况2：如果提供了NodeID和PeerID，则根据它们查询连接
	if connection.NodeID != 0 && connection.PeerID != 0 {
		result := s.db.Where(
			"(node_id = ? AND peer_id = ?) OR (node_id = ? AND peer_id = ?)",
			connection.NodeID, connection.PeerID,
			connection.PeerID, connection.NodeID,
		).First(&conn)

		if result.Error == nil {
			return &conn, nil
		} else if result.Error != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("querying wireguard connection: %w", result.Error)
		}

		// 未找到连接，需要创建新的连接
		var maxPort int
		result = s.db.Model(&types.WireguardConnection{}).Select("COALESCE(MAX(port), 0)").Scan(&maxPort)
		if result.Error != nil {
			return nil, fmt.Errorf("getting max port: %w", result.Error)
		}

		// 新的端口号为 max(basePort, maxPortInDB) + 1
		newPort := basePort
		if maxPort >= basePort {
			newPort = maxPort + 1
		}

		// 创建新的连接记录
		conn = types.WireguardConnection{
			NodeID: connection.NodeID,
			PeerID: connection.PeerID,
			Port:   newPort,
		}
		result = s.db.Create(&conn)
		if result.Error != nil {
			return nil, fmt.Errorf("creating wireguard connection: %w", result.Error)
		}

		return &conn, nil
	}

	return nil, fmt.Errorf("invalid connection parameters; must provide either port, or node_id and peer_id")
}
